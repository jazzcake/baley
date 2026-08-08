package httpapi

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Owner approval often involves crossing from an Agent surface to a signed-in
// browser. Keep the one-time request long enough for that hand-off.
const mcpConnectionTTL = 30 * time.Minute

var (
	errMCPConnectionNotFound = errors.New("MCP connection request not found")
	errMCPConnectionSecret   = errors.New("MCP connection secret mismatch")
)

type MCPConnectionBroker struct {
	mu      sync.Mutex
	now     func() time.Time
	entries map[string]*mcpConnection
}

type mcpConnection struct {
	ID, WorkspaceID, AgentActorID string
	SecretHash                    [32]byte
	Status                        string
	AgentToken                    string
	CreatedAt, ExpiresAt          time.Time
}

type MCPConnectionView struct {
	ID, WorkspaceID, AgentActorID, Status string
	ExpiresAt                             time.Time
}

func NewMCPConnectionBroker() *MCPConnectionBroker {
	return &MCPConnectionBroker{now: time.Now, entries: map[string]*mcpConnection{}}
}

func (b *MCPConnectionBroker) Create(workspaceID, agentActorID string) (MCPConnectionView, string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.cleanupLocked()
	id, err := connectionRandomID()
	if err != nil {
		return MCPConnectionView{}, "", err
	}
	secret, err := connectionRandomSecret()
	if err != nil {
		return MCPConnectionView{}, "", err
	}
	now := b.now().UTC()
	entry := &mcpConnection{ID: id, WorkspaceID: workspaceID, AgentActorID: agentActorID,
		SecretHash: sha256.Sum256([]byte(secret)), Status: "pending", CreatedAt: now, ExpiresAt: now.Add(mcpConnectionTTL)}
	b.entries[id] = entry
	return connectionView(entry), secret, nil
}

func (b *MCPConnectionBroker) Get(id string) (MCPConnectionView, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.cleanupLocked()
	entry := b.entries[id]
	if entry == nil {
		return MCPConnectionView{}, errMCPConnectionNotFound
	}
	return connectionView(entry), nil
}

func (b *MCPConnectionBroker) Approve(id, workspaceID, token string) (MCPConnectionView, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.cleanupLocked()
	entry := b.entries[id]
	if entry == nil || entry.WorkspaceID != workspaceID {
		return MCPConnectionView{}, errMCPConnectionNotFound
	}
	if entry.Status != "pending" {
		return connectionView(entry), nil
	}
	entry.AgentToken, entry.Status = token, "approved"
	return connectionView(entry), nil
}

func (b *MCPConnectionBroker) Poll(id, secret string) (MCPConnectionView, string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.cleanupLocked()
	entry := b.entries[id]
	if entry == nil {
		return MCPConnectionView{}, "", errMCPConnectionNotFound
	}
	providedHash := sha256.Sum256([]byte(secret))
	if subtle.ConstantTimeCompare(providedHash[:], entry.SecretHash[:]) != 1 {
		return MCPConnectionView{}, "", errMCPConnectionSecret
	}
	view := connectionView(entry)
	if entry.Status != "approved" {
		return view, "", nil
	}
	token := entry.AgentToken
	delete(b.entries, id)
	return view, token, nil
}

func (b *MCPConnectionBroker) cleanupLocked() {
	now := b.now().UTC()
	for id, entry := range b.entries {
		if !now.Before(entry.ExpiresAt) {
			delete(b.entries, id)
		}
	}
}

func connectionView(entry *mcpConnection) MCPConnectionView {
	return MCPConnectionView{ID: entry.ID, WorkspaceID: entry.WorkspaceID, AgentActorID: entry.AgentActorID, Status: entry.Status, ExpiresAt: entry.ExpiresAt}
}

func connectionRandomID() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	raw[6], raw[8] = raw[6]&0x0f|0x40, raw[8]&0x3f|0x80
	encoded := base64.RawURLEncoding.EncodeToString(raw)
	return encoded, nil
}

func connectionRandomSecret() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func (a *API) mcpBroker() *MCPConnectionBroker {
	return a.MCPConnections
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
	view, secret, err := a.mcpBroker().Create(input.WorkspaceID, input.AgentActorID)
	if err != nil {
		writeError(w, err)
		return
	}
	origin := "http://127.0.0.1:5174"
	if len(a.AllowedOrigins) > 0 {
		origin = strings.TrimRight(a.AllowedOrigins[0], "/")
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id": view.ID, "workspaceId": view.WorkspaceID, "status": view.Status, "expiresAt": view.ExpiresAt,
		"connectionSecret": secret,
		"approvalUrl":      origin + "/workspaces/" + view.WorkspaceID + "/mcp-connect/" + view.ID,
	})
}

func (a *API) pollMCPConnection(w http.ResponseWriter, r *http.Request) {
	view, token, err := a.mcpBroker().Poll(r.PathValue("connectionId"), r.Header.Get("X-Baley-Connection-Secret"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]string{"code": "mcp_connection_not_found", "message": "MCP connection request not found"}})
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
	view, err := a.mcpBroker().Get(r.PathValue("connectionId"))
	if err != nil || view.WorkspaceID != r.PathValue("workspaceId") {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]string{"code": "mcp_connection_not_found", "message": "MCP connection request not found"}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": view.ID, "workspaceId": view.WorkspaceID, "agentActorId": view.AgentActorID, "status": view.Status, "expiresAt": view.ExpiresAt})
}

func (a *API) approveMCPConnection(w http.ResponseWriter, r *http.Request) {
	state, ok := a.requireOwner(r)
	if !ok {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": map[string]string{"code": "forbidden", "message": "Owner capability required"}})
		return
	}
	view, err := a.mcpBroker().Get(r.PathValue("connectionId"))
	if err != nil || view.WorkspaceID != r.PathValue("workspaceId") {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": map[string]string{"code": "mcp_connection_not_found", "message": "MCP connection request not found"}})
		return
	}
	if view.Status == "approved" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	issued, err := a.Repo.IssueAgentToken(r.Context(), view.WorkspaceID, view.AgentActorID,
		"mcp-connect-"+view.ID, state.Principal.ActorID, nil, nil)
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": map[string]string{"code": "mcp_connection_approval_failed", "message": err.Error()}})
		return
	}
	if _, err = a.mcpBroker().Approve(view.ID, view.WorkspaceID, issued.Token); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
