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

func TestTaskReportImplementedAgainstPostgres(t *testing.T) {
	url := os.Getenv("BALEY_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("BALEY_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	t.Setenv("BALEY_LEASE_TOKEN_SECRET", "task-report-integration-secret")
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
	if _, err = repo.Pool.Exec(ctx, "UPDATE tasks SET status='in_progress' WHERE workspace_id=$1 AND id='user-test'", postgres.DemoWorkspaceID); err != nil {
		t.Fatal(err)
	}

	service := application.NewService(repo)
	args, _ := json.Marshal(map[string]any{
		"workspaceId": postgres.DemoWorkspaceID,
		"taskId":      110,
		"assessment":  "Implementation and validation completed; residual risk is documented.",
	})
	base := application.CommandRequest{Name: "task.report_implemented", Arguments: args, Envelope: application.CommandEnvelope{
		IdempotencyKey: "task-report-missing-ack", ExpectedWorkspaceRevision: 1, ExecutedByActorID: postgres.DemoAgentActorID,
		ProceedReason: "Fixture intentionally verifies warning acknowledgement.",
	}}
	preview, err := service.Preview(ctx, base)
	if err != nil || len(preview.Errors) != 0 || len(preview.Warnings) != 4 {
		t.Fatalf("unexpected preview: %#v %v", preview, err)
	}
	if _, err = service.Execute(ctx, base); commandErrorCode(err) != domain.CodeInvalidStateTransition {
		t.Fatalf("missing acknowledgement error=%v", err)
	}

	base.Envelope.IdempotencyKey = "task-report-applied"
	base.Envelope.AcknowledgedWarningCodes = []string{
		domain.CodeMissingDetailedPlan,
		domain.CodeMissingIndependentReview,
		domain.CodeMissingCompletionReport,
		domain.CodeDanglingPath,
	}
	result, err := service.Execute(ctx, base)
	if err != nil || result.WorkspaceRevision != 2 || len(result.EventIDs) != 1 {
		t.Fatalf("execute result=%#v err=%v", result, err)
	}
	snapshot, err := repo.LoadSnapshot(ctx, postgres.DemoWorkspaceID)
	if err != nil {
		t.Fatal(err)
	}
	task := taskByPublicID(snapshot.Tasks, 110)
	if task == nil || task.Status != "implemented" || task.ImplementedAssessment == "" || snapshot.Workspace.Revision != 2 {
		t.Fatalf("implemented Task unavailable: %#v", task)
	}
	events, err := repo.Events(ctx, postgres.DemoWorkspaceID)
	if err != nil || len(events) != 1 || events[0].EventType != "task.implemented_reported" {
		t.Fatalf("events=%#v err=%v", events, err)
	}
	var payload map[string]any
	if json.Unmarshal(events[0].Payload, &payload) != nil || len(payload["warnings"].([]any)) != 4 || len(payload["acknowledgedWarningCodes"].([]any)) != 4 {
		t.Fatalf("warning evidence=%s", events[0].Payload)
	}
}

func commandErrorCode(err error) string {
	if commandErr, ok := err.(*application.CommandError); ok {
		return commandErr.Code
	}
	return ""
}

func taskByPublicID(tasks []application.TaskProjection, publicID int) *application.TaskProjection {
	for index := range tasks {
		if tasks[index].PublicID == publicID {
			return &tasks[index]
		}
	}
	return nil
}
