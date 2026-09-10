package integration_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jazzcake/baley/server/internal/persistence/postgres"
)

func TestMigration26TaskJournalUpDownAndAppendOnly(t *testing.T) {
	url := os.Getenv("BALEY_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("BALEY_TEST_DATABASE_URL is not set")
	}
	requireDisposableDatabase(t, url)
	migrations := filepath.Join("..", "migrations")
	if err := postgres.Migrate(url, migrations, "up"); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	repo, err := postgres.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Pool.Close()
	// Migration 29 owns append-only Events, migration 28 owns conversational
	// evidence, and migration 27 is forward-only. Move all three version markers
	// down first, then
	// exercise migration 26's actual schema rollback on the empty projection.
	if _, err = repo.Pool.Exec(ctx, "SET session_replication_role='replica'; TRUNCATE task_journal_entries; SET session_replication_role='origin'"); err != nil {
		t.Fatal(err)
	}
	if err := postgres.Migrate(url, migrations, "down"); err != nil {
		t.Fatal(err)
	}
	if err := postgres.Migrate(url, migrations, "down"); err != nil {
		t.Fatal(err)
	}
	if err := postgres.Migrate(url, migrations, "down"); err != nil {
		t.Fatal(err)
	}
	if err := postgres.Migrate(url, migrations, "down"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := postgres.Migrate(url, migrations, "up"); err != nil {
			t.Errorf("restore migrations 26 through 29: %v", err)
		}
	})
	var tableName *string
	if err = repo.Pool.QueryRow(ctx, "SELECT to_regclass('task_journal_entries')::text").Scan(&tableName); err != nil || tableName != nil {
		t.Fatalf("migration down retained journal table: name=%v err=%v", tableName, err)
	}
	if err = postgres.Migrate(url, migrations, "up"); err != nil {
		t.Fatal(err)
	}
	for _, relation := range []string{
		"task_journal_entries", "task_journal_entries_workspace_time_idx",
		"task_journal_entries_task_time_idx", "task_journal_entries_context_gin_idx",
	} {
		var existing *string
		if err = repo.Pool.QueryRow(ctx, "SELECT to_regclass($1)::text", relation).Scan(&existing); err != nil || existing == nil {
			t.Fatalf("relation %s missing: %v", relation, err)
		}
	}
	var triggerCount, compositeConstraintCount int
	if err = repo.Pool.QueryRow(ctx, `SELECT count(*) FROM pg_trigger
		WHERE tgrelid='task_journal_entries'::regclass AND tgname IN ('task_journal_entries_append_only','task_journal_entries_truncate_append_only') AND NOT tgisinternal`).Scan(&triggerCount); err != nil || triggerCount != 2 {
		t.Fatalf("append-only trigger count=%d err=%v", triggerCount, err)
	}
	if err = repo.Pool.QueryRow(ctx, `SELECT count(*) FROM pg_constraint
		WHERE conname IN ('commands_workspace_id_id_unique','events_workspace_id_id_unique')`).Scan(&compositeConstraintCount); err != nil || compositeConstraintCount != 2 {
		t.Fatalf("composite source constraints=%d err=%v", compositeConstraintCount, err)
	}
}
