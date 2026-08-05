package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeRunner struct {
	calls [][]string
}

func (r *fakeRunner) Run(_ context.Context, arguments ...string) ([]byte, error) {
	r.calls = append(r.calls, append([]string(nil), arguments...))
	if len(r.calls) == 1 {
		return []byte(`{"ok":true,"result":{"terminals":[{"handle":"older","connected":true,"lastOutputAt":1},{"handle":"newer","connected":true,"lastOutputAt":2}]}}`), nil
	}
	return []byte(`{"ok":true}`), nil
}

func TestFocusVerifiesExecutionAndSwitchesLatestConnectedTerminal(t *testing.T) {
	baley := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Cookie"); got != "session=test" {
			t.Fatalf("forwarded cookie = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"execution-1","taskId":"task-1","provider":"orca","externalId":"worktree-1","hostId":"local","status":"active"}`)
	}))
	defer baley.Close()

	runner := &fakeRunner{}
	b := &bridge{baleyBase: baley.URL, client: baley.Client(), runner: runner}
	req := httptest.NewRequest(http.MethodPost, "/v1/orca/focus", strings.NewReader(`{"workspaceId":"workspace-1","taskId":"task-1","externalExecutionId":"execution-1"}`))
	req.Header.Set("Cookie", "session=test")
	response := httptest.NewRecorder()

	b.focus(response, req)

	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"outcome":"focused"`) {
		t.Fatalf("response = %d %s", response.Code, response.Body.String())
	}
	if len(runner.calls) != 2 {
		t.Fatalf("runner calls = %d", len(runner.calls))
	}
	if got := strings.Join(runner.calls[0], " "); got != "terminal list --worktree worktree-1 --limit 50 --json" {
		t.Fatalf("list command = %q", got)
	}
	if got := strings.Join(runner.calls[1], " "); got != "terminal switch --terminal newer --json" {
		t.Fatalf("switch command = %q", got)
	}
}

func TestFocusRejectsTaskMismatchBeforeCallingOrca(t *testing.T) {
	baley := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"id":"execution-1","taskId":"another-task","provider":"orca","externalId":"worktree-1","hostId":"local","status":"active"}`)
	}))
	defer baley.Close()

	runner := &fakeRunner{}
	b := &bridge{baleyBase: baley.URL, client: baley.Client(), runner: runner}
	req := httptest.NewRequest(http.MethodPost, "/v1/orca/focus", strings.NewReader(`{"workspaceId":"workspace-1","taskId":"task-1","externalExecutionId":"execution-1"}`))
	response := httptest.NewRecorder()

	b.focus(response, req)

	if response.Code != http.StatusConflict || len(runner.calls) != 0 {
		t.Fatalf("response = %d, runner calls = %d", response.Code, len(runner.calls))
	}
}

func TestCORSAllowsHealthWithoutOriginAndRejectsFocusWithoutOrigin(t *testing.T) {
	b := &bridge{origins: map[string]bool{"http://localhost:5173": true}}
	handler := b.cors(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))

	health := httptest.NewRecorder()
	handler.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if health.Code != http.StatusNoContent {
		t.Fatalf("health status = %d", health.Code)
	}

	focus := httptest.NewRecorder()
	handler.ServeHTTP(focus, httptest.NewRequest(http.MethodPost, "/v1/orca/focus", nil))
	if focus.Code != http.StatusForbidden {
		t.Fatalf("focus status = %d", focus.Code)
	}
}
