package integration_test

import (
	"context"
	"os"
	"testing"

	"github.com/jazzcake/baley/server/internal/application"
	"github.com/jazzcake/baley/server/internal/persistence/postgres"
)

func TestPilotMeasurementRecordRegisterAgainstPostgres(t *testing.T) {
	url := os.Getenv("BALEY_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("BALEY_TEST_DATABASE_URL is not set")
	}
	requireDisposableDatabase(t, url)
	t.Setenv("BALEY_LEASE_TOKEN_SECRET", "pilot-measurement-integration-secret")
	ctx := context.Background()
	repo, err := postgres.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Pool.Close()
	if _, err = repo.Pool.Exec(ctx, `SET session_replication_role='replica';
		TRUNCATE security_events,agent_tokens,workspace_memberships,
		account_sessions,account_credentials,accounts,mutation_attempts,
		events,human_approval_attestations,commands,workspace_counters,
		run_git_observations,commit_references,task_record_indexes,repositories,runs,
		gate_entry_tasks,gate_tasks,gates,task_dependencies,tasks,lanes,phases,
		workspaces,actors CASCADE;
		SET session_replication_role='origin'`); err != nil {
		t.Fatal(err)
	}
	if err = repo.SeedDemo(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Pool.Exec(ctx, `UPDATE phases SET state=CASE id
		WHEN 'build' THEN 'completed' WHEN 'validate' THEN 'active' ELSE state END
		WHERE workspace_id=$1`, postgres.DemoWorkspaceID); err != nil {
		t.Fatal(err)
	}
	service := application.NewService(repo)
	_, err = service.Execute(ctx, request("run.start", map[string]any{
		"workspaceId": postgres.DemoWorkspaceID,
		"taskId":      110,
		"clientRunId": "6279cb62-d52f-4642-942c-15e7bd72c901",
		"kind":        "completion_reporting",
	}, "pilot-measurement-run", 1))
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := repo.LoadSnapshot(ctx, postgres.DemoWorkspaceID)
	if err != nil || len(snapshot.Runs) != 1 {
		t.Fatalf("measurement Run missing: %+v %v", snapshot.Runs, err)
	}
	result, err := service.Execute(ctx, request("record.register", map[string]any{
		"workspaceId":     postgres.DemoWorkspaceID,
		"recordId":        "6279cb62-d52f-4642-942c-15e7bd72c902",
		"taskId":          110,
		"runId":           snapshot.Runs[0].ID,
		"recordType":      "pilot-measurement",
		"repositoryId":    postgres.DemoRepositoryID,
		"relativePath":    "task-records/embedding-enablement-acceptance/pilot-measurement-01.md",
		"shortSummary":    "Validated Embedding Enablement acceptance sample",
		"workingTreeHash": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}, "pilot-measurement-register", 2))
	if err != nil || result.WorkspaceRevision != 3 {
		t.Fatalf("pilot-measurement record.register failed: %+v %v", result, err)
	}
	snapshot, err = repo.LoadSnapshot(ctx, postgres.DemoWorkspaceID)
	if err != nil || len(snapshot.Records) != 1 || snapshot.Records[0].Type != "pilot-measurement" {
		t.Fatalf("pilot-measurement projection missing: %+v %v", snapshot.Records, err)
	}
}
