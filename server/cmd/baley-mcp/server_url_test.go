package main

import "testing"

func TestValidateServerURLAllowsTailnetHTTPS(t *testing.T) {
	parsed, err := validateServerURL("https://jazzcake-home.tail87e929.ts.net/api")
	if err != nil {
		t.Fatalf("Tailnet HTTPS URL rejected: %v", err)
	}
	if parsed.Scheme != "https" || parsed.Path != "/api" {
		t.Fatalf("unexpected parsed URL: %#v", parsed)
	}
}

func TestValidateServerURLRejectsRemoteHTTP(t *testing.T) {
	if _, err := validateServerURL("http://baley.example/api"); err == nil {
		t.Fatal("accepted a remote HTTP URL")
	}
}

func TestValidateServerURLAllowsLoopbackHTTP(t *testing.T) {
	for _, raw := range []string{"http://127.0.0.1:8080", "http://localhost:8080", "http://[::1]:8080"} {
		if _, err := validateServerURL(raw); err != nil {
			t.Fatalf("validateServerURL(%q): %v", raw, err)
		}
	}
}

func TestValidateServerURLRejectsUnsafeURLs(t *testing.T) {
	for _, raw := range []string{
		"ftp://127.0.0.1/resource",
		"https://user:secret@baley.example/api",
		"https://baley.example/api?token=secret",
		"https://baley.example/api#fragment",
		"/relative",
	} {
		if _, err := validateServerURL(raw); err == nil {
			t.Fatalf("accepted unsafe URL %q", raw)
		}
	}
}

func TestParseModeRequiresExplicitSupportedCommand(t *testing.T) {
	for _, mode := range []string{"serve-http", "migrate-legacy", "rollback-legacy", "diagnose"} {
		got, err := parseMode([]string{mode})
		if err != nil || got != mode {
			t.Fatalf("parseMode(%q) = %q, %v", mode, got, err)
		}
	}
	for _, args := range [][]string{nil, {}, {"unknown"}, {"serve-http", "extra"}} {
		if _, err := parseMode(args); err == nil {
			t.Fatalf("parseMode(%q) unexpectedly succeeded", args)
		}
	}
}
