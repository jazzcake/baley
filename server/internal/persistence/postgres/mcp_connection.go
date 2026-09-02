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
)

type MCPConnectionRequest struct {
	ID, WorkspaceID, AgentActorID, GatewayID, Status, LinkedByActorID string
	SecretHash                                                        []byte
	CreatedAt, ExpiresAt                                              time.Time
	LinkedAt, ConsumedAt                                              *time.Time
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

func (r *Repository) LinkMCPConnection(ctx context.Context, id, workspaceID, memberActorID string, now time.Time) (MCPConnectionRequest, error) {
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
	if request.Status == "pending" {
		if _, err = tx.Exec(ctx, "UPDATE mcp_connection_requests SET status='linked',linked_by_actor_id=$1,linked_at=$2 WHERE id=$3", memberActorID, now, id); err != nil {
			return MCPConnectionRequest{}, err
		}
		request.Status, request.LinkedByActorID, request.LinkedAt = "linked", memberActorID, &now
	}
	if err = tx.Commit(ctx); err != nil {
		return MCPConnectionRequest{}, err
	}
	return request, nil
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

const mcpConnectionSelect = `SELECT id,workspace_id,agent_actor_id,gateway_id,secret_hash,status,COALESCE(linked_by_actor_id,''),created_at,expires_at,linked_at,consumed_at FROM mcp_connection_requests`

type mcpRow interface{ Scan(...any) error }

func scanMCPConnection(row mcpRow) (MCPConnectionRequest, error) {
	var v MCPConnectionRequest
	err := row.Scan(&v.ID, &v.WorkspaceID, &v.AgentActorID, &v.GatewayID, &v.SecretHash, &v.Status, &v.LinkedByActorID, &v.CreatedAt, &v.ExpiresAt, &v.LinkedAt, &v.ConsumedAt)
	return v, err
}
