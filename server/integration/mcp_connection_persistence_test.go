package integration_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jazzcake/baley/server/internal/persistence/postgres"
)

func TestMCPConnectionRequestsSurviveRepositoryRestartAndConsumeAtomically(t *testing.T) {
	url := os.Getenv("BALEY_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("BALEY_TEST_DATABASE_URL is not set")
	}
	requireDisposableDatabase(t, url)
	if err := postgres.Migrate(url, filepath.Join("..", "migrations"), "up"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BALEY_LEASE_TOKEN_SECRET", "mcp-connection-persistence-test-secret")
	ctx := context.Background()
	repo, err := postgres.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Pool.Close()
	if _, err = repo.Pool.Exec(ctx, "SET session_replication_role='replica'; TRUNCATE security_events,agent_tokens,mcp_connection_requests,workspace_memberships,account_sessions,account_credentials,accounts,events,human_approval_attestations,commands,workspace_counters,runs,gate_entry_tasks,gate_tasks,gates,task_dependencies,tasks,lanes,phases,workspaces,actors CASCADE; SET session_replication_role='origin'"); err != nil {
		t.Fatal(err)
	}
	if err = repo.SeedDemo(ctx); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	secret := "restart-safe-secret"
	if _, err = repo.CreateMCPConnection(ctx, "restart-request", postgres.DemoWorkspaceID, postgres.DemoAgentActorID, postgres.DigestSecret(secret), now, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}

	// A fresh repository represents an API process after restart: the request and
	// its hashed secret remain, while no credential material was stored.
	restarted, err := postgres.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Pool.Close()
	request, err := restarted.MCPConnection(ctx, "restart-request", now.Add(time.Minute))
	if err != nil || request.Status != "pending" {
		t.Fatalf("pending request was not recovered: %#v %v", request, err)
	}
	if _, err = restarted.ApproveMCPConnection(ctx, "restart-request", postgres.DemoWorkspaceID, postgres.DemoHumanActorID, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	consumed, token, err := restarted.PollMCPConnectionAndIssueAgentToken(ctx, "restart-request", postgres.DigestSecret(secret), now.Add(2*time.Minute))
	if err != nil || consumed.Status != "consumed" || token == "" {
		t.Fatalf("approved request was not consumed once: %#v token=%q err=%v", consumed, token, err)
	}
	if _, _, err = restarted.PollMCPConnectionAndIssueAgentToken(ctx, "restart-request", postgres.DigestSecret(secret), now.Add(3*time.Minute)); !errors.Is(err, postgres.ErrMCPConnectionConsumed) {
		t.Fatalf("second poll issued another credential: %v", err)
	}
	var storedHash []byte
	if err = restarted.Pool.QueryRow(ctx, "SELECT secret_hash FROM mcp_connection_requests WHERE id='restart-request'").Scan(&storedHash); err != nil {
		t.Fatal(err)
	}
	if string(storedHash) == secret || string(storedHash) == token {
		t.Fatal("connection secret or issued token was persisted in plaintext")
	}

	if _, err = restarted.CreateMCPConnection(ctx, "rejected-request", postgres.DemoWorkspaceID, postgres.DemoAgentActorID, postgres.DigestSecret("reject-secret"), now, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if rejected, err := restarted.RejectMCPConnection(ctx, "rejected-request", postgres.DemoWorkspaceID, postgres.DemoHumanActorID, now.Add(time.Minute)); err != nil || rejected.Status != "rejected" {
		t.Fatalf("rejection did not persist: %#v %v", rejected, err)
	}
	if rejected, token, err := restarted.PollMCPConnectionAndIssueAgentToken(ctx, "rejected-request", postgres.DigestSecret("reject-secret"), now.Add(2*time.Minute)); err != nil || rejected.Status != "rejected" || token != "" {
		t.Fatalf("rejected request poll=%#v token=%q err=%v", rejected, token, err)
	}

	if _, err = restarted.CreateMCPConnection(ctx, "expired-request", postgres.DemoWorkspaceID, postgres.DemoAgentActorID, postgres.DigestSecret("expired-secret"), now, now.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err = restarted.MCPConnection(ctx, "expired-request", now); !errors.Is(err, postgres.ErrMCPConnectionNotFound) {
		t.Fatalf("expired request was recoverable: %v", err)
	}
}
