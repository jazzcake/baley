package integration_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jazzcake/baley/server/internal/persistence/postgres"
)

func TestMigration27BackfillsDurableBirdViewPositions(t *testing.T) {
	url := os.Getenv("BALEY_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("BALEY_TEST_DATABASE_URL is not set")
	}
	requireDisposableDatabase(t, url)
	migrations := filepath.Join("..", "migrations")
	if err := postgres.Migrate(url, migrations, "up"); err != nil {
		t.Fatal(err)
	}
	if err := postgres.Migrate(url, migrations, "down"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := postgres.Migrate(url, migrations, "up"); err != nil {
			t.Errorf("restore latest migration: %v", err)
		}
	})

	ctx := context.Background()
	repo, err := postgres.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Pool.Close()
	if _, err = repo.Pool.Exec(ctx, `TRUNCATE bird_view_events,bird_view_commands,bird_view_phase_bindings,bird_view_gate_bindings,bird_view_task_bindings,bird_view_backlog_bindings,bird_view_edges,bird_view_nodes,bird_views CASCADE`); err != nil {
		t.Fatal(err)
	}
	if err = repo.SeedDemo(ctx); err != nil {
		t.Fatal(err)
	}
	if err = repo.BootstrapOwner(ctx, postgres.DemoWorkspaceID, "10000000-0000-4000-8000-000000000027", postgres.DemoHumanActorID, "migration-27", "migration-27", "Migration 27", "test-only"); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Pool.Exec(ctx, `
		INSERT INTO bird_views(id,account_id,title) VALUES('20000000-0000-4000-8000-000000000027','10000000-0000-4000-8000-000000000027','Legacy map');
		INSERT INTO bird_view_nodes(account_id,bird_view_id,id,title) VALUES
		('10000000-0000-4000-8000-000000000027','20000000-0000-4000-8000-000000000027','30000000-0000-4000-8000-000000000027','First'),
		('10000000-0000-4000-8000-000000000027','20000000-0000-4000-8000-000000000027','30000000-0000-4000-8000-000000000028','Second')`); err != nil {
		t.Fatal(err)
	}
	if err = postgres.Migrate(url, migrations, "up"); err != nil {
		t.Fatal(err)
	}
	var firstX, firstY, secondX, secondY float64
	if err = repo.Pool.QueryRow(ctx, `SELECT min(position_x),min(position_y),max(position_x),max(position_y) FROM bird_view_nodes WHERE bird_view_id='20000000-0000-4000-8000-000000000027'`).Scan(&firstX, &firstY, &secondX, &secondY); err != nil {
		t.Fatal(err)
	}
	if firstX != 0 || firstY != 0 || secondX != 320 || secondY != 0 {
		t.Fatalf("legacy position backfill=(%v,%v)-(%v,%v)", firstX, firstY, secondX, secondY)
	}
}
