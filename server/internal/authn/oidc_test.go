package authn

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
)

type oidcStoreStub struct {
	flow     OIDCFlow
	identity ExternalIdentity
}

func (s *oidcStoreStub) CreateOIDCFlow(_ context.Context, flow OIDCFlow) error {
	s.flow = flow
	return nil
}
func (s *oidcStoreStub) ConsumeOIDCFlow(_ context.Context, stateHash []byte, now time.Time) (OIDCFlow, error) {
	if !bytes.Equal(stateHash, s.flow.StateHash) || !now.Before(s.flow.ExpiresAt) {
		return OIDCFlow{}, errors.New("flow not found")
	}
	return s.flow, nil
}
func (s *oidcStoreStub) ExternalIdentity(context.Context, string, string, time.Time) (ExternalIdentity, error) {
	return s.identity, nil
}
func (s *oidcStoreStub) CreateExternalIdentityAccount(context.Context, string, string, string, string, time.Time) (ExternalIdentity, error) {
	return s.identity, nil
}
func (s *oidcStoreStub) LinkExternalIdentity(context.Context, string, string, string, string, string, time.Time) (ExternalIdentity, error) {
	return s.identity, nil
}
func (*oidcStoreStub) RevokeAccountSessions(context.Context, string, time.Time) error { return nil }

func TestOIDCStateEncryptionIsKeyBoundAndTamperEvident(t *testing.T) {
	key := []byte("01234567890123456789012345678901")
	plain := []byte("pkce-verifier-not-a-token")
	ciphertext, err := encryptOIDC(key, plain)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(ciphertext, plain) {
		t.Fatal("PKCE verifier is present in stored ciphertext")
	}
	got, err := decryptOIDC(key, ciphertext)
	if err != nil || !bytes.Equal(got, plain) {
		t.Fatalf("round trip = %q, %v", got, err)
	}
	ciphertext[len(ciphertext)-1] ^= 1
	if _, err = decryptOIDC(key, ciphertext); err == nil {
		t.Fatal("tampered OIDC flow ciphertext was accepted")
	}
}

func TestNormalizeOIDCProviderRequiresTLSIssuerAndRedirect(t *testing.T) {
	base := OIDCProviderConfig{ID: "internal", ClientID: "client", ClientSecret: "secret", Issuer: "https://id.example", RedirectURL: "https://app.example/v1/auth/oidc/internal/callback"}
	if _, err := normalizeOIDCProvider(base); err != nil {
		t.Fatal(err)
	}
	base.Issuer = "http://id.example"
	if _, err := normalizeOIDCProvider(base); err == nil {
		t.Fatal("HTTP issuer accepted")
	}
	base.Issuer, base.RedirectURL = "https://id.example", "not a URL"
	if _, err := normalizeOIDCProvider(base); err == nil {
		t.Fatal("invalid redirect accepted")
	}
	base.RedirectURL = "http://app.example/v1/auth/oidc/internal/callback"
	if _, err := normalizeOIDCProvider(base); err == nil {
		t.Fatal("HTTP redirect accepted")
	}
}

func TestOIDCCallbackIssuesConfiguredLongLivedSession(t *testing.T) {
	base := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	var issuer, nonce string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"issuer": issuer, "authorization_endpoint": issuer + "/authorize", "token_endpoint": issuer + "/token",
				"jwks_uri": issuer + "/keys", "response_types_supported": []string{"code"},
				"subject_types_supported": []string{"public"}, "id_token_signing_alg_values_supported": []string{"RS256"},
			})
		case "/keys":
			_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
				"kty": "RSA", "kid": "test-key", "use": "sig", "alg": "RS256",
				"n": base64.RawURLEncoding.EncodeToString(key.PublicKey.N.Bytes()),
				"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.PublicKey.E)).Bytes()),
			}}})
		case "/token":
			idToken, err := signedTestIDToken(key, issuer, nonce, time.Now().UTC())
			if err != nil {
				http.Error(w, "could not sign test token", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "access-token", "token_type": "Bearer", "expires_in": 3600, "id_token": idToken,
			})
		default:
			http.NotFound(w, r)
		}
	})
	providerServer := httptest.NewTLSServer(handler)
	defer providerServer.Close()
	issuer = providerServer.URL

	identity := ExternalIdentity{AccountID: "account-1", ActorID: "actor-1", LoginID: "oidc:account-1", DisplayName: "Google Owner", Status: "active"}
	sessionStore := &sessionStoreStub{account: AccountCredential{
		AccountID: identity.AccountID, ActorID: identity.ActorID, LoginID: identity.LoginID, DisplayName: identity.DisplayName, Status: identity.Status,
	}}
	sessions, err := NewServiceWithPolicy(sessionStore, SessionPolicy{IdleTTL: 30 * 24 * time.Hour, AbsoluteTTL: 90 * 24 * time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	sessions.now = func() time.Time { return base }
	provider := OIDCProviderConfig{
		ID: "google", Label: "Google", Issuer: issuer, ClientID: "client-id", ClientSecret: "client-secret",
		RedirectURL: "https://baley.example/v1/auth/oidc/google/callback",
	}
	oidcStore := &oidcStoreStub{identity: identity}
	oidcService, err := NewOIDCService(oidcStore, sessions, []OIDCProviderConfig{provider}, "test-state-secret")
	if err != nil {
		t.Fatal(err)
	}
	oidcService.now = func() time.Time { return base }
	ctx := oidc.ClientContext(context.Background(), providerServer.Client())
	authURL, err := oidcService.Start(ctx, "google", "browser-binding", "login", "")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(authURL)
	if err != nil {
		t.Fatal(err)
	}
	state, nonce := parsed.Query().Get("state"), parsed.Query().Get("nonce")
	if state == "" || nonce == "" {
		t.Fatalf("OIDC authorization URL lacks state or nonce: %s", authURL)
	}

	result, intent, err := oidcService.Complete(ctx, "google", state, "browser-binding", "authorization-code")
	if err != nil {
		t.Fatal(err)
	}
	if intent != "login" || result.AccountID != identity.AccountID {
		t.Fatalf("callback result intent/account = %q/%q", intent, result.AccountID)
	}
	if !sessionStore.created.IdleExpiresAt.Equal(base.Add(30*24*time.Hour)) || !sessionStore.created.AbsoluteAt.Equal(base.Add(90*24*time.Hour)) {
		t.Fatalf("OIDC session expiry = %s/%s", sessionStore.created.IdleExpiresAt, sessionStore.created.AbsoluteAt)
	}
}

func signedTestIDToken(key *rsa.PrivateKey, issuer, nonce string, now time.Time) (string, error) {
	header, _ := json.Marshal(map[string]string{"alg": "RS256", "kid": "test-key", "typ": "JWT"})
	claims, _ := json.Marshal(map[string]any{
		"iss": issuer, "sub": "google-subject", "aud": "client-id", "exp": now.Add(time.Hour).Unix(),
		"iat": now.Unix(), "nonce": nonce, "name": "Google Owner",
	})
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(claims)
	digest := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return strings.Join([]string{unsigned, base64.RawURLEncoding.EncodeToString(signature)}, "."), nil
}
