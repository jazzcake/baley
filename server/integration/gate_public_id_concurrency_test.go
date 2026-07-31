package integration_test

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/jazzcake/baley/server/internal/application"
	"github.com/jazzcake/baley/server/internal/domain"
	"github.com/jazzcake/baley/server/internal/persistence/postgres"
)

func TestGatePublicIDsSerializeAndRollbackWithoutGaps(t *testing.T) {
	url := os.Getenv("BALEY_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("BALEY_TEST_DATABASE_URL is not set")
	}
	requireDisposableDatabase(t, url)
	t.Setenv("BALEY_LEASE_TOKEN_SECRET", "gate-public-id-concurrency-secret")
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
	service := application.NewService(repo)
	wid := postgres.DemoWorkspaceID
	revision := int64(1)
	for index, phase := range []struct{ id, name string }{
		{id: "deploy", name: "Deploy"},
		{id: "release", name: "Release"},
		{id: "observe", name: "Observe"},
	} {
		result, executeErr := service.Execute(ctx, request("phase.create", map[string]any{
			"workspaceId": wid, "phaseId": phase.id, "name": phase.name,
		}, "gate-counter-phase-"+phase.id, revision))
		if executeErr != nil {
			t.Fatalf("phase %s: %v", phase.id, executeErr)
		}
		revision = result.WorkspaceRevision
		if revision != int64(index+2) {
			t.Fatalf("phase revision=%d", revision)
		}
	}

	type outcome struct {
		id     string
		result application.ExecutionResult
		err    error
	}
	requests := []struct {
		id, from, to string
	}{
		{id: "validate-deploy", from: "validate", to: "deploy"},
		{id: "deploy-release", from: "deploy", to: "release"},
	}
	outcomes := make(chan outcome, len(requests))
	var wait sync.WaitGroup
	for _, value := range requests {
		value := value
		wait.Add(1)
		go func() {
			defer wait.Done()
			result, executeErr := service.Execute(ctx, request("gate.create", map[string]any{
				"workspaceId": wid, "gateId": value.id, "name": value.id,
				"fromPhaseId": value.from, "toPhaseId": value.to,
			}, "concurrent-gate-"+value.id, revision))
			outcomes <- outcome{id: value.id, result: result, err: executeErr}
		}()
	}
	wait.Wait()
	close(outcomes)

	var winner, loser string
	for value := range outcomes {
		if value.err == nil {
			if winner != "" || value.result.WorkspaceRevision != revision+1 {
				t.Fatalf("unexpected concurrent Gate success: %#v", value)
			}
			winner = value.id
			continue
		}
		if loser != "" {
			t.Fatalf("more than one concurrent Gate failed: %v", value.err)
		}
		assertCode(t, value.err, domain.CodeStaleRevision)
		loser = value.id
	}
	if winner == "" || loser == "" {
		t.Fatalf("concurrent outcomes winner=%q loser=%q", winner, loser)
	}
	snapshot, err := repo.LoadSnapshot(ctx, wid)
	if err != nil {
		t.Fatal(err)
	}
	if created := application.FindGateByReference(snapshot.Gates, "G#2"); created == nil || created.ID != winner || snapshot.NextGatePublicID != 3 {
		t.Fatalf("serialized first Gate allocation mismatch: gates=%#v next=%d", snapshot.Gates, snapshot.NextGatePublicID)
	}
	var retry struct{ id, from, to string }
	for _, value := range requests {
		if value.id == loser {
			retry = value
		}
	}
	retryResult, err := service.Execute(ctx, request("gate.create", map[string]any{
		"workspaceId": wid, "gateId": retry.id, "name": retry.id,
		"fromPhaseId": retry.from, "toPhaseId": retry.to,
	}, "retry-gate-"+retry.id, snapshot.Workspace.Revision))
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err = repo.LoadSnapshot(ctx, wid)
	if err != nil {
		t.Fatal(err)
	}
	if created := application.FindGateByReference(snapshot.Gates, "G#3"); created == nil || created.ID != loser || snapshot.NextGatePublicID != 4 {
		t.Fatalf("serialized retry Gate allocation mismatch: gates=%#v next=%d", snapshot.Gates, snapshot.NextGatePublicID)
	}

	if _, err = repo.Pool.Exec(ctx, `
		CREATE OR REPLACE FUNCTION reject_gate_public_id_rollback_test() RETURNS trigger
		LANGUAGE plpgsql AS $$
		BEGIN
		  IF NEW.id='rollback-gate' THEN
		    RAISE EXCEPTION 'forced Gate insert failure';
		  END IF;
		  RETURN NEW;
		END $$;
		CREATE TRIGGER reject_gate_public_id_rollback
		  BEFORE INSERT ON gates
		  FOR EACH ROW EXECUTE FUNCTION reject_gate_public_id_rollback_test();
	`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = repo.Pool.Exec(context.Background(), `
			DROP TRIGGER IF EXISTS reject_gate_public_id_rollback ON gates;
			DROP FUNCTION IF EXISTS reject_gate_public_id_rollback_test();
		`)
	})
	_, err = service.Execute(ctx, request("gate.create", map[string]any{
		"workspaceId": wid, "gateId": "rollback-gate", "name": "Rollback Gate",
		"fromPhaseId": "release", "toPhaseId": "observe",
	}, "rollback-gate-failure", retryResult.WorkspaceRevision))
	if err == nil {
		t.Fatal("forced Gate insert failure unexpectedly succeeded")
	}
	snapshot, err = repo.LoadSnapshot(ctx, wid)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.NextGatePublicID != 4 || snapshot.Workspace.Revision != retryResult.WorkspaceRevision {
		t.Fatalf("failed Gate consumed a public ID: next=%d revision=%d", snapshot.NextGatePublicID, snapshot.Workspace.Revision)
	}
	if _, err = repo.Pool.Exec(ctx, `
		DROP TRIGGER reject_gate_public_id_rollback ON gates;
		DROP FUNCTION reject_gate_public_id_rollback_test();
	`); err != nil {
		t.Fatal(err)
	}
	final, err := service.Execute(ctx, request("gate.create", map[string]any{
		"workspaceId": wid, "gateId": "release-observe", "name": "Release Observe",
		"fromPhaseId": "release", "toPhaseId": "observe",
	}, "gate-after-rollback", snapshot.Workspace.Revision))
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err = repo.LoadSnapshot(ctx, wid)
	if err != nil {
		t.Fatal(err)
	}
	if created := application.FindGateByReference(snapshot.Gates, "G#4"); created == nil || created.ID != "release-observe" || snapshot.NextGatePublicID != 5 || final.WorkspaceRevision != retryResult.WorkspaceRevision+1 {
		t.Fatalf("rollback reuse policy mismatch: gates=%#v next=%d revision=%d", snapshot.Gates, snapshot.NextGatePublicID, final.WorkspaceRevision)
	}
}
