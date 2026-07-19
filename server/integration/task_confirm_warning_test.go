package integration_test

import (
	"context"
	"encoding/json"
	"os"
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
	ctx := context.Background()
	t.Setenv("BALEY_LEASE_TOKEN_SECRET", "task-confirm-warning-test-secret")
	repo, err := postgres.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Pool.Close()
	if _, err = repo.Pool.Exec(ctx, "TRUNCATE events,human_approval_attestations,commands,workspace_counters,runs,gate_tasks,gates,task_dependencies,tasks,lanes,phases,workspaces,actors CASCADE"); err != nil {
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
	req.Envelope.ProceedReason = "Task #110 is the intentional terminal validation task."
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

func hasDiagnostic(values []application.Diagnostic, code string) bool {
	for _, value := range values {
		if value.Code == code {
			return true
		}
	}
	return false
}
