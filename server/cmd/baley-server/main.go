package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/jazzcake/baley/server/internal/application"
	"github.com/jazzcake/baley/server/internal/persistence/postgres"
	"github.com/jazzcake/baley/server/internal/transport/httpapi"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: baley-server migrate [up|down] | serve")
	}
	dbURL := env("BALEY_DATABASE_URL", "postgres://baley:baley@127.0.0.1:54329/baley?sslmode=disable")
	if os.Args[1] == "migrate" {
		direction := "up"
		if len(os.Args) > 2 {
			direction = os.Args[2]
		}
		dir := env("BALEY_MIGRATIONS_DIR", filepath.Join("migrations"))
		if err := postgres.Migrate(dbURL, dir, direction); err != nil {
			log.Fatal(err)
		}
		return
	}
	if os.Args[1] != "serve" {
		log.Fatal("unknown command")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	repo, err := postgres.Open(ctx, dbURL)
	if err != nil {
		log.Fatal(err)
	}
	defer repo.Pool.Close()
	addr := env("BALEY_HTTP_ADDR", "127.0.0.1:8080")
	host, _, err := net.SplitHostPort(addr)
	if err != nil || !(host == "127.0.0.1" || host == "localhost" || host == "::1") {
		log.Fatal("BALEY_HTTP_ADDR must bind to loopback")
	}
	api := &httpapi.API{Service: application.NewService(repo), Repo: repo, AllowedOrigin: env("BALEY_VIEWER_ORIGIN", "http://127.0.0.1:5173")}
	server := &http.Server{Addr: addr, Handler: api.Handler(), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		shutdownCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		_ = server.Shutdown(shutdownCtx)
	}()
	log.Printf("Baley server listening on http://%s", addr)
	if err = server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
