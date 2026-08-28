package main

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
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
	AgentToken    string    `json:"agentToken"`
	GatewaySecret string    `json:"gatewaySecret,omitempty"`
	ConnectedAt   time.Time `json:"connectedAt"`
}

type credentialStore struct {
	Version            int                                   `json:"version"`
	ServerURL          string                                `json:"serverUrl"`
	GatewayID          string                                `json:"gatewayId,omitempty"`
	Workspaces         map[string]workspaceCredential        `json:"workspaces"`
	PendingConnections map[string]pendingWorkspaceConnection `json:"pendingConnections,omitempty"`
}

// credentialStoreDisk keeps the server URL available for safe routing while the
// Workspace tokens and pending connection secrets stay encrypted. The encryption
// key is derived from the local HTTP gateway token, which is already the
// per-user secret required to reach this loopback MCP service.
type credentialStoreDisk struct {
	Version    int    `json:"version"`
	ServerURL  string `json:"serverUrl"`
	Ciphertext string `json:"ciphertext,omitempty"`
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
	GatewayID        string    `json:"gatewayId"`
	GatewaySecret    string    `json:"gatewaySecret"`
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
	store, err := c.readCredentialStore(ctx)
	if err != nil {
		return "", nil, err
	}
	if credential, ok := store.Workspaces[workspaceID]; ok && credential.AgentToken != "" {
		return credential.AgentToken, nil, nil
	}
	if credential, ok := store.Workspaces[workspaceID]; ok && credential.GatewaySecret != "" && store.GatewayID != "" {
		response, resumeErr := c.resumeWorkspaceGateway(ctx, workspaceID, store.GatewayID, credential.GatewaySecret)
		if resumeErr == nil && response.AgentToken != "" {
			credential.AgentToken, credential.ConnectedAt = response.AgentToken, time.Now().UTC()
			store.Workspaces[workspaceID] = credential
			if err = c.writeCredentialStore(ctx, store); err != nil {
				return "", nil, err
			}
			return credential.AgentToken, nil, nil
		}
		var statusError connectionHTTPStatusError
		if resumeErr != nil && (!errors.As(resumeErr, &statusError) || statusError.StatusCode != http.StatusUnauthorized) {
			return "", nil, resumeErr
		}
		delete(store.Workspaces, workspaceID)
		if err = c.writeCredentialStore(ctx, store); err != nil {
			return "", nil, err
		}
	}

	if pending, ok := store.PendingConnections[workspaceID]; ok {
		response, err := c.pollWorkspaceConnection(ctx, pending)
		if err != nil {
			var statusError connectionHTTPStatusError
			if !errors.As(err, &statusError) || statusError.StatusCode != http.StatusNotFound {
				return "", nil, err
			}
			delete(store.PendingConnections, workspaceID)
			if writeErr := c.writeCredentialStore(ctx, store); writeErr != nil {
				return "", nil, writeErr
			}
			// A not-found connection request has expired or been consumed. Start
			// a replacement in this call rather than leaving this MCP process stuck.
		} else if response.AgentToken != "" {
			// Polling issues a one-time Agent token and atomically marks the
			// connection consumed. The response can therefore be either approved
			// (before a repository implementation marks consumption) or consumed.
			// The token itself is the authoritative successful hand-off.
			store.Workspaces[workspaceID] = workspaceCredential{AgentToken: response.AgentToken, GatewaySecret: response.GatewaySecret, ConnectedAt: time.Now().UTC()}
			delete(store.PendingConnections, workspaceID)
			if err = c.writeCredentialStore(ctx, store); err != nil {
				return "", nil, err
			}
			return response.AgentToken, nil, nil
		} else {
			return "", connectionRequired(workspaceID, pending.ApprovalURL), nil
		}
	}

	if store.GatewayID == "" {
		store.GatewayID, err = newGatewayID()
		if err != nil {
			return "", nil, err
		}
	}
	pending, err := c.createWorkspaceConnection(ctx, workspaceID, store.GatewayID)
	if err != nil {
		return "", nil, err
	}
	store.PendingConnections[workspaceID] = pending
	if err = c.writeCredentialStore(ctx, store); err != nil {
		return "", nil, err
	}
	return "", connectionRequired(workspaceID, pending.ApprovalURL), nil
}

