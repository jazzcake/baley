package integration_test

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/jazzcake/baley/server/internal/application"
	"github.com/jazzcake/baley/server/internal/domain"
	"github.com/jazzcake/baley/server/internal/persistence/postgres"
)

func TestTaskConfirmWarningAcknowledgementIsAtomicAndRetryable(t *testing.T) {
	url := os.Getenv("BALEY_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("BALEY_TEST_DATABASE_URL is not set")
	}
	requireDisposableDatabase(t, url)
	ctx := context.Background()
	t.Setenv("BALEY_LEASE_TOKEN_SECRET", "task-confirm-warning-test-secret")
	repo, err := postgres.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Pool.Close()
	if _, err = repo.Pool.Exec(ctx, "SET session_replication_role='replica'; TRUNCATE events,human_approval_attestations,commands,workspace_counters,runs,gate_tasks,gates,task_dependencies,tasks,lanes,phases,workspaces,actors CASCADE; SET session_replication_role='origin'"); err != nil {
		t.Fatal(err)
	}
	if err = repo.SeedDemo(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Pool.Exec(ctx, "UPDATE tasks SET status='implemented' WHERE workspace_id=$1 AND public_id=110", postgres.DemoWorkspaceID); err != nil {
		t.Fatal(err)
	}

	service := application.NewService(repo)
	req := request("task.confirm", map[string]any{"workspaceId": postgres.DemoWorkspaceID, "taskId": 110}, "warning-retry", 1)
	preview, err := service.Preview(ctx, req)
	if err != nil || !hasDiagnostic(preview.Warnings, domain.CodeDanglingPath) {
		t.Fatalf("dangling preview missing: %#v %v", preview, err)
	}
	req.Envelope.HumanApprovalAttestation = &application.HumanApprovalAttestation{
		ApprovedByActorID: postgres.DemoHumanActorID, ApprovedCommandHash: preview.CommandHash,
	}

	assertNoWrites := func(label string) {
		t.Helper()
		var revision, commands, events, approvals int
		var status string
		if err := repo.Pool.QueryRow(ctx, "SELECT revision FROM workspaces WHERE id=$1", postgres.DemoWorkspaceID).Scan(&revision); err != nil {
			t.Fatal(err)
		}
		if err := repo.Pool.QueryRow(ctx, "SELECT count(*) FROM commands WHERE workspace_id=$1", postgres.DemoWorkspaceID).Scan(&commands); err != nil {
			t.Fatal(err)
		}
		if err := repo.Pool.QueryRow(ctx, "SELECT count(*) FROM events WHERE workspace_id=$1", postgres.DemoWorkspaceID).Scan(&events); err != nil {
			t.Fatal(err)
		}
		if err := repo.Pool.QueryRow(ctx, "SELECT count(*) FROM human_approval_attestations WHERE workspace_id=$1", postgres.DemoWorkspaceID).Scan(&approvals); err != nil {
			t.Fatal(err)
		}
		if err := repo.Pool.QueryRow(ctx, "SELECT status FROM tasks WHERE workspace_id=$1 AND public_id=110", postgres.DemoWorkspaceID).Scan(&status); err != nil {
			t.Fatal(err)
		}
		if revision != 1 || commands != 0 || events != 0 || approvals != 0 || status != "implemented" {
			t.Fatalf("%s mutated state: revision=%d commands=%d events=%d approvals=%d status=%s", label, revision, commands, events, approvals, status)
		}
	}

	if _, err = service.Execute(ctx, req); commandErrorCode(err) != domain.CodeInvalidStateTransition {
		t.Fatalf("missing acknowledgement error=%v", err)
	}
	assertNoWrites("missing acknowledgement")

	mismatch := req
	mismatch.Envelope.IdempotencyKey = "warning-mismatch"
	mismatch.Envelope.AcknowledgedWarningCodes = []string{"phase_order_inversion"}
	if _, err = service.Execute(ctx, mismatch); commandErrorCode(err) != domain.CodeInvalidStateTransition {
		t.Fatalf("mismatched acknowledgement error=%v", err)
	}
	assertNoWrites("mismatched acknowledgement")

	req.Envelope.AcknowledgedWarningCodes = []string{domain.CodeDanglingPath}
	req.Envelope.ProceedReason = "Acknowledge the current topology warning without assigning terminal semantics."
	retryPreview, err := service.Preview(ctx, req)
	if err != nil || retryPreview.CommandHash != preview.CommandHash {
		t.Fatalf("envelope evidence changed canonical hash: %s != %s (%v)", retryPreview.CommandHash, preview.CommandHash, err)
	}
	result, err := service.Execute(ctx, req)
	if err != nil || result.WorkspaceRevision != 2 || len(result.EventIDs) != 2 {
		t.Fatalf("acknowledged retry failed: %#v %v", result, err)
	}
	task, err := repo.Task(ctx, postgres.DemoWorkspaceID, 110)
	if err != nil || task.Status != "confirmed" {
		t.Fatalf("task not confirmed: %#v %v", task, err)
	}
	events, err := repo.Events(ctx, postgres.DemoWorkspaceID)
	if err != nil {
		t.Fatal(err)
	}
	var evidence map[string]any
	for _, event := range events {
		if event.EventType == "task.confirmed" {
			if err := json.Unmarshal(event.Payload, &evidence); err != nil {
				t.Fatal(err)
			}
		}
	}
	if evidence["proceedReason"] != req.Envelope.ProceedReason {
		t.Fatalf("proceed reason evidence missing: %#v", evidence)
	}
	codes, ok := evidence["acknowledgedWarningCodes"].([]any)
	if !ok || len(codes) != 1 || codes[0] != domain.CodeDanglingPath {
		t.Fatalf("warning evidence missing: %#v", evidence)
	}
}

func TestSequentialTaskConfirmationsUseFreshAttestationsForOneApprovalStatement(t *testing.T) {
	url := os.Getenv("BALEY_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("BALEY_TEST_DATABASE_URL is not set")
	}
	requireDisposableDatabase(t, url)
	ctx := context.Background()
	t.Setenv("BALEY_LEASE_TOKEN_SECRET", "task-confirm-sequence-test-secret")
	repo, err := postgres.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Pool.Close()
	if _, err = repo.Pool.Exec(ctx, "SET session_replication_role='replica'; TRUNCATE events,human_approval_attestations,commands,workspace_counters,runs,gate_tasks,gates,task_dependencies,tasks,lanes,phases,workspaces,actors CASCADE; SET session_replication_role='origin'"); err != nil {
		t.Fatal(err)
	}
	if err = repo.SeedDemo(ctx); err != nil {
		t.Fatal(err)
	}

	service := application.NewService(repo)
	statementHash := "sha256:grouped-confirmation-statement"
	conversationRef := "conversation:grouped-confirmation"
	taskIDs := []int{101, 104}
	baselines := make(map[int]application.PreviewResult, len(taskIDs))
	for index, taskID := range taskIDs {
		baselineRequest := request("task.confirm", map[string]any{"workspaceId": postgres.DemoWorkspaceID, "taskId": taskID}, "grouped-baseline-"+string(rune('a'+index)), 1)
		baseline, baselineErr := service.Preview(ctx, baselineRequest)
		if baselineErr != nil || baseline.ExpectedWorkspaceRevision != 1 || len(baseline.Warnings) != 0 {
			t.Fatalf("task %d baseline preview mismatch: %#v %v", taskID, baseline, baselineErr)
		}
		baselines[taskID] = baseline
	}

	revision := int64(1)
	commandHashes := make([]string, 0, 2)
	for index, taskID := range taskIDs {
		req := request("task.confirm", map[string]any{"workspaceId": postgres.DemoWorkspaceID, "taskId": taskID}, "grouped-confirm-"+string(rune('a'+index)), revision)
		preview, previewErr := service.Preview(ctx, req)
		if previewErr != nil || preview.ExpectedWorkspaceRevision != revision || len(preview.Warnings) != 0 {
			t.Fatalf("task %d preview mismatch: %#v %v", taskID, preview, previewErr)
		}
		baseline := baselines[taskID]
		if preview.RequiredCapability != baseline.RequiredCapability ||
			!reflect.DeepEqual(preview.ProjectedDiff, baseline.ProjectedDiff) ||
			!reflect.DeepEqual(preview.Errors, baseline.Errors) ||
			!reflect.DeepEqual(preview.Warnings, baseline.Warnings) ||
			!reflect.DeepEqual(preview.Advisories, baseline.Advisories) ||
			preview.DecisionSnapshotHash != baseline.DecisionSnapshotHash {
			t.Fatalf("task %d fresh preview changed from grouped approval baseline: baseline=%#v fresh=%#v", taskID, baseline, preview)
		}
		req.Envelope.HumanApprovalAttestation = &application.HumanApprovalAttestation{
			ApprovedByActorID:   postgres.DemoHumanActorID,
			ApprovedCommandHash: preview.CommandHash,
			StatementHash:       statementHash,
			ConversationRef:     conversationRef,
		}
		result, executeErr := service.Execute(ctx, req)
		if executeErr != nil || result.WorkspaceRevision != revision+1 {
			t.Fatalf("task %d sequential confirmation failed: %#v %v", taskID, result, executeErr)
		}
		commandHashes = append(commandHashes, preview.CommandHash)
		revision = result.WorkspaceRevision
	}
	if commandHashes[0] == commandHashes[1] {
		t.Fatalf("distinct command previews reused one hash: %v", commandHashes)
	}

	rows, err := repo.Pool.Query(ctx, `SELECT workspace_revision,approved_command_hash,statement_hash,conversation_ref
		FROM human_approval_attestations WHERE workspace_id=$1 ORDER BY workspace_revision`, postgres.DemoWorkspaceID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	index := 0
	for rows.Next() {
		var storedRevision int64
		var commandHash, storedStatementHash, storedConversationRef string
		if err = rows.Scan(&storedRevision, &commandHash, &storedStatementHash, &storedConversationRef); err != nil {
			t.Fatal(err)
		}
		if index >= len(commandHashes) || storedRevision != int64(index+1) || commandHash != commandHashes[index] || storedStatementHash != statementHash || storedConversationRef != conversationRef {
			t.Fatalf("grouped approval evidence mismatch at %d: revision=%d hash=%s statement=%s conversation=%s", index, storedRevision, commandHash, storedStatementHash, storedConversationRef)
		}
		index++
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	if index != 2 {
		t.Fatalf("approval attestation count=%d", index)
	}
}

func TestServerRejectsStaleGroupedApprovalAttestationAfterPreFirstDrift(t *testing.T) {
	url := os.Getenv("BALEY_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("BALEY_TEST_DATABASE_URL is not set")
	}
	requireDisposableDatabase(t, url)
	ctx := context.Background()
	t.Setenv("BALEY_LEASE_TOKEN_SECRET", "task-confirm-group-drift-test-secret")
	repo, err := postgres.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Pool.Close()
	if _, err = repo.Pool.Exec(ctx, "SET session_replication_role='replica'; TRUNCATE events,human_approval_attestations,commands,workspace_counters,runs,gate_tasks,gates,task_dependencies,tasks,lanes,phases,workspaces,actors CASCADE; SET session_replication_role='origin'"); err != nil {
		t.Fatal(err)
	}
	if err = repo.SeedDemo(ctx); err != nil {
		t.Fatal(err)
	}

	service := application.NewService(repo)
	baselineRequest := request("task.confirm", map[string]any{"workspaceId": postgres.DemoWorkspaceID, "taskId": 101}, "group-baseline-task-101", 1)
	baseline, err := service.Preview(ctx, baselineRequest)
	if err != nil || baseline.ExpectedWorkspaceRevision != 1 {
		t.Fatalf("baseline preview mismatch: %#v %v", baseline, err)
	}

	external := request("lane.update", map[string]any{"workspaceId": postgres.DemoWorkspaceID, "laneId": "server", "name": "Server", "goal": "External revision", "summary": "Concurrent change after grouped approval"}, "group-external-revision", 1)
	externalResult, err := service.Execute(ctx, external)
	if err != nil || externalResult.WorkspaceRevision != 2 {
		t.Fatalf("external revision fixture failed: %#v %v", externalResult, err)
	}

	staleExecute := baselineRequest
	staleExecute.Envelope.HumanApprovalAttestation = &application.HumanApprovalAttestation{ApprovedByActorID: postgres.DemoHumanActorID, ApprovedCommandHash: baseline.CommandHash, StatementHash: "sha256:grouped-approval"}
	if _, err = service.Execute(ctx, staleExecute); commandErrorCode(err) != domain.CodeStaleRevision {
		t.Fatalf("stale grouped baseline executed: %v", err)
	}

	freshRequest := request("task.confirm", map[string]any{"workspaceId": postgres.DemoWorkspaceID, "taskId": 101}, "group-fresh-task-101", 2)
	fresh, err := service.Preview(ctx, freshRequest)
	if err != nil || fresh.ExpectedWorkspaceRevision != 2 || fresh.CommandHash == baseline.CommandHash {
		t.Fatalf("fresh preview did not expose revision drift: baseline=%#v fresh=%#v err=%v", baseline, fresh, err)
	}
	freshRequest.Envelope.HumanApprovalAttestation = &application.HumanApprovalAttestation{ApprovedByActorID: postgres.DemoHumanActorID, ApprovedCommandHash: baseline.CommandHash, StatementHash: "sha256:grouped-approval"}
	if _, err = service.Execute(ctx, freshRequest); commandErrorCode(err) != domain.CodeHumanApprovalMismatch {
		t.Fatalf("old grouped attestation accepted for fresh preview: %v", err)
	}
	task, err := repo.Task(ctx, postgres.DemoWorkspaceID, 101)
	if err != nil || task.Status != "implemented" {
		t.Fatalf("drifted grouped confirmation mutated Task: %#v %v", task, err)
	}
}

func hasDiagnostic(values []application.Diagnostic, code string) bool {
	for _, value := range values {
		if value.Code == code {
			return true
		}
	}
	return false
}
