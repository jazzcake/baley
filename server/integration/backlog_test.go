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

func TestLaneBacklogVerticalSliceAgainstPostgres(t *testing.T) {
	url := os.Getenv("BALEY_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("BALEY_TEST_DATABASE_URL is not set")
	}
	requireDisposableDatabase(t, url)
	ctx := context.Background()
	t.Setenv("BALEY_LEASE_TOKEN_SECRET", "backlog-integration-secret")
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
	wid := postgres.DemoWorkspaceID

	createOne := map[string]any{"workspaceId": wid, "backlogUuid": "00000000-0000-4000-8000-000000000081", "laneId": "client", "title": " First candidate ", "description": " phase free "}
	preview, err := service.Preview(ctx, request("backlog.create", createOne, "backlog-create-1", 1))
	if err != nil || len(preview.Errors) != 0 {
		t.Fatalf("create preview: %#v %v", preview, err)
	}
	before, _ := repo.LoadSnapshot(ctx, wid)
	if before.NextBacklogPublicID != 1 || len(before.BacklogItems) != 0 {
		t.Fatalf("preview wrote state: %#v", before.BacklogItems)
	}
	first, err := service.Execute(ctx, request("backlog.create", createOne, "backlog-create-1", 1))
	if err != nil || first.WorkspaceRevision != 2 {
		t.Fatalf("create: %#v %v", first, err)
	}
	createTwo := map[string]any{"workspaceId": wid, "backlogUuid": "00000000-0000-4000-8000-000000000082", "laneId": "client", "title": "Second candidate"}
	if _, err = service.Execute(ctx, request("backlog.create", createTwo, "backlog-create-2", 2)); err != nil {
		t.Fatal(err)
	}

	if _, err = service.Execute(ctx, request("backlog.reorder", map[string]any{"workspaceId": wid, "laneId": "client", "orderedBacklogPublicIds": []int{2, 1}}, "backlog-reorder", 3)); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Execute(ctx, request("backlog.move", map[string]any{"workspaceId": wid, "backlogPublicId": 2, "targetLaneId": "server"}, "backlog-move", 4)); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Execute(ctx, request("backlog.update", map[string]any{"workspaceId": wid, "backlogPublicId": 2, "description": ""}, "backlog-update", 5)); err != nil {
		t.Fatal(err)
	}
	laneClose, err := service.Preview(ctx, request("lane.close_out", map[string]any{"workspaceId": wid, "laneId": "client", "reason": "done"}, "backlog-lane-close", 6))
	if err != nil {
		t.Fatal(err)
	}
	foundBacklogBlock := false
	for _, diagnostic := range laneClose.Errors {
		foundBacklogBlock = foundBacklogBlock || diagnostic.Code == domain.CodeLaneHasActiveBacklog
	}
	if !foundBacklogBlock {
		t.Fatalf("active Backlog did not block lane termination: %#v", laneClose)
	}

	promote := map[string]any{
		"workspaceId": wid, "backlogPublicId": 1, "taskUuid": "00000000-0000-4000-8000-000000000091",
		"phaseId": "validate", "predecessorTaskIds": []int{101},
	}
	promotePreview, err := service.Preview(ctx, request("backlog.promote", promote, "backlog-promote", 6))
	if err != nil || len(promotePreview.Errors) != 0 {
		t.Fatalf("promote preview: %#v %v", promotePreview, err)
	}
	if _, err = repo.Pool.Exec(ctx, `CREATE FUNCTION fail_backlog_promoted_event() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
		IF NEW.event_type='backlog.promoted' THEN RAISE EXCEPTION 'injected backlog promotion rollback'; END IF;
		RETURN NEW; END $$;
		CREATE TRIGGER fail_backlog_promoted_event BEFORE INSERT ON events FOR EACH ROW EXECUTE FUNCTION fail_backlog_promoted_event()`); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Execute(ctx, request("backlog.promote", promote, "backlog-promote", 6)); err == nil {
		t.Fatal("injected promotion failure unexpectedly committed")
	}
	if _, dropErr := repo.Pool.Exec(ctx, "DROP TRIGGER fail_backlog_promoted_event ON events; DROP FUNCTION fail_backlog_promoted_event()"); dropErr != nil {
		t.Fatal(dropErr)
	}
	rolledBack, loadErr := repo.LoadSnapshot(ctx, wid)
	if loadErr != nil || rolledBack.Workspace.Revision != 6 || backlogByPublicID(rolledBack.BacklogItems, 1).Status != "active" || taskByPublicID(rolledBack.Tasks, 111) != nil || rolledBack.NextTaskPublicID != 111 {
		t.Fatalf("promotion was not atomic on injected failure: %#v %v", rolledBack, loadErr)
	}
	if _, err = service.Execute(ctx, request("backlog.promote", promote, "backlog-promote", 6)); err != nil {
		t.Fatal(err)
	}
	snapshot, err := repo.LoadSnapshot(ctx, wid)
	if err != nil {
		t.Fatal(err)
	}
	promoted := backlogByPublicID(snapshot.BacklogItems, 1)
	task := taskByPublicID(snapshot.Tasks, 111)
	if promoted == nil || promoted.Status != "promoted" || promoted.Position != nil || promoted.PromotedTaskID == nil || task == nil || task.LaneID != "client" || task.PhaseID != "validate" || task.Title != "First candidate" || task.Status != "pending" || snapshot.NextBacklogPublicID != 3 || snapshot.NextTaskPublicID != 112 {
		t.Fatalf("atomic promotion mismatch: promoted=%#v task=%#v counters=%d/%d", promoted, task, snapshot.NextBacklogPublicID, snapshot.NextTaskPublicID)
	}
	if len(snapshot.Gates[0].Conditions) != 3 {
		t.Fatal("promotion changed Gate conditions")
	}

	if _, err = service.Execute(ctx, request("backlog.discard", map[string]any{"workspaceId": wid, "backlogPublicId": 2, "reason": "superseded"}, "backlog-discard", 7)); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Execute(ctx, request("backlog.update", map[string]any{"workspaceId": wid, "backlogPublicId": 2, "title": "again"}, "backlog-terminal-update", 8)); err == nil {
		t.Fatal("discarded backlog item remained mutable")
	}
	list, err := repo.BacklogList(ctx, wid, "", "", 0, 50)
	if err != nil || len(list) != 2 {
		t.Fatalf("terminal provenance missing: %#v %v", list, err)
	}
	graph, _ := repo.LoadSnapshot(ctx, wid)
	active := 0
	for _, item := range graph.BacklogItems {
		if item.Status == string(domain.BacklogActive) {
			active++
		}
	}
	if active != 0 {
		t.Fatalf("active backlog remained: %#v", graph.BacklogItems)
	}
}

