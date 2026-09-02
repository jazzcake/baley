package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
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
	var renewals int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/mcp/gateway-sessions":
			if r.Method != http.MethodPost {
				t.Fatalf("method=%s", r.Method)
			}
			renewals++
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
	third := &client{base: server.URL, http: server.Client(), credentialStorePath: path, secretStore: keychain}
	result, _, err = third.workspaceGet(context.Background(), nil, workspaceInput{WorkspaceID: workspaceID})
	if err != nil || result == nil || result.IsError || renewals != 2 {
		t.Fatalf("fresh keychain-backed process did not renew a new Agent credential: result=%#v err=%v renewals=%d", result, err, renewals)
	}
	for _, value := range keychain.values {
		if strings.Contains(value, agentToken) || strings.Contains(value, `"agentToken"`) {
			t.Fatalf("keychain payload persisted an Agent credential: %s", value)
		}
	}
}

func TestKeychainRegisteredGatewayAutoEnrollsMemberWorkspaceWithoutExtraLoginLink(t *testing.T) {
	const proofWorkspaceID = "410f335e-ddb2-443f-be3c-7d1d18ccd534"
	const targetWorkspaceID = "510f335e-ddb2-443f-be3c-7d1d18ccd534"
	const proofSecret = "registered-device-proof"
	const targetSecret = "new-member-workspace-secret"
	const agentToken = "member-workspace-token"
	keychain := &memorySecretStore{}
	var enrollments, browserRequests int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/mcp/gateway-enrollments":
			enrollments++
			var input map[string]string
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				t.Fatal(err)
			}
			if input["workspaceId"] != targetWorkspaceID || input["proofWorkspaceId"] != proofWorkspaceID || input["gatewayId"] != "device-1" || input["proofGatewaySecret"] != proofSecret || input["agentActorId"] != "" {
				t.Fatalf("unexpected auto-enrollment proof: %#v", input)
			}
			_, _ = w.Write([]byte(`{"workspaceId":"` + targetWorkspaceID + `","agentToken":"` + agentToken + `","gatewayId":"device-1","gatewaySecret":"` + targetSecret + `"}`))
		case "/v1/mcp/login-links":
			browserRequests++
			http.Error(w, "another browser login link should not be needed", http.StatusInternalServerError)
		case "/v1/workspaces/" + targetWorkspaceID:
			if r.Header.Get("Authorization") != "Bearer "+agentToken {
				t.Fatalf("authorization=%q", r.Header.Get("Authorization"))
			}
			_, _ = w.Write([]byte(`{"id":"` + targetWorkspaceID + `"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()
	path := filepath.Join(t.TempDir(), "credentials.json")
	c := &client{base: upstream.URL, http: upstream.Client(), credentialStorePath: path, secretStore: keychain}
	if err := c.writeCredentialStore(context.Background(), &credentialStore{ServerURL: upstream.URL, GatewayID: "device-1", Workspaces: map[string]workspaceCredential{proofWorkspaceID: {GatewaySecret: proofSecret}}}); err != nil {
		t.Fatal(err)
	}
	result, _, err := c.workspaceGet(context.Background(), nil, workspaceInput{WorkspaceID: targetWorkspaceID})
	if err != nil || result == nil || result.IsError || enrollments != 1 || browserRequests != 0 {
		t.Fatalf("member Workspace was not auto-enrolled: result=%#v err=%v enrollments=%d browser=%d", result, err, enrollments, browserRequests)
	}
	store, err := c.readCredentialStore(context.Background())
	if err != nil || store.Workspaces[targetWorkspaceID].GatewaySecret != targetSecret {
		t.Fatalf("auto-enrollment did not persist target device credential: %#v %v", store.Workspaces[targetWorkspaceID], err)
	}
}

func TestCredentialStoreLockPreservesConcurrentGatewayEnrollments(t *testing.T) {
	const proofWorkspaceID = "proof-workspace"
	keychain := &memorySecretStore{}
	var mu sync.Mutex
	var enrolled = map[string]bool{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/mcp/gateway-enrollments" {
			http.NotFound(w, r)
			return
		}
		var input map[string]string
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			t.Fatal(err)
		}
		workspaceID := input["workspaceId"]
		mu.Lock()
		enrolled[workspaceID] = true
		mu.Unlock()
		_, _ = w.Write([]byte(`{"workspaceId":"` + workspaceID + `","agentToken":"token-` + workspaceID + `","gatewayId":"device-1","gatewaySecret":"secret-` + workspaceID + `"}`))
	}))
	defer upstream.Close()
	path := filepath.Join(t.TempDir(), "credentials.json")
	seed := &client{base: upstream.URL, http: upstream.Client(), credentialStorePath: path, secretStore: keychain}
	if err := seed.writeCredentialStore(context.Background(), &credentialStore{ServerURL: upstream.URL, GatewayID: "device-1", Workspaces: map[string]workspaceCredential{proofWorkspaceID: {GatewaySecret: "proof-secret"}}}); err != nil {
		t.Fatal(err)
	}
	clients := []*client{
		{base: upstream.URL, http: upstream.Client(), credentialStorePath: path, secretStore: keychain},
		{base: upstream.URL, http: upstream.Client(), credentialStorePath: path, secretStore: keychain},
	}
	workspaces := []string{"member-a", "member-b"}
	errCh := make(chan error, len(workspaces))
	var group sync.WaitGroup
	for index, workspaceID := range workspaces {
		group.Add(1)
		go func(c *client, target string) {
			defer group.Done()
			token, pending, err := c.workspaceCredential(context.Background(), target)
			if err != nil || pending != nil || token == "" {
				errCh <- fmt.Errorf("%s enrollment token=%q pending=%v err=%w", target, token, pending != nil, err)
			}
		}(clients[index], workspaceID)
	}
	group.Wait()
	close(errCh)
	for err := range errCh {
		t.Error(err)
	}
	store, err := seed.readCredentialStore(context.Background())
	if err != nil || store.Workspaces["member-a"].GatewaySecret != "secret-member-a" || store.Workspaces["member-b"].GatewaySecret != "secret-member-b" {
		t.Fatalf("concurrent Gateway writes lost a Workspace credential: store=%#v err=%v", store.Workspaces, err)
	}
	mu.Lock()
	defer mu.Unlock()
	if !enrolled["member-a"] || !enrolled["member-b"] {
		t.Fatalf("concurrent Gateway enrollments did not both reach the API: %#v", enrolled)
	}
	release, err := seed.lockCredentialStore(context.Background())
	if err != nil {
		t.Fatalf("credential store lock remained held after concurrent enrollments: %v", err)
	}
	release()
}

func TestCredentialStoreLockIsReleasedAfterKilledOwner(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "credentials.json")
	ready := filepath.Join(t.TempDir(), "holder-ready")
	command := exec.Command(os.Args[0], "-test.run=TestCredentialStoreLockHolder")
	command.Env = append(os.Environ(), "BALEY_CREDENTIAL_LOCK_HOLDER=1", "BALEY_CREDENTIAL_LOCK_PATH="+path, "BALEY_CREDENTIAL_LOCK_READY="+ready, "BALEY_CREDENTIAL_LOCK_CRASH_AFTER_MS=200")
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("credential lock helper did not acquire its lock")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err := command.Wait(); err != nil {
		// The helper exits without running defers, which simulates process death.
		if _, ok := err.(*exec.ExitError); !ok {
			t.Fatal(err)
		}
	}
	c := &client{credentialStorePath: path}
	release, err := c.lockCredentialStore(context.Background())
	if err != nil {
		t.Fatalf("OS-managed credential lock remained held after owner death: %v", err)
	}
	release()
}

func TestCredentialStoreLockHolder(t *testing.T) {
	if os.Getenv("BALEY_CREDENTIAL_LOCK_HOLDER") != "1" {
		return
	}
	path, ready := os.Getenv("BALEY_CREDENTIAL_LOCK_PATH"), os.Getenv("BALEY_CREDENTIAL_LOCK_READY")
	c := &client{credentialStorePath: path}
	release, err := c.lockCredentialStore(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if err = os.WriteFile(ready, []byte("ready"), 0o600); err != nil {
		t.Fatal(err)
	}
	crashAfter, err := time.ParseDuration(os.Getenv("BALEY_CREDENTIAL_LOCK_CRASH_AFTER_MS") + "ms")
	if err != nil || crashAfter <= 0 {
		t.Fatal("credential lock helper requires a crash delay")
	}
	time.Sleep(crashAfter)
	os.Exit(88)
}

func TestLegacyTokenStoreMigratesToKeychainAndForcesGatewayRenewal(t *testing.T) {
	const workspaceID = "410f335e-ddb2-443f-be3c-7d1d18ccd534"
	const legacyToken = "legacy-local-gateway-token"
	const gatewaySecret = "registered-gateway-secret"
	path := filepath.Join(t.TempDir(), "credentials.json")
	legacy := &client{base: "https://baley.example/api", gatewayToken: legacyToken, credentialStorePath: path}
	legacyPayload := []byte(`{"version":5,"serverUrl":"` + legacy.base + `","gatewayId":"device-1","workspaces":{"` + workspaceID + `":{"agentToken":"old-agent-token","gatewaySecret":"` + gatewaySecret + `"}}}`)
	ciphertext, err := legacy.encryptCredentialStore(legacyPayload)
	if err != nil {
		t.Fatal(err)
	}
	if err = writePrivateFile(path, []byte(`{"version":4,"serverUrl":"`+legacy.base+`","ciphertext":"`+ciphertext+`"}`)); err != nil {
		t.Fatal(err)
	}

	keychain := &memorySecretStore{}
	migrated := &client{base: legacy.base, gatewayToken: legacyToken, credentialStorePath: path, secretStore: keychain}
	store, err := migrated.readCredentialStore(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	credential := store.Workspaces[workspaceID]
	if credential.GatewaySecret != gatewaySecret {
		t.Fatalf("migration did not preserve gateway secret: %#v", credential)
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
	for _, value := range keychain.values {
		if strings.Contains(value, "old-agent-token") || strings.Contains(value, `"agentToken"`) {
			t.Fatalf("migration retained a persisted Agent credential: %s", value)
		}
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

func TestExpiredLegacyRollbackIsRemoved(t *testing.T) {
	path := filepath.Join(t.TempDir(), "credentials.json")
	c := &client{credentialStorePath: path}
	if err := writePrivateFile(c.legacyBackupPath(), []byte(`{"version":4,"ciphertext":"encrypted"}`)); err != nil {
		t.Fatal(err)
	}
	marker, err := json.Marshal(legacyMigrationMarker{Version: 1, ExpiresAt: time.Now().UTC().Add(-time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	if err = writePrivateFile(c.legacyMarkerPath(), marker); err != nil {
		t.Fatal(err)
	}
	if eligible, _ := c.legacyRollbackEligible(); eligible {
		t.Fatal("expired rollback remained eligible")
	}
	for _, candidate := range []string{c.legacyBackupPath(), c.legacyMarkerPath()} {
		if _, err := os.Stat(candidate); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("expired migration artifact remained at %s: %v", candidate, err)
		}
	}
}
