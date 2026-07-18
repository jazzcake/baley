package main

import (
	"context"
	"github.com/jazzcake/baley/server/internal/persistence/postgres"
	"log"
	"os"
)

func main() {
	url := os.Getenv("BALEY_DATABASE_URL")
	if url == "" {
		url = "postgres://baley:baley@127.0.0.1:54329/baley?sslmode=disable"
	}
	ctx := context.Background()
	repo, err := postgres.Open(ctx, url)
	if err != nil {
		log.Fatal(err)
	}
	defer repo.Pool.Close()
	if err = repo.SeedDemo(ctx); err != nil {
		log.Fatal(err)
	}
	log.Printf("seeded workspace %s", postgres.DemoWorkspaceID)
}
