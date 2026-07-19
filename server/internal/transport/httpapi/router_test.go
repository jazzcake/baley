package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORSAllowsConfiguredViewerOrigins(t *testing.T) {
	handler := (&API{AllowedOrigins: []string{
		"http://127.0.0.1:5173",
		"http://localhost:5173",
	}}).Handler()

	for _, origin := range []string{"http://127.0.0.1:5173", "http://localhost:5173"} {
		t.Run(origin, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
			request.Header.Set("Origin", origin)
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != http.StatusOK {
				t.Fatalf("status=%d", response.Code)
			}
			if got := response.Header().Get("Access-Control-Allow-Origin"); got != origin {
				t.Fatalf("Access-Control-Allow-Origin=%q, want %q", got, origin)
			}
		})
	}
}

func TestCORSDoesNotAllowUnknownOrigin(t *testing.T) {
	handler := (&API{AllowedOrigins: []string{"http://localhost:5173"}}).Handler()
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	request.Header.Set("Origin", "https://example.com")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("unexpected Access-Control-Allow-Origin=%q", got)
	}
}

func TestCORSPreflightUsesConfiguredOrigin(t *testing.T) {
	handler := (&API{AllowedOrigins: []string{"http://localhost:5173"}}).Handler()
	request := httptest.NewRequest(http.MethodOptions, "/v1/workspaces/workspace/graph", nil)
	request.Header.Set("Origin", "http://localhost:5173")
	request.Header.Set("Access-Control-Request-Method", http.MethodGet)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status=%d", response.Code)
	}
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Fatalf("Access-Control-Allow-Origin=%q", got)
	}
}
