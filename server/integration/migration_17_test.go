package integration_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jazzcake/baley/server/internal/persistence/postgres"
)

func TestMigration17RemovesApprovalGrantStorage(t *testing.T) {
	url := os.Getenv("BALEY_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("BALEY_TEST_DATABASE_URL is not set")
	}
	requireDisposableDatabase(t, url)
	t.Setenv("BALEY_LEASE_TOKEN_SECRET", "migration-17-integration-secret")
	migrations := filepath.Join("..", "migrations")
	if err := postgres.Migrate(url, migrations, "down"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := postgres.Migrate(url, migrations, "up"); err != nil {
			t.Errorf("restore migration 17: %v", err)
		}
	})

	ctx := context.Background()
	repo, err := postgres.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	var tableName *string
	if err = repo.Pool.QueryRow(ctx, "SELECT to_regclass('approval_grants')::text").Scan(&tableName); err != nil || tableName == nil {
		repo.Pool.Close()
		t.Fatalf("pre-migration approval_grants missing: %v", err)
	}
	var columnCount int
	if err = repo.Pool.QueryRow(ctx, `SELECT count(*) FROM information_schema.columns
		WHERE table_schema='public' AND table_name='human_approval_attestations' AND column_name='approval_grant_id'`).Scan(&columnCount); err != nil || columnCount != 1 {
		repo.Pool.Close()
		t.Fatalf("pre-migration approval_grant_id count=%d err=%v", columnCount, err)
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
	tableName = nil
	if err = repo.Pool.QueryRow(ctx, "SELECT to_regclass('approval_grants')::text").Scan(&tableName); err != nil || tableName != nil {
		t.Fatalf("post-migration approval_grants still exists: %v %v", tableName, err)
	}
	if err = repo.Pool.QueryRow(ctx, `SELECT count(*) FROM information_schema.columns
		WHERE table_schema='public' AND table_name='human_approval_attestations' AND column_name='approval_grant_id'`).Scan(&columnCount); err != nil || columnCount != 0 {
		t.Fatalf("post-migration approval_grant_id count=%d err=%v", columnCount, err)
	}
}
