package authn

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

var (
	ErrOIDCUnavailable = errors.New("OIDC provider is not configured")
	ErrOIDCInvalid     = errors.New("OIDC authentication could not be verified")
)

type OIDCProviderConfig struct {
	ID, Label, Issuer, ClientID, ClientSecret, RedirectURL string
	Scopes                                                 []string
}

type OIDCFlow struct {
	StateHash, NonceHash, BindingHash, VerifierCiphertext []byte
	ProviderID, Intent, LinkAccountID                     string
	ExpiresAt                                             time.Time
}

type ExternalIdentity struct {
	AccountID, ActorID, LoginID, DisplayName, Status string
}

// OIDCStore is deliberately separate from password authentication. It keeps
// an external principal's immutable issuer/subject pair as its identity.
type OIDCStore interface {
	CreateOIDCFlow(context.Context, OIDCFlow) error
	ConsumeOIDCFlow(context.Context, []byte, time.Time) (OIDCFlow, error)
	ExternalIdentity(context.Context, string, string, time.Time) (ExternalIdentity, error)
	CreateExternalIdentityAccount(context.Context, string, string, string, string, time.Time) (ExternalIdentity, error)
	LinkExternalIdentity(context.Context, string, string, string, string, string, time.Time) (ExternalIdentity, error)
	RevokeAccountSessions(context.Context, string, time.Time) error
}

type OIDCService struct {
	store     OIDCStore
	sessions  *Service
	providers map[string]OIDCProviderConfig
	stateKey  []byte
	now       func() time.Time
	newClient func(context.Context, OIDCProviderConfig) (*oidc.Provider, error)
}

func NewOIDCService(store OIDCStore, sessions *Service, providers []OIDCProviderConfig, stateSecret string) (*OIDCService, error) {
	if len(providers) == 0 {
		return nil, nil
	}
	key := sha256.Sum256([]byte(stateSecret))
	if strings.TrimSpace(stateSecret) == "" {
		return nil, errors.New("BALEY_OIDC_STATE_SECRET is required when OIDC is configured")
	}
	values := make(map[string]OIDCProviderConfig, len(providers))
	for _, raw := range providers {
		value, err := normalizeOIDCProvider(raw)
		if err != nil {
			return nil, err
		}
		if _, exists := values[value.ID]; exists {
			return nil, fmt.Errorf("duplicate OIDC provider %q", value.ID)
		}
		values[value.ID] = value
	}
	return &OIDCService{store: store, sessions: sessions, providers: values, stateKey: key[:], now: time.Now,
		newClient: func(ctx context.Context, config OIDCProviderConfig) (*oidc.Provider, error) {
			return oidc.NewProvider(ctx, config.Issuer)
		}}, nil
}

func normalizeOIDCProvider(value OIDCProviderConfig) (OIDCProviderConfig, error) {
	value.ID, value.Label = strings.TrimSpace(value.ID), strings.TrimSpace(value.Label)
	value.Issuer, value.ClientID, value.ClientSecret, value.RedirectURL = strings.TrimSpace(value.Issuer), strings.TrimSpace(value.ClientID), strings.TrimSpace(value.ClientSecret), strings.TrimSpace(value.RedirectURL)
	if value.ID == "" || len(value.ID) > 80 || value.ClientID == "" || value.ClientSecret == "" {
		return value, errors.New("OIDC provider id, client ID and client secret are required")
	}
	issuer, err := url.Parse(value.Issuer)
	if err != nil || issuer.Scheme != "https" || issuer.Host == "" || issuer.RawQuery != "" || issuer.Fragment != "" {
		return value, fmt.Errorf("OIDC provider %q has an invalid issuer", value.ID)
	}
	redirect, err := url.Parse(value.RedirectURL)
	if err != nil || redirect.Scheme == "" || redirect.Host == "" || redirect.RawQuery != "" || redirect.Fragment != "" {
		return value, fmt.Errorf("OIDC provider %q has an invalid redirect URL", value.ID)
	}
	if value.Label == "" {
		value.Label = value.ID
	}
	if len(value.Scopes) == 0 {
		value.Scopes = []string{oidc.ScopeOpenID, "profile", "email"}
	}
	if value.Scopes[0] != oidc.ScopeOpenID {
		value.Scopes = append([]string{oidc.ScopeOpenID}, value.Scopes...)
	}
	return value, nil
}

func (s *OIDCService) Providers() []map[string]string {
	items := make([]map[string]string, 0, len(s.providers))
	for _, value := range s.providers {
		items = append(items, map[string]string{"id": value.ID, "label": value.Label})
	}
	return items
}

