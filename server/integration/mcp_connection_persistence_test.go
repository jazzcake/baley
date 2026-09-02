package integration_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
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
	const browserSessionID = "00000000-0000-4000-8000-000000000011"
	if _, err = repo.Pool.Exec(ctx, `INSERT INTO account_sessions(id,account_id,token_hash,csrf_token_hash,created_at,last_seen_at,idle_expires_at,absolute_expires_at)
		VALUES ($1,'00000000-0000-4000-8000-000000000010',
		decode(repeat('01',32),'hex'),decode(repeat('02',32),'hex'),$2,$2,$3,$3)`, browserSessionID, now, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	const otherActorID = "00000000-0000-4000-8000-000000000012"
	const otherAccountID = "00000000-0000-4000-8000-000000000013"
	const otherSessionID = "00000000-0000-4000-8000-000000000014"
	const expiredSessionID = "00000000-0000-4000-8000-000000000015"
	if _, err = repo.Pool.Exec(ctx, `INSERT INTO actors(id,display_name,actor_type) VALUES($1,'Other Owner','human')`, otherActorID); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Pool.Exec(ctx, `INSERT INTO accounts(id,actor_id,login_id,normalized_login_id,display_name,status)
		VALUES ($1,$2,'other-owner','other-owner','Other Owner','active')`, otherAccountID, otherActorID); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Pool.Exec(ctx, `INSERT INTO account_sessions(id,account_id,token_hash,csrf_token_hash,created_at,last_seen_at,idle_expires_at,absolute_expires_at)
		VALUES ($1,$2,$3,$4,$5,$5,$6,$6),($7,'00000000-0000-4000-8000-000000000010',$8,$9,$5,$5,$10,$10)`,
		otherSessionID, otherAccountID, postgres.DigestSecret("other-session-token"), postgres.DigestSecret("other-session-csrf"), now,
		now.Add(time.Hour), expiredSessionID, postgres.DigestSecret("expired-session-token"), postgres.DigestSecret("expired-session-csrf"), now.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []struct {
		requestID string
		sessionID string
	}{
		{requestID: "wrong-actor-session-request", sessionID: otherSessionID},
		{requestID: "expired-session-request", sessionID: expiredSessionID},
	} {
		if _, err = repo.CreateMCPConnection(ctx, invalid.requestID, postgres.DemoWorkspaceID, postgres.DemoAgentActorID, "gateway-"+invalid.requestID, postgres.DigestSecret("secret-"+invalid.requestID), now, now.Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
		if _, err = repo.BeginMCPLoginLink(ctx, invalid.requestID, postgres.DemoWorkspaceID, postgres.DemoHumanActorID, invalid.sessionID, postgres.DigestSecret("code-"+invalid.requestID), now, now.Add(time.Minute)); !errors.Is(err, postgres.ErrMCPLoginSession) {
			t.Fatalf("invalid browser session %s bound an MCP link: %v", invalid.sessionID, err)
		}
	}
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
	if _, err = restarted.BeginMCPLoginLink(ctx, "restart-request", postgres.DemoWorkspaceID, postgres.DemoHumanActorID, browserSessionID, postgres.DigestSecret("login-code"), now.Add(time.Minute), now.Add(3*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err = restarted.RedeemMCPLoginLinkAndIssueAgentToken(ctx, "restart-request", postgres.DigestSecret(secret), postgres.DigestSecret("stolen-code"), now.Add(90*time.Second)); !errors.Is(err, postgres.ErrMCPLoginCode) {
		t.Fatalf("connection secret alone redeemed browser login: %v", err)
	}
	consumed, issued, gatewaySecret, err := restarted.RedeemMCPLoginLinkAndIssueAgentToken(ctx, "restart-request", postgres.DigestSecret(secret), postgres.DigestSecret("login-code"), now.Add(2*time.Minute))
	if err != nil || consumed.Status != "consumed" || issued.Token == "" || gatewaySecret == "" {
		t.Fatalf("linked request was not consumed once: %#v issued=%#v err=%v", consumed, issued, err)
	}
	if _, err = restarted.PollMCPLoginLink(ctx, "restart-request", postgres.DigestSecret(secret), now.Add(3*time.Minute)); !errors.Is(err, postgres.ErrMCPConnectionConsumed) {
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
	// Multiple clients sharing the single local Gateway may hold concurrent
	// session credentials. Renewing one must not invalidate the others.
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
		if _, err = restarted.Pool.Exec(ctx, `INSERT INTO workspaces(id,name,state,revision) VALUES($1,'Non-operating member target','active',1)`, target.id); err != nil {
			t.Fatal(err)
		}
		if _, err = restarted.Pool.Exec(ctx, `INSERT INTO workspace_memberships(workspace_id,actor_id,role,active,created_by_actor_id) VALUES($1,$2,'owner',true,$2),($1,$3,$4,true,$2)`, target.id, otherActorID, postgres.DemoHumanActorID, target.role); err != nil {
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
	const removedMemberWorkspaceID = "00000000-0000-4000-8000-000000000095"
	if _, err = restarted.Pool.Exec(ctx, `INSERT INTO workspaces(id,name,state,revision) VALUES($1,'Removed member target','active',1)`, removedMemberWorkspaceID); err != nil {
		t.Fatal(err)
	}
	if _, err = restarted.Pool.Exec(ctx, `INSERT INTO workspace_memberships(workspace_id,actor_id,role,active,created_by_actor_id)
		VALUES($1,$2,'owner',true,$2),($1,$3,'operator',true,$2),($1,$4,'operator',true,$2)`, removedMemberWorkspaceID, otherActorID, postgres.DemoHumanActorID, postgres.DemoAgentActorID); err != nil {
		t.Fatal(err)
	}
	if _, err = restarted.CreateMCPConnection(ctx, "removed-member-request", removedMemberWorkspaceID, postgres.DemoAgentActorID, "gateway-removed-member", postgres.DigestSecret("removed-member-secret"), now, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err = restarted.BeginMCPLoginLink(ctx, "removed-member-request", removedMemberWorkspaceID, postgres.DemoHumanActorID, browserSessionID, postgres.DigestSecret("removed-member-code"), now.Add(4*time.Minute), now.Add(6*time.Minute)); err != nil {
		t.Fatal(err)
	}
	removed := false
	if err = restarted.UpdateMember(ctx, removedMemberWorkspaceID, postgres.DemoHumanActorID, otherActorID, nil, &removed); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err = restarted.RedeemMCPLoginLinkAndIssueAgentToken(ctx, "removed-member-request", postgres.DigestSecret("removed-member-secret"), postgres.DigestSecret("removed-member-code"), now.Add(5*time.Minute)); !errors.Is(err, postgres.ErrMCPGatewayNotFound) {
		t.Fatalf("removed human member redeemed a pending MCP link: %v", err)
	}
	// A newly linked local gateway ID replaces the old registration generation.
	// No credential from the copied/old gateway store may survive that replacement.
	if _, err = restarted.CreateMCPConnection(ctx, "replacement-request", postgres.DemoWorkspaceID, postgres.DemoAgentActorID, consumed.GatewayID, postgres.DigestSecret("replacement-secret"), now, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err = restarted.BeginMCPLoginLink(ctx, "replacement-request", postgres.DemoWorkspaceID, postgres.DemoHumanActorID, browserSessionID, postgres.DigestSecret("replacement-code"), now.Add(5*time.Minute), now.Add(7*time.Minute)); err != nil {
		t.Fatal(err)
	}
	_, replacement, replacementGatewaySecret, err := restarted.RedeemMCPLoginLinkAndIssueAgentToken(ctx, "replacement-request", postgres.DigestSecret("replacement-secret"), postgres.DigestSecret("replacement-code"), now.Add(6*time.Minute))
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
	if _, err = restarted.BeginMCPLoginLink(ctx, "membership-change-request", postgres.DemoWorkspaceID, postgres.DemoHumanActorID, browserSessionID, postgres.DigestSecret("membership-code"), now.Add(8*time.Minute), now.Add(10*time.Minute)); err != nil {
		t.Fatal(err)
	}
	_, _, membershipGatewaySecret, err := restarted.RedeemMCPLoginLinkAndIssueAgentToken(ctx, "membership-change-request", postgres.DigestSecret("membership-change-secret"), postgres.DigestSecret("membership-code"), now.Add(9*time.Minute))
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
	if _, err = restarted.BeginMCPLoginLink(ctx, "logout-request", postgres.DemoWorkspaceID, postgres.DemoHumanActorID, browserSessionID, postgres.DigestSecret("logout-code"), now.Add(11*time.Minute), now.Add(13*time.Minute)); err != nil {
		t.Fatal(err)
	}
	_, logoutToken, logoutGatewaySecret, err := restarted.RedeemMCPLoginLinkAndIssueAgentToken(ctx, "logout-request", postgres.DigestSecret("logout-secret"), postgres.DigestSecret("logout-code"), now.Add(12*time.Minute))
	if err != nil || logoutToken.Token == "" || logoutGatewaySecret == "" {
		t.Fatalf("logout gateway enrollment failed: %#v %v", logoutToken, err)
	}
	if _, err = restarted.CreateMCPConnection(ctx, "logout-pending-request", postgres.DemoWorkspaceID, postgres.DemoAgentActorID, "gateway-logout-pending", postgres.DigestSecret("logout-pending-secret"), now, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err = restarted.BeginMCPLoginLink(ctx, "logout-pending-request", postgres.DemoWorkspaceID, postgres.DemoHumanActorID, browserSessionID, postgres.DigestSecret("logout-pending-code"), now.Add(12*time.Minute), now.Add(14*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err = restarted.RevokeSession(ctx, browserSessionID, now.Add(13*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err = restarted.RedeemMCPLoginLinkAndIssueAgentToken(ctx, "logout-pending-request", postgres.DigestSecret("logout-pending-secret"), postgres.DigestSecret("logout-pending-code"), now.Add(13*time.Minute)); !errors.Is(err, postgres.ErrMCPConnectionNotFound) && !errors.Is(err, postgres.ErrMCPLoginSession) {
		t.Fatalf("logout allowed a pending browser link to issue a new gateway credential: %v", err)
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
	if pending, err := restarted.PollMCPLoginLink(ctx, "abandoned-request", postgres.DigestSecret("abandoned-secret"), now.Add(2*time.Minute)); err != nil || pending.Status != "pending" {
		t.Fatalf("abandoned request poll=%#v err=%v", pending, err)
	}

	if _, err = restarted.CreateMCPConnection(ctx, "expired-request", postgres.DemoWorkspaceID, postgres.DemoAgentActorID, "gateway-expired-identity", postgres.DigestSecret("expired-secret"), now.Add(-2*time.Minute), now.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err = restarted.MCPConnection(ctx, "expired-request", now); !errors.Is(err, postgres.ErrMCPConnectionNotFound) {
		t.Fatalf("expired request was recoverable: %v", err)
	}

	// Exercise the exact logout/callback race repeatedly. Both transactions use
	// request-before-session locking, so neither may be selected as a PostgreSQL
	// deadlock victim. If redeem wins, the completed logout must still revoke
	// the newly issued Agent credential before either goroutine is observed here.
	for i := 0; i < 8; i++ {
		sessionID := fmt.Sprintf("10000000-0000-4000-8000-%012d", i)
		requestID := fmt.Sprintf("logout-race-request-%d", i)
		secret, code := "logout-race-secret-"+requestID, "logout-race-code-"+requestID
		if _, err = restarted.Pool.Exec(ctx, `INSERT INTO account_sessions(id,account_id,token_hash,csrf_token_hash,created_at,last_seen_at,idle_expires_at,absolute_expires_at)
			VALUES($1,'00000000-0000-4000-8000-000000000010',$2,$3,$4,$4,$5,$5)`, sessionID,
			postgres.DigestSecret("token-"+sessionID), postgres.DigestSecret("csrf-"+sessionID), now, now.Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
		if _, err = restarted.CreateMCPConnection(ctx, requestID, postgres.DemoWorkspaceID, postgres.DemoAgentActorID, "gateway-"+requestID, postgres.DigestSecret(secret), now, now.Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
		if _, err = restarted.BeginMCPLoginLink(ctx, requestID, postgres.DemoWorkspaceID, postgres.DemoHumanActorID, sessionID, postgres.DigestSecret(code), now, now.Add(time.Hour)); err != nil {
			t.Fatal(err)
		}

		start := make(chan struct{})
		var wait sync.WaitGroup
		var logoutErr, redeemErr error
		var raceToken string
		wait.Add(2)
		go func() {
			defer wait.Done()
			<-start
			logoutErr = restarted.RevokeSession(ctx, sessionID, now.Add(20*time.Minute))
		}()
		go func() {
			defer wait.Done()
			<-start
			_, issued, _, redeem := restarted.RedeemMCPLoginLinkAndIssueAgentToken(ctx, requestID, postgres.DigestSecret(secret), postgres.DigestSecret(code), now.Add(20*time.Minute))
			redeemErr, raceToken = redeem, issued.Token
		}()
		close(start)
		wait.Wait()
		if logoutErr != nil {
			t.Fatalf("concurrent logout %d failed: %v", i, logoutErr)
		}
		if redeemErr != nil && !errors.Is(redeemErr, postgres.ErrMCPConnectionNotFound) && !errors.Is(redeemErr, postgres.ErrMCPLoginSession) {
			t.Fatalf("concurrent redeem %d failed unexpectedly: %v", i, redeemErr)
		}
		if raceToken != "" {
			if _, err = restarted.AgentByTokenHash(ctx, postgres.DigestSecret(raceToken), now.Add(20*time.Minute)); err == nil {
				t.Fatalf("concurrent logout %d left the redeemed Agent credential active", i)
			}
		}
	}
}
