package application

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jazzcake/baley/server/internal/domain"
)

func TestLaneBriefEvaluationTimeAllowsOnlyBoundedDatabaseClockSkew(t *testing.T) {
	now := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)
	withinTolerance := now.Add(250 * time.Millisecond)
	if got := laneBriefEvaluationTime(now, withinTolerance); !got.Equal(withinTolerance) {
		t.Fatalf("bounded database clock skew was not absorbed: %s", got)
	}
	beyondTolerance := now.Add(laneBriefClockSkewTolerance + time.Millisecond)
	if got := laneBriefEvaluationTime(now, beyondTolerance); !got.Equal(now) {
		t.Fatalf("unbounded future timestamp was silently accepted: %s", got)
	}
}

func TestVerifyLocalTaskRecordDetectsWorkingTreeDriftAndDeletion(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "baley-test@example.com")
	runGit(t, root, "config", "user.name", "Baley Test")
	runGit(t, root, "remote", "add", "origin", "https://example.com/repo.git")
	relativePath := "task-records/task/report.md"
	path := filepath.Join(root, filepath.FromSlash(relativePath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("verified report\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", relativePath)
	runGit(t, root, "commit", "-m", "record fixture")
	commit := strings.TrimSpace(runGit(t, root, "rev-parse", "HEAD"))
	blob := strings.TrimSpace(runGit(t, root, "rev-parse", "HEAD:"+relativePath))
	repositories := []RepositoryProjection{{
		ID: "repo", RemoteURL: "https://example.com/repo.git", IsRecordRepository: true, TaskRecordsRoot: "task-records",
	}}
	record := TaskRecordProjection{
		ID: "record", TaskID: "task", RepositoryID: "repo", RelativePath: relativePath,
		Type: string(domain.RecordCompletionReport), State: string(domain.RecordVerified),
		CommitSHA: commit, BlobSHA: blob, ShortSummary: "done",
	}
	if status, reason := verifyLocalTaskRecord(root, nil, repositories, record); status != "" || reason != "" {
		t.Fatalf("aligned record rejected: %s %s", status, reason)
	}
	if err := os.WriteFile(path, []byte("changed report\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if status, _ := verifyLocalTaskRecord(root, nil, repositories, record); status != "stale" {
		t.Fatalf("working-tree drift status=%s", status)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if status, _ := verifyLocalTaskRecord(root, nil, repositories, record); status != "missing" {
		t.Fatalf("deleted record status=%s", status)
	}
}

func TestVerifyLocalTaskRecordChecksReportedWorkingTreeHash(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "remote", "add", "origin", "https://example.com/repo.git")
	relativePath := "task-records/task/report.md"
	path := filepath.Join(root, filepath.FromSlash(relativePath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	content := []byte("reported working tree\n")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(content)
	record := TaskRecordProjection{
		ID: "record", TaskID: "task", RepositoryID: "repo", RelativePath: relativePath,
		Type: string(domain.RecordCompletionReport), State: string(domain.RecordReportedUncommitted),
		WorkingTreeHash: "sha256:" + hex.EncodeToString(digest[:]), ShortSummary: "done",
	}
	repositories := []RepositoryProjection{{
		ID: "repo", RemoteURL: "https://example.com/repo.git", IsRecordRepository: true, TaskRecordsRoot: "task-records",
	}}
	if status, _ := verifyLocalTaskRecord(root, nil, repositories, record); status != "" {
		t.Fatalf("matching working-tree hash status=%s", status)
	}
	if err := os.WriteFile(path, []byte("mutated\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if status, _ := verifyLocalTaskRecord(root, nil, repositories, record); status != "stale" {
		t.Fatalf("mutated reported record status=%s", status)
	}
}

func runGit(t *testing.T, root string, arguments ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, arguments...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", arguments, err, output)
	}
	return string(output)
}
