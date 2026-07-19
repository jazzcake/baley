package integration_test

import (
	"context"
	"encoding/json"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jazzcake/baley/server/internal/application"
	"github.com/jazzcake/baley/server/internal/domain"
	"github.com/jazzcake/baley/server/internal/persistence/postgres"
)

func TestRecordAndGitIndexAgainstPostgres(t *testing.T) {
	url := os.Getenv("BALEY_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("BALEY_TEST_DATABASE_URL is not set")
	}
	t.Setenv("BALEY_LEASE_TOKEN_SECRET", "record-git-integration-secret")
	ctx := context.Background()
	repo, err := postgres.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Pool.Close()
	if _, err = repo.Pool.Exec(ctx, "TRUNCATE events,human_approval_attestations,commands,workspace_counters,run_git_observations,commit_references,task_record_indexes,repositories,runs,gate_tasks,gates,task_dependencies,tasks,lanes,phases,workspaces,actors CASCADE"); err != nil {
		t.Fatal(err)
	}
	if err = repo.SeedDemo(ctx); err != nil {
		t.Fatal(err)
	}
	service := application.NewService(repo)

	start := request("run.start", map[string]any{"workspaceId": postgres.DemoWorkspaceID, "taskId": 110, "clientRunId": "00000000-0000-4000-8000-000000000031", "kind": "detailed_planning"}, "record-run-start", 1)
	if _, err = service.Execute(ctx, start); err != nil {
		t.Fatal(err)
	}
	snapshot, _ := repo.LoadSnapshot(ctx, postgres.DemoWorkspaceID)
	runID := snapshot.Runs[0].ID
	recordID := "00000000-0000-4000-8000-000000000032"
	workingHash := "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	recordArgs := map[string]any{"workspaceId": postgres.DemoWorkspaceID, "recordId": recordID, "taskId": 110, "runId": runID, "recordType": "detailed-plan", "repositoryId": postgres.DemoRepositoryID, "relativePath": "task-records/task-110/detailed-plan-01.md", "workingTreeHash": workingHash, "shortSummary": "Detailed plan for user testing"}
	registered, err := service.Execute(ctx, request("record.register", recordArgs, "record-register", 2))
	if err != nil || registered.WorkspaceRevision != 3 || len(registered.EventIDs) != 1 {
		t.Fatalf("record.register failed: %#v %v", registered, err)
	}
	retry := request("record.register", recordArgs, "record-register-retry", 3)
	retried, err := service.Execute(ctx, retry)
	if err != nil || !retried.Idempotent || retried.WorkspaceRevision != 3 || len(retried.EventIDs) != 0 {
		t.Fatalf("record registration retry mismatch: %#v %v", retried, err)
	}
	if projectionKeys(t, registered.Projection, "record") != projectionKeys(t, retried.Projection, "record") {
		t.Fatalf("record projection shape changed between applied and idempotent results")
	}
	conflictingArgs := cloneMap(recordArgs)
	conflictingArgs["workingTreeHash"] = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	_, err = service.Execute(ctx, request("record.register", conflictingArgs, "record-register-conflict", 3))
	assertCode(t, err, domain.CodeRecordHashConflict)
	invalidPathArgs := cloneMap(recordArgs)
	invalidPathArgs["recordId"] = "00000000-0000-4000-8000-000000000033"
	invalidPathArgs["relativePath"] = "../outside.md"
	_, err = service.Execute(ctx, request("record.register", invalidPathArgs, "record-invalid-path", 3))
	assertCode(t, err, domain.CodeInvalidRecordPath)
	duplicatePathArgs := cloneMap(recordArgs)
	duplicatePathArgs["recordId"] = "00000000-0000-4000-8000-000000000038"
	_, err = service.Execute(ctx, request("record.register", duplicatePathArgs, "record-duplicate-path", 3))
	assertCode(t, err, domain.CodeIdempotencyConflict)

	commitSHA := "1111111111111111111111111111111111111111"
	blobSHA := "2222222222222222222222222222222222222222"
	attach := request("record.attach_commit", map[string]any{"workspaceId": postgres.DemoWorkspaceID, "recordId": recordID, "commitSha": commitSHA, "blobSha": blobSHA}, "record-commit", 3)
	attached, err := service.Execute(ctx, attach)
	if err != nil || attached.WorkspaceRevision != 4 || len(attached.EventIDs) != 1 {
		t.Fatalf("record.attach_commit failed: %#v %v", attached, err)
	}
	attachRetry := attach
	attachRetry.Envelope.IdempotencyKey = "record-commit-retry"
	attachRetry.Envelope.ExpectedWorkspaceRevision = 4
	attachedAgain, err := service.Execute(ctx, attachRetry)
	if err != nil || !attachedAgain.Idempotent || attachedAgain.WorkspaceRevision != 4 || len(attachedAgain.EventIDs) != 0 {
		t.Fatalf("record commit retry mismatch: %#v %v", attachedAgain, err)
	}

	commitID := "00000000-0000-4000-8000-000000000034"
	commitResult, err := service.Execute(ctx, request("commit.attach", map[string]any{"workspaceId": postgres.DemoWorkspaceID, "commitId": commitID, "taskId": 110, "runId": runID, "repositoryId": postgres.DemoRepositoryID, "commitSha": commitSHA, "relation": "produced"}, "commit-reference", 4))
	if err != nil || commitResult.WorkspaceRevision != 5 {
		t.Fatalf("commit.attach failed: %#v %v", commitResult, err)
	}
	commitArgs := map[string]any{"workspaceId": postgres.DemoWorkspaceID, "commitId": commitID, "taskId": 110, "runId": runID, "repositoryId": postgres.DemoRepositoryID, "commitSha": commitSHA, "relation": "produced"}
	commitRetry, err := service.Execute(ctx, request("commit.attach", commitArgs, "commit-reference-retry", 5))
	if err != nil || !commitRetry.Idempotent || commitRetry.WorkspaceRevision != 5 {
		t.Fatalf("commit retry mismatch: %#v %v", commitRetry, err)
	}
	duplicateCommitArgs := cloneMap(commitArgs)
	duplicateCommitArgs["commitId"] = "00000000-0000-4000-8000-000000000039"
	_, err = service.Execute(ctx, request("commit.attach", duplicateCommitArgs, "commit-reference-duplicate", 5))
	assertCode(t, err, domain.CodeIdempotencyConflict)
	observationID := "00000000-0000-4000-8000-000000000035"
	observedAt := time.Date(2026, 7, 19, 1, 0, 0, 987654321, time.FixedZone("KST", 9*60*60))
	observationArgs := map[string]any{"workspaceId": postgres.DemoWorkspaceID, "observationId": observationID, "runId": runID, "repositoryId": postgres.DemoRepositoryID, "observedAt": observedAt, "headCommitSha": commitSHA, "branchHint": "main", "worktreeLabel": "primary", "dirty": false}
	observationResult, err := service.Execute(ctx, request("git.observe", observationArgs, "git-observe", 5))
	if err != nil || observationResult.WorkspaceRevision != 6 {
		t.Fatalf("git.observe failed: %#v %v", observationResult, err)
	}
	observationRetry, err := service.Execute(ctx, request("git.observe", observationArgs, "git-observe-retry", 6))
	if err != nil || !observationRetry.Idempotent || observationRetry.WorkspaceRevision != 6 {
		t.Fatalf("observation retry mismatch: %#v %v", observationRetry, err)
	}
	invalidObservation := cloneMap(observationArgs)
	invalidObservation["observationId"] = "00000000-0000-4000-8000-000000000036"
	invalidObservation["worktreeLabel"] = "C:/absolute/worktree"
	_, err = service.Execute(ctx, request("git.observe", invalidObservation, "git-observe-invalid", 6))
	assertCode(t, err, domain.CodeInvalidRecordPath)
	repositoryID := "00000000-0000-4000-8000-000000000037"
	repositoryArgs := map[string]any{"workspaceId": postgres.DemoWorkspaceID, "repositoryId": repositoryID, "name": "Secondary", "remoteUrl": "https://github.com/jazzcake/secondary", "defaultBranch": "main", "isRecordRepository": false}
	repositoryResult, err := service.Execute(ctx, request("repository.register", repositoryArgs, "repository-register", 6))
	if err != nil || repositoryResult.WorkspaceRevision != 7 || len(repositoryResult.EventIDs) != 1 {
		t.Fatalf("repository.register failed: %#v %v", repositoryResult, err)
	}
	repositoryRetry, err := service.Execute(ctx, request("repository.register", repositoryArgs, "repository-register-retry", 7))
	if err != nil || !repositoryRetry.Idempotent || repositoryRetry.WorkspaceRevision != 7 {
		t.Fatalf("repository retry mismatch: %#v %v", repositoryRetry, err)
	}
	if projectionKeys(t, repositoryResult.Projection, "repository") != projectionKeys(t, repositoryRetry.Projection, "repository") {
		t.Fatalf("repository projection shape changed between applied and idempotent results")
	}

	snapshot, err = repo.LoadSnapshot(ctx, postgres.DemoWorkspaceID)
	if err != nil || snapshot.Workspace.Revision != 7 || len(snapshot.Repositories) != 2 || len(snapshot.Records) != 1 || len(snapshot.Commits) != 1 || len(snapshot.GitObservations) != 1 {
		t.Fatalf("Record/Git snapshot mismatch: %#v %v", snapshot, err)
	}
	if snapshot.Records[0].State != "committed_unverified" || snapshot.Records[0].CommitSHA != commitSHA || snapshot.Commits[0].VerificationState != "reported" {
		t.Fatalf("Record/Git state mismatch: records=%#v commits=%#v", snapshot.Records, snapshot.Commits)
	}
	events, err := repo.Events(ctx, postgres.DemoWorkspaceID)
	if err != nil || len(events) != 7 {
		t.Fatalf("Record/Git Events mismatch: %d %v", len(events), err)
	}
	for _, event := range events {
		if event.EntityType == "" || event.EntityID == "" || event.ExecutedByActorID != postgres.DemoAgentActorID {
			t.Fatalf("Event audit envelope missing: %#v", event)
		}
	}
	if _, err = repo.Pool.Exec(ctx, "UPDATE workspaces SET state='closed' WHERE id=$1", postgres.DemoWorkspaceID); err != nil {
		t.Fatal(err)
	}
	closedCommands := []application.CommandRequest{
		request("repository.register", repositoryArgs, "closed-repository", 7),
		request("record.register", recordArgs, "closed-record", 7),
		request("record.attach_commit", map[string]any{"workspaceId": postgres.DemoWorkspaceID, "recordId": recordID, "commitSha": commitSHA, "blobSha": blobSHA}, "closed-record-commit", 7),
		request("commit.attach", commitArgs, "closed-commit", 7),
		request("git.observe", observationArgs, "closed-observation", 7),
	}
	for _, command := range closedCommands {
		_, err = service.Execute(ctx, command)
		assertCode(t, err, domain.CodeInvalidStateTransition)
	}
}

func projectionKeys(t *testing.T, projection any, nested string) string {
	t.Helper()
	raw, err := json.Marshal(projection)
	if err != nil {
		t.Fatal(err)
	}
	var outer map[string]json.RawMessage
	if json.Unmarshal(raw, &outer) != nil {
		t.Fatalf("invalid projection: %s", raw)
	}
	var inner map[string]any
	if json.Unmarshal(outer[nested], &inner) != nil {
		t.Fatalf("invalid nested projection: %s", raw)
	}
	keys := make([]string, 0, len(inner))
	for key := range inner {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return strings.Join(keys, ",")
}

func cloneMap(input map[string]any) map[string]any {
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}
