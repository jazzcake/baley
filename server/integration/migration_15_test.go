package integration_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jazzcake/baley/server/internal/persistence/postgres"
)

func TestMigration15BackfillsGatePublicNumbersAndCounter(t *testing.T) {
	url := os.Getenv("BALEY_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("BALEY_TEST_DATABASE_URL is not set")
	}
	requireDisposableDatabase(t, url)
	t.Setenv("BALEY_LEASE_TOKEN_SECRET", "migration-15-integration-secret")
	migrations := filepath.Join("..", "migrations")
	for range 2 {
		if err := postgres.Migrate(url, migrations, "down"); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		if err := postgres.Migrate(url, migrations, "up"); err != nil {
			t.Errorf("restore migration 15: %v", err)
		}
	})

	ctx := context.Background()
	repo, err := postgres.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Pool.Exec(ctx, `
		SET session_replication_role='replica';
		TRUNCATE events,human_approval_attestations,commands,workspace_counters,runs,
		  gate_entry_tasks,gate_tasks,gates,task_dependencies,tasks,lanes,phases,
		  workspaces,actors CASCADE;
		SET session_replication_role='origin';
		INSERT INTO workspaces(id,name) VALUES('migration-15','Migration 15');
		INSERT INTO phases(workspace_id,id,name,position,state) VALUES
		  ('migration-15','build','Build',0,'active'),
		  ('migration-15','validate','Validate',1,'planned'),
		  ('migration-15','pilot','Pilot',2,'planned');
		INSERT INTO gates(workspace_id,id,name,from_phase_id,to_phase_id) VALUES
		  ('migration-15','z-build-ready','Build Ready','build','validate'),
		  ('migration-15','a-pilot-ready','Pilot Ready','validate','pilot');
		INSERT INTO workspace_counters(workspace_id,next_task_public_id,next_backlog_public_id)
		  VALUES('migration-15',1,1);
	`); err != nil {
		repo.Pool.Close()
		t.Fatal(err)
	}
	repo.Pool.Close()

	if err = postgres.Migrate(url, migrations, "up"); err != nil {
		t.Fatal(err)
	}
	repo, err = postgres.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Pool.Close()

	rows, err := repo.Pool.Query(ctx, "SELECT id,public_id FROM gates WHERE workspace_id='migration-15' ORDER BY public_id")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got := map[string]int{}
	for rows.Next() {
		var id string
		var publicID int
		if err = rows.Scan(&id, &publicID); err != nil {
			t.Fatal(err)
		}
		got[id] = publicID
	}
	if got["z-build-ready"] != 1 || got["a-pilot-ready"] != 2 {
		t.Fatalf("unexpected Gate backfill: %#v", got)
	}
	var next int
	if err = repo.Pool.QueryRow(ctx, "SELECT next_gate_public_id FROM workspace_counters WHERE workspace_id='migration-15'").Scan(&next); err != nil || next != 3 {
		t.Fatalf("unexpected Gate counter: next=%d err=%v", next, err)
	}
	if _, err = repo.Pool.Exec(ctx, "UPDATE gates SET alias='release-ready' WHERE workspace_id='migration-15' AND public_id=1"); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Pool.Exec(ctx, "UPDATE gates SET alias='release-ready' WHERE workspace_id='migration-15' AND public_id=2"); err == nil {
		t.Fatal("duplicate Gate alias accepted")
	}
}
