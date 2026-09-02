package postgres

import (
	"context"
	"crypto/subtle"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jazzcake/baley/server/internal/authz"
)

var (
	ErrMCPGatewayNotFound = errors.New("MCP gateway registration not found")
	ErrMCPGatewaySecret   = errors.New("MCP gateway credential mismatch")
)

// MCPGatewayRegistration is the durable, Workspace-scoped relationship between
// a human member and one local MCP gateway. Its secret is returned only at the
// signed-in login hand-off; only its hash is retained by the server.
type MCPGatewayRegistration struct {
	ID, WorkspaceID, AccountActorID, AgentActorID, GatewayID, Status string
	SecretHash                                                       []byte
	Generation                                                       int
	CreatedAt                                                        time.Time
	RevokedAt                                                        *time.Time
}

// MCPGatewayEnrollment is returned only to a local Gateway that proves it
// already holds an active device credential for the same Account.
type MCPGatewayEnrollment struct {
	AgentToken    string
	GatewaySecret string
}

func (r *Repository) enrollMCPGatewayTx(ctx context.Context, tx pgx.Tx, workspaceID, accountActorID, agentActorID, gatewayID string, now time.Time) (MCPGatewayRegistration, string, error) {
	if strings.TrimSpace(gatewayID) == "" {
		return MCPGatewayRegistration{}, "", errors.New("MCP gateway identity is required")
	}
	if _, err := memberAgentScopesTx(ctx, tx, workspaceID, accountActorID); err != nil {
		return MCPGatewayRegistration{}, "", ErrMCPGatewayNotFound
	}
	// Serialize replacement with ResumeMCPGateway.  Without this lock, a resume
	// can issue a token after the replacement revokes old tokens but before the
	// registration's secret is rotated, leaving an old-secret token usable.
	var existingRegistrationID string
	err := tx.QueryRow(ctx, `SELECT id FROM mcp_gateway_registrations
		WHERE workspace_id=$1 AND gateway_id=$2 FOR UPDATE`, workspaceID, gatewayID).Scan(&existingRegistrationID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return MCPGatewayRegistration{}, "", err
	}
	secret, secretHash, err := randomOpaqueSecret()
	if err != nil {
		return MCPGatewayRegistration{}, "", err
	}
	registrationID, err := randomUUIDString()
	if err != nil {
		return MCPGatewayRegistration{}, "", err
	}
	// A newly linked gateway replaces prior gateways for that member in this
	// Workspace. Reusing an existing gateway ID also rotates its secret and
	// invalidates every previously issued Agent token immediately.
	if _, err = tx.Exec(ctx, `UPDATE agent_tokens SET revoked_at=$1
		WHERE revoked_at IS NULL AND gateway_registration_id IN (
			SELECT id FROM mcp_gateway_registrations
			WHERE workspace_id=$2 AND (account_actor_id=$3 OR gateway_id=$4))`, now, workspaceID, accountActorID, gatewayID); err != nil {
		return MCPGatewayRegistration{}, "", err
	}
	if _, err = tx.Exec(ctx, `UPDATE mcp_gateway_registrations
		SET status='replaced',revoked_at=$1,revoked_by_actor_id=$2,revoke_reason='gateway_replaced'
		WHERE workspace_id=$3 AND account_actor_id=$2 AND status='active' AND gateway_id<>$4`, now, accountActorID, workspaceID, gatewayID); err != nil {
		return MCPGatewayRegistration{}, "", err
	}
	var value MCPGatewayRegistration
	err = tx.QueryRow(ctx, `INSERT INTO mcp_gateway_registrations(
		id,workspace_id,account_actor_id,agent_actor_id,gateway_id,gateway_secret_hash,status,generation,created_at)
		VALUES($1,$2,$3,$4,$5,$6,'active',1,$7)
		ON CONFLICT(workspace_id,gateway_id) DO UPDATE SET
		  account_actor_id=EXCLUDED.account_actor_id,agent_actor_id=EXCLUDED.agent_actor_id,
		  gateway_secret_hash=EXCLUDED.gateway_secret_hash,status='active',generation=mcp_gateway_registrations.generation+1,
		  created_at=EXCLUDED.created_at,revoked_at=NULL,revoked_by_actor_id=NULL,revoke_reason=NULL
		RETURNING id,workspace_id,account_actor_id,agent_actor_id,gateway_id,gateway_secret_hash,status,generation,created_at,revoked_at`,
		registrationID, workspaceID, accountActorID, agentActorID, gatewayID, secretHash, now).
		Scan(&value.ID, &value.WorkspaceID, &value.AccountActorID, &value.AgentActorID, &value.GatewayID, &value.SecretHash, &value.Status, &value.Generation, &value.CreatedAt, &value.RevokedAt)
	if err != nil {
		return MCPGatewayRegistration{}, "", err
	}
	if err = insertSecurityEvent(ctx, tx, workspaceID, "", accountActorID, "mcp_gateway.registered", "mcp_gateway", value.ID, map[string]any{"gatewayId": gatewayID, "generation": value.Generation}); err != nil {
		return MCPGatewayRegistration{}, "", err
	}
	return value, secret, nil
}

