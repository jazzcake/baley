package integration_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jazzcake/baley/server/internal/authz"
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
	// This persistence scenario exercises account-bound gateway enrollment.
	// The generic demo seed intentionally stays pre-account for older scenarios,
	// so make the required Owner/Operator memberships explicit here.
	if _, err = repo.Pool.Exec(ctx, `INSERT INTO accounts(id,actor_id,login_id,normalized_login_id,display_name,status)
		VALUES ('00000000-0000-4000-8000-000000000010',$1,'demo-owner','demo-owner','Demo Owner','active')`, postgres.DemoHumanActorID); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Pool.Exec(ctx, `INSERT INTO workspace_memberships(workspace_id,actor_id,role,active,created_by_actor_id)
		VALUES ($1,$2,'owner',true,$2),($1,$3,'operator',true,$2)`, postgres.DemoWorkspaceID, postgres.DemoHumanActorID, postgres.DemoAgentActorID); err != nil {
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
	if _, err = restarted.LinkMCPConnection(ctx, "restart-request", postgres.DemoWorkspaceID, postgres.DemoHumanActorID, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	consumed, issued, gatewaySecret, err := restarted.PollMCPLoginLinkAndIssueAgentToken(ctx, "restart-request", postgres.DigestSecret(secret), now.Add(2*time.Minute))
	if err != nil || consumed.Status != "consumed" || issued.Token == "" || gatewaySecret == "" {
		t.Fatalf("linked request was not consumed once: %#v issued=%#v err=%v", consumed, issued, err)
	}
	if _, _, _, err = restarted.PollMCPLoginLinkAndIssueAgentToken(ctx, "restart-request", postgres.DigestSecret(secret), now.Add(3*time.Minute)); !errors.Is(err, postgres.ErrMCPConnectionConsumed) {
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
	// Codex runs one stdio MCP process per client session. A second process
	// renewing through the same device gateway must not invalidate the first
	// process and force it into a new browser login flow.
	if _, err = restarted.AgentByTokenHash(ctx, postgres.DigestSecret(issued.Token), now.Add(4*time.Minute)); err != nil {
		t.Fatalf("gateway renewal invalidated an existing live MCP session: %v", err)
	}
	// Once a device has a Keychain-held registration for this signed-in Account,
	// a second Workspace with active membership is enrolled without presenting a
	// separate browser login link. The proof remains a registration
	// secret, not merely the copied gateway ID.
	const autoWorkspaceID = "00000000-0000-4000-8000-000000000099"
	if _, err = restarted.Pool.Exec(ctx, `INSERT INTO workspaces(id,name,state,revision) VALUES($1,'Auto enrollment target','active',1)`, autoWorkspaceID); err != nil {
		t.Fatal(err)
	}
	if _, err = restarted.Pool.Exec(ctx, `INSERT INTO workspace_memberships(workspace_id,actor_id,role,active,created_by_actor_id) VALUES($1,$2,'owner',true,$2),($1,$3,'operator',true,$2)`, autoWorkspaceID, postgres.DemoHumanActorID, postgres.DemoAgentActorID); err != nil {
		t.Fatal(err)
	}
	auto, err := restarted.AutoEnrollMCPGateway(ctx, autoWorkspaceID, consumed.GatewayID, postgres.DemoWorkspaceID, gatewaySecret, now.Add(4*time.Minute))
	if err != nil || auto.AgentToken == "" || auto.GatewaySecret == "" {
		t.Fatalf("registered device did not auto-enroll active member Workspace: %#v %v", auto, err)
	}
	if _, err = restarted.AgentByTokenHash(ctx, postgres.DigestSecret(auto.AgentToken), now.Add(4*time.Minute)); err != nil {
		t.Fatalf("auto-enrolled Workspace credential was not accepted: %v", err)
	}
	if _, err = restarted.AutoEnrollMCPGateway(ctx, "00000000-0000-4000-8000-000000000098", consumed.GatewayID, postgres.DemoWorkspaceID, gatewaySecret, now.Add(4*time.Minute)); !errors.Is(err, postgres.ErrMCPGatewayNotFound) {
		t.Fatalf("gateway enrolled a Workspace without active membership: %v", err)
	}
	for _, target := range []struct {
		id   string
		role string
	}{
		{id: "00000000-0000-4000-8000-000000000097", role: "viewer"},
		{id: "00000000-0000-4000-8000-000000000096", role: "approver"},
	} {
		const otherOwnerActorID = "00000000-0000-4000-8000-000000000012"
		if _, err = restarted.Pool.Exec(ctx, `INSERT INTO actors(id,display_name,actor_type) VALUES($1,'Other Owner','human') ON CONFLICT DO NOTHING`, otherOwnerActorID); err != nil {
			t.Fatal(err)
		}
		if _, err = restarted.Pool.Exec(ctx, `INSERT INTO accounts(id,actor_id,login_id,normalized_login_id,display_name,status)
			VALUES ('00000000-0000-4000-8000-000000000013',$1,'other-owner','other-owner','Other Owner','active') ON CONFLICT DO NOTHING`, otherOwnerActorID); err != nil {
			t.Fatal(err)
		}
		if _, err = restarted.Pool.Exec(ctx, `INSERT INTO workspaces(id,name,state,revision) VALUES($1,'Non-operating member target','active',1)`, target.id); err != nil {
			t.Fatal(err)
		}
		if _, err = restarted.Pool.Exec(ctx, `INSERT INTO workspace_memberships(workspace_id,actor_id,role,active,created_by_actor_id) VALUES($1,$2,'owner',true,$2),($1,$3,$4,true,$2)`, target.id, otherOwnerActorID, postgres.DemoHumanActorID, target.role); err != nil {
			t.Fatal(err)
		}
		readonly, err := restarted.AutoEnrollMCPGateway(ctx, target.id, consumed.GatewayID, postgres.DemoWorkspaceID, gatewaySecret, now.Add(4*time.Minute))
		if err != nil || readonly.AgentToken == "" {
			t.Fatalf("%s member did not receive read-only MCP access: %#v %v", target.role, readonly, err)
		}
		record, err := restarted.AgentByTokenHash(ctx, postgres.DigestSecret(readonly.AgentToken), now.Add(4*time.Minute))
		if err != nil || len(record.Scopes) != 1 || record.Scopes[0] != authz.WorkspaceRead {
			t.Fatalf("%s member scopes=%v err=%v, want workspace:read only", target.role, record.Scopes, err)
		}
	}
	// A newly linked local gateway ID replaces the old registration generation.
	// No credential from the copied/old gateway store may survive that replacement.
	if _, err = restarted.CreateMCPConnection(ctx, "replacement-request", postgres.DemoWorkspaceID, postgres.DemoAgentActorID, consumed.GatewayID, postgres.DigestSecret("replacement-secret"), now, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err = restarted.LinkMCPConnection(ctx, "replacement-request", postgres.DemoWorkspaceID, postgres.DemoHumanActorID, now.Add(5*time.Minute)); err != nil {
		t.Fatal(err)
	}
	_, replacement, replacementGatewaySecret, err := restarted.PollMCPLoginLinkAndIssueAgentToken(ctx, "replacement-request", postgres.DigestSecret("replacement-secret"), now.Add(6*time.Minute))
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
	if _, err = restarted.LinkMCPConnection(ctx, "membership-change-request", postgres.DemoWorkspaceID, postgres.DemoHumanActorID, now.Add(8*time.Minute)); err != nil {
		t.Fatal(err)
	}
	_, _, membershipGatewaySecret, err := restarted.PollMCPLoginLinkAndIssueAgentToken(ctx, "membership-change-request", postgres.DigestSecret("membership-change-secret"), now.Add(9*time.Minute))
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

	if _, err = restarted.CreateMCPConnection(ctx, "logout-request", postgres.DemoWorkspaceID, postgres.DemoAgentActorID, "gateway-logout", postgres.DigestSecret("logout-secret"), now, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err = restarted.LinkMCPConnection(ctx, "logout-request", postgres.DemoWorkspaceID, postgres.DemoHumanActorID, now.Add(11*time.Minute)); err != nil {
		t.Fatal(err)
	}
	_, logoutToken, logoutGatewaySecret, err := restarted.PollMCPLoginLinkAndIssueAgentToken(ctx, "logout-request", postgres.DigestSecret("logout-secret"), now.Add(12*time.Minute))
	if err != nil || logoutToken.Token == "" || logoutGatewaySecret == "" {
		t.Fatalf("logout gateway enrollment failed: %#v %v", logoutToken, err)
	}
	if _, err = restarted.Pool.Exec(ctx, `INSERT INTO account_sessions(id,account_id,token_hash,csrf_token_hash,created_at,last_seen_at,idle_expires_at,absolute_expires_at)
		VALUES ('00000000-0000-4000-8000-000000000011','00000000-0000-4000-8000-000000000010',
		decode(repeat('01',32),'hex'),decode(repeat('02',32),'hex'),$1,$1,$2,$2)`, now, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err = restarted.RevokeSession(ctx, "00000000-0000-4000-8000-000000000011", now.Add(13*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err = restarted.AgentByTokenHash(ctx, postgres.DigestSecret(logoutToken.Token), now.Add(13*time.Minute)); err == nil {
		t.Fatal("logout left its gateway Agent credential valid")
	}
	if _, err = restarted.ResumeMCPGateway(ctx, postgres.DemoWorkspaceID, "gateway-logout", logoutGatewaySecret, now.Add(13*time.Minute)); !errors.Is(err, postgres.ErrMCPGatewayNotFound) {
		t.Fatalf("logout gateway resumed: %v", err)
	}

	if _, err = restarted.CreateMCPConnection(ctx, "abandoned-request", postgres.DemoWorkspaceID, postgres.DemoAgentActorID, "gateway-abandoned-identity", postgres.DigestSecret("abandoned-secret"), now, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if pending, issued, _, err := restarted.PollMCPLoginLinkAndIssueAgentToken(ctx, "abandoned-request", postgres.DigestSecret("abandoned-secret"), now.Add(2*time.Minute)); err != nil || pending.Status != "pending" || issued.Token != "" {
		t.Fatalf("abandoned request poll=%#v issued=%#v err=%v", pending, issued, err)
	}

	if _, err = restarted.CreateMCPConnection(ctx, "expired-request", postgres.DemoWorkspaceID, postgres.DemoAgentActorID, "gateway-expired-identity", postgres.DigestSecret("expired-secret"), now.Add(-2*time.Minute), now.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err = restarted.MCPConnection(ctx, "expired-request", now); !errors.Is(err, postgres.ErrMCPConnectionNotFound) {
		t.Fatalf("expired request was recoverable: %v", err)
	}
}
