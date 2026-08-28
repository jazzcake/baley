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
	KeyRef             string                                `json:"-"`
	GatewayID          string                                `json:"gatewayId,omitempty"`
	Workspaces         map[string]workspaceCredential        `json:"workspaces"`
	PendingConnections map[string]pendingWorkspaceConnection `json:"pendingConnections,omitempty"`
}

// credentialStoreDisk keeps only non-secret routing metadata and a random
// keychain reference. Version 4's Ciphertext is deliberately retained only for
// a one-way migration from the former gateway-token-derived store.
type credentialStoreDisk struct {
	Version    int    `json:"version"`
	ServerURL  string `json:"serverUrl"`
	KeyRef     string `json:"keyRef,omitempty"`
	Ciphertext string `json:"ciphertext,omitempty"`
}

type legacyMigrationMarker struct {
	Version   int       `json:"version"`
	ExpiresAt time.Time `json:"expiresAt"`
}

const legacyRollbackWindow = 15 * time.Minute

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
			if err = c.writeCredentialStore(ctx, &store); err != nil {
				return "", nil, err
			}
			return credential.AgentToken, nil, nil
		}
		var statusError connectionHTTPStatusError
		if resumeErr != nil && (!errors.As(resumeErr, &statusError) || statusError.StatusCode != http.StatusUnauthorized) {
			return "", nil, resumeErr
		}
		delete(store.Workspaces, workspaceID)
		if err = c.writeCredentialStore(ctx, &store); err != nil {
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
			if writeErr := c.writeCredentialStore(ctx, &store); writeErr != nil {
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
			if err = c.writeCredentialStore(ctx, &store); err != nil {
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
	if err = c.writeCredentialStore(ctx, &store); err != nil {
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
		"code":        "workspace_login_required",
		"workspaceId": workspaceID,
		"status":      "pending",
		"approvalUrl": approvalURL,
		"message":     "Open the connection URL while signed in to Baley. The local gateway will be linked automatically, then retry this request.",
	}
	return &mcp.CallToolResult{
		Content:           []mcp.Content{&mcp.TextContent{Text: "Sign in to Baley to link this local gateway automatically: " + approvalURL + ". Then retry the same request."}},
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
	store := credentialStore{Version: 5, ServerURL: c.base, Workspaces: map[string]workspaceCredential{}, PendingConnections: map[string]pendingWorkspaceConnection{}}
	path := c.credentialStorePath
	if path == "" {
		return store, errors.New("Baley credential store is not configured")
	}
	c.cleanupExpiredLegacyBackup()
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
	migrateToKeychain := false
	if disk.Ciphertext != "" {
		if c.gatewayToken == "" {
			return store, errors.New("legacy Baley credential store requires BALEY_MCP_GATEWAY_TOKEN once for migration")
		}
		plaintext, decryptErr := c.decryptCredentialStore(disk.Ciphertext)
		if decryptErr != nil {
			return store, fmt.Errorf("decrypt Baley credential store: %w", decryptErr)
		}
		if err = json.Unmarshal(plaintext, &store); err != nil {
			return store, errors.New("Baley credential store contains invalid encrypted JSON")
		}
		migrateToKeychain = c.secretStore != nil
		if migrateToKeychain {
			if err = c.backupLegacyCredentialStore(raw); err != nil {
				return store, fmt.Errorf("prepare legacy credential rollback: %w", err)
			}
		}
	} else if disk.KeyRef != "" {
		if c.secretStore == nil {
			return store, errors.New("Baley keychain-backed credential store is unavailable in this runtime")
		}
		encoded, keychainErr := c.secretStore.Get(credentialKeychainService, disk.KeyRef)
		if keychainErr != nil {
			return store, fmt.Errorf("read Baley device secret from OS keychain: %w", keychainErr)
		}
		plaintext, decodeErr := base64.RawURLEncoding.DecodeString(encoded)
		if decodeErr != nil || json.Unmarshal(plaintext, &store) != nil {
			return store, errors.New("Baley keychain credential payload is invalid")
		}
		store.KeyRef = disk.KeyRef
	} else if err = json.Unmarshal(raw, &store); err != nil {
		return store, errors.New("Baley credential store is invalid JSON")
	} else {
		// Pre-v5 plaintext metadata is safe to migrate even if it only contains a
		// short-lived Agent token. Never leave it as a fallback once keychain
		// support is active.
		migrateToKeychain = c.secretStore != nil
	}
	store.Version = 5
	store.ServerURL = c.base
	if store.Workspaces == nil {
		store.Workspaces = map[string]workspaceCredential{}
	}
	if store.PendingConnections == nil {
		store.PendingConnections = map[string]pendingWorkspaceConnection{}
	}
	if migrateToKeychain {
		// A migrated registered gateway must renew on the next call. This proves
		// the keychain copy can be used and cannot prolong a stale Agent token.
		for workspaceID, credential := range store.Workspaces {
			if credential.GatewaySecret != "" {
				credential.AgentToken = ""
				store.Workspaces[workspaceID] = credential
			}
		}
	}
	if migrateToKeychain || (disk.Ciphertext == "" && disk.KeyRef == "" && c.gatewayToken != "" && c.secretStore == nil) {
		if err = c.writeCredentialStore(ctx, &store); err != nil {
			return store, fmt.Errorf("migrate Baley credential store: %w", err)
		}
	}
	return store, nil
}

func (c *client) legacyBackupPath() string { return c.credentialStorePath + ".legacy.bak" }

func (c *client) legacyMarkerPath() string { return c.credentialStorePath + ".migration.json" }

func writePrivateFile(path string, raw []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".baley-*.tmp")
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
		return err
	}
	return os.Rename(temporaryName, path)
}

func (c *client) backupLegacyCredentialStore(raw []byte) error {
	if err := writePrivateFile(c.legacyBackupPath(), raw); err != nil {
		return err
	}
	marker, err := json.Marshal(legacyMigrationMarker{Version: 1, ExpiresAt: time.Now().UTC().Add(legacyRollbackWindow)})
	if err != nil {
		return err
	}
	return writePrivateFile(c.legacyMarkerPath(), marker)
}

func (c *client) legacyRollbackEligible() (bool, time.Time) {
	c.cleanupExpiredLegacyBackup()
	raw, err := os.ReadFile(c.legacyMarkerPath())
	if err != nil {
		return false, time.Time{}
	}
	var marker legacyMigrationMarker
	if json.Unmarshal(raw, &marker) != nil || marker.Version != 1 || !marker.ExpiresAt.After(time.Now().UTC()) {
		return false, time.Time{}
	}
	if _, err = os.Stat(c.legacyBackupPath()); err != nil {
		return false, time.Time{}
	}
	return true, marker.ExpiresAt
}

func (c *client) cleanupExpiredLegacyBackup() {
	raw, err := os.ReadFile(c.legacyMarkerPath())
	if err != nil {
		return
	}
	var marker legacyMigrationMarker
	if json.Unmarshal(raw, &marker) == nil && marker.Version == 1 && marker.ExpiresAt.After(time.Now().UTC()) {
		return
	}
	// This is an encrypted legacy backup created by this process. Expiry is
	// enforced on every normal credential-store read, not only diagnostics.
	_ = os.Remove(c.legacyBackupPath())
	_ = os.Remove(c.legacyMarkerPath())
}

// migrateLegacyCredentialStore performs the one permitted use of the former
// local gateway token. The reissued Agent credential proves the copied gateway
// registration remains valid; a revoke or membership removal is never masked.
func (c *client) migrateLegacyCredentialStore(ctx context.Context) error {
	if c.secretStore == nil {
		return errors.New("OS keychain is unavailable; cannot migrate Baley credentials safely")
	}
	if c.gatewayToken == "" {
		return errors.New("BALEY_MCP_GATEWAY_TOKEN is required once to migrate a legacy Baley credential store")
	}
	store, err := c.readCredentialStore(ctx)
	if err != nil {
		return err
	}
	for workspaceID, credential := range store.Workspaces {
		if credential.GatewaySecret == "" || store.GatewayID == "" {
			continue
		}
		response, resumeErr := c.resumeWorkspaceGateway(ctx, workspaceID, store.GatewayID, credential.GatewaySecret)
		if resumeErr != nil {
			var statusError connectionHTTPStatusError
			if errors.As(resumeErr, &statusError) && statusError.StatusCode == http.StatusUnauthorized {
				delete(store.Workspaces, workspaceID)
				continue
			}
			return fmt.Errorf("validate migrated gateway registration: %w", resumeErr)
		}
		if response.AgentToken == "" {
			return errors.New("Baley returned no Agent credential while validating migrated gateway")
		}
		credential.AgentToken, credential.ConnectedAt = response.AgentToken, time.Now().UTC()
		store.Workspaces[workspaceID] = credential
	}
	return c.writeCredentialStore(ctx, &store)
}

func (c *client) rollbackLegacyCredentialStore() error {
	if c.secretStore == nil {
		return errors.New("OS keychain is unavailable; no tokenless credential store can be rolled back")
	}
	if c.gatewayToken == "" {
		return errors.New("BALEY_MCP_GATEWAY_TOKEN is required for the one-time local legacy rollback")
	}
	eligible, _ := c.legacyRollbackEligible()
	if !eligible {
		return errors.New("no eligible legacy credential rollback is available")
	}
	backup, err := os.ReadFile(c.legacyBackupPath())
	if err != nil {
		return err
	}
	var legacy credentialStoreDisk
	if json.Unmarshal(backup, &legacy) != nil || legacy.Ciphertext == "" {
		return errors.New("legacy credential rollback data is invalid")
	}
	raw, err := os.ReadFile(c.credentialStorePath)
	if err != nil {
		return err
	}
	var current credentialStoreDisk
	if json.Unmarshal(raw, &current) != nil {
		return errors.New("current Baley credential store is invalid")
	}
	if err = writePrivateFile(c.credentialStorePath, backup); err != nil {
		return err
	}
	if current.KeyRef != "" {
		if err = c.secretStore.Delete(credentialKeychainService, current.KeyRef); err != nil {
			return fmt.Errorf("remove rolled-back Baley keychain credential: %w", err)
		}
	}
	_ = os.Remove(c.legacyBackupPath())
	_ = os.Remove(c.legacyMarkerPath())
	return nil
}

func (c *client) writeCredentialStore(ctx context.Context, store *credentialStore) error {
	path := c.credentialStorePath
	if path == "" {
		return errors.New("Baley credential store is not configured")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create Baley credential directory: %w", err)
	}
	store.Version = 5
	store.ServerURL = c.base
	raw, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	if c.secretStore != nil {
		if store.KeyRef == "" {
			store.KeyRef, err = credentialStoreKeyRef()
			if err != nil {
				return err
			}
			raw, err = json.MarshalIndent(store, "", "  ")
			if err != nil {
				return err
			}
		}
		if err = c.secretStore.Set(credentialKeychainService, store.KeyRef, base64.RawURLEncoding.EncodeToString(raw)); err != nil {
			return fmt.Errorf("write Baley device secret to OS keychain: %w", err)
		}
		raw, err = json.MarshalIndent(credentialStoreDisk{Version: 5, ServerURL: c.base, KeyRef: store.KeyRef}, "", "  ")
		if err != nil {
			return err
		}
	} else if c.gatewayToken != "" {
		ciphertext, encryptErr := c.encryptCredentialStore(raw)
		if encryptErr != nil {
			return fmt.Errorf("encrypt Baley credential store: %w", encryptErr)
		}
		raw, err = json.MarshalIndent(credentialStoreDisk{Version: 4, ServerURL: c.base, Ciphertext: ciphertext}, "", "  ")
		if err != nil {
			return err
		}
	}
	if err = writePrivateFile(path, raw); err != nil {
		return fmt.Errorf("write Baley credential store: %w", err)
	}
	return nil
}

func credentialStoreKeyRef() (string, error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate Baley keychain reference: %w", err)
	}
	return "credential-store-" + base64.RawURLEncoding.EncodeToString(raw), nil
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
	return c.writeCredentialStore(ctx, &store)
}