func agentScopesForMemberRole(role authz.Role) []authz.Capability {
	agentAllowed := map[authz.Capability]bool{}
	for _, capability := range authz.DefaultCatalog.Roles[authz.RoleOperator] {
		agentAllowed[capability] = true
	}
	var scopes []authz.Capability
	for _, capability := range authz.DefaultCatalog.Roles[role] {
		if agentAllowed[capability] {
			scopes = append(scopes, capability)
		}
	}
	return scopes
}

func memberAgentScopesTx(ctx context.Context, tx pgx.Tx, workspaceID, accountActorID string) ([]authz.Capability, error) {
	var role authz.Role
	err := tx.QueryRow(ctx, `SELECT membership.role
		FROM workspace_memberships membership
		JOIN workspaces workspace ON workspace.id=membership.workspace_id AND workspace.state='active'
		JOIN accounts account ON account.actor_id=membership.actor_id AND account.status='active'
		JOIN actors actor ON actor.id=membership.actor_id AND actor.actor_type='human'
		WHERE membership.workspace_id=$1 AND membership.actor_id=$2 AND membership.active FOR UPDATE`, workspaceID, accountActorID).Scan(&role)
	if err != nil {
		return nil, err
	}
	scopes := agentScopesForMemberRole(role)
	if len(scopes) == 0 {
		return nil, ErrMCPGatewayNotFound
	}
	return scopes, nil
}

func (r *Repository) ResumeMCPGateway(ctx context.Context, workspaceID, gatewayID, secret string, now time.Time) (AgentTokenResult, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return AgentTokenResult{}, err
	}
	defer tx.Rollback(ctx)
	var registration MCPGatewayRegistration
	var memberRole authz.Role
	err = tx.QueryRow(ctx, `SELECT registration.id,registration.workspace_id,registration.account_actor_id,registration.agent_actor_id,
		registration.gateway_id,registration.gateway_secret_hash,registration.status,registration.generation,registration.created_at,registration.revoked_at
		,membership.role
		FROM mcp_gateway_registrations registration
		JOIN workspaces workspace ON workspace.id=registration.workspace_id AND workspace.state='active'
		JOIN workspace_memberships membership ON membership.workspace_id=registration.workspace_id
		  AND membership.actor_id=registration.account_actor_id AND membership.active
		JOIN accounts account ON account.actor_id=registration.account_actor_id AND account.status='active'
		WHERE registration.workspace_id=$1 AND registration.gateway_id=$2 AND registration.status='active' FOR UPDATE`, workspaceID, gatewayID).
		Scan(&registration.ID, &registration.WorkspaceID, &registration.AccountActorID, &registration.AgentActorID, &registration.GatewayID, &registration.SecretHash, &registration.Status, &registration.Generation, &registration.CreatedAt, &registration.RevokedAt, &memberRole)
	if errors.Is(err, pgx.ErrNoRows) {
		return AgentTokenResult{}, ErrMCPGatewayNotFound
	}
	if err != nil {
		return AgentTokenResult{}, err
	}
	if subtle.ConstantTimeCompare(DigestSecret(secret), registration.SecretHash) != 1 {
		return AgentTokenResult{}, ErrMCPGatewaySecret
	}
	// A local gateway secret is device-scoped, while multiple Codex clients share
	// one local Gateway. Renewing a session must therefore not revoke tokens held
	// by the other live clients on the same registered device:
	// doing so makes their next request look like a lost gateway credential and
	// incorrectly sends the user back through browser login. Gateway
	// replacement, explicit revocation, logout, membership changes, and archive
	// still revoke every token associated with this registration.
	issuanceID, err := randomUUIDString()
	if err != nil {
		return AgentTokenResult{}, err
	}
	issued, err := r.issueAgentTokenTx(ctx, tx, workspaceID, registration.AgentActorID, "mcp-gateway-"+issuanceID, registration.AccountActorID, agentScopesForMemberRole(memberRole), nil, &registration.ID)
	if err != nil {
		return AgentTokenResult{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return AgentTokenResult{}, err
	}
	return issued, nil
}

