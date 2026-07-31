package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jazzcake/baley/server/internal/application"
	"github.com/jazzcake/baley/server/internal/persistence/postgres"
	"github.com/jazzcake/baley/server/internal/transport/httpapi"
)

func TestMutationAttemptsAreWorkspaceScopedAppendOnlyAndRedacted(t *testing.T) {
	url := os.Getenv("BALEY_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("BALEY_TEST_DATABASE_URL is not set")
	}
	requireDisposableDatabase(t, url)
	ctx := context.Background()
	t.Setenv("BALEY_LEASE_TOKEN_SECRET", "mutation-attempt-integration-secret")
	repo, err := postgres.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Pool.Close()
	if _, err = repo.Pool.Exec(ctx, "SET session_replication_role='replica'; TRUNCATE events,human_approval_attestations,commands,backlog_items,workspace_counters,run_git_observations,commit_references,task_record_indexes,repositories,runs,gate_entry_tasks,gate_tasks,gates,task_dependencies,tasks,lanes,phases,workspaces,actors CASCADE; SET session_replication_role='origin'"); err != nil {
		t.Fatal(err)
	}
	if err = repo.SeedDemo(ctx); err != nil {
		t.Fatal(err)
	}

	service := application.NewService(repo)
	arguments, _ := json.Marshal(map[string]any{
		"workspaceId": postgres.DemoWorkspaceID,
		"backlogUuid": "00000000-0000-4000-8000-000000000098",
		"laneId":      "client", "title": "Audit attempt fixture",
	})
	request := application.CommandRequest{Name: "backlog.create", Arguments: arguments, Envelope: application.CommandEnvelope{
		IdempotencyKey: "raw-secret-idempotency-key", ExpectedWorkspaceRevision: 1, ExecutedByActorID: postgres.DemoAgentActorID,
	}}
	applied, err := service.Execute(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.Execute(ctx, request); err != nil {
		t.Fatal(err)
	}
	rejected := request
	rejected.Envelope.IdempotencyKey = "raw-rejected-key"
	rejected.Envelope.ExpectedWorkspaceRevision = 1
	if _, err = service.Execute(ctx, rejected); err == nil {
		t.Fatal("expected stale revision rejection")
	}

	items, err := repo.MutationAttempts(ctx, postgres.DemoWorkspaceID, "", "backlog.create", time.Time{}, "", 20)
	if err != nil {
		t.Fatal(err)
	}
	outcomes := map[string]bool{}
	for _, item := range items {
		outcomes[item.Outcome] = true
		if item.WorkspaceID != postgres.DemoWorkspaceID {
			t.Fatalf("cross-workspace attempt leaked: %#v", item)
		}
		if strings.Contains(item.IdempotencyKeyHash, "raw-") || strings.Contains(item.ArgumentDigest, "taskId") {
			t.Fatalf("raw sensitive request material persisted: %#v", item)
		}
		if item.Source == "command_service" && (item.Outcome == "succeeded" || item.Outcome == "idempotent") && item.CommandHash == "" {
			t.Fatalf("%s command-service attempt has no command hash: %#v", item.Outcome, item)
		}
	}
	for _, expected := range []string{"succeeded", "idempotent", "rejected"} {
		if !outcomes[expected] {
			t.Fatalf("missing %s attempt in %#v", expected, outcomes)
		}
	}
	if applied.CommandID == "" || len(applied.EventIDs) == 0 {
		t.Fatalf("successful attempt has no command/event evidence: %#v", applied)
	}

	oversized := `{"name":"task.update","arguments":{"workspaceId":"` + postgres.DemoWorkspaceID + `","description":"` + strings.Repeat("x", 1<<20)
	httpRequest := httptest.NewRequest(http.MethodPost, "/v1/commands/execute", strings.NewReader(oversized))
	httpResponse := httptest.NewRecorder()
	(&httpapi.API{Service: service, Repo: repo}).Handler().ServeHTTP(httpResponse, httpRequest)
	if httpResponse.Code != http.StatusBadRequest {
		t.Fatalf("oversized execute status=%d", httpResponse.Code)
	}
	oversizedAttempts, err := repo.MutationAttempts(ctx, postgres.DemoWorkspaceID, "rejected", "task.update", time.Time{}, "", 10)
	if err != nil || len(oversizedAttempts) == 0 || oversizedAttempts[0].ArgumentDigest == "" {
		t.Fatalf("oversized rejected attempt unavailable: %#v %v", oversizedAttempts, err)
	}

	if _, err = repo.Pool.Exec(ctx, `CREATE OR REPLACE FUNCTION fail_mutation_attempt_fixture() RETURNS trigger
		LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'fixture event failure'; END $$;
		CREATE TRIGGER fail_mutation_attempt_fixture BEFORE INSERT ON events
		FOR EACH ROW EXECUTE FUNCTION fail_mutation_attempt_fixture()`); err != nil {
		t.Fatal(err)
	}
	failedArguments, _ := json.Marshal(map[string]any{
		"workspaceId": postgres.DemoWorkspaceID,
		"backlogUuid": "00000000-0000-4000-8000-000000000097",
		"laneId":      "client", "title": "Rolled back audit fixture",
	})
	failedRequest := application.CommandRequest{Name: "backlog.create", Arguments: failedArguments, Envelope: application.CommandEnvelope{
		IdempotencyKey: "raw-failed-key", ExpectedWorkspaceRevision: 2, ExecutedByActorID: postgres.DemoAgentActorID,
	}}
	_, executeFailure := service.Execute(ctx, failedRequest)
	_, cleanupFailure := repo.Pool.Exec(ctx, `DROP TRIGGER fail_mutation_attempt_fixture ON events;
		DROP FUNCTION fail_mutation_attempt_fixture()`)
	if cleanupFailure != nil {
		t.Fatal(cleanupFailure)
	}
	if executeFailure == nil {
		t.Fatal("expected injected persistence failure")
	}
	failedItems, err := repo.MutationAttempts(ctx, postgres.DemoWorkspaceID, "failed", "backlog.create", time.Time{}, "", 20)
	if err != nil || len(failedItems) == 0 {
		t.Fatalf("rolled-back failure attempt unavailable: %#v %v", failedItems, err)
	}

	if _, err = repo.Pool.Exec(ctx, "UPDATE tasks SET current_summary='direct SQL audit' WHERE workspace_id=$1 AND public_id=110", postgres.DemoWorkspaceID); err != nil {
		t.Fatal(err)
	}
	direct, err := repo.MutationAttempts(ctx, postgres.DemoWorkspaceID, "succeeded", "sql.tasks.update", time.Time{}, "", 10)
	if err != nil || len(direct) == 0 || direct[0].Source != "database_trigger" {
		t.Fatalf("direct SQL attempt unavailable: %#v %v", direct, err)
	}
	if _, err = repo.Pool.Exec(ctx, "UPDATE mutation_attempts SET outcome='failed' WHERE id=$1", direct[0].ID); err == nil {
		t.Fatal("append-only mutation_attempts accepted UPDATE")
	}
	if _, err = repo.Pool.Exec(ctx, "DELETE FROM mutation_attempts WHERE id=$1", direct[0].ID); err == nil {
		t.Fatal("append-only mutation_attempts accepted DELETE")
	}
	if _, err = repo.Pool.Exec(ctx, "TRUNCATE mutation_attempts"); err == nil {
		t.Fatal("append-only mutation_attempts accepted TRUNCATE")
	}

	if _, err = repo.Pool.Exec(ctx, "SET session_replication_role='replica'; TRUNCATE task_acceptance_evidence,task_acceptance_assignments; SET session_replication_role='origin'; TRUNCATE tasks CASCADE"); err != nil {
		t.Fatal(err)
	}
	truncated, err := repo.MutationAttempts(ctx, postgres.DemoWorkspaceID, "succeeded", "sql.tasks.truncate", time.Time{}, "", 10)
	if err != nil || len(truncated) == 0 || truncated[0].Source != "database_trigger" {
		t.Fatalf("direct task TRUNCATE audit unavailable: %#v %v", truncated, err)
	}

	tiedAt := time.Now().UTC().Truncate(time.Microsecond)
	tiePrefix := fmt.Sprintf("tie-%d-", time.Now().UnixNano())
	for _, id := range []string{tiePrefix + "a", tiePrefix + "b"} {
		if err = repo.RecordMutationAttempt(ctx, application.MutationAttemptProjection{
			ID: id, WorkspaceID: postgres.DemoWorkspaceID, CommandName: "tie.fixture",
			Source: "command_service", Outcome: "succeeded", EventIDs: []string{},
			DiagnosticCodes: []string{}, OccurredAt: tiedAt,
		}); err != nil {
			t.Fatal(err)
		}
	}
	firstPage, err := repo.MutationAttempts(ctx, postgres.DemoWorkspaceID, "", "tie.fixture", time.Time{}, "", 1)
	if err != nil || len(firstPage) != 1 {
		t.Fatalf("first tuple cursor page failed: %#v %v", firstPage, err)
	}
	secondPage, err := repo.MutationAttempts(ctx, postgres.DemoWorkspaceID, "", "tie.fixture", firstPage[0].OccurredAt, firstPage[0].ID, 1)
	if err != nil || len(secondPage) != 1 || firstPage[0].ID == secondPage[0].ID {
		t.Fatalf("tuple cursor omitted a tied row: first=%#v second=%#v err=%v", firstPage, secondPage, err)
	}

	if err = repo.RecordMutationAttempt(ctx, application.MutationAttemptProjection{
		ID: fmt.Sprintf("other-workspace-%d", time.Now().UnixNano()), WorkspaceID: "other-workspace",
		CommandName: "backlog.create", Source: "command_service", Outcome: "succeeded",
		EventIDs: []string{}, DiagnosticCodes: []string{}, OccurredAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	visible, err := repo.MutationAttempts(ctx, postgres.DemoWorkspaceID, "", "backlog.create", time.Time{}, "", 50)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range visible {
		if item.WorkspaceID == "other-workspace" {
			t.Fatal("workspace isolation failed")
		}
	}
}
