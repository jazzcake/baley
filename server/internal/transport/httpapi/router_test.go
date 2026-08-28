package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jazzcake/baley/server/internal/application"
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

func TestReadinessAndVersionEndpointsRemainPublic(t *testing.T) {
	api := &API{
		AuthMode:   "enforced",
		Build:      BuildInfo{Version: "v1.2.3", Commit: "abcdef", BuiltAt: "2026-08-01T00:00:00Z", SchemaVersion: 16},
		ReadyCheck: func(_ context.Context) (int64, error) { return 16, nil },
	}
	handler := api.Handler()

	for _, path := range []string{"/readyz", "/versionz"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("%s status=%d body=%s", path, response.Code, response.Body.String())
		}
		if got := response.Header().Get("X-Baley-Version"); got != "v1.2.3" {
			t.Fatalf("%s X-Baley-Version=%q", path, got)
		}
		if got := response.Header().Get("X-Request-ID"); len(got) < 8 {
			t.Fatalf("%s X-Request-ID=%q", path, got)
		}
	}
}

func TestReadinessDoesNotExposeDatabaseError(t *testing.T) {
	api := &API{ReadyCheck: func(_ context.Context) (int64, error) {
		return 15, errors.New("postgres://user:password@database/baley")
	}}
	request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	response := httptest.NewRecorder()
	api.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d", response.Code)
	}
	if strings.Contains(response.Body.String(), "password") || strings.Contains(response.Body.String(), "postgres://") {
		t.Fatalf("readiness leaked internal error: %s", response.Body.String())
	}
}

func TestObservabilityReusesOnlySafeRequestID(t *testing.T) {
	api := &API{}
	for _, test := range []struct {
		name, input, want string
	}{
		{name: "safe", input: "upstream-123", want: "upstream-123"},
		{name: "unsafe", input: "bad\nvalue"},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/healthz?token=not-logged", nil)
			request.Header.Set("X-Request-ID", test.input)
			response := httptest.NewRecorder()
			api.Handler().ServeHTTP(response, request)
			got := response.Header().Get("X-Request-ID")
			if test.want != "" && got != test.want {
				t.Fatalf("X-Request-ID=%q, want %q", got, test.want)
			}
			if test.want == "" && (got == "" || got == test.input) {
				t.Fatalf("unsafe request ID was not replaced: %q", got)
			}
		})
	}
}

func TestDecodeStrictRejectsTrailingJSON(t *testing.T) {
	var request application.CommandRequest
	raw := `{"name":"task.update","arguments":{"workspaceId":"w","taskId":1},"envelope":{"idempotencyKey":"k","executedByActorId":"a"}} {}`
	if err := decodeStrict([]byte(raw), &request); err == nil || !strings.Contains(err.Error(), "exactly one JSON value") {
		t.Fatalf("trailing JSON accepted: %v", err)
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

func TestPhaseTasksRejectsInvalidPageBoundsBeforeRepositoryRead(t *testing.T) {
	handler := (&API{}).Handler()
	for _, test := range []struct {
		path string
		code string
	}{
		{path: "/v1/workspaces/workspace/phases/active/tasks?cursor=-1", code: "invalid_cursor"},
		{path: "/v1/workspaces/workspace/phases/active/tasks?cursor=not-a-number", code: "invalid_cursor"},
		{path: "/v1/workspaces/workspace/phases/active/tasks?limit=0", code: "invalid_limit"},
		{path: "/v1/workspaces/workspace/phases/active/tasks?limit=101", code: "invalid_limit"},
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, test.path, nil))
		if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), test.code) {
			t.Fatalf("%s status=%d body=%s", test.path, response.Code, response.Body.String())
		}
	}
}
