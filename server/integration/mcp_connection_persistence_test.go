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
	if _, err = repo.CreateMCPConnection(ctx, "restart-request", postgres.DemoWorkspaceID, postgres.DemoAgentActorID, "gateway-restart-safe-identity", postgres.DigestSecret(secret), now, now.Add(time.Hour)); err != nil {
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
	consumed, issued, gatewaySecret, err := restarted.PollMCPConnectionAndIssueAgentToken(ctx, "restart-request", postgres.DigestSecret(secret), now.Add(2*time.Minute))
	if err != nil || consumed.Status != "consumed" || issued.Token == "" || gatewaySecret == "" {
		t.Fatalf("approved request was not consumed once: %#v issued=%#v err=%v", consumed, issued, err)
	}
	if _, _, _, err = restarted.PollMCPConnectionAndIssueAgentToken(ctx, "restart-request", postgres.DigestSecret(secret), now.Add(3*time.Minute)); !errors.Is(err, postgres.ErrMCPConnectionConsumed) {
		t.Fatalf("second poll issued another credential: %v", err)
	}
	var storedHash []byte
	if err = restarted.Pool.QueryRow(ctx, "SELECT secret_hash FROM mcp_connection_requests WHERE id='restart-request'").Scan(&storedHash); err != nil {
		t.Fatal(err)
	}
	if string(storedHash) == secret || string(storedHash) == issued.Token || string(storedHash) == gatewaySecret {
		t.Fatal("connection secret or issued token was persisted in plaintext")
	}
	resumed, err := restarted.ResumeMCPGateway(ctx, postgres.DemoWorkspaceID, consumed.GatewayID, gatewaySecret, now.Add(4*time.Minute))
	if err != nil || resumed.Token == "" || resumed.Token == issued.Token {
		t.Fatalf("registered gateway did not receive a fresh session credential: %#v %v", resumed, err)
	}
	if _, err = restarted.AgentByTokenHash(ctx, postgres.DigestSecret(resumed.Token), now.Add(4*time.Minute)); err != nil {
		t.Fatalf("resumed gateway credential was not accepted: %v", err)
	}
	// A reapproved local gateway ID replaces the old registration generation.
	// No credential from the copied/old gateway store may survive that replacement.
	if _, err = restarted.CreateMCPConnection(ctx, "replacement-request", postgres.DemoWorkspaceID, postgres.DemoAgentActorID, consumed.GatewayID, postgres.DigestSecret("replacement-secret"), now, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err = restarted.ApproveMCPConnection(ctx, "replacement-request", postgres.DemoWorkspaceID, postgres.DemoHumanActorID, now.Add(5*time.Minute)); err != nil {
		t.Fatal(err)
	}
	_, replacement, replacementGatewaySecret, err := restarted.PollMCPConnectionAndIssueAgentToken(ctx, "replacement-request", postgres.DigestSecret("replacement-secret"), now.Add(6*time.Minute))
	if err != nil || replacement.Token == "" || replacementGatewaySecret == "" {
		t.Fatalf("replacement gateway enrollment failed: %#v %v", replacement, err)
	}
	if _, err = restarted.AgentByTokenHash(ctx, postgres.DigestSecret(resumed.Token), now.Add(6*time.Minute)); err == nil {
		t.Fatal("replaced gateway left its previous session credential valid")
	}
	if _, err = restarted.ResumeMCPGateway(ctx, postgres.DemoWorkspaceID, consumed.GatewayID, gatewaySecret, now.Add(6*time.Minute)); !errors.Is(err, postgres.ErrMCPGatewaySecret) {
		t.Fatalf("old gateway secret resumed after replacement: %v", err)
	}
	if err = restarted.RevokeMCPGateway(ctx, postgres.DemoWorkspaceID, consumed.GatewayID, postgres.DemoHumanActorID, "suspected_compromise", now.Add(7*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err = restarted.AgentByTokenHash(ctx, postgres.DigestSecret(replacement.Token), now.Add(7*time.Minute)); err == nil {
		t.Fatal("gateway revocation did not immediately invalidate its Agent credential")
	}
	if _, err = restarted.ResumeMCPGateway(ctx, postgres.DemoWorkspaceID, consumed.GatewayID, replacementGatewaySecret, now.Add(7*time.Minute)); !errors.Is(err, postgres.ErrMCPGatewayNotFound) {
		t.Fatalf("revoked gateway resumed: %v", err)
	}
	if _, err = restarted.CreateMCPConnection(ctx, "membership-change-request", postgres.DemoWorkspaceID, postgres.DemoAgentActorID, "gateway-membership-change", postgres.DigestSecret("membership-change-secret"), now, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err = restarted.ApproveMCPConnection(ctx, "membership-change-request", postgres.DemoWorkspaceID, postgres.DemoHumanActorID, now.Add(8*time.Minute)); err != nil {
		t.Fatal(err)
	}
	_, _, membershipGatewaySecret, err := restarted.PollMCPConnectionAndIssueAgentToken(ctx, "membership-change-request", postgres.DigestSecret("membership-change-secret"), now.Add(9*time.Minute))
	if err != nil || membershipGatewaySecret == "" {
		t.Fatalf("membership-change gateway enrollment failed: %v", err)
	}
	inactive := false
	if err = restarted.UpdateMember(ctx, postgres.DemoWorkspaceID, postgres.DemoAgentActorID, postgres.DemoHumanActorID, nil, &inactive); err != nil {
		t.Fatal(err)
	}
	if _, err = restarted.ResumeMCPGateway(ctx, postgres.DemoWorkspaceID, "gateway-membership-change", membershipGatewaySecret, now.Add(10*time.Minute)); !errors.Is(err, postgres.ErrMCPGatewayNotFound) {
		t.Fatalf("removed Agent membership restored itself through gateway resume: %v", err)
	}

	if _, err = restarted.CreateMCPConnection(ctx, "rejected-request", postgres.DemoWorkspaceID, postgres.DemoAgentActorID, "gateway-rejected-identity", postgres.DigestSecret("reject-secret"), now, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if rejected, err := restarted.RejectMCPConnection(ctx, "rejected-request", postgres.DemoWorkspaceID, postgres.DemoHumanActorID, now.Add(time.Minute)); err != nil || rejected.Status != "rejected" {
		t.Fatalf("rejection did not persist: %#v %v", rejected, err)
	}
	if rejected, issued, _, err := restarted.PollMCPConnectionAndIssueAgentToken(ctx, "rejected-request", postgres.DigestSecret("reject-secret"), now.Add(2*time.Minute)); err != nil || rejected.Status != "rejected" || issued.Token != "" {
		t.Fatalf("rejected request poll=%#v issued=%#v err=%v", rejected, issued, err)
	}

	if _, err = restarted.CreateMCPConnection(ctx, "expired-request", postgres.DemoWorkspaceID, postgres.DemoAgentActorID, "gateway-expired-identity", postgres.DigestSecret("expired-secret"), now, now.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err = restarted.MCPConnection(ctx, "expired-request", now); !errors.Is(err, postgres.ErrMCPConnectionNotFound) {
		t.Fatalf("expired request was recoverable: %v", err)
	}
}
