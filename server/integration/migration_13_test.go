package integration_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jazzcake/baley/server/internal/persistence/postgres"
)

func TestMigration13CleansLegacyAutomaticGateEntries(t *testing.T) {
	url := os.Getenv("BALEY_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("BALEY_TEST_DATABASE_URL is not set")
	}
	requireDisposableDatabase(t, url)
	t.Setenv("BALEY_LEASE_TOKEN_SECRET", "migration-13-integration-secret")
	migrations := filepath.Join("..", "migrations")
	// Migrations 14 through 16 are additive and now sit above the migration under test.
	// Step down through 16, 15, 14, and 13 so this test still exercises the
	// legacy automatic-entry cleanup boundary.
	for range 4 {
		if err := postgres.Migrate(url, migrations, "down"); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		if err := postgres.Migrate(url, migrations, "up"); err != nil {
			t.Errorf("restore migration 13: %v", err)
		}
	})
	ctx := context.Background()
	repo, err := postgres.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Pool.Exec(ctx, "SET session_replication_role='replica'; TRUNCATE events,human_approval_attestations,commands,workspace_counters,runs,gate_entry_tasks,gate_tasks,gates,task_dependencies,tasks,lanes,phases,workspaces,actors CASCADE; SET session_replication_role='origin'"); err != nil {
		repo.Pool.Close()
		t.Fatal(err)
	}
	if err = repo.SeedDemo(ctx); err != nil {
		repo.Pool.Close()
		t.Fatal(err)
	}
	if _, err = repo.Pool.Exec(ctx, "INSERT INTO gate_entry_tasks(workspace_id,gate_id,task_id,selection_source) VALUES($1,'pilot-ready','user-test','automatic')", postgres.DemoWorkspaceID); err != nil {
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
	var automatic int
	if err = repo.Pool.QueryRow(ctx, "SELECT count(*) FROM gate_entry_tasks WHERE selection_source='automatic'").Scan(&automatic); err != nil {
		t.Fatal(err)
	}
	if automatic != 0 {
		t.Fatalf("legacy automatic rows remain: %d", automatic)
	}
	if _, err = repo.Pool.Exec(ctx, "INSERT INTO gate_entry_tasks(workspace_id,gate_id,task_id,selection_source) VALUES($1,'pilot-ready','user-test','automatic')", postgres.DemoWorkspaceID); err == nil {
		t.Fatal("migration 13 accepted a persisted automatic entry")
	}
}
