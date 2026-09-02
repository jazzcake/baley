package integration

import (
	"context"
	"os"
	"testing"

	"github.com/jazzcake/baley/server/internal/persistence/postgres"
)

func TestRepositoryReadinessAgainstPostgres(t *testing.T) {
	url := os.Getenv("BALEY_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("BALEY_TEST_DATABASE_URL is not set")
	}
	t.Setenv("BALEY_LEASE_TOKEN_SECRET", "readiness-integration-secret")
	repo, err := postgres.Open(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Pool.Close()

	version, err := repo.Readiness(context.Background(), 25)
	if err != nil || version != 25 {
		t.Fatalf("Readiness() version=%d err=%v", version, err)
	}
	if _, err = repo.Readiness(context.Background(), 24); err == nil {
		t.Fatal("Readiness accepted an unexpected migration version")
	}
}