func (c *client) createWorkspaceConnection(ctx context.Context, workspaceID, gatewayID string) (pendingWorkspaceConnection, error) {
	raw, err := json.Marshal(map[string]string{"workspaceId": workspaceID, "agentActorId": c.agentActorID, "gatewayId": gatewayID})
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

func (c *client) resumeWorkspaceGateway(ctx context.Context, workspaceID, gatewayID, gatewaySecret string) (connectionResponse, error) {
	raw, err := json.Marshal(map[string]string{"workspaceId": workspaceID, "gatewayId": gatewayID, "gatewaySecret": gatewaySecret})
	if err != nil {
		return connectionResponse{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/v1/mcp/gateway-sessions", bytes.NewReader(raw))
	if err != nil {
		return connectionResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	var response connectionResponse
	err = c.connectionRequest(req, http.StatusOK, &response)
	return response, err
}

func newGatewayID() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
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

func (c *client) readCredentialStore(ctx context.Context) (credentialStore, error) {
	store := credentialStore{Version: 4, ServerURL: c.base, Workspaces: map[string]workspaceCredential{}, PendingConnections: map[string]pendingWorkspaceConnection{}}
	path := c.credentialStorePath
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return store, nil
	}
	if err != nil {
		return store, fmt.Errorf("read Baley credential store: %w", err)
	}
	var disk credentialStoreDisk
	if err = json.Unmarshal(raw, &disk); err != nil {
		return store, errors.New("Baley credential store is invalid JSON")
	}
	if disk.ServerURL != "" && strings.TrimRight(disk.ServerURL, "/") != c.base {
		return store, errors.New("Baley credential store belongs to a different server URL")
	}
	if disk.Ciphertext != "" {
		if c.gatewayToken == "" {
			return store, errors.New("Baley encrypted credential store requires BALEY_MCP_GATEWAY_TOKEN")
		}
		plaintext, decryptErr := c.decryptCredentialStore(disk.Ciphertext)
		if decryptErr != nil {
			return store, fmt.Errorf("decrypt Baley credential store: %w", decryptErr)
		}
		if err = json.Unmarshal(plaintext, &store); err != nil {
			return store, errors.New("Baley credential store contains invalid encrypted JSON")
		}
	} else if err = json.Unmarshal(raw, &store); err != nil {
		return store, errors.New("Baley credential store is invalid JSON")
	}
	store.Version = 4
	store.ServerURL = c.base
	if store.Workspaces == nil {
		store.Workspaces = map[string]workspaceCredential{}
	}
	if store.PendingConnections == nil {
		store.PendingConnections = map[string]pendingWorkspaceConnection{}
	}
	if disk.Ciphertext == "" && c.gatewayToken != "" {
		if err = c.writeCredentialStore(ctx, store); err != nil {
			return store, fmt.Errorf("migrate Baley credential store: %w", err)
		}
	}
	return store, nil
}

func (c *client) writeCredentialStore(ctx context.Context, store credentialStore) error {
	path := c.credentialStorePath
	if path == "" {
		return errors.New("Baley credential store is not configured")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create Baley credential directory: %w", err)
	}
	store.Version = 4
	store.ServerURL = c.base
	for _, credential := range store.Workspaces {
		if credential.GatewaySecret != "" && c.gatewayToken == "" {
			return errors.New("registered MCP gateway credential requires encrypted local storage")
		}
	}
	raw, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	if c.gatewayToken != "" {
		ciphertext, encryptErr := c.encryptCredentialStore(raw)
		if encryptErr != nil {
			return fmt.Errorf("encrypt Baley credential store: %w", encryptErr)
		}
		raw, err = json.MarshalIndent(credentialStoreDisk{Version: 4, ServerURL: c.base, Ciphertext: ciphertext}, "", "  ")
		if err != nil {
			return err
		}
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".credentials-*.tmp")
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
	if err = os.Rename(temporaryName, path); err != nil {
		return fmt.Errorf("replace Baley credential store: %w", err)
	}
	return nil
}

func (c *client) credentialStoreAEAD() (cipher.AEAD, error) {
	if c.gatewayToken == "" {
		return nil, errors.New("gateway token is empty")
	}
	mac := hmac.New(sha256.New, []byte(c.gatewayToken))
	_, _ = mac.Write([]byte("baley/mcp-credential-store/v1\n" + c.base))
	block, err := aes.NewCipher(mac.Sum(nil))
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func (c *client) encryptCredentialStore(plaintext []byte) (string, error) {
	aead, err := c.credentialStoreAEAD()
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return "", err
	}
	ciphertext := aead.Seal(nil, nonce, plaintext, []byte("baley/mcp-credential-store/v1\n"+c.base))
	return base64.RawURLEncoding.EncodeToString(append(nonce, ciphertext...)), nil
}

func (c *client) decryptCredentialStore(encoded string) ([]byte, error) {
	aead, err := c.credentialStoreAEAD()
	if err != nil {
		return nil, err
	}
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, errors.New("ciphertext is not base64url")
	}
	if len(raw) < aead.NonceSize() {
		return nil, errors.New("ciphertext is too short")
	}
	return aead.Open(nil, raw[:aead.NonceSize()], raw[aead.NonceSize():], []byte("baley/mcp-credential-store/v1\n"+c.base))
}

func (c *client) removeWorkspaceCredential(ctx context.Context, workspaceID string) error {
	c.connectionMu.Lock()
	defer c.connectionMu.Unlock()
	store, err := c.readCredentialStore(ctx)
	if err != nil {
		return err
	}
	if credential, ok := store.Workspaces[workspaceID]; ok {
		if credential.GatewaySecret == "" {
			delete(store.Workspaces, workspaceID)
		} else {
			credential.AgentToken = ""
			store.Workspaces[workspaceID] = credential
		}
	}
	delete(store.PendingConnections, workspaceID)
	return c.writeCredentialStore(ctx, store)
}
