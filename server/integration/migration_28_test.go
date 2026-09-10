package integration_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jazzcake/baley/server/internal/persistence/postgres"
)

func TestMigration28ConversationalDecisionEvidenceUpDown(t *testing.T) {
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
	assertMigration28Schema(t, ctx, repo, true, 28)
	if err = postgres.Migrate(url, migrations, "down"); err != nil {
		t.Fatal(err)
	}
	assertMigration28Schema(t, ctx, repo, false, 27)
	if err = postgres.Migrate(url, migrations, "up"); err != nil {
		t.Fatal(err)
	}
	assertMigration28Schema(t, ctx, repo, true, 28)
}

func assertMigration28Schema(t *testing.T, ctx context.Context, repo *postgres.Repository, present bool, version int) {
	t.Helper()
	var gotVersion int
	if err := repo.Pool.QueryRow(ctx, "SELECT version_id FROM goose_db_version WHERE is_applied ORDER BY id DESC LIMIT 1").Scan(&gotVersion); err != nil || gotVersion != version {
		t.Fatalf("schema version=%d want=%d err=%v", gotVersion, version, err)
	}
	var tableName *string
	if err := repo.Pool.QueryRow(ctx, "SELECT to_regclass('conversational_decision_evidence')::text").Scan(&tableName); err != nil {
		t.Fatal(err)
	}
	var columnExists bool
	if err := repo.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema='public' AND table_name='human_approval_attestations' AND column_name='decision_evidence_id')`).Scan(&columnExists); err != nil {
		t.Fatal(err)
	}
	if (tableName != nil) != present || columnExists != present {
		t.Fatalf("migration 28 present=%v table=%v column=%v", present, tableName, columnExists)
	}
}
