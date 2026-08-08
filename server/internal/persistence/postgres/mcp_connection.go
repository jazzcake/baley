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
	ID, WorkspaceID, AgentActorID, Status, ApprovedByActorID string
	SecretHash                                               []byte
	CreatedAt, ExpiresAt                                     time.Time
	ApprovedAt, RejectedAt, ConsumedAt                       *time.Time
}

func (r *Repository) CreateMCPConnection(ctx context.Context, id, workspaceID, agentActorID string, secretHash []byte, now, expiresAt time.Time) (MCPConnectionRequest, error) {
	request := MCPConnectionRequest{ID: id, WorkspaceID: workspaceID, AgentActorID: agentActorID, SecretHash: secretHash, Status: "pending", CreatedAt: now, ExpiresAt: expiresAt}
	_, err := r.Pool.Exec(ctx, `INSERT INTO mcp_connection_requests(id,workspace_id,agent_actor_id,secret_hash,status,created_at,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, request.ID, request.WorkspaceID, request.AgentActorID, request.SecretHash, request.Status, request.CreatedAt, request.ExpiresAt)
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

func (r *Repository) ApproveMCPConnection(ctx context.Context, id, workspaceID, ownerActorID string, now time.Time) (MCPConnectionRequest, error) {
	return r.setMCPConnectionDecision(ctx, id, workspaceID, ownerActorID, now, "approved")
}
func (r *Repository) RejectMCPConnection(ctx context.Context, id, workspaceID, ownerActorID string, now time.Time) (MCPConnectionRequest, error) {
	return r.setMCPConnectionDecision(ctx, id, workspaceID, ownerActorID, now, "rejected")
}

func (r *Repository) setMCPConnectionDecision(ctx context.Context, id, workspaceID, ownerActorID string, now time.Time, decision string) (MCPConnectionRequest, error) {
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
		column := "approved_at"
		if decision == "rejected" {
			column = "rejected_at"
		}
		if _, err = tx.Exec(ctx, "UPDATE mcp_connection_requests SET status=$1,approved_by_actor_id=$2,"+column+"=$3 WHERE id=$4", decision, ownerActorID, now, id); err != nil {
			return MCPConnectionRequest{}, err
		}
		request.Status, request.ApprovedByActorID = decision, ownerActorID
		if decision == "approved" {
			request.ApprovedAt = &now
		} else {
			request.RejectedAt = &now
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return MCPConnectionRequest{}, err
	}
	return request, nil
}

func (r *Repository) PollMCPConnectionAndIssueAgentToken(ctx context.Context, id string, secretHash []byte, now time.Time) (MCPConnectionRequest, string, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return MCPConnectionRequest{}, "", err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "DELETE FROM mcp_connection_requests WHERE expires_at <= $1", now); err != nil {
		return MCPConnectionRequest{}, "", err
	}
	request, err := scanMCPConnection(tx.QueryRow(ctx, mcpConnectionSelect+" WHERE id=$1 FOR UPDATE", id))
	if errors.Is(err, pgx.ErrNoRows) {
		return MCPConnectionRequest{}, "", ErrMCPConnectionNotFound
	}
	if err != nil {
		return MCPConnectionRequest{}, "", err
	}
	if subtle.ConstantTimeCompare(secretHash, request.SecretHash) != 1 {
		return MCPConnectionRequest{}, "", ErrMCPConnectionSecret
	}
	if request.Status == "consumed" {
		return MCPConnectionRequest{}, "", ErrMCPConnectionConsumed
	}
	if request.Status != "approved" {
		if err = tx.Commit(ctx); err != nil {
			return MCPConnectionRequest{}, "", err
		}
		return request, "", nil
	}
	issued, err := r.issueAgentTokenTx(ctx, tx, request.WorkspaceID, request.AgentActorID, "mcp-connect-"+request.ID, request.ApprovedByActorID, nil, nil)
	if err != nil {
		return MCPConnectionRequest{}, "", err
	}
	if _, err = tx.Exec(ctx, "UPDATE mcp_connection_requests SET status='consumed',consumed_at=$1 WHERE id=$2", now, id); err != nil {
		return MCPConnectionRequest{}, "", err
	}
	request.Status, request.ConsumedAt = "consumed", &now
	if err = tx.Commit(ctx); err != nil {
		return MCPConnectionRequest{}, "", err
	}
	return request, issued.Token, nil
}

const mcpConnectionSelect = `SELECT id,workspace_id,agent_actor_id,secret_hash,status,COALESCE(approved_by_actor_id,''),created_at,expires_at,approved_at,rejected_at,consumed_at FROM mcp_connection_requests`

type mcpRow interface{ Scan(...any) error }

func scanMCPConnection(row mcpRow) (MCPConnectionRequest, error) {
	var v MCPConnectionRequest
	err := row.Scan(&v.ID, &v.WorkspaceID, &v.AgentActorID, &v.SecretHash, &v.Status, &v.ApprovedByActorID, &v.CreatedAt, &v.ExpiresAt, &v.ApprovedAt, &v.RejectedAt, &v.ConsumedAt)
	return v, err
}
