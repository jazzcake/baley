package integration_test

import (
	"context"
	"os"
	"testing"

	"github.com/jazzcake/baley/server/internal/application"
	"github.com/jazzcake/baley/server/internal/domain"
	"github.com/jazzcake/baley/server/internal/persistence/postgres"
)

func TestGateEntryTaskExplicitBindingAndAutomaticFallback(t *testing.T) {
	url := os.Getenv("BALEY_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("BALEY_TEST_DATABASE_URL is not set")
	}
	requireDisposableDatabase(t, url)
	ctx := context.Background()
	t.Setenv("BALEY_LEASE_TOKEN_SECRET", "gate-entry-integration-secret")
	repo, err := postgres.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Pool.Close()
	if _, err = repo.Pool.Exec(ctx, "SET session_replication_role='replica'; TRUNCATE events,human_approval_attestations,commands,workspace_counters,runs,gate_entry_tasks,gate_tasks,gates,task_dependencies,tasks,lanes,phases,workspaces,actors CASCADE; SET session_replication_role='origin'"); err != nil {
		t.Fatal(err)
	}
	if err = repo.SeedDemo(ctx); err != nil {
		t.Fatal(err)
	}
	service := application.NewService(repo)
	snapshot, err := repo.LoadSnapshot(ctx, postgres.DemoWorkspaceID)
	if err != nil {
		t.Fatal(err)
	}
	gate := snapshot.Gates[0]
	if len(gate.EntryTasks) == 0 || gate.EntryTasks[0].SelectionSource != "automatic" {
		t.Fatalf("automatic target-phase roots missing: %#v", gate.EntryTasks)
	}

	attach := request("gate.attach_entry_task", map[string]any{"workspaceId": postgres.DemoWorkspaceID, "gateId": gate.ID, "taskId": 110}, "attach-entry", snapshot.Workspace.Revision)
	preview, err := service.Preview(ctx, attach)
	if err != nil {
		t.Fatal(err)
	}
	if preview.CommandHash == "" {
		t.Fatal("attach preview command hash missing")
	}
	if _, err = service.Execute(ctx, attach); err != nil {
		t.Fatal(err)
	}
	snapshot, err = repo.LoadSnapshot(ctx, postgres.DemoWorkspaceID)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Gates[0].EntryTasks) != 1 || snapshot.Gates[0].EntryTasks[0].SelectionSource != "explicit" {
		t.Fatalf("explicit entry binding not projected: %#v", snapshot.Gates[0].EntryTasks)
	}

	invalid := request("gate.attach_entry_task", map[string]any{"workspaceId": postgres.DemoWorkspaceID, "gateId": gate.ID, "taskId": 101}, "attach-wrong-phase", snapshot.Workspace.Revision)
	invalidPreview, err := service.Preview(ctx, invalid)
	if err != nil {
		t.Fatal(err)
	}
	if len(invalidPreview.Errors) != 1 || invalidPreview.Errors[0].Code != domain.CodeInvalidStateTransition {
		t.Fatalf("wrong-phase entry was not rejected: %#v", invalidPreview.Errors)
	}

	detach := request("gate.detach_entry_task", map[string]any{"workspaceId": postgres.DemoWorkspaceID, "gateId": gate.ID, "taskId": 110}, "detach-entry", snapshot.Workspace.Revision)
	if _, err = service.Execute(ctx, detach); err != nil {
		t.Fatal(err)
	}
	snapshot, err = repo.LoadSnapshot(ctx, postgres.DemoWorkspaceID)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Gates[0].EntryTasks) == 0 || snapshot.Gates[0].EntryTasks[0].SelectionSource != "automatic" {
		t.Fatalf("automatic fallback not restored: %#v", snapshot.Gates[0].EntryTasks)
	}
}
