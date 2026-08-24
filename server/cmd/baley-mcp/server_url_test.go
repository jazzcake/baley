package main

import "testing"

func TestValidateServerURLAllowsTailnetHTTPSForHTTPGateway(t *testing.T) {
	parsed, err := validateServerURL("https://jazzcake-home.tail87e929.ts.net/api", "serve-http")
	if err != nil {
		t.Fatalf("Tailnet HTTPS URL rejected: %v", err)
	}
	if parsed.Scheme != "https" || parsed.Path != "/api" {
		t.Fatalf("unexpected parsed URL: %#v", parsed)
	}
}

func TestValidateServerURLKeepsStdioLoopbackOnly(t *testing.T) {
	if _, err := validateServerURL("https://jazzcake-home.tail87e929.ts.net/api", "stdio"); err == nil {
		t.Fatal("stdio accepted a remote HTTPS URL")
	}
}
