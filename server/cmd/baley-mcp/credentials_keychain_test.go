package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

type memorySecretStore struct {
	mu     sync.Mutex
	values map[string]string
}

func (s *memorySecretStore) secretKey(service, key string) string { return service + "/" + key }

func (s *memorySecretStore) Get(service, key string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.values[s.secretKey(service, key)]
	if !ok {
		return "", errors.New("secret not found")
	}
	return value, nil
}

func (s *memorySecretStore) Set(service, key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.values == nil {
		s.values = map[string]string{}
	}
	s.values[s.secretKey(service, key)] = value
	return nil
}

func (s *memorySecretStore) Delete(service, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.values, s.secretKey(service, key))
	return nil
}

func TestKeychainStoreResumesGatewayWithoutGatewayToken(t *testing.T) {
	const workspaceID = "410f335e-ddb2-443f-be3c-7d1d18ccd534"
	const gatewaySecret = "registered-gateway-secret"
	const agentToken = "resumed-agent-token"
	keychain := &memorySecretStore{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/mcp/gateway-sessions":
			if r.Method != http.MethodPost {
				t.Fatalf("method=%s", r.Method)
			}
			_, _ = w.Write([]byte(`{"agentToken":"` + agentToken + `"}`))
		case "/v1/workspaces/" + workspaceID:
			if r.Header.Get("Authorization") != "Bearer "+agentToken {
				t.Fatalf("authorization=%q", r.Header.Get("Authorization"))
			}
			_, _ = w.Write([]byte(`{"id":"` + workspaceID + `"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	path := filepath.Join(t.TempDir(), "credentials.json")
	first := &client{base: server.URL, http: server.Client(), credentialStorePath: path, secretStore: keychain}
	if err := first.writeCredentialStore(context.Background(), &credentialStore{ServerURL: server.URL, GatewayID: "device-1", Workspaces: map[string]workspaceCredential{workspaceID: {GatewaySecret: gatewaySecret}}}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), gatewaySecret) || strings.Contains(string(raw), agentToken) || !strings.Contains(string(raw), "keyRef") {
		t.Fatalf("credential file leaked secret or lacks key reference: %s", raw)
	}

	second := &client{base: server.URL, http: server.Client(), credentialStorePath: path, secretStore: keychain}
	result, _, err := second.workspaceGet(context.Background(), nil, workspaceInput{WorkspaceID: workspaceID})
	if err != nil || result == nil || result.IsError {
		t.Fatalf("tokenless gateway resume failed: result=%#v err=%v", result, err)
	}
}

func TestLegacyTokenStoreMigratesToKeychainAndForcesGatewayRenewal(t *testing.T) {
	const workspaceID = "410f335e-ddb2-443f-be3c-7d1d18ccd534"
	const legacyToken = "legacy-local-gateway-token"
	const gatewaySecret = "registered-gateway-secret"
	path := filepath.Join(t.TempDir(), "credentials.json")
	legacy := &client{base: "https://baley.example/api", gatewayToken: legacyToken, credentialStorePath: path}
	if err := legacy.writeCredentialStore(context.Background(), &credentialStore{ServerURL: legacy.base, GatewayID: "device-1", Workspaces: map[string]workspaceCredential{workspaceID: {AgentToken: "old-agent-token", GatewaySecret: gatewaySecret}}}); err != nil {
		t.Fatal(err)
	}

	keychain := &memorySecretStore{}
	migrated := &client{base: legacy.base, gatewayToken: legacyToken, credentialStorePath: path, secretStore: keychain}
	store, err := migrated.readCredentialStore(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	credential := store.Workspaces[workspaceID]
	if credential.GatewaySecret != gatewaySecret || credential.AgentToken != "" {
		t.Fatalf("migration did not preserve gateway secret and clear stale Agent token: %#v", credential)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "ciphertext") || strings.Contains(string(raw), gatewaySecret) || !strings.Contains(string(raw), "keyRef") {
		t.Fatalf("legacy store was not replaced by non-secret keychain metadata: %s", raw)
	}
	if len(keychain.values) != 1 {
		t.Fatalf("keychain entries=%d, want 1", len(keychain.values))
	}
	if eligible, _ := migrated.legacyRollbackEligible(); !eligible {
		t.Fatal("legacy rollback was not available immediately after migration")
	}
	if err := migrated.rollbackLegacyCredentialStore(); err != nil {
		t.Fatal(err)
	}
	rolledBack, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(rolledBack), "ciphertext") || strings.Contains(string(rolledBack), gatewaySecret) {
		t.Fatalf("legacy rollback failed: raw=%s err=%v", rolledBack, err)
	}
	if len(keychain.values) != 0 {
		t.Fatalf("keychain entries=%d after rollback, want 0", len(keychain.values))
	}
}

func TestMigrateLegacyCredentialStoreRevalidatesAndDropsRevokedGateway(t *testing.T) {
	const workspaceID = "410f335e-ddb2-443f-be3c-7d1d18ccd534"
	const legacyToken = "legacy-local-gateway-token"
	path := filepath.Join(t.TempDir(), "credentials.json")
	legacy := &client{base: "https://baley.example/api", gatewayToken: legacyToken, credentialStorePath: path}
	if err := legacy.writeCredentialStore(context.Background(), &credentialStore{ServerURL: legacy.base, GatewayID: "device-1", Workspaces: map[string]workspaceCredential{workspaceID: {GatewaySecret: "revoked"}}}); err != nil {
		t.Fatal(err)
	}
	keychain := &memorySecretStore{}
	migrated := &client{base: legacy.base, gatewayToken: legacyToken, credentialStorePath: path, secretStore: keychain, http: &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusUnauthorized, Header: make(http.Header), Body: http.NoBody, Request: r}, nil
	})}}
	if err := migrated.migrateLegacyCredentialStore(context.Background()); err != nil {
		t.Fatal(err)
	}
	store, err := migrated.readCredentialStore(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(store.Workspaces) != 0 {
		t.Fatalf("revoked gateway survived migration: %#v", store.Workspaces)
	}
}
