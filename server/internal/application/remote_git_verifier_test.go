package application

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCommandRemoteGitVerifierUsesFetchedRefAndCommittedBytes(t *testing.T) {
	root := t.TempDir()
	remote, work := filepath.Join(root, "remote.git"), filepath.Join(root, "work")
	runTestGit(t, root, "init", "--bare", remote)
	runTestGit(t, root, "init", work)
	runTestGit(t, work, "config", "user.email", "baley@example.test")
	runTestGit(t, work, "config", "user.name", "Baley Test")
	relative := "task-records/187/completion-report-01.md"
	path := filepath.Join(work, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	content := []byte("provider authoritative evidence\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, work, "add", relative)
	runTestGit(t, work, "commit", "-m", "evidence")
	runTestGit(t, work, "branch", "-M", "main")
	runTestGit(t, work, "remote", "add", "origin", remote)
	runTestGit(t, work, "push", "origin", "main")
	commitSHA := testGitOutput(t, work, "rev-parse", "HEAD")
	blobSHA := testGitOutput(t, work, "rev-parse", "HEAD:"+relative)
	digest := sha256.Sum256(content)
	contentHash := "sha256:" + hex.EncodeToString(digest[:])
	repository := RepositoryProjection{ID: "repository", RemoteURL: remote, DefaultBranch: "main"}
	commit := CommitReferenceProjection{ID: "commit", RepositoryID: repository.ID, CommitSHA: commitSHA}
	record := TaskRecordProjection{ID: "record", RepositoryID: repository.ID, RelativePath: relative, WorkingTreeHash: contentHash, CommitSHA: commitSHA, BlobSHA: blobSHA}

	evidence, err := newCommandRemoteGitVerifier().Verify(context.Background(), repository, commit, "refs/heads/main", []TaskRecordProjection{record})
	if err != nil || evidence.RefTipSHA != commitSHA || evidence.RemoteRef != "refs/heads/main" || len(evidence.Records) != 1 || evidence.Records[0].ContentHash != contentHash {
		t.Fatalf("verification evidence=%+v err=%v", evidence, err)
	}
	badContent := record
	badContent.WorkingTreeHash = "sha256:" + string(make([]byte, 64))
	if _, err = newCommandRemoteGitVerifier().Verify(context.Background(), repository, commit, "refs/heads/main", []TaskRecordProjection{badContent}); err == nil {
		t.Fatal("mismatched content hash was accepted")
	}
	badCommit := commit
	badCommit.CommitSHA = "ffffffffffffffffffffffffffffffffffffffff"
	if _, err = newCommandRemoteGitVerifier().Verify(context.Background(), repository, badCommit, "refs/heads/main", []TaskRecordProjection{record}); err == nil {
		t.Fatal("commit absent from configured ref was accepted")
	}
	if _, err = newCommandRemoteGitVerifier().Verify(context.Background(), repository, commit, "--upload-pack=evil", []TaskRecordProjection{record}); err == nil {
		t.Fatal("option-shaped configured branch was accepted")
	}
}

func runTestGit(t *testing.T, directory string, arguments ...string) {
	t.Helper()
	command := exec.Command("git", arguments...)
	command.Dir = directory
	command.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", arguments, err, output)
	}
}

func testGitOutput(t *testing.T, directory string, arguments ...string) string {
	t.Helper()
	command := exec.Command("git", arguments...)
	command.Dir = directory
	output, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	return string(bytes.TrimSpace(output))
}
