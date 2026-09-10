package integration_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jazzcake/baley/server/internal/application"
	"github.com/jazzcake/baley/server/internal/domain"
	"github.com/jazzcake/baley/server/internal/persistence/postgres"
	"github.com/jazzcake/baley/server/internal/transport/httpapi"
)

type staticRemoteVerifier struct {
	calls int
	fail  bool
	hook  func()
}

func (v *staticRemoteVerifier) Verify(_ context.Context, repository application.RepositoryProjection, commit application.CommitReferenceProjection, remoteRef string, records []application.TaskRecordProjection) (application.RemoteVerificationEvidence, error) {
	v.calls++
	if v.fail {
		return application.RemoteVerificationEvidence{}, errors.New("simulated provider failure")
	}
	evidence := application.RemoteVerificationEvidence{
		RepositoryID: repository.ID, RemoteURL: repository.RemoteURL, RemoteRef: remoteRef,
		RefTipSHA: commit.CommitSHA, CommitSHA: commit.CommitSHA, VerifiedAt: time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC), Verifier: "integration-provider",
	}
	for _, record := range records {
		evidence.Records = append(evidence.Records, application.RemoteRecordEvidence{RecordID: record.ID, RelativePath: record.RelativePath, BlobSHA: record.BlobSHA, ContentHash: record.WorkingTreeHash})
	}
	if v.hook != nil {
		hook := v.hook
		v.hook = nil
		hook()
	}
	return evidence, nil
}

