package integration_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jazzcake/baley/server/internal/application"
	"github.com/jazzcake/baley/server/internal/domain"
	"github.com/jazzcake/baley/server/internal/persistence/postgres"
)

func TestRunStartAgainstPostgres(t *testing.T) {
	url := os.Getenv("BALEY_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("BALEY_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	t.Setenv("BALEY_LEASE_TOKEN_SECRET", "run-start-integration-secret")
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
	if _, err = repo.Pool.Exec(ctx, "INSERT INTO workspaces(id,name,revision) VALUES('00000000-0000-4000-8000-000000000099','Draft Workspace',1)"); err != nil {
		t.Fatal(err)
	}
	var draftState string
	if err = repo.Pool.QueryRow(ctx, "SELECT state FROM workspaces WHERE id='00000000-0000-4000-8000-000000000099'").Scan(&draftState); err != nil || draftState != "draft" {
		t.Fatalf("new Workspace default state=%q err=%v", draftState, err)
	}
	if _, err = repo.Pool.Exec(ctx, "DELETE FROM workspaces WHERE id='00000000-0000-4000-8000-000000000099'"); err != nil {
		t.Fatal(err)
	}
	service := application.NewService(repo)

	invalid := request("run.start", map[string]any{"workspaceId": postgres.DemoWorkspaceID, "taskId": 110, "clientRunId": "00000000-0000-4000-8000-000000000010", "kind": "implementation"}, "run-invalid", 1)
	_, err = service.Execute(ctx, invalid)
	assertCode(t, err, domain.CodePhaseInactive)
	invalidUUID := request("run.start", map[string]any{"workspaceId": postgres.DemoWorkspaceID, "taskId": 110, "clientRunId": "not-a-uuid", "kind": "detailed_planning"}, "run-invalid-uuid", 1)
	_, err = service.Execute(ctx, invalidUUID)
	assertCode(t, err, domain.CodeInvalidStateTransition)
	missingParent := request("run.start", map[string]any{"workspaceId": postgres.DemoWorkspaceID, "taskId": 110, "clientRunId": "00000000-0000-4000-8000-000000000013", "kind": "detailed_planning", "parentRunId": "missing-run"}, "run-missing-parent", 1)
	_, err = service.Execute(ctx, missingParent)
	assertCode(t, err, domain.CodeNotFound)
	snapshot, err := repo.LoadSnapshot(ctx, postgres.DemoWorkspaceID)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Workspace.Revision != 1 || len(snapshot.Runs) != 0 || findProjectedTask(snapshot, 110).Status != "pending" {
		t.Fatalf("invalid start wrote state: %#v", snapshot)
	}

	start := request("run.start", map[string]any{"workspaceId": postgres.DemoWorkspaceID, "taskId": 110, "clientRunId": "00000000-0000-4000-8000-000000000011", "kind": "detailed_planning", "sessionRef": "desktop-integration"}, "run-start", 1)
	preview, err := service.Preview(ctx, start)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Errors) != 0 || preview.RequiredCapability != "run:operate" {
		t.Fatalf("unexpected preview: %#v", preview)
	}
	result, err := service.Execute(ctx, start)
	if err != nil {
		t.Fatal(err)
	}
	if result.LeaseToken == "" || result.WorkspaceRevision != 2 || len(result.EventIDs) != 2 {
		t.Fatalf("unexpected run result: %#v", result)
	}
	initialLeaseToken := result.LeaseToken
	retry, err := service.Execute(ctx, start)
	if err != nil || !retry.Idempotent || retry.CommandID != result.CommandID || retry.LeaseToken != initialLeaseToken {
		t.Fatalf("run retry mismatch: %#v %v", retry, err)
	}
	crossKeyRetry := start
	crossKeyRetry.Envelope.IdempotencyKey = "run-start-retry"
	crossKeyRetry.Envelope.ExpectedWorkspaceRevision = 2
	retry, err = service.Execute(ctx, crossKeyRetry)
	if err != nil || !retry.Idempotent || retry.CommandID != result.CommandID || retry.LeaseToken != initialLeaseToken {
		t.Fatalf("client Run retry mismatch: %#v %v", retry, err)
	}
	if _, err = repo.Pool.Exec(ctx, "UPDATE runs SET heartbeat_at=$1,lease_expires_at=$2 WHERE workspace_id=$3", time.Now().UTC().Add(-2*time.Minute), time.Now().UTC().Add(-time.Minute), postgres.DemoWorkspaceID); err != nil {
		t.Fatal(err)
	}
	expiredRetry := crossKeyRetry
	expiredRetry.Envelope.IdempotencyKey = "run-start-expired-retry"
	retry, err = service.Execute(ctx, expiredRetry)
	if err != nil || retry.LeaseToken != initialLeaseToken {
		t.Fatalf("expired Run retry must return the original result without reviving its lease: %#v %v", retry, err)
	}
	conflict := request("run.start", map[string]any{"workspaceId": postgres.DemoWorkspaceID, "taskId": 101, "clientRunId": "00000000-0000-4000-8000-000000000011", "kind": "detailed_planning"}, "run-start-conflict", 2)
	_, err = service.Execute(ctx, conflict)
	assertCode(t, err, domain.CodeIdempotencyConflict)

	snapshot, err = repo.LoadSnapshot(ctx, postgres.DemoWorkspaceID)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Workspace.Revision != 2 || len(snapshot.Runs) != 1 || findProjectedTask(snapshot, 110).Status != "in_progress" {
		t.Fatalf("run transaction incomplete: %#v", snapshot)
	}
	run := snapshot.Runs[0]
	if run.ClientRunID != "00000000-0000-4000-8000-000000000011" || run.Status != "running" || run.Version != 1 || run.TaskID != "user-test" {
		t.Fatalf("unexpected persisted Run: %#v", run)
	}
	if !run.LeaseExpiresAt.Before(time.Now().UTC()) {
		t.Fatalf("idempotent retry revived an expired lease: %#v", run)
	}
	var storedHash string
	if err = repo.Pool.QueryRow(ctx, "SELECT lease_token_hash FROM runs WHERE workspace_id=$1 AND id=$2", postgres.DemoWorkspaceID, run.ID).Scan(&storedHash); err != nil {
		t.Fatal(err)
	}
	if storedHash == "" || storedHash == initialLeaseToken || storedHash != domain.HashLeaseToken(initialLeaseToken) {
		t.Fatalf("lease token storage mismatch: %q", storedHash)
	}
	var commandStoresLeaseToken bool
	if err = repo.Pool.QueryRow(ctx, "SELECT result ? 'leaseToken' FROM commands WHERE workspace_id=$1 AND id=$2", postgres.DemoWorkspaceID, result.CommandID).Scan(&commandStoresLeaseToken); err != nil {
		t.Fatal(err)
	}
	if commandStoresLeaseToken {
		t.Fatal("raw lease token persisted in command result")
	}
	events, err := repo.Events(ctx, postgres.DemoWorkspaceID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].EventType != "task.started" || events[1].EventType != "run.started" {
		t.Fatalf("unexpected Run events: %#v", events)
	}
}

func findProjectedTask(snapshot application.Snapshot, publicID int) application.TaskProjection {
	for _, task := range snapshot.Tasks {
		if task.PublicID == publicID {
			return task
		}
	}
	return application.TaskProjection{}
}
