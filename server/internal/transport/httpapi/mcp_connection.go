package httpapi

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jazzcake/baley/server/internal/persistence/postgres"
)

// Gateway enrollment crosses from an Agent surface to a signed-in browser.
// Keep the one-time login link long enough for that authentication hand-off.
const mcpConnectionTTL = 30 * time.Minute
const mcpLoginCodeTTL = 2 * time.Minute

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

func (a *API) createMCPLoginLink(w http.ResponseWriter, r *http.Request) {
	var input struct {
		WorkspaceID  string `json:"workspaceId"`
		AgentActorID string `json:"agentActorId"`
		GatewayID    string `json:"gatewayId"`
	}
	if !decode(w, r, &input) {
		return
	}
	input.WorkspaceID, input.AgentActorID, input.GatewayID = strings.TrimSpace(input.WorkspaceID), strings.TrimSpace(input.AgentActorID), strings.TrimSpace(input.GatewayID)
	if !isLowerUUID(input.WorkspaceID) || input.AgentActorID == "" || len(input.GatewayID) < 20 || len(input.GatewayID) > 256 {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": map[string]string{"code": "invalid_mcp_connection", "message": "Workspace UUID, Agent actor ID, and local gateway identity are required"}})
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
	view, err := a.Repo.CreateMCPConnection(r.Context(), id, input.WorkspaceID, input.AgentActorID, input.GatewayID, postgres.DigestSecret(secret), now, now.Add(mcpConnectionTTL))
	if err != nil {
		writeError(w, err)
		return
	}
	origin := strings.TrimRight(a.MCPLoginOrigin, "/")
	if origin == "" && len(a.AllowedOrigins) > 0 {
		origin = strings.TrimRight(a.AllowedOrigins[0], "/")
	}
	if origin == "" {
		origin = "http://127.0.0.1:5174"
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": view.ID, "workspaceId": view.WorkspaceID, "status": view.Status, "expiresAt": view.ExpiresAt, "connectionSecret": secret, "loginUrl": origin + "/workspaces/" + view.WorkspaceID + "/mcp-login/" + view.ID})
}

func (a *API) pollMCPLoginLink(w http.ResponseWriter, r *http.Request) {
	secret := r.Header.Get("X-Baley-Connection-Secret")
	view, issued, gatewaySecret, err := a.Repo.PollMCPLoginLinkAndIssueAgentToken(r.Context(), r.PathValue("connectionId"), postgres.DigestSecret(secret), time.Now().UTC())
	if errors.Is(err, postgres.ErrMCPConnectionNotFound) || errors.Is(err, postgres.ErrMCPConnectionSecret) || errors.Is(err, postgres.ErrMCPConnectionConsumed) {
		mcpConnectionNotFound(w)
		return
	}
	if err != nil {
		writeError(w, err)
		return
	}
	result := map[string]any{"id": view.ID, "workspaceId": view.WorkspaceID, "status": view.Status, "expiresAt": view.ExpiresAt}
	if issued.Token != "" {
		result["agentToken"] = issued.Token
		result["gatewayId"] = view.GatewayID
		result["gatewaySecret"] = gatewaySecret
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *API) redeemMCPLoginLink(w http.ResponseWriter, r *http.Request) {
	secret := strings.TrimSpace(r.Header.Get("X-Baley-Connection-Secret"))
	code := strings.TrimSpace(r.Header.Get("X-Baley-Login-Code"))
	if secret == "" || code == "" {
		mcpConnectionNotFound(w)
		return
	}
	view, issued, gatewaySecret, err := a.Repo.RedeemMCPLoginLinkAndIssueAgentToken(r.Context(), r.PathValue("connectionId"), postgres.DigestSecret(secret), postgres.DigestSecret(code), time.Now().UTC())
	if errors.Is(err, postgres.ErrMCPConnectionNotFound) || errors.Is(err, postgres.ErrMCPConnectionSecret) || errors.Is(err, postgres.ErrMCPConnectionConsumed) || errors.Is(err, postgres.ErrMCPLoginCode) {
		mcpConnectionNotFound(w)
		return
	}
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": view.ID, "workspaceId": view.WorkspaceID, "status": view.Status, "agentToken": issued.Token, "gatewayId": view.GatewayID, "gatewaySecret": gatewaySecret})
}

func (a *API) resumeMCPGateway(w http.ResponseWriter, r *http.Request) {
	var input struct {
		WorkspaceID   string `json:"workspaceId"`
		GatewayID     string `json:"gatewayId"`
		GatewaySecret string `json:"gatewaySecret"`
	}
	if !decode(w, r, &input) {
		return
	}
	input.WorkspaceID, input.GatewayID, input.GatewaySecret = strings.TrimSpace(input.WorkspaceID), strings.TrimSpace(input.GatewayID), strings.TrimSpace(input.GatewaySecret)
	if !isLowerUUID(input.WorkspaceID) || len(input.GatewayID) < 20 || input.GatewaySecret == "" {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": map[string]string{"code": "invalid_mcp_gateway", "message": "Workspace, gateway identity, and credential are required"}})
		return
	}
	issued, err := a.Repo.ResumeMCPGateway(r.Context(), input.WorkspaceID, input.GatewayID, input.GatewaySecret, time.Now().UTC())
	if errors.Is(err, postgres.ErrMCPGatewayNotFound) || errors.Is(err, postgres.ErrMCPGatewaySecret) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]string{"code": "mcp_gateway_login_required", "message": "Gateway registration is unavailable; sign in to Baley on this device"}})
		return
	}
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"workspaceId": input.WorkspaceID, "agentToken": issued.Token})
}

