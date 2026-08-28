package authn

import (
	"bytes"
	"testing"
)

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
