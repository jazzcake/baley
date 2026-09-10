package application

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
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

func TestCommandRemoteGitVerifierSupportsSHA256Repositories(t *testing.T) {
	root := t.TempDir()
	remote, work := filepath.Join(root, "remote.git"), filepath.Join(root, "work")
	runTestGit(t, root, "init", "--bare", "--object-format=sha256", remote)
	runTestGit(t, root, "init", "--object-format=sha256", work)
	runTestGit(t, work, "config", "user.email", "baley@example.test")
	runTestGit(t, work, "config", "user.name", "Baley Test")
	relative := "task-records/187/sha256.md"
	path := filepath.Join(work, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	content := []byte("sha256 repository evidence\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, work, "add", relative)
	runTestGit(t, work, "commit", "-m", "sha256 evidence")
	runTestGit(t, work, "branch", "-M", "main")
	runTestGit(t, work, "remote", "add", "origin", remote)
	runTestGit(t, work, "push", "origin", "main")
	commitSHA := testGitOutput(t, work, "rev-parse", "HEAD")
	blobSHA := testGitOutput(t, work, "rev-parse", "HEAD:"+relative)
	digest := sha256.Sum256(content)
	record := TaskRecordProjection{ID: "record", RepositoryID: "repository", RelativePath: relative, WorkingTreeHash: "sha256:" + hex.EncodeToString(digest[:]), CommitSHA: commitSHA, BlobSHA: blobSHA}
	evidence, err := newCommandRemoteGitVerifier().Verify(context.Background(), RepositoryProjection{ID: "repository", RemoteURL: remote}, CommitReferenceProjection{ID: "commit", RepositoryID: "repository", CommitSHA: commitSHA}, "refs/heads/main", []TaskRecordProjection{record})
	if err != nil || len(commitSHA) != 64 || len(blobSHA) != 64 || evidence.CommitSHA != commitSHA || len(evidence.Records) != 1 {
		t.Fatalf("SHA-256 verification evidence=%+v commit=%q blob=%q err=%v", evidence, commitSHA, blobSHA, err)
	}
}

func TestCommandRemoteGitVerifierKillsTransportProcessTreeOnTimeout(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "child.pid")
	t.Setenv("BALEY_REMOTE_GIT_HELPER", "1")
	t.Setenv("BALEY_REMOTE_GIT_CHILD_PID", pidFile)
	oldFactory, oldTimeout := gitCommandContext, remoteVerificationTimeout
	gitCommandContext = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return exec.CommandContext(ctx, os.Args[0], "-test.run=TestRemoteGitProcessTreeHelper", "--", "parent")
	}
	remoteVerificationTimeout = 300 * time.Millisecond
	t.Cleanup(func() { gitCommandContext, remoteVerificationTimeout = oldFactory, oldTimeout })
	started := time.Now()
	_, err := newCommandRemoteGitVerifier().Verify(context.Background(), RepositoryProjection{ID: "repository", RemoteURL: "https://example.test/repository.git"}, CommitReferenceProjection{CommitSHA: strings.Repeat("a", 40)}, "refs/heads/main", []TaskRecordProjection{{ID: "record"}})
	if err == nil || time.Since(started) > 3*time.Second {
		t.Fatalf("hard timeout did not fail promptly: elapsed=%s err=%v", time.Since(started), err)
	}
	raw, readErr := os.ReadFile(pidFile)
	if readErr != nil {
		t.Fatalf("helper child PID was not recorded: %v", readErr)
	}
	pid, parseErr := strconv.Atoi(strings.TrimSpace(string(raw)))
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	deadline := time.Now().Add(2 * time.Second)
	for processAlive(pid) && time.Now().Before(deadline) {
		time.Sleep(25 * time.Millisecond)
	}
	if processAlive(pid) {
		t.Fatalf("transport child process %d survived verifier timeout", pid)
	}
}

func TestRemoteGitProcessTreeHelper(t *testing.T) {
	if os.Getenv("BALEY_REMOTE_GIT_HELPER") != "1" {
		return
	}
	mode := os.Args[len(os.Args)-1]
	if mode == "child" {
		time.Sleep(30 * time.Second)
		os.Exit(0)
	}
	child := exec.Command(os.Args[0], "-test.run=TestRemoteGitProcessTreeHelper", "--", "child")
	child.Env = os.Environ()
	if err := child.Start(); err != nil {
		os.Exit(2)
	}
	if err := os.WriteFile(os.Getenv("BALEY_REMOTE_GIT_CHILD_PID"), []byte(strconv.Itoa(child.Process.Pid)), 0o600); err != nil {
		os.Exit(3)
	}
	time.Sleep(30 * time.Second)
	os.Exit(0)
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
