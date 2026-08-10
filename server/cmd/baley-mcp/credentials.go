package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type workspaceCredential struct {
	AgentToken  string    `json:"agentToken"`
	ConnectedAt time.Time `json:"connectedAt"`
}

type credentialStore struct {
	Version            int                                   `json:"version"`
	ServerURL          string                                `json:"serverUrl"`
	Workspaces         map[string]workspaceCredential        `json:"workspaces"`
	PendingConnections map[string]pendingWorkspaceConnection `json:"pendingConnections,omitempty"`
}

type pendingWorkspaceConnection struct {
	ID, Secret, ApprovalURL string
}

type connectionHTTPStatusError struct {
	StatusCode int
}

func (e connectionHTTPStatusError) Error() string {
	return fmt.Sprintf("Baley Workspace connection failed with HTTP %d", e.StatusCode)
}

type connectionResponse struct {
	ID               string    `json:"id"`
	WorkspaceID      string    `json:"workspaceId"`
	Status           string    `json:"status"`
	ExpiresAt        time.Time `json:"expiresAt"`
	ConnectionSecret string    `json:"connectionSecret"`
	ApprovalURL      string    `json:"approvalUrl"`
	AgentToken       string    `json:"agentToken"`
}

func requestWorkspaceID(path string, payload any) string {
	const prefix = "/v1/workspaces/"
	if index := strings.Index(path, prefix); index >= 0 {
		value := path[index+len(prefix):]
		if slash := strings.IndexByte(value, '/'); slash >= 0 {
			value = value[:slash]
		}
		return strings.TrimSpace(value)
	}
	if payload == nil {
		return ""
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	var command struct {
		Arguments struct {
			WorkspaceID string `json:"workspaceId"`
		} `json:"arguments"`
		WorkspaceID string `json:"workspaceId"`
	}
	if json.Unmarshal(raw, &command) != nil {
		return ""
	}
	if command.Arguments.WorkspaceID != "" {
		return command.Arguments.WorkspaceID
	}
	return command.WorkspaceID
}

func (c *client) workspaceCredential(ctx context.Context, workspaceID string) (string, *mcp.CallToolResult, error) {
	c.connectionMu.Lock()
	defer c.connectionMu.Unlock()
	store, err := c.readCredentialStore()
	if err != nil {
		return "", nil, err
	}
	if credential, ok := store.Workspaces[workspaceID]; ok && credential.AgentToken != "" {
		return credential.AgentToken, nil, nil
	}

	if pending, ok := store.PendingConnections[workspaceID]; ok {
		response, err := c.pollWorkspaceConnection(ctx, pending)
		if err != nil {
			var statusError connectionHTTPStatusError
			if !errors.As(err, &statusError) || statusError.StatusCode != http.StatusNotFound {
				return "", nil, err
			}
			delete(store.PendingConnections, workspaceID)
			if writeErr := c.writeCredentialStore(store); writeErr != nil {
				return "", nil, writeErr
			}
			// A not-found connection request has expired or been consumed. Start
			// a replacement in this call rather than leaving this MCP process stuck.
		} else if response.Status == "approved" && response.AgentToken != "" {
			store.Workspaces[workspaceID] = workspaceCredential{AgentToken: response.AgentToken, ConnectedAt: time.Now().UTC()}
			delete(store.PendingConnections, workspaceID)
			if err = c.writeCredentialStore(store); err != nil {
				return "", nil, err
			}
			return response.AgentToken, nil, nil
		} else {
			return "", connectionRequired(workspaceID, pending.ApprovalURL), nil
		}
	}

	pending, err := c.createWorkspaceConnection(ctx, workspaceID)
	if err != nil {
		return "", nil, err
	}
	store.PendingConnections[workspaceID] = pending
	if err = c.writeCredentialStore(store); err != nil {
		return "", nil, err
	}
	return "", connectionRequired(workspaceID, pending.ApprovalURL), nil
}

func (c *client) createWorkspaceConnection(ctx context.Context, workspaceID string) (pendingWorkspaceConnection, error) {
	raw, err := json.Marshal(map[string]string{"workspaceId": workspaceID, "agentActorId": c.agentActorID})
	if err != nil {
		return pendingWorkspaceConnection{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/v1/mcp/connections", bytes.NewReader(raw))
	if err != nil {
		return pendingWorkspaceConnection{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	var response connectionResponse
	if err = c.connectionRequest(req, http.StatusCreated, &response); err != nil {
		return pendingWorkspaceConnection{}, err
	}
	if response.ID == "" || response.ConnectionSecret == "" || response.ApprovalURL == "" {
		return pendingWorkspaceConnection{}, errors.New("Baley returned an incomplete Workspace connection request")
	}
	return pendingWorkspaceConnection{ID: response.ID, Secret: response.ConnectionSecret, ApprovalURL: response.ApprovalURL}, nil
}

func (c *client) pollWorkspaceConnection(ctx context.Context, pending pendingWorkspaceConnection) (connectionResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/v1/mcp/connections/"+pending.ID, nil)
	if err != nil {
		return connectionResponse{}, err
	}
	req.Header.Set("X-Baley-Connection-Secret", pending.Secret)
	var response connectionResponse
	err = c.connectionRequest(req, http.StatusOK, &response)
	return response, err
}

func (c *client) connectionRequest(req *http.Request, expected int, target any) error {
	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("Baley Workspace connection transport: %w", err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return err
	}
	if res.StatusCode != expected {
		return connectionHTTPStatusError{StatusCode: res.StatusCode}
	}
	if err = json.Unmarshal(raw, target); err != nil {
		return errors.New("Baley returned an invalid Workspace connection response")
	}
	return nil
}

func connectionRequired(workspaceID, approvalURL string) *mcp.CallToolResult {
	structured := map[string]any{
		"code":        "workspace_connection_required",
		"workspaceId": workspaceID,
		"status":      "pending",
		"approvalUrl": approvalURL,
		"message":     "Open the approval URL as the Workspace Owner, approve Operator access, then retry this same request.",
	}
	return &mcp.CallToolResult{
		Content:           []mcp.Content{&mcp.TextContent{Text: "Workspace Owner approval is required once. Open " + approvalURL + " and then retry the same request."}},
		StructuredContent: structured,
		IsError:           true,
	}
}

func pendingStructured(result *mcp.CallToolResult) any {
	if result == nil {
		return nil
	}
	return result.StructuredContent
}

func (c *client) readCredentialStore() (credentialStore, error) {
	store := credentialStore{Version: 2, ServerURL: c.base, Workspaces: map[string]workspaceCredential{}, PendingConnections: map[string]pendingWorkspaceConnection{}}
	raw, err := os.ReadFile(c.credentialStorePath)
	if errors.Is(err, os.ErrNotExist) {
		return store, nil
	}
	if err != nil {
		return store, fmt.Errorf("read Baley credential store: %w", err)
	}
	if err = json.Unmarshal(raw, &store); err != nil {
		return store, errors.New("Baley credential store is invalid JSON")
	}
	if store.ServerURL != "" && strings.TrimRight(store.ServerURL, "/") != c.base {
		return store, errors.New("Baley credential store belongs to a different server URL")
	}
	store.ServerURL = c.base
	if store.Workspaces == nil {
		store.Workspaces = map[string]workspaceCredential{}
	}
	if store.PendingConnections == nil {
		store.PendingConnections = map[string]pendingWorkspaceConnection{}
	}
	if store.Version < 2 {
		store.Version = 2
	}
	return store, nil
}

func (c *client) writeCredentialStore(store credentialStore) error {
	if err := os.MkdirAll(filepath.Dir(c.credentialStorePath), 0o700); err != nil {
		return fmt.Errorf("create Baley credential directory: %w", err)
	}
	raw, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(c.credentialStorePath), ".credentials-*.tmp")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err = temporary.Chmod(0o600); err == nil {
		_, err = temporary.Write(raw)
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("write Baley credential store: %w", err)
	}
	if err = os.Rename(temporaryName, c.credentialStorePath); err != nil {
		return fmt.Errorf("replace Baley credential store: %w", err)
	}
	return nil
}

func (c *client) removeWorkspaceCredential(workspaceID string) error {
	c.connectionMu.Lock()
	defer c.connectionMu.Unlock()
	store, err := c.readCredentialStore()
	if err != nil {
		return err
	}
	delete(store.Workspaces, workspaceID)
	delete(store.PendingConnections, workspaceID)
	return c.writeCredentialStore(store)
}
