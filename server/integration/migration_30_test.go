package integration_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jazzcake/baley/server/internal/persistence/postgres"
)

func TestMigration30AllowsConversationalTaskDiscardAction(t *testing.T) {
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
	var definition string
	if err = repo.Pool.QueryRow(ctx, `SELECT pg_get_constraintdef(oid)
		FROM pg_constraint
		WHERE conrelid='conversational_decision_evidence'::regclass
		  AND conname='conversational_decision_evidence_action_check'`).Scan(&definition); err != nil {
		t.Fatal(err)
	}
	if definition == "" || !containsAll(definition, "task.confirm", "task.discard") {
		t.Fatalf("action constraint=%q", definition)
	}
	if err = postgres.Migrate(url, migrations, "down"); err != nil {
		t.Fatal(err)
	}
	if err = repo.Pool.QueryRow(ctx, `SELECT pg_get_constraintdef(oid)
		FROM pg_constraint
		WHERE conrelid='conversational_decision_evidence'::regclass
		  AND conname='conversational_decision_evidence_action_check'`).Scan(&definition); err != nil {
		t.Fatal(err)
	}
	if !containsAll(definition, "task.confirm") || containsAll(definition, "task.discard") {
		t.Fatalf("rolled-back action constraint=%q", definition)
	}
	if err = postgres.Migrate(url, migrations, "up"); err != nil {
		t.Fatal(err)
	}
}

func containsAll(value string, fragments ...string) bool {
	for _, fragment := range fragments {
		if !strings.Contains(value, fragment) {
			return false
		}
	}
	return true
}
