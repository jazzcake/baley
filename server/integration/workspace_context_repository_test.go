package integration_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jazzcake/baley/server/internal/persistence/postgres"
)

func TestCompactWorkspaceContextAndPhasePageAgainstPostgres(t *testing.T) {
	url := os.Getenv("BALEY_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("BALEY_TEST_DATABASE_URL is not set")
	}
	requireDisposableDatabase(t, url)
	t.Setenv("BALEY_LEASE_TOKEN_SECRET", "compact-context-integration-secret")
	ctx := context.Background()
	repo, err := postgres.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Pool.Close()
	workspaceID := fmt.Sprintf("compact-context-%d", time.Now().UnixNano())
	defer func() { _, _ = repo.Pool.Exec(ctx, "DELETE FROM workspaces WHERE id=$1", workspaceID) }()
	for _, statement := range []string{
		"INSERT INTO workspaces(id,name,state) VALUES($1,'Compact','active')",
		"INSERT INTO workspace_counters(workspace_id,next_task_public_id,next_backlog_public_id,next_gate_public_id) VALUES($1,100,1,1)",
		"INSERT INTO lanes(workspace_id,id,name,state) VALUES($1,'server','Server','active')",
		"INSERT INTO phases(workspace_id,id,name,position,state) VALUES($1,'done','Done',0,'completed'),($1,'active','Active',1,'active')",
		"INSERT INTO tasks(workspace_id,id,public_id,lane_id,phase_id,title,description,status) VALUES($1,'done-task',1,'server','done','hidden','hidden body','confirmed'),($1,'active-a',2,'server','active','first','first body','pending'),($1,'active-b',3,'server','active','second','second body','in_progress')",
	} {
		if _, err = repo.Pool.Exec(ctx, statement, workspaceID); err != nil {
			t.Fatal(err)
		}
	}
	compact, err := repo.WorkspaceContext(ctx, workspaceID)
	if err != nil || len(compact.Phases) != 1 || compact.Phases[0].ID != "active" || compact.Phases[0].LaneCounts[0].StatusCounts["pending"] != 1 || compact.Phases[0].LaneCounts[0].StatusCounts["in_progress"] != 1 {
		t.Fatalf("compact context=%#v err=%v", compact, err)
	}
	page, cursor, more, err := repo.PhaseTasks(ctx, workspaceID, "active", 0, 1)
	if err != nil || len(page) != 1 || page[0].PublicID != 2 || cursor != 2 || !more {
		t.Fatalf("page=%#v cursor=%d more=%v err=%v", page, cursor, more, err)
	}
	if _, _, _, err = repo.PhaseTasks(ctx, workspaceID, "done", 0, 1); err == nil {
		t.Fatal("completed Phase was expandable")
	}
}