// autoEnrollMCPGateway lets a locally registered device add another Workspace
// from active membership. The proof is a secret from an already active
// registration, not an untrusted gateway ID or an Agent identity. First-device
// onboarding uses the signed-in browser link above.
func (a *API) autoEnrollMCPGateway(w http.ResponseWriter, r *http.Request) {
	var input struct {
		WorkspaceID        string `json:"workspaceId"`
		GatewayID          string `json:"gatewayId"`
		ProofWorkspaceID   string `json:"proofWorkspaceId"`
		ProofGatewaySecret string `json:"proofGatewaySecret"`
	}
	if !decode(w, r, &input) {
		return
	}
	input.WorkspaceID = strings.TrimSpace(input.WorkspaceID)
	input.GatewayID = strings.TrimSpace(input.GatewayID)
	input.ProofWorkspaceID = strings.TrimSpace(input.ProofWorkspaceID)
	input.ProofGatewaySecret = strings.TrimSpace(input.ProofGatewaySecret)
	if !isLowerUUID(input.WorkspaceID) || !isLowerUUID(input.ProofWorkspaceID) || len(input.GatewayID) < 20 || input.ProofGatewaySecret == "" {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": map[string]string{"code": "invalid_mcp_gateway", "message": "Target Workspace, existing Workspace proof, and local gateway identity are required"}})
		return
	}
	result, err := a.Repo.AutoEnrollMCPGateway(r.Context(), input.WorkspaceID, input.GatewayID, input.ProofWorkspaceID, input.ProofGatewaySecret, time.Now().UTC())
	if errors.Is(err, postgres.ErrMCPGatewayNotFound) || errors.Is(err, postgres.ErrMCPGatewaySecret) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]string{"code": "mcp_gateway_login_required", "message": "A registered local gateway and active Workspace membership are required"}})
		return
	}
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"workspaceId": input.WorkspaceID, "agentToken": result.AgentToken, "gatewayId": input.GatewayID, "gatewaySecret": result.GatewaySecret})
}

func (a *API) revokeMCPGateway(w http.ResponseWriter, r *http.Request) {
	state, ok := a.requireOwner(r)
	if !ok {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": map[string]string{"code": "forbidden", "message": "Owner capability required"}})
		return
	}
	var input struct {
		Reason string `json:"reason"`
	}
	if !decode(w, r, &input) {
		return
	}
	if strings.TrimSpace(input.Reason) == "" {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": map[string]string{"code": "invalid_mcp_gateway", "message": "Revocation reason is required"}})
		return
	}
	err := a.Repo.RevokeMCPGateway(r.Context(), r.PathValue("workspaceId"), r.PathValue("gatewayId"), state.Principal.ActorID, input.Reason, time.Now().UTC())
	if errors.Is(err, postgres.ErrMCPGatewayNotFound) {
		mcpConnectionNotFound(w)
		return
	}
	if err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) getMCPLoginLink(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireMCPLinkMember(r); !ok {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": map[string]string{"code": "forbidden", "message": "Active Workspace membership required"}})
		return
	}
	view, err := a.Repo.MCPConnection(r.Context(), r.PathValue("connectionId"), time.Now().UTC())
	if err != nil || view.WorkspaceID != r.PathValue("workspaceId") {
		mcpConnectionNotFound(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": view.ID, "workspaceId": view.WorkspaceID, "agentActorId": view.AgentActorID, "status": view.Status, "expiresAt": view.ExpiresAt})
}

func (a *API) completeMCPLoginLink(w http.ResponseWriter, r *http.Request) {
	state, ok := a.requireMCPLinkMember(r)
	if !ok {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": map[string]string{"code": "forbidden", "message": "Active Workspace membership required"}})
		return
	}
	code, err := connectionRandomSecret()
	if err != nil {
		writeError(w, err)
		return
	}
	now := time.Now().UTC()
	_, err = a.Repo.BeginMCPLoginLink(r.Context(), r.PathValue("connectionId"), r.PathValue("workspaceId"), state.Principal.ActorID, postgres.DigestSecret(code), now, now.Add(mcpLoginCodeTTL))
	if errors.Is(err, postgres.ErrMCPConnectionNotFound) || errors.Is(err, postgres.ErrMCPConnectionConsumed) {
		mcpConnectionNotFound(w)
		return
	}
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": map[string]string{"code": "mcp_login_link_failed", "message": err.Error()}})
		return
	}
	origin := strings.TrimRight(a.MCPLoopbackCallbackOrigin, "/")
	if origin == "" {
		origin = "http://127.0.0.1:8090"
	}
	query := url.Values{"connectionId": {r.PathValue("connectionId")}, "code": {code}}
	writeJSON(w, http.StatusOK, map[string]any{"callbackUrl": origin + "/mcp-login/callback?" + query.Encode()})
}
