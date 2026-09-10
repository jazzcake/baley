package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var remoteVerificationTimeout = 2 * time.Minute

var gitCommandContext = exec.CommandContext

var ErrRemoteVerificationFailed = errors.New("provider-authoritative remote Git verification failed")

type RemoteRecordEvidence struct {
	RecordID     string `json:"recordId"`
	RelativePath string `json:"relativePath"`
	BlobSHA      string `json:"blobSha"`
	ContentHash  string `json:"contentHash"`
}

type RemoteVerificationEvidence struct {
	RepositoryID string                 `json:"repositoryId"`
	RemoteURL    string                 `json:"remoteUrl"`
	RemoteRef    string                 `json:"remoteRef"`
	RefTipSHA    string                 `json:"refTipSha"`
	CommitSHA    string                 `json:"commitSha"`
	VerifiedAt   time.Time              `json:"verifiedAt"`
	Verifier     string                 `json:"verifier"`
	Records      []RemoteRecordEvidence `json:"records"`
}

type RemoteGitVerifier interface {
	Verify(context.Context, RepositoryProjection, CommitReferenceProjection, string, []TaskRecordProjection) (RemoteVerificationEvidence, error)
}

type commandRemoteGitVerifier struct{}

func newCommandRemoteGitVerifier() RemoteGitVerifier { return commandRemoteGitVerifier{} }

func (commandRemoteGitVerifier) Verify(ctx context.Context, repository RepositoryProjection, commit CommitReferenceProjection, remoteRef string, records []TaskRecordProjection) (RemoteVerificationEvidence, error) {
	verifyCtx, cancel := context.WithTimeout(ctx, remoteVerificationTimeout)
	defer cancel()
	remoteRef = strings.TrimSpace(remoteRef)
	if len(records) == 0 || !strings.HasPrefix(remoteRef, "refs/heads/") || strings.TrimPrefix(remoteRef, "refs/heads/") == "" || strings.ContainsAny(remoteRef, "\x00\r\n") {
		return RemoteVerificationEvidence{}, ErrRemoteVerificationFailed
	}
	check := protectedGitCommand(verifyCtx, "git", "check-ref-format", remoteRef)
	if err := check.Run(); err != nil {
		return RemoteVerificationEvidence{}, ErrRemoteVerificationFailed
	}
	root, err := os.MkdirTemp("", "baley-remote-verify-")
	if err != nil {
		return RemoteVerificationEvidence{}, ErrRemoteVerificationFailed
	}
	defer os.RemoveAll(root)
	initArguments := []string{"init", "--bare", "--quiet"}
	if len(commit.CommitSHA) == 64 {
		initArguments = append(initArguments, "--object-format=sha256")
	}
	if err = gitVerifyCommand(verifyCtx, root, initArguments...).Run(); err != nil {
		return RemoteVerificationEvidence{}, ErrRemoteVerificationFailed
	}
	refTarget := "refs/baley/verified"
	if err = gitVerifyCommand(verifyCtx, root, "fetch", "--quiet", "--no-tags", "--force", "--", repository.RemoteURL, "+"+remoteRef+":"+refTarget).Run(); err != nil {
		return RemoteVerificationEvidence{}, ErrRemoteVerificationFailed
	}
	refTip, err := gitVerifyOutput(verifyCtx, root, "rev-parse", "--verify", refTarget+"^{commit}")
	if err != nil || !validRemoteObjectID(refTip) {
		return RemoteVerificationEvidence{}, ErrRemoteVerificationFailed
	}
	if err = gitVerifyCommand(verifyCtx, root, "merge-base", "--is-ancestor", commit.CommitSHA, refTarget).Run(); err != nil {
		return RemoteVerificationEvidence{}, ErrRemoteVerificationFailed
	}
	evidence := RemoteVerificationEvidence{
		RepositoryID: repository.ID, RemoteURL: repository.RemoteURL, RemoteRef: remoteRef,
		RefTipSHA: refTip, CommitSHA: commit.CommitSHA, VerifiedAt: time.Now().UTC(), Verifier: "git-fetch-v1",
		Records: make([]RemoteRecordEvidence, 0, len(records)),
	}
	for _, record := range records {
		objectExpression := commit.CommitSHA + ":" + filepath.ToSlash(record.RelativePath)
		blobSHA, resolveErr := gitVerifyOutput(verifyCtx, root, "rev-parse", "--verify", objectExpression)
		if resolveErr != nil || blobSHA != record.BlobSHA || !validRemoteObjectID(blobSHA) {
			return RemoteVerificationEvidence{}, ErrRemoteVerificationFailed
		}
		objectType, typeErr := gitVerifyOutput(verifyCtx, root, "cat-file", "-t", blobSHA)
		if typeErr != nil || objectType != "blob" {
			return RemoteVerificationEvidence{}, ErrRemoteVerificationFailed
		}
		content, contentErr := gitVerifyCommand(verifyCtx, root, "cat-file", "blob", blobSHA).Output()
		if contentErr != nil {
			return RemoteVerificationEvidence{}, ErrRemoteVerificationFailed
		}
		digest := sha256.Sum256(content)
		contentHash := "sha256:" + hex.EncodeToString(digest[:])
		if record.WorkingTreeHash == "" || contentHash != record.WorkingTreeHash {
			return RemoteVerificationEvidence{}, ErrRemoteVerificationFailed
		}
		evidence.Records = append(evidence.Records, RemoteRecordEvidence{
			RecordID: record.ID, RelativePath: record.RelativePath, BlobSHA: blobSHA, ContentHash: contentHash,
		})
	}
	return evidence, nil
}

func gitVerifyCommand(ctx context.Context, gitDir string, arguments ...string) *exec.Cmd {
	args := append([]string{"-c", "credential.interactive=never", "--git-dir", gitDir}, arguments...)
	command := protectedGitCommand(ctx, "git", args...)
	command.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_SSH_COMMAND=ssh -o BatchMode=yes")
	return command
}

func protectedGitCommand(ctx context.Context, name string, arguments ...string) *exec.Cmd {
	command := gitCommandContext(ctx, name, arguments...)
	configureProcessTree(command)
	return command
}

func gitVerifyOutput(ctx context.Context, gitDir string, arguments ...string) (string, error) {
	output, err := gitVerifyCommand(ctx, gitDir, arguments...).Output()
	if err != nil {
		return "", fmt.Errorf("git verification command failed: %w", err)
	}
	return strings.ToLower(strings.TrimSpace(string(output))), nil
}

func validRemoteObjectID(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