func TestConcurrentBacklogCreateUsesRevisionAndIndependentCounter(t *testing.T) {
	url := os.Getenv("BALEY_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("BALEY_TEST_DATABASE_URL is not set")
	}
	requireDisposableDatabase(t, url)
	ctx := context.Background()
	t.Setenv("BALEY_LEASE_TOKEN_SECRET", "backlog-concurrency-secret")
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
	wid := postgres.DemoWorkspaceID
	requests := []application.CommandRequest{
		request("backlog.create", map[string]any{"workspaceId": wid, "backlogUuid": "00000000-0000-4000-8000-000000000083", "laneId": "client", "title": "A"}, "concurrent-a", 1),
		request("backlog.create", map[string]any{"workspaceId": wid, "backlogUuid": "00000000-0000-4000-8000-000000000084", "laneId": "client", "title": "B"}, "concurrent-b", 1),
	}
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range requests {
		wg.Add(1)
		go func(index int) { defer wg.Done(); _, errs[index] = service.Execute(ctx, requests[index]) }(i)
	}
	wg.Wait()
	successes, stale := 0, 0
	for _, executeErr := range errs {
		if executeErr == nil {
			successes++
		} else if commandCode(executeErr) == domain.CodeStaleRevision {
			stale++
		}
	}
	if successes != 1 || stale != 1 {
		t.Fatalf("concurrent outcomes: %#v", errs)
	}
	snapshot, err := repo.LoadSnapshot(ctx, wid)
	if err != nil || len(snapshot.BacklogItems) != 1 || snapshot.NextBacklogPublicID != 2 || snapshot.NextTaskPublicID != 111 {
		t.Fatalf("counter state: %#v %v", snapshot, err)
	}
	loser := 0
	if errs[0] == nil {
		loser = 1
	}
	requests[loser].Envelope.ExpectedWorkspaceRevision = snapshot.Workspace.Revision
	requests[loser].Envelope.IdempotencyKey += "-retry"
	if _, err = service.Execute(ctx, requests[loser]); err != nil {
		t.Fatal(err)
	}
	snapshot, _ = repo.LoadSnapshot(ctx, wid)
	if len(snapshot.BacklogItems) != 2 || snapshot.NextBacklogPublicID != 3 || snapshot.NextTaskPublicID != 111 {
		t.Fatalf("retry counter state: %#v", snapshot)
	}
}

func backlogByPublicID(values []application.BacklogItemProjection, publicID int) *application.BacklogItemProjection {
	for i := range values {
		if values[i].PublicID == publicID {
			return &values[i]
		}
	}
	return nil
}

func commandCode(err error) string {
	if value, ok := err.(*application.CommandError); ok {
		return value.Code
	}
	return ""
}