func (s *OIDCService) Start(ctx context.Context, providerID, binding, intent, linkAccountID string) (string, error) {
	provider, ok := s.providers[strings.TrimSpace(providerID)]
	if !ok {
		return "", ErrOIDCUnavailable
	}
	if intent != "login" && intent != "link" {
		return "", ErrOIDCInvalid
	}
	if intent == "link" && strings.TrimSpace(linkAccountID) == "" {
		return "", ErrOIDCInvalid
	}
	state, stateHash, err := oidcRandom()
	if err != nil {
		return "", err
	}
	nonce, nonceHash, err := oidcRandom()
	if err != nil {
		return "", err
	}
	verifier, _, err := oidcRandom()
	if err != nil {
		return "", err
	}
	bindingHash := sha256.Sum256([]byte(binding))
	ciphertext, err := encryptOIDC(s.stateKey, []byte(verifier))
	if err != nil {
		return "", err
	}
	now := s.now().UTC()
	if err = s.store.CreateOIDCFlow(ctx, OIDCFlow{StateHash: stateHash, NonceHash: nonceHash, BindingHash: bindingHash[:], VerifierCiphertext: ciphertext, ProviderID: provider.ID, Intent: intent, LinkAccountID: linkAccountID, ExpiresAt: now.Add(10 * time.Minute)}); err != nil {
		return "", err
	}
	config := oauth2.Config{ClientID: provider.ClientID, ClientSecret: provider.ClientSecret, Endpoint: oauth2.Endpoint{AuthURL: strings.TrimRight(provider.Issuer, "/") + "/authorize"}, RedirectURL: provider.RedirectURL, Scopes: provider.Scopes}
	// AuthURL comes from discovery in Complete's provider, but the OIDC standard
	// discovery endpoint is the only portable source. Fetch it here too.
	discovery, err := s.newClient(ctx, provider)
	if err != nil {
		return "", ErrOIDCUnavailable
	}
	config.Endpoint = discovery.Endpoint()
	return config.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier), oidc.Nonce(nonce)), nil
}

func (s *OIDCService) Complete(ctx context.Context, providerID, state, binding, code string) (LoginResult, string, error) {
	provider, ok := s.providers[strings.TrimSpace(providerID)]
	if !ok {
		return LoginResult{}, "", ErrOIDCInvalid
	}
	stateHash := sha256.Sum256([]byte(state))
	flow, err := s.store.ConsumeOIDCFlow(ctx, stateHash[:], s.now().UTC())
	if err != nil || flow.ProviderID != provider.ID {
		return LoginResult{}, "", ErrOIDCInvalid
	}
	bindingHash := sha256.Sum256([]byte(binding))
	if !equalBytes(flow.BindingHash, bindingHash[:]) {
		return LoginResult{}, "", ErrOIDCInvalid
	}
	verifier, err := decryptOIDC(s.stateKey, flow.VerifierCiphertext)
	if err != nil {
		return LoginResult{}, "", ErrOIDCInvalid
	}
	discovery, err := s.newClient(ctx, provider)
	if err != nil {
		return LoginResult{}, "", ErrOIDCUnavailable
	}
	config := oauth2.Config{ClientID: provider.ClientID, ClientSecret: provider.ClientSecret, Endpoint: discovery.Endpoint(), RedirectURL: provider.RedirectURL}
	token, err := config.Exchange(ctx, code, oauth2.VerifierOption(string(verifier)))
	if err != nil {
		return LoginResult{}, "", ErrOIDCInvalid
	}
	raw, ok := token.Extra("id_token").(string)
	if !ok || raw == "" {
		return LoginResult{}, "", ErrOIDCInvalid
	}
	idToken, err := discovery.Verifier(&oidc.Config{ClientID: provider.ClientID}).Verify(ctx, raw)
	if err != nil || idToken.Issuer != provider.Issuer {
		return LoginResult{}, "", ErrOIDCInvalid
	}
	nonceHash := sha256.Sum256([]byte(idToken.Nonce))
	if !equalBytes(flow.NonceHash, nonceHash[:]) || strings.TrimSpace(idToken.Subject) == "" {
		return LoginResult{}, "", ErrOIDCInvalid
	}
	displayName := oidcDisplayName(idToken)
	var identity ExternalIdentity
	if flow.Intent == "link" {
		identity, err = s.store.LinkExternalIdentity(ctx, flow.LinkAccountID, provider.ID, idToken.Issuer, idToken.Subject, displayName, s.now().UTC())
	} else {
		identity, err = s.store.ExternalIdentity(ctx, idToken.Issuer, idToken.Subject, s.now().UTC())
		if err != nil {
			identity, err = s.store.CreateExternalIdentityAccount(ctx, provider.ID, idToken.Issuer, idToken.Subject, displayName, s.now().UTC())
		}
	}
	if err != nil || identity.Status != "active" {
		return LoginResult{}, "", ErrOIDCInvalid
	}
	if flow.Intent == "link" {
		_ = s.store.RevokeAccountSessions(ctx, identity.AccountID, s.now().UTC())
	}
	result, err := s.sessions.CreateSessionForAccount(ctx, AccountCredential{AccountID: identity.AccountID, ActorID: identity.ActorID, LoginID: identity.LoginID, DisplayName: identity.DisplayName, Status: identity.Status})
	if err != nil {
		return LoginResult{}, "", err
	}
	return result, flow.Intent, nil
}

func oidcDisplayName(token *oidc.IDToken) string {
	var claims struct {
		Name string `json:"name"`
	}
	_ = token.Claims(&claims)
	value := strings.TrimSpace(claims.Name)
	if value == "" {
		return "OIDC user"
	}
	if len([]rune(value)) > 120 {
		return string([]rune(value)[:120])
	}
	return value
}
func oidcRandom() (string, []byte, error) {
	raw := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, raw); err != nil {
		return "", nil, err
	}
	value := base64.RawURLEncoding.EncodeToString(raw)
	hash := sha256.Sum256([]byte(value))
	return value, hash[:], nil
}
func encryptOIDC(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}
func decryptOIDC(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) < gcm.NonceSize() {
		return nil, ErrOIDCInvalid
	}
	return gcm.Open(nil, ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():], nil)
}

// Compile-time use of encoding/json makes it explicit that no untyped claims
// (notably email) are persisted as identity data.
var _ = json.RawMessage{}
