package domain

import (
	"encoding/hex"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var scpRemotePattern = regexp.MustCompile(`^[^/@\s]+@[^/:\s]+:.+$`)

type Repository struct {
	ID, WorkspaceID, Name, RemoteURL, DefaultBranch string
	IsRecordRepository                              bool
	TaskRecordsRoot                                 string
}

func NewRepository(value Repository) (Repository, error) {
	if strings.TrimSpace(value.ID) == "" || strings.TrimSpace(value.WorkspaceID) == "" || strings.TrimSpace(value.Name) == "" || !validRemoteURL(value.RemoteURL) {
		return Repository{}, &Violation{Code: CodeInvalidStateTransition}
	}
	value.Name, value.RemoteURL, value.DefaultBranch = strings.TrimSpace(value.Name), strings.TrimSpace(value.RemoteURL), strings.TrimSpace(value.DefaultBranch)
	if value.IsRecordRepository {
		root, err := normalizeRepositoryRelative(value.TaskRecordsRoot)
		if err != nil {
			return Repository{}, err
		}
		value.TaskRecordsRoot = root
	} else if strings.TrimSpace(value.TaskRecordsRoot) != "" {
		return Repository{}, &Violation{Code: CodeInvalidRecordPath}
	}
	return value, nil
}

type CommitRelation string

const (
	CommitBase       CommitRelation = "base"
	CommitProduced   CommitRelation = "produced"
	CommitReviewed   CommitRelation = "reviewed"
	CommitSuperseded CommitRelation = "superseded"
)

var CommitRelations = []CommitRelation{CommitBase, CommitProduced, CommitReviewed, CommitSuperseded}

type CommitVerificationState string

const (
	CommitReported       CommitVerificationState = "reported"
	CommitRemoteVerified CommitVerificationState = "remote_verified"
)

var CommitVerificationStates = []CommitVerificationState{CommitReported, CommitRemoteVerified}

type CommitReference struct {
	ID, WorkspaceID, TaskID, RunID, RepositoryID, CommitSHA string
	Relation                                                CommitRelation
	VerificationState                                       CommitVerificationState
}

func NewCommitReference(value CommitReference) (CommitReference, error) {
	if strings.TrimSpace(value.ID) == "" || strings.TrimSpace(value.WorkspaceID) == "" || strings.TrimSpace(value.TaskID) == "" || strings.TrimSpace(value.RepositoryID) == "" || !validGitObjectID(value.CommitSHA) || !containsCommitRelation(value.Relation) {
		return CommitReference{}, &Violation{Code: CodeInvalidStateTransition}
	}
	value.CommitSHA = strings.ToLower(strings.TrimSpace(value.CommitSHA))
	value.VerificationState = CommitReported
	return value, nil
}

func (c CommitReference) MarkRemoteVerified() CommitReference {
	c.VerificationState = CommitRemoteVerified
	return c
}

type RunGitObservation struct {
	ID, WorkspaceID, RunID, RepositoryID     string
	ObservedAt                               time.Time
	HeadCommitSHA, BranchHint, WorktreeLabel string
	Dirty                                    *bool
}

func NewRunGitObservation(value RunGitObservation) (RunGitObservation, error) {
	if strings.TrimSpace(value.ID) == "" || strings.TrimSpace(value.WorkspaceID) == "" || strings.TrimSpace(value.RunID) == "" || strings.TrimSpace(value.RepositoryID) == "" || value.ObservedAt.IsZero() {
		return RunGitObservation{}, &Violation{Code: CodeInvalidStateTransition}
	}
	if value.HeadCommitSHA != "" && !validGitObjectID(value.HeadCommitSHA) {
		return RunGitObservation{}, &Violation{Code: CodeInvalidStateTransition}
	}
	label := strings.TrimSpace(value.WorktreeLabel)
	if strings.ContainsRune(label, 0) || strings.ContainsRune(value.BranchHint, 0) || strings.HasPrefix(label, "/") || strings.HasPrefix(label, "~") || pathSchemePattern.MatchString(label) || strings.HasPrefix(label, `\`) {
		return RunGitObservation{}, &Violation{Code: CodeInvalidRecordPath}
	}
	value.HeadCommitSHA = strings.ToLower(strings.TrimSpace(value.HeadCommitSHA))
	value.BranchHint, value.WorktreeLabel = strings.TrimSpace(value.BranchHint), label
	// PostgreSQL timestamptz stores microseconds. Canonicalizing before both
	// persistence and comparison keeps entity-level retries stable.
	value.ObservedAt = value.ObservedAt.UTC().Truncate(time.Microsecond)
	return value, nil
}

func validGitObjectID(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	return validHexLength(value, len(value))
}
func validHexLength(value string, length int) bool {
	if len(value) != length {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
func containsCommitRelation(value CommitRelation) bool {
	for _, candidate := range CommitRelations {
		if value == candidate {
			return true
		}
	}
	return false
}

func validRemoteURL(value string) bool {
	value = strings.TrimSpace(value)
	if strings.ContainsRune(value, 0) {
		return false
	}
	for _, scheme := range []string{"https", "http", "ssh", "git"} {
		parsed, err := url.Parse(value)
		if err != nil || parsed.Scheme != scheme || parsed.Host == "" || parsed.RawQuery != "" || parsed.Fragment != "" {
			continue
		}
		if scheme == "ssh" {
			if parsed.User == nil {
				return true
			}
			_, hasPassword := parsed.User.Password()
			return !hasPassword && parsed.User.Username() != ""
		}
		return parsed.User == nil
	}
	return scpRemotePattern.MatchString(value)
}