func TestRecordAndGitIndexAgainstPostgres(t *testing.T) {
	url := os.Getenv("BALEY_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("BALEY_TEST_DATABASE_URL is not set")
	}
	requireDisposableDatabase(t, url)
	t.Setenv("BALEY_LEASE_TOKEN_SECRET", "record-git-integration-secret")
	ctx := context.Background()
	repo, err := postgres.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Pool.Close()
	if _, err = repo.Pool.Exec(ctx, "SET session_replication_role='replica'; TRUNCATE events,human_approval_attestations,commands,workspace_counters,run_git_observations,commit_references,task_record_indexes,repositories,runs,gate_tasks,gates,task_dependencies,tasks,lanes,phases,workspaces,actors CASCADE; SET session_replication_role='origin'"); err != nil {
		t.Fatal(err)
	}
	if err = repo.SeedDemo(ctx); err != nil {
		t.Fatal(err)
	}
	verifier := &staticRemoteVerifier{}
	service := application.NewServiceWithRemoteVerifier(repo, verifier)

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
	verifyRequest := request("commit.verify_remote", map[string]any{"workspaceId": postgres.DemoWorkspaceID, "commitId": commitID, "remoteRef": "refs/heads/main"}, "commit-remote-verify", 5)
	verifier.fail = true
	failedVerify := verifyRequest
	failedVerify.Envelope.IdempotencyKey = "commit-remote-verify-provider-failure"
	_, err = service.Execute(ctx, failedVerify)
	assertCode(t, err, domain.CodeCommitRemoteUnverified)
	snapshot, err = repo.LoadSnapshot(ctx, postgres.DemoWorkspaceID)
	if err != nil || snapshot.Workspace.Revision != 5 || snapshot.Records[0].State != "committed_unverified" || snapshot.Commits[0].VerificationState != "reported" {
		t.Fatalf("provider failure mutated state: records=%#v commits=%#v revision=%d err=%v", snapshot.Records, snapshot.Commits, snapshot.Workspace.Revision, err)
	}
	verifier.fail = false
	verifier.hook = func() {
		if _, hookErr := repo.Pool.Exec(context.Background(), "UPDATE task_record_indexes SET state='verified' WHERE workspace_id=$1 AND id=$2", postgres.DemoWorkspaceID, recordID); hookErr != nil {
			t.Errorf("concurrent count-mismatch setup failed: %v", hookErr)
		}
	}
	countMismatch := verifyRequest
	countMismatch.Envelope.IdempotencyKey = "commit-remote-verify-count-mismatch"
	_, err = service.Execute(ctx, countMismatch)
	assertCode(t, err, domain.CodeCommitRemoteUnverified)
	snapshot, err = repo.LoadSnapshot(ctx, postgres.DemoWorkspaceID)
	if err != nil || snapshot.Workspace.Revision != 5 || snapshot.Records[0].State != "verified" || snapshot.Commits[0].VerificationState != "reported" {
		t.Fatalf("count mismatch did not roll back commit atomically: records=%#v commits=%#v revision=%d err=%v", snapshot.Records, snapshot.Commits, snapshot.Workspace.Revision, err)
	}
	if _, err = repo.Pool.Exec(ctx, "UPDATE task_record_indexes SET state='committed_unverified' WHERE workspace_id=$1 AND id=$2", postgres.DemoWorkspaceID, recordID); err != nil {
		t.Fatal(err)
	}
	handler := (&httpapi.API{Service: service, Repo: repo}).Handler()
	verifyBody := serveCommand(t, handler, "/v1/commands/execute", verifyRequest)
	var verifyResult application.ExecutionResult
	if err = json.Unmarshal(verifyBody, &verifyResult); err != nil || verifyResult.WorkspaceRevision != 6 || len(verifyResult.EventIDs) != 2 || verifier.calls != 3 {
		t.Fatalf("remote verification HTTP result mismatch: body=%s calls=%d err=%v", verifyBody, verifier.calls, err)
	}
	snapshot, err = repo.LoadSnapshot(ctx, postgres.DemoWorkspaceID)
	if err != nil || snapshot.Commits[0].RemoteRef != "refs/heads/main" {
		t.Fatalf("verified remote ref was not restored from immutable event evidence: commits=%#v err=%v", snapshot.Commits, err)
	}
	wrongRef := request("commit.verify_remote", map[string]any{"workspaceId": postgres.DemoWorkspaceID, "commitId": commitID, "remoteRef": "refs/heads/release"}, "commit-remote-verify-wrong-ref", 6)
	_, err = service.Execute(ctx, wrongRef)
	assertCode(t, err, domain.CodeCommitRemoteUnverified)
	if verifier.calls != 3 {
		t.Fatalf("different ref reached provider after immutable ref binding: calls=%d", verifier.calls)
	}
	verifyRetry := verifyRequest
	verifyRetry.Envelope.ExpectedWorkspaceRevision = 6
	verifyRetry.Envelope.IdempotencyKey = "commit-remote-verify-state-retry"
	verifiedAgain, err := service.Execute(ctx, verifyRetry)
	if err != nil || !verifiedAgain.Idempotent || verifiedAgain.WorkspaceRevision != 6 || len(verifiedAgain.EventIDs) != 0 || verifier.calls != 3 {
		t.Fatalf("remote verification replay mismatch: %#v calls=%d err=%v", verifiedAgain, verifier.calls, err)
	}
	lateRecordID := "00000000-0000-4000-8000-000000000040"
	lateArgs := cloneMap(recordArgs)
	lateArgs["recordId"] = lateRecordID
	lateArgs["relativePath"] = "task-records/task-110/late-report-01.md"
	lateRegistered, err := service.Execute(ctx, request("record.register", lateArgs, "late-record-register", 6))
	if err != nil || lateRegistered.WorkspaceRevision != 7 {
		t.Fatalf("late record registration failed: %#v %v", lateRegistered, err)
	}
	lateAttached, err := service.Execute(ctx, request("record.attach_commit", map[string]any{"workspaceId": postgres.DemoWorkspaceID, "recordId": lateRecordID, "commitSha": commitSHA, "blobSha": blobSHA}, "late-record-attach", 7))
	if err != nil || lateAttached.WorkspaceRevision != 8 {
		t.Fatalf("late record attachment failed: %#v %v", lateAttached, err)
	}
	lateVerify := request("commit.verify_remote", map[string]any{"workspaceId": postgres.DemoWorkspaceID, "commitId": commitID, "remoteRef": "refs/heads/main"}, "late-record-verify", 8)
	lateVerified, err := service.Execute(ctx, lateVerify)
	if err != nil || lateVerified.WorkspaceRevision != 9 || len(lateVerified.EventIDs) != 2 || verifier.calls != 4 {
		t.Fatalf("late record verification failed: %#v calls=%d err=%v", lateVerified, verifier.calls, err)
	}
	observationID := "00000000-0000-4000-8000-000000000035"
	observedAt := time.Date(2026, 7, 19, 1, 0, 0, 987654321, time.FixedZone("KST", 9*60*60))
	observationArgs := map[string]any{"workspaceId": postgres.DemoWorkspaceID, "observationId": observationID, "runId": runID, "repositoryId": postgres.DemoRepositoryID, "observedAt": observedAt, "headCommitSha": commitSHA, "branchHint": "main", "worktreeLabel": "primary", "dirty": false}
	observationResult, err := service.Execute(ctx, request("git.observe", observationArgs, "git-observe", 9))
	if err != nil || observationResult.WorkspaceRevision != 10 {
		t.Fatalf("git.observe failed: %#v %v", observationResult, err)
	}
	observationRetry, err := service.Execute(ctx, request("git.observe", observationArgs, "git-observe-retry", 10))
	if err != nil || !observationRetry.Idempotent || observationRetry.WorkspaceRevision != 10 {
		t.Fatalf("observation retry mismatch: %#v %v", observationRetry, err)
	}
	invalidObservation := cloneMap(observationArgs)
	invalidObservation["observationId"] = "00000000-0000-4000-8000-000000000036"
	invalidObservation["worktreeLabel"] = "C:/absolute/worktree"
	_, err = service.Execute(ctx, request("git.observe", invalidObservation, "git-observe-invalid", 10))
	assertCode(t, err, domain.CodeInvalidRecordPath)
	repositoryID := "00000000-0000-4000-8000-000000000037"
	repositoryArgs := map[string]any{"workspaceId": postgres.DemoWorkspaceID, "repositoryId": repositoryID, "name": "Secondary", "remoteUrl": "https://github.com/jazzcake/secondary", "defaultBranch": "main", "isRecordRepository": false}
	repositoryResult, err := service.Execute(ctx, request("repository.register", repositoryArgs, "repository-register", 10))
	if err != nil || repositoryResult.WorkspaceRevision != 11 || len(repositoryResult.EventIDs) != 1 {
		t.Fatalf("repository.register failed: %#v %v", repositoryResult, err)
	}
	repositoryRetry, err := service.Execute(ctx, request("repository.register", repositoryArgs, "repository-register-retry", 11))
	if err != nil || !repositoryRetry.Idempotent || repositoryRetry.WorkspaceRevision != 11 {
		t.Fatalf("repository retry mismatch: %#v %v", repositoryRetry, err)
	}
	if projectionKeys(t, repositoryResult.Projection, "repository") != projectionKeys(t, repositoryRetry.Projection, "repository") {
		t.Fatalf("repository projection shape changed between applied and idempotent results")
	}
	zeroCommitID := "00000000-0000-4000-8000-000000000041"
	zeroCommitSHA := "3333333333333333333333333333333333333333"
	zeroAttached, err := service.Execute(ctx, request("commit.attach", map[string]any{"workspaceId": postgres.DemoWorkspaceID, "commitId": zeroCommitID, "taskId": 110, "runId": runID, "repositoryId": postgres.DemoRepositoryID, "commitSha": zeroCommitSHA, "relation": "reviewed"}, "zero-record-commit", 11))
	if err != nil || zeroAttached.WorkspaceRevision != 12 {
		t.Fatalf("zero-record commit setup failed: %#v %v", zeroAttached, err)
	}
	_, err = service.Execute(ctx, request("commit.verify_remote", map[string]any{"workspaceId": postgres.DemoWorkspaceID, "commitId": zeroCommitID, "remoteRef": "refs/heads/main"}, "zero-record-verify", 12))
	assertCode(t, err, domain.CodeCommitRemoteUnverified)
	if verifier.calls != 4 {
		t.Fatalf("zero-record verification reached provider: calls=%d", verifier.calls)
	}

	snapshot, err = repo.LoadSnapshot(ctx, postgres.DemoWorkspaceID)
	if err != nil || snapshot.Workspace.Revision != 12 || len(snapshot.Repositories) != 2 || len(snapshot.Records) != 2 || len(snapshot.Commits) != 2 || len(snapshot.GitObservations) != 1 {
		t.Fatalf("Record/Git snapshot mismatch: %#v %v", snapshot, err)
	}
	if snapshot.Records[0].State != "verified" || snapshot.Records[0].CommitSHA != commitSHA || snapshot.Commits[0].VerificationState != "remote_verified" {
		t.Fatalf("Record/Git state mismatch: records=%#v commits=%#v", snapshot.Records, snapshot.Commits)
	}
	events, err := repo.Events(ctx, postgres.DemoWorkspaceID)
	if err != nil || len(events) != 14 {
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
		request("repository.register", repositoryArgs, "closed-repository", 12),
		request("record.register", recordArgs, "closed-record", 12),
		request("record.attach_commit", map[string]any{"workspaceId": postgres.DemoWorkspaceID, "recordId": recordID, "commitSha": commitSHA, "blobSha": blobSHA}, "closed-record-commit", 12),
		request("commit.attach", commitArgs, "closed-commit", 12),
		request("commit.verify_remote", map[string]any{"workspaceId": postgres.DemoWorkspaceID, "commitId": commitID, "remoteRef": "refs/heads/main"}, "closed-remote-verify", 12),
		request("git.observe", observationArgs, "closed-observation", 12),
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
