package main

import "testing"

func TestResolveRuntimeConfigAuthModeContract(t *testing.T) {
	tests := []struct {
		name, environment, requestedMode string
		wantMode                         string
		wantSecure                       bool
		wantError                        bool
	}{
		{name: "development defaults to legacy", environment: "development", wantMode: "legacy"},
		{name: "test permits explicit legacy", environment: "test", requestedMode: "legacy", wantMode: "legacy"},
		{name: "development permits enforced", environment: "development", requestedMode: "enforced", wantMode: "enforced"},
		{name: "production defaults to enforced", environment: "production", wantMode: "enforced", wantSecure: true},
		{name: "staging defaults to enforced", environment: "staging", wantMode: "enforced", wantSecure: true},
		{name: "production rejects legacy", environment: "production", requestedMode: "legacy", wantError: true},
		{name: "staging rejects legacy", environment: "staging", requestedMode: "legacy", wantError: true},
		{name: "unknown mode is rejected", environment: "development", requestedMode: "optional", wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveRuntimeConfig(tt.environment, tt.requestedMode, "")
			if (err != nil) != tt.wantError {
				t.Fatalf("resolveRuntimeConfig() error = %v, wantError %v", err, tt.wantError)
			}
			if err == nil && (got.AuthMode != tt.wantMode || got.CookieSecure != tt.wantSecure) {
				t.Fatalf("resolveRuntimeConfig() = %#v, want mode=%q secure=%v", got, tt.wantMode, tt.wantSecure)
			}
		})
	}
}

func TestResolveRuntimeConfigCookieOverride(t *testing.T) {
	got, err := resolveRuntimeConfig("development", "enforced", "false")
	if err != nil {
		t.Fatal(err)
	}
	if got.CookieSecure {
		t.Fatal("explicit BALEY_COOKIE_SECURE=false was not honored")
	}
	if _, err = resolveRuntimeConfig("production", "enforced", "false"); err == nil {
		t.Fatal("production accepted insecure authentication cookies")
	}
	if _, err = resolveRuntimeConfig("development", "legacy", "sometimes"); err == nil {
		t.Fatal("invalid boolean was accepted")
	}
}
