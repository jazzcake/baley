package gitmeta

import (
	"context"
	"encoding/hex"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"
)

type CommandResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Runner executes a Git command in a local directory. Implementations must
// invoke Git directly with argv and must not interpolate args through a shell.
type Runner interface {
	Run(context.Context, string, ...string) (CommandResult, error)
}

type CollectRequest struct {
	RepositoryID        string    `json:"repositoryId"`
	LocalRepositoryPath string    `json:"-"`
	WorktreeLabel       string    `json:"worktreeLabel,omitempty"`
	RelativePaths       []string  `json:"relativePaths"`
	ObservedAt          time.Time `json:"observedAt"`
}

type FileMetadata struct {
	RelativePath string `json:"relativePath"`
	CommitSHA    string `json:"commitSha,omitempty"`
	BlobSHA      string `json:"blobSha,omitempty"`
}

type Observation struct {
	RepositoryID  string         `json:"repositoryId"`
	ObservedAt    time.Time      `json:"observedAt"`
	HeadCommitSHA string         `json:"headCommitSha,omitempty"`
	BranchHint    string         `json:"branchHint,omitempty"`
	WorktreeLabel string         `json:"worktreeLabel,omitempty"`
	Dirty         *bool          `json:"dirty,omitempty"`
	Files         []FileMetadata `json:"files"`
}

type Failure struct {
	Operation    string `json:"operation"`
	RelativePath string `json:"relativePath,omitempty"`
	ExitCode     int    `json:"exitCode,omitempty"`
}

type Collection struct {
	Observation Observation `json:"observation"`
	Failures    []Failure   `json:"failures"`
}

var (
	pathScheme   = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*:`)
	safeLabelPat = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
)

func Collect(ctx context.Context, runner Runner, request CollectRequest) (Collection, error) {
	if runner == nil || strings.TrimSpace(request.RepositoryID) == "" || strings.TrimSpace(request.LocalRepositoryPath) == "" || request.ObservedAt.IsZero() || !safeLabel(request.WorktreeLabel) {
		return Collection{}, ErrInvalidRequest
	}
	paths := make([]string, 0, len(request.RelativePaths))
	seen := make(map[string]bool, len(request.RelativePaths))
	for _, value := range request.RelativePaths {
		normalized, ok := normalizeRelative(value)
		if !ok || normalized != value || seen[normalized] {
			return Collection{}, ErrInvalidRequest
		}
		seen[normalized] = true
		paths = append(paths, normalized)
	}
	sort.Strings(paths)
	result := Collection{Observation: Observation{
		RepositoryID: strings.TrimSpace(request.RepositoryID), ObservedAt: request.ObservedAt,
		WorktreeLabel: strings.TrimSpace(request.WorktreeLabel), Files: make([]FileMetadata, 0, len(paths)),
	}}

	if command, err := runner.Run(ctx, request.LocalRepositoryPath, readOnlyArgs("rev-parse", "--verify", "HEAD")...); err != nil || command.ExitCode != 0 || !validObjectID(strings.TrimSpace(command.Stdout)) {
		result.Failures = append(result.Failures, Failure{Operation: "head", ExitCode: command.ExitCode})
	} else {
		result.Observation.HeadCommitSHA = strings.ToLower(strings.TrimSpace(command.Stdout))
	}
	if command, err := runner.Run(ctx, request.LocalRepositoryPath, readOnlyArgs("symbolic-ref", "--quiet", "--short", "HEAD")...); err != nil || command.ExitCode > 1 || strings.ContainsRune(command.Stdout, 0) || command.ExitCode == 0 && strings.TrimSpace(command.Stdout) == "" {
		result.Failures = append(result.Failures, Failure{Operation: "branch", ExitCode: command.ExitCode})
	} else if command.ExitCode == 0 {
		result.Observation.BranchHint = strings.TrimSpace(command.Stdout)
	}
	if command, err := runner.Run(ctx, request.LocalRepositoryPath, readOnlyArgs("status", "--porcelain=v1", "-z")...); err != nil || command.ExitCode != 0 {
		result.Failures = append(result.Failures, Failure{Operation: "dirty", ExitCode: command.ExitCode})
	} else {
		dirty := command.Stdout != ""
		result.Observation.Dirty = &dirty
	}

	for _, relativePath := range paths {
		metadata := FileMetadata{RelativePath: relativePath}
		if command, err := runner.Run(ctx, request.LocalRepositoryPath, readOnlyArgs("log", "-n", "1", "--format=%H", "--", relativePath)...); err != nil || command.ExitCode != 0 || !validObjectID(strings.TrimSpace(command.Stdout)) {
			result.Failures = append(result.Failures, Failure{Operation: "file_commit", RelativePath: relativePath, ExitCode: command.ExitCode})
		} else {
			metadata.CommitSHA = strings.ToLower(strings.TrimSpace(command.Stdout))
		}
		if command, err := runner.Run(ctx, request.LocalRepositoryPath, readOnlyArgs("rev-parse", "HEAD:"+relativePath)...); err != nil || command.ExitCode != 0 || !validObjectID(strings.TrimSpace(command.Stdout)) {
			result.Failures = append(result.Failures, Failure{Operation: "file_blob", RelativePath: relativePath, ExitCode: command.ExitCode})
		} else {
			metadata.BlobSHA = strings.ToLower(strings.TrimSpace(command.Stdout))
		}
		result.Observation.Files = append(result.Observation.Files, metadata)
	}
	sort.Slice(result.Failures, func(i, j int) bool {
		if result.Failures[i].Operation != result.Failures[j].Operation {
			return result.Failures[i].Operation < result.Failures[j].Operation
		}
		return result.Failures[i].RelativePath < result.Failures[j].RelativePath
	})
	return result, nil
}

type collectorError string

func (e collectorError) Error() string { return string(e) }

const ErrInvalidRequest collectorError = "invalid Git metadata collection request"

func normalizeRelative(value string) (string, bool) {
	if value == "" || strings.TrimSpace(value) != value || strings.ContainsRune(value, 0) || strings.Contains(value, `\`) || strings.HasPrefix(value, "/") || strings.HasPrefix(value, "~") || pathScheme.MatchString(value) {
		return "", false
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == ".." {
			return "", false
		}
	}
	normalized := path.Clean(value)
	return normalized, normalized != "." && normalized != ".." && !strings.HasPrefix(normalized, "../")
}

func safeLabel(value string) bool {
	trimmed := strings.TrimSpace(value)
	return trimmed == "" || trimmed == value && safeLabelPat.MatchString(trimmed)
}

func readOnlyArgs(args ...string) []string {
	return append([]string{"--no-optional-locks"}, args...)
}

func validObjectID(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