// AutoEnrollMCPGateway derives the Account from a previously linked local
// Gateway registration and enrolls that same device into a second Workspace
// only when the Account has active membership there. Possession of the
// Keychain-held source secret is required; first-device enrollment is bound to
// the signed-in browser Account.
func (r *Repository) AutoEnrollMCPGateway(ctx context.Context, workspaceID, gatewayID, proofWorkspaceID, proofGatewaySecret string, now time.Time) (MCPGatewayEnrollment, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return MCPGatewayEnrollment{}, err
	}
	defer tx.Rollback(ctx)
	var accountActorID, sourceAgentActorID string
	var proofHash []byte
	err = tx.QueryRow(ctx, `SELECT registration.account_actor_id,registration.agent_actor_id,registration.gateway_secret_hash
		FROM mcp_gateway_registrations registration
		JOIN workspaces workspace ON workspace.id=registration.workspace_id AND workspace.state='active'
		JOIN workspace_memberships membership ON membership.workspace_id=registration.workspace_id
		  AND membership.actor_id=registration.account_actor_id AND membership.active
		JOIN accounts account ON account.actor_id=registration.account_actor_id AND account.status='active'
		WHERE registration.workspace_id=$1 AND registration.gateway_id=$2 AND registration.status='active' FOR UPDATE`, proofWorkspaceID, gatewayID).
		Scan(&accountActorID, &sourceAgentActorID, &proofHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return MCPGatewayEnrollment{}, ErrMCPGatewayNotFound
	}
	if err != nil {
		return MCPGatewayEnrollment{}, err
	}
	if subtle.ConstantTimeCompare(DigestSecret(proofGatewaySecret), proofHash) != 1 {
		return MCPGatewayEnrollment{}, ErrMCPGatewaySecret
	}
	registration, gatewaySecret, err := r.enrollMCPGatewayTx(ctx, tx, workspaceID, accountActorID, sourceAgentActorID, gatewayID, now)
	if err != nil {
		return MCPGatewayEnrollment{}, err
	}
	scopes, err := memberAgentScopesTx(ctx, tx, workspaceID, accountActorID)
	if err != nil {
		return MCPGatewayEnrollment{}, err
	}
	issuanceID, err := randomUUIDString()
	if err != nil {
		return MCPGatewayEnrollment{}, err
	}
	issued, err := r.issueAgentTokenTx(ctx, tx, workspaceID, registration.AgentActorID, "mcp-gateway-auto-"+issuanceID, registration.AccountActorID, scopes, nil, &registration.ID)
	if err != nil {
		return MCPGatewayEnrollment{}, err
	}
	if err = insertSecurityEvent(ctx, tx, workspaceID, "", accountActorID, "mcp_gateway.auto_enrolled", "mcp_gateway", registration.ID, map[string]any{"gatewayId": gatewayID, "proofWorkspaceId": proofWorkspaceID, "generation": registration.Generation}); err != nil {
		return MCPGatewayEnrollment{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return MCPGatewayEnrollment{}, err
	}
	return MCPGatewayEnrollment{AgentToken: issued.Token, GatewaySecret: gatewaySecret}, nil
}

func (r *Repository) RevokeMCPGateway(ctx context.Context, workspaceID, gatewayID, revokedByActorID, reason string, now time.Time) error {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = r.revokeMCPGatewayTx(ctx, tx, workspaceID, gatewayID, revokedByActorID, reason, now); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) revokeMCPGatewaysForMemberTx(ctx context.Context, tx pgx.Tx, workspaceID, actorID, revokedByActorID, reason string, now time.Time) error {
	rows, err := tx.Query(ctx, `SELECT gateway_id FROM mcp_gateway_registrations
		WHERE workspace_id=$1 AND (account_actor_id=$2 OR agent_actor_id=$2) AND status='active' FOR UPDATE`, workspaceID, actorID)
	if err != nil {
		return err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return err
		}
		ids = append(ids, id)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	for _, id := range ids {
		if err = r.revokeMCPGatewayTx(ctx, tx, workspaceID, id, revokedByActorID, reason, now); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) revokeMCPGatewayTx(ctx context.Context, tx pgx.Tx, workspaceID, gatewayID, revokedByActorID, reason string, now time.Time) error {
	var registrationID string
	err := tx.QueryRow(ctx, `UPDATE mcp_gateway_registrations SET status='revoked',revoked_at=$1,revoked_by_actor_id=$2,revoke_reason=$3
		WHERE workspace_id=$4 AND gateway_id=$5 AND status='active' RETURNING id`, now, revokedByActorID, strings.TrimSpace(reason), workspaceID, gatewayID).Scan(&registrationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrMCPGatewayNotFound
	}
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "UPDATE agent_tokens SET revoked_at=$1 WHERE gateway_registration_id=$2 AND revoked_at IS NULL", now, registrationID); err != nil {
		return err
	}
	return insertSecurityEvent(ctx, tx, workspaceID, "", revokedByActorID, "mcp_gateway.revoked", "mcp_gateway", registrationID, map[string]any{"gatewayId": gatewayID, "reason": strings.TrimSpace(reason)})
}
