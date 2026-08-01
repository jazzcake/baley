package main

import (
	"strings"
	"testing"
)

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

func TestResolveViewerOriginsProductionContract(t *testing.T) {
	t.Setenv("BALEY_VIEWER_ORIGIN", "")
	t.Setenv("BALEY_VIEWER_ORIGINS", "")
	if _, err := resolveViewerOrigins("production"); err == nil || !strings.Contains(err.Error(), "required") {
		t.Fatalf("production accepted missing viewer origin: %v", err)
	}

	t.Setenv("BALEY_VIEWER_ORIGINS", "http://baley.example")
	if _, err := resolveViewerOrigins("production"); err == nil || !strings.Contains(err.Error(), "https") {
		t.Fatalf("production accepted HTTP viewer origin: %v", err)
	}

	t.Setenv("BALEY_VIEWER_ORIGINS", "https://baley.example, https://baley.example/")
	origins, err := resolveViewerOrigins("production")
	if err != nil {
		t.Fatal(err)
	}
	if len(origins) != 1 || origins[0] != "https://baley.example" {
		t.Fatalf("origins=%v", origins)
	}
}

func TestResolveViewerOriginsDevelopmentDefaults(t *testing.T) {
	t.Setenv("BALEY_VIEWER_ORIGIN", "")
	t.Setenv("BALEY_VIEWER_ORIGINS", "")
	origins, err := resolveViewerOrigins("development")
	if err != nil {
		t.Fatal(err)
	}
	if len(origins) != 2 {
		t.Fatalf("origins=%v", origins)
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
