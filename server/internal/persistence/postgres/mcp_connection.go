package postgres

import (
	"context"
	"crypto/subtle"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

var (
	ErrMCPConnectionNotFound = errors.New("MCP connection request not found")
	ErrMCPConnectionSecret   = errors.New("MCP connection secret mismatch")
	ErrMCPConnectionConsumed = errors.New("MCP connection request already consumed")
	ErrMCPLoginCode          = errors.New("MCP login code mismatch")
	ErrMCPLoginSession       = errors.New("MCP login browser session is no longer active")
)

type MCPConnectionRequest struct {
	ID, WorkspaceID, AgentActorID, GatewayID, Status, LinkedByActorID string
	SecretHash, LoginCodeHash                                         []byte
	LoginActorID, LoginSessionID                                      string
	CreatedAt, ExpiresAt                                              time.Time
	LinkedAt, ConsumedAt, LoginCodeExpiresAt                          *time.Time
}

func (r *Repository) CreateMCPConnection(ctx context.Context, id, workspaceID, agentActorID, gatewayID string, secretHash []byte, now, expiresAt time.Time) (MCPConnectionRequest, error) {
	request := MCPConnectionRequest{ID: id, WorkspaceID: workspaceID, AgentActorID: agentActorID, GatewayID: gatewayID, SecretHash: secretHash, Status: "pending", CreatedAt: now, ExpiresAt: expiresAt}
	_, err := r.Pool.Exec(ctx, `INSERT INTO mcp_connection_requests(id,workspace_id,agent_actor_id,gateway_id,secret_hash,status,created_at,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, request.ID, request.WorkspaceID, request.AgentActorID, request.GatewayID, request.SecretHash, request.Status, request.CreatedAt, request.ExpiresAt)
	return request, err
}

func (r *Repository) MCPConnection(ctx context.Context, id string, now time.Time) (MCPConnectionRequest, error) {
	_, _ = r.Pool.Exec(ctx, "DELETE FROM mcp_connection_requests WHERE expires_at <= $1", now)
	request, err := scanMCPConnection(r.Pool.QueryRow(ctx, mcpConnectionSelect+" WHERE id=$1", id))
	if errors.Is(err, pgx.ErrNoRows) {
		return MCPConnectionRequest{}, ErrMCPConnectionNotFound
	}
	return request, err
}

func (r *Repository) BeginMCPLoginLink(ctx context.Context, id, workspaceID, memberActorID, sessionID string, codeHash []byte, now, codeExpiresAt time.Time) (MCPConnectionRequest, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return MCPConnectionRequest{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "DELETE FROM mcp_connection_requests WHERE expires_at <= $1", now); err != nil {
		return MCPConnectionRequest{}, err
	}
	request, err := scanMCPConnection(tx.QueryRow(ctx, mcpConnectionSelect+" WHERE id=$1 FOR UPDATE", id))
	if errors.Is(err, pgx.ErrNoRows) {
		return MCPConnectionRequest{}, ErrMCPConnectionNotFound
	}
	if err != nil {
		return MCPConnectionRequest{}, err
	}
	if request.WorkspaceID != workspaceID {
		return MCPConnectionRequest{}, ErrMCPConnectionNotFound
	}
	if request.Status != "pending" {
		return MCPConnectionRequest{}, ErrMCPConnectionConsumed
	}
	var activeSessionActorID string
	err = tx.QueryRow(ctx, `SELECT account.actor_id
		FROM account_sessions session
		JOIN accounts account ON account.id=session.account_id AND account.status='active'
		WHERE session.id=$1 AND session.revoked_at IS NULL
		  AND session.idle_expires_at>$2 AND session.absolute_expires_at>$2
		FOR UPDATE OF session`, sessionID, now).Scan(&activeSessionActorID)
	if errors.Is(err, pgx.ErrNoRows) || activeSessionActorID != memberActorID {
		return MCPConnectionRequest{}, ErrMCPLoginSession
	}
	if err != nil {
		return MCPConnectionRequest{}, err
	}
	if _, err = tx.Exec(ctx, "UPDATE mcp_connection_requests SET login_code_hash=$1,login_code_expires_at=$2,login_actor_id=$3,login_session_id=$4 WHERE id=$5", codeHash, codeExpiresAt, memberActorID, sessionID, id); err != nil {
		return MCPConnectionRequest{}, err
	}
	request.LoginCodeHash, request.LoginCodeExpiresAt, request.LoginActorID, request.LoginSessionID = codeHash, &codeExpiresAt, memberActorID, sessionID
	if err = tx.Commit(ctx); err != nil {
		return MCPConnectionRequest{}, err
	}
	return request, nil
}

func (r *Repository) RedeemMCPLoginLinkAndIssueAgentToken(ctx context.Context, id string, secretHash, codeHash []byte, now time.Time) (MCPConnectionRequest, AgentTokenResult, string, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return MCPConnectionRequest{}, AgentTokenResult{}, "", err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "DELETE FROM mcp_connection_requests WHERE expires_at <= $1", now); err != nil {
		return MCPConnectionRequest{}, AgentTokenResult{}, "", err
	}
	request, err := scanMCPConnection(tx.QueryRow(ctx, mcpConnectionSelect+" WHERE id=$1 FOR UPDATE", id))
	if errors.Is(err, pgx.ErrNoRows) {
		return MCPConnectionRequest{}, AgentTokenResult{}, "", ErrMCPConnectionNotFound
	}
	if err != nil {
		return MCPConnectionRequest{}, AgentTokenResult{}, "", err
	}
	if request.Status != "pending" {
		return MCPConnectionRequest{}, AgentTokenResult{}, "", ErrMCPConnectionConsumed
	}
	if subtle.ConstantTimeCompare(secretHash, request.SecretHash) != 1 {
		return MCPConnectionRequest{}, AgentTokenResult{}, "", ErrMCPConnectionSecret
	}
	if request.LoginCodeExpiresAt == nil || !request.LoginCodeExpiresAt.After(now) || subtle.ConstantTimeCompare(codeHash, request.LoginCodeHash) != 1 {
		return MCPConnectionRequest{}, AgentTokenResult{}, "", ErrMCPLoginCode
	}
	var activeSessionActorID string
	err = tx.QueryRow(ctx, `SELECT account.actor_id
		FROM account_sessions session
		JOIN accounts account ON account.id=session.account_id AND account.status='active'
		WHERE session.id=$1 AND session.revoked_at IS NULL
		  AND session.idle_expires_at>$2 AND session.absolute_expires_at>$2
		FOR UPDATE OF session`, request.LoginSessionID, now).Scan(&activeSessionActorID)
	if errors.Is(err, pgx.ErrNoRows) || activeSessionActorID != request.LoginActorID {
		return MCPConnectionRequest{}, AgentTokenResult{}, "", ErrMCPLoginSession
	}
	if err != nil {
		return MCPConnectionRequest{}, AgentTokenResult{}, "", err
	}
	registration, gatewaySecret, err := r.enrollMCPGatewayTx(ctx, tx, request.WorkspaceID, request.LoginActorID, request.AgentActorID, request.GatewayID, now)
	if err != nil {
		return MCPConnectionRequest{}, AgentTokenResult{}, "", err
	}
	scopes, err := memberAgentScopesTx(ctx, tx, request.WorkspaceID, request.LoginActorID)
	if err != nil {
		return MCPConnectionRequest{}, AgentTokenResult{}, "", err
	}
	issued, err := r.issueAgentTokenTx(ctx, tx, request.WorkspaceID, request.AgentActorID, "mcp-login-"+request.ID, request.LoginActorID, scopes, nil, &registration.ID)
	if err != nil {
		return MCPConnectionRequest{}, AgentTokenResult{}, "", err
	}
	if _, err = tx.Exec(ctx, "UPDATE mcp_connection_requests SET status='consumed',linked_by_actor_id=$1,linked_at=$2,consumed_at=$2 WHERE id=$3", request.LoginActorID, now, id); err != nil {
		return MCPConnectionRequest{}, AgentTokenResult{}, "", err
	}
	request.Status, request.LinkedByActorID, request.LinkedAt, request.ConsumedAt = "consumed", request.LoginActorID, &now, &now
	if err = tx.Commit(ctx); err != nil {
		return MCPConnectionRequest{}, AgentTokenResult{}, "", err
	}
	return request, issued, gatewaySecret, nil
}

func (r *Repository) PollMCPLoginLink(ctx context.Context, id string, secretHash []byte, now time.Time) (MCPConnectionRequest, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return MCPConnectionRequest{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "DELETE FROM mcp_connection_requests WHERE expires_at <= $1", now); err != nil {
		return MCPConnectionRequest{}, err
	}
	request, err := scanMCPConnection(tx.QueryRow(ctx, mcpConnectionSelect+" WHERE id=$1 FOR UPDATE", id))
	if errors.Is(err, pgx.ErrNoRows) {
		return MCPConnectionRequest{}, ErrMCPConnectionNotFound
	}
	if err != nil {
		return MCPConnectionRequest{}, err
	}
	if subtle.ConstantTimeCompare(secretHash, request.SecretHash) != 1 {
		return MCPConnectionRequest{}, ErrMCPConnectionSecret
	}
	if request.Status == "consumed" {
		return MCPConnectionRequest{}, ErrMCPConnectionConsumed
	}
	if err = tx.Commit(ctx); err != nil {
		return MCPConnectionRequest{}, err
	}
	return request, nil
}

const mcpConnectionSelect = `SELECT id,workspace_id,agent_actor_id,gateway_id,secret_hash,status,COALESCE(linked_by_actor_id,''),created_at,expires_at,linked_at,consumed_at,login_code_hash,login_code_expires_at,COALESCE(login_actor_id,''),COALESCE(login_session_id::text,'') FROM mcp_connection_requests`

type mcpRow interface{ Scan(...any) error }

func scanMCPConnection(row mcpRow) (MCPConnectionRequest, error) {
	var v MCPConnectionRequest
	err := row.Scan(&v.ID, &v.WorkspaceID, &v.AgentActorID, &v.GatewayID, &v.SecretHash, &v.Status, &v.LinkedByActorID, &v.CreatedAt, &v.ExpiresAt, &v.LinkedAt, &v.ConsumedAt, &v.LoginCodeHash, &v.LoginCodeExpiresAt, &v.LoginActorID, &v.LoginSessionID)
	return v, err
}
