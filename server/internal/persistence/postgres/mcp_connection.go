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
)

type MCPConnectionRequest struct {
	ID, WorkspaceID, AgentActorID, GatewayID, Status, LinkedByActorID string
	SecretHash, LoginCodeHash                                         []byte
	LoginActorID                                                      string
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

func (r *Repository) BeginMCPLoginLink(ctx context.Context, id, workspaceID, memberActorID string, codeHash []byte, now, codeExpiresAt time.Time) (MCPConnectionRequest, error) {
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
	if _, err = tx.Exec(ctx, "UPDATE mcp_connection_requests SET login_code_hash=$1,login_code_expires_at=$2,login_actor_id=$3 WHERE id=$4", codeHash, codeExpiresAt, memberActorID, id); err != nil {
		return MCPConnectionRequest{}, err
	}
	request.LoginCodeHash, request.LoginCodeExpiresAt, request.LoginActorID = codeHash, &codeExpiresAt, memberActorID
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

func (r *Repository) PollMCPLoginLinkAndIssueAgentToken(ctx context.Context, id string, secretHash []byte, now time.Time) (MCPConnectionRequest, AgentTokenResult, string, error) {
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
	if subtle.ConstantTimeCompare(secretHash, request.SecretHash) != 1 {
		return MCPConnectionRequest{}, AgentTokenResult{}, "", ErrMCPConnectionSecret
	}
	if request.Status == "consumed" {
		return MCPConnectionRequest{}, AgentTokenResult{}, "", ErrMCPConnectionConsumed
	}
	if request.Status != "linked" {
		if err = tx.Commit(ctx); err != nil {
			return MCPConnectionRequest{}, AgentTokenResult{}, "", err
		}
		return request, AgentTokenResult{}, "", nil
	}
	registration, gatewaySecret, err := r.enrollMCPGatewayTx(ctx, tx, request.WorkspaceID, request.LinkedByActorID, request.AgentActorID, request.GatewayID, now)
	if err != nil {
		return MCPConnectionRequest{}, AgentTokenResult{}, "", err
	}
	scopes, err := memberAgentScopesTx(ctx, tx, request.WorkspaceID, request.LinkedByActorID)
	if err != nil {
		return MCPConnectionRequest{}, AgentTokenResult{}, "", err
	}
	issued, err := r.issueAgentTokenTx(ctx, tx, request.WorkspaceID, request.AgentActorID, "mcp-login-"+request.ID, request.LinkedByActorID, scopes, nil, &registration.ID)
	if err != nil {
		return MCPConnectionRequest{}, AgentTokenResult{}, "", err
	}
	if _, err = tx.Exec(ctx, "UPDATE mcp_connection_requests SET status='consumed',consumed_at=$1 WHERE id=$2", now, id); err != nil {
		return MCPConnectionRequest{}, AgentTokenResult{}, "", err
	}
	request.Status, request.ConsumedAt = "consumed", &now
	if err = tx.Commit(ctx); err != nil {
		return MCPConnectionRequest{}, AgentTokenResult{}, "", err
	}
	return request, issued, gatewaySecret, nil
}

const mcpConnectionSelect = `SELECT id,workspace_id,agent_actor_id,gateway_id,secret_hash,status,COALESCE(linked_by_actor_id,''),created_at,expires_at,linked_at,consumed_at,login_code_hash,login_code_expires_at,COALESCE(login_actor_id,'') FROM mcp_connection_requests`

type mcpRow interface{ Scan(...any) error }

func scanMCPConnection(row mcpRow) (MCPConnectionRequest, error) {
	var v MCPConnectionRequest
	err := row.Scan(&v.ID, &v.WorkspaceID, &v.AgentActorID, &v.GatewayID, &v.SecretHash, &v.Status, &v.LinkedByActorID, &v.CreatedAt, &v.ExpiresAt, &v.LinkedAt, &v.ConsumedAt, &v.LoginCodeHash, &v.LoginCodeExpiresAt, &v.LoginActorID)
	return v, err
}
