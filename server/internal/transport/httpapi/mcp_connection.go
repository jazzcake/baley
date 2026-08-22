package httpapi

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jazzcake/baley/server/internal/persistence/postgres"
)

// Owner approval often involves crossing from an Agent surface to a signed-in
// browser. Keep the one-time request long enough for that hand-off.
const mcpConnectionTTL = 30 * time.Minute

func connectionRandomID() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	raw[6], raw[8] = raw[6]&0x0f|0x40, raw[8]&0x3f|0x80
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func connectionRandomSecret() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func mcpConnectionNotFound(w http.ResponseWriter) {
	writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]string{"code": "mcp_connection_not_found", "message": "MCP connection request not found"}})
}

func (a *API) createMCPConnection(w http.ResponseWriter, r *http.Request) {
	var input struct {
		WorkspaceID  string `json:"workspaceId"`
		AgentActorID string `json:"agentActorId"`
	}
	if !decode(w, r, &input) {
		return
	}
	input.WorkspaceID, input.AgentActorID = strings.TrimSpace(input.WorkspaceID), strings.TrimSpace(input.AgentActorID)
	if !isLowerUUID(input.WorkspaceID) || input.AgentActorID == "" {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": map[string]string{"code": "invalid_mcp_connection", "message": "Workspace UUID and Agent actor ID are required"}})
		return
	}
	id, err := connectionRandomID()
	if err != nil {
		writeError(w, err)
		return
	}
	secret, err := connectionRandomSecret()
	if err != nil {
		writeError(w, err)
		return
	}
	now := time.Now().UTC()
	view, err := a.Repo.CreateMCPConnection(r.Context(), id, input.WorkspaceID, input.AgentActorID, postgres.DigestSecret(secret), now, now.Add(mcpConnectionTTL))
	if err != nil {
		writeError(w, err)
		return
	}
	origin := strings.TrimRight(a.ApprovalOrigin, "/")
	if origin == "" && len(a.AllowedOrigins) > 0 {
		origin = strings.TrimRight(a.AllowedOrigins[0], "/")
	}
	if origin == "" {
		origin = "http://127.0.0.1:5174"
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": view.ID, "workspaceId": view.WorkspaceID, "status": view.Status, "expiresAt": view.ExpiresAt, "connectionSecret": secret, "approvalUrl": origin + "/workspaces/" + view.WorkspaceID + "/mcp-connect/" + view.ID})
}

func (a *API) pollMCPConnection(w http.ResponseWriter, r *http.Request) {
	secret := r.Header.Get("X-Baley-Connection-Secret")
	view, token, err := a.Repo.PollMCPConnectionAndIssueAgentToken(r.Context(), r.PathValue("connectionId"), postgres.DigestSecret(secret), time.Now().UTC())
	if errors.Is(err, postgres.ErrMCPConnectionNotFound) || errors.Is(err, postgres.ErrMCPConnectionSecret) || errors.Is(err, postgres.ErrMCPConnectionConsumed) {
		mcpConnectionNotFound(w)
		return
	}
	if err != nil {
		writeError(w, err)
		return
	}
	result := map[string]any{"id": view.ID, "workspaceId": view.WorkspaceID, "status": view.Status, "expiresAt": view.ExpiresAt}
	if token != "" {
		result["agentToken"] = token
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) getMCPConnection(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireOwner(r); !ok {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": map[string]string{"code": "forbidden", "message": "Owner capability required"}})
		return
	}
	view, err := a.Repo.MCPConnection(r.Context(), r.PathValue("connectionId"), time.Now().UTC())
	if err != nil || view.WorkspaceID != r.PathValue("workspaceId") {
		mcpConnectionNotFound(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": view.ID, "workspaceId": view.WorkspaceID, "agentActorId": view.AgentActorID, "status": view.Status, "expiresAt": view.ExpiresAt})
}

func (a *API) approveMCPConnection(w http.ResponseWriter, r *http.Request) {
	a.decideMCPConnection(w, r, true)
}
func (a *API) rejectMCPConnection(w http.ResponseWriter, r *http.Request) {
	a.decideMCPConnection(w, r, false)
}
func (a *API) decideMCPConnection(w http.ResponseWriter, r *http.Request, approve bool) {
	state, ok := a.requireOwner(r)
	if !ok {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": map[string]string{"code": "forbidden", "message": "Owner capability required"}})
		return
	}
	var err error
	if approve {
		_, err = a.Repo.ApproveMCPConnection(r.Context(), r.PathValue("connectionId"), r.PathValue("workspaceId"), state.Principal.ActorID, time.Now().UTC())
	} else {
		_, err = a.Repo.RejectMCPConnection(r.Context(), r.PathValue("connectionId"), r.PathValue("workspaceId"), state.Principal.ActorID, time.Now().UTC())
	}
	if errors.Is(err, postgres.ErrMCPConnectionNotFound) {
		mcpConnectionNotFound(w)
		return
	}
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": map[string]string{"code": "mcp_connection_approval_failed", "message": err.Error()}})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
