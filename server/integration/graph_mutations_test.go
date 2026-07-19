package integration_test

import (
	"context"
	"os"
	"testing"

	"github.com/jazzcake/baley/server/internal/application"
	"github.com/jazzcake/baley/server/internal/domain"
	"github.com/jazzcake/baley/server/internal/persistence/postgres"
)

func TestTaskAndDependencyMutationsAgainstPostgres(t *testing.T) {
	url := os.Getenv("BALEY_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("BALEY_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	t.Setenv("BALEY_LEASE_TOKEN_SECRET", "graph-mutation-integration-secret")
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
	execute := func(name string, args map[string]any, key string, revision int64) application.ExecutionResult {
		t.Helper()
		result, executeErr := service.Execute(ctx, request(name, args, key, revision))
		if executeErr != nil {
			t.Fatalf("%s failed: %v", name, executeErr)
		}
		return result
	}
	wid := postgres.DemoWorkspaceID
	connect := map[string]any{"workspaceId": wid, "predecessorTaskId": 101, "successorTaskId": 106}
	if result := execute("dependency.connect", connect, "connect-cross-lane", 1); result.WorkspaceRevision != 2 {
		t.Fatalf("revision=%d", result.WorkspaceRevision)
	}
	_, err = service.Execute(ctx, request("dependency.connect", map[string]any{"workspaceId": wid, "predecessorTaskId": 106, "successorTaskId": 101}, "connect-cycle", 2))
	assertCode(t, err, domain.CodeDependencyCycle)
	snapshot, _ := repo.LoadSnapshot(ctx, wid)
	if snapshot.Workspace.Revision != 2 || len(snapshot.Dependencies) != 2 {
		t.Fatalf("cycle was not rolled back: revision=%d edges=%#v", snapshot.Workspace.Revision, snapshot.Dependencies)
	}
	execute("task.block", map[string]any{"workspaceId": wid, "taskId": 110, "reason": "Waiting for an external fixture"}, "task-block", 2)
	execute("task.unblock", map[string]any{"workspaceId": wid, "taskId": 110, "reason": "Fixture is available"}, "task-unblock", 3)
	execute("task.update", map[string]any{"workspaceId": wid, "taskId": 110, "title": "User acceptance", "description": "Validate the Viewer", "currentSummary": "Ready to execute", "nextAction": "Start validation"}, "task-update", 4)
	execute("task.set_terminal", map[string]any{"workspaceId": wid, "taskId": 110, "reason": "Intentional validation leaf"}, "task-terminal", 5)
	_, err = service.Execute(ctx, request("dependency.connect", map[string]any{"workspaceId": wid, "predecessorTaskId": 110, "successorTaskId": 101}, "terminal-conflict", 6))
	assertCode(t, err, domain.CodeTerminalPathConflict)
	patch := map[string]any{"workspaceId": wid, "add": []map[string]any{{"predecessorTaskId": 110, "successorTaskId": 101}}, "terminalUpdates": []map[string]any{{"taskId": 110, "terminalReason": nil}}}
	patchRequest := request("dependency.patch", patch, "terminal-clear-and-connect", 6)
	patchRequest.Envelope.AcknowledgedWarningCodes = []string{domain.CodePhaseOrderInversion}
	if _, err = service.Execute(ctx, patchRequest); err != nil {
		t.Fatalf("dependency.patch failed: %v", err)
	}
	snapshot, _ = repo.LoadSnapshot(ctx, wid)
	task := taskByPublicID(snapshot.Tasks, 110)
	if snapshot.Workspace.Revision != 7 || len(snapshot.Dependencies) != 3 || task == nil || task.TerminalReason != "" || task.Title != "User acceptance" {
		t.Fatalf("graph mutation snapshot mismatch: %#v", snapshot)
	}
	execute("phase.create", map[string]any{"workspaceId": wid, "phaseId": "deploy", "name": "Deploy"}, "phase-create", 7)
	if _, err = repo.Pool.Exec(ctx, "UPDATE workspace_counters SET next_task_public_id=150 WHERE workspace_id=$1", wid); err != nil {
		t.Fatal(err)
	}
	execute("task.create", map[string]any{"workspaceId": wid, "taskUuid": "00000000-0000-4000-8000-000000000041", "laneId": "client", "phaseId": "validate", "title": "Acceptance report", "predecessorTaskIds": []int{110}}, "task-create", 8)
	snapshot, _ = repo.LoadSnapshot(ctx, wid)
	if created := taskByPublicID(snapshot.Tasks, 150); created == nil || snapshot.NextTaskPublicID != 151 {
		t.Fatalf("Workspace counter did not issue Task #150: %#v", snapshot)
	}
	execute("task.set_terminal", map[string]any{"workspaceId": wid, "taskId": 150, "reason": "Temporary leaf"}, "created-task-terminal", 9)
	clear := request("task.clear_terminal", map[string]any{"workspaceId": wid, "taskId": 150}, "created-task-clear", 10)
	clear.Envelope.ProceedReason = "The path will be connected later"
	_, err = service.Execute(ctx, clear)
	assertCode(t, err, domain.CodeInvalidStateTransition)
	clear.Envelope.AcknowledgedWarningCodes = []string{domain.CodeDanglingPath}
	if _, err = service.Execute(ctx, clear); err != nil {
		t.Fatalf("acknowledged terminal clear failed: %v", err)
	}
	execute("gate.create", map[string]any{"workspaceId": wid, "gateId": "validation-ready", "name": "Validation Ready", "fromPhaseId": "validate", "toPhaseId": "deploy"}, "gate-create", 11)
	execute("gate.attach_task", map[string]any{"workspaceId": wid, "gateId": "validation-ready", "taskId": 110}, "gate-attach-future", 12)
	execute("gate.detach_task", map[string]any{"workspaceId": wid, "gateId": "validation-ready", "taskId": 110}, "gate-detach-future", 13)
	execute("lane.create", map[string]any{"workspaceId": wid, "laneId": "ops", "name": "Operations"}, "lane-create", 14)
	execute("lane.update", map[string]any{"workspaceId": wid, "laneId": "ops", "name": "Operations", "goal": "Ship safely", "summary": "Deployment work"}, "lane-update", 15)
	closePreview, err := service.Preview(ctx, request("lane.close_out", map[string]any{"workspaceId": wid, "laneId": "ops", "reason": "Deployment complete"}, "lane-close-preview", 16))
	if err != nil || closePreview.RequiredCapability != "lane:approve" || !diagnosticCodePresent(closePreview.Errors, domain.CodeHumanApprovalRequired) {
		t.Fatalf("lane close approval preview mismatch: %#v %v", closePreview, err)
	}
	discardPreview, err := service.Preview(ctx, request("task.discard", map[string]any{"workspaceId": wid, "taskId": 150, "reason": "No longer needed"}, "task-discard-preview", 16))
	if err != nil || discardPreview.RequiredCapability != "task:approve" || !diagnosticCodePresent(discardPreview.Errors, domain.CodeHumanApprovalRequired) {
		t.Fatalf("task discard approval preview mismatch: %#v %v", discardPreview, err)
	}
	extraneous := request("lane.update", map[string]any{"workspaceId": wid, "laneId": "ops", "name": "Operations", "goal": "Ship safely", "summary": "Deployment work"}, "extraneous-approval", 16)
	extraneous.Envelope.HumanApprovalAttestation = &application.HumanApprovalAttestation{ApprovedByActorID: postgres.DemoHumanActorID, ApprovedCommandHash: "sha256:unused"}
	_, err = service.Execute(ctx, extraneous)
	assertCode(t, err, domain.CodeHumanApprovalMismatch)
}

func diagnosticCodePresent(values []domain.Diagnostic, code string) bool {
	for _, value := range values {
		if value.Code == code {
			return true
		}
	}
	return false
}
