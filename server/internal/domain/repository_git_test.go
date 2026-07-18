package domain

import (
	"testing"
	"time"
)

func TestRepositoryValidation(t *testing.T) {
	repository, err := NewRepository(Repository{ID: "repo", WorkspaceID: "workspace", Name: " Main ", RemoteURL: "https://example.com/repo.git", IsRecordRepository: true, TaskRecordsRoot: "task-records/./"})
	if err != nil || repository.TaskRecordsRoot != "task-records" || repository.Name != "Main" {
		t.Fatalf("unexpected repository: %+v %v", repository, err)
	}
	for _, remote := range []string{"ssh://git@github.com/org/repo.git", "git@github.com:org/repo.git"} {
		if _, err := NewRepository(Repository{ID: "repo", WorkspaceID: "workspace", Name: "Main", RemoteURL: remote}); err != nil {
			t.Fatalf("valid SSH remote %q rejected: %v", remote, err)
		}
	}
	_, err = NewRepository(Repository{ID: "repo", WorkspaceID: "workspace", Name: "Main", RemoteURL: "https://example.com/repo.git", TaskRecordsRoot: "task-records"})
	assertViolation(t, err, CodeInvalidRecordPath)
	for _, remote := range []string{"/Users/me/repo", "file:///tmp/repo", `C:\repo`, "local", "https://user:token@example.com/repo.git", "ssh://git:token@github.com/org/repo.git", "https://example.com/repo.git?token=secret", "https://example.com/repo.git#secret", "https://example.com/repo\x00.git"} {
		_, err = NewRepository(Repository{ID: "repo", WorkspaceID: "workspace", Name: "Main", RemoteURL: remote})
		assertViolation(t, err, CodeInvalidStateTransition)
	}
}

func TestCommitReferenceSupportsMultipleRepositoriesWithoutBranch(t *testing.T) {
	for _, repositoryID := range []string{"server", "client"} {
		value, err := NewCommitReference(CommitReference{ID: "commit-" + repositoryID, WorkspaceID: "workspace", TaskID: "task", RepositoryID: repositoryID, CommitSHA: repeatHex("A", 40), Relation: CommitProduced})
		if err != nil || value.RepositoryID != repositoryID || value.VerificationState != CommitReported || value.CommitSHA != repeatHex("a", 40) {
			t.Fatalf("unexpected reference: %+v %v", value, err)
		}
		verified := value.MarkRemoteVerified()
		if verified.VerificationState != CommitRemoteVerified {
			t.Fatal("verification state not updated")
		}
	}
	_, err := NewCommitReference(CommitReference{ID: "bad", WorkspaceID: "workspace", TaskID: "task", RepositoryID: "repo", CommitSHA: "not-sha", Relation: CommitProduced})
	assertViolation(t, err, CodeInvalidStateTransition)
	_, err = NewCommitReference(CommitReference{ID: "bad", WorkspaceID: "workspace", TaskID: "task", RepositoryID: "repo", CommitSHA: repeatHex("a", 40), Relation: CommitRelation("unknown")})
	assertViolation(t, err, CodeInvalidStateTransition)
}

func TestRunGitObservationIsMetadataOnlyAndRejectsAbsoluteWorktreePath(t *testing.T) {
	dirty := true
	value, err := NewRunGitObservation(RunGitObservation{ID: "obs", WorkspaceID: "workspace", RunID: "run", RepositoryID: "repo", ObservedAt: time.Now(), HeadCommitSHA: repeatHex("a", 64), BranchHint: " feature/run ", WorktreeLabel: "wave-2", Dirty: &dirty})
	if err != nil || value.BranchHint != "feature/run" || value.WorktreeLabel != "wave-2" || value.Dirty == nil {
		t.Fatalf("unexpected observation: %+v %v", value, err)
	}
	for _, path := range []string{"/tmp/worktree", "~/worktree", `\root-relative`, `C:\worktree`, "C:/worktree", `\\server\share`} {
		value.WorktreeLabel = path
		_, err = NewRunGitObservation(value)
		assertViolation(t, err, CodeInvalidRecordPath)
	}
	value.WorktreeLabel = "label\x00secret"
	_, err = NewRunGitObservation(value)
	assertViolation(t, err, CodeInvalidRecordPath)
}
