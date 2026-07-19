package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/jazzcake/baley/server/internal/application"
	"github.com/jazzcake/baley/server/internal/persistence/postgres"
	"github.com/jazzcake/baley/server/internal/transport/httpapi"
)

func TestRunStartHTTPContract(t *testing.T) {
	url := os.Getenv("BALEY_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("BALEY_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	t.Setenv("BALEY_LEASE_TOKEN_SECRET", "run-start-http-integration-secret")
	repo, err := postgres.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Pool.Close()
	if _, err = repo.Pool.Exec(ctx, "TRUNCATE events,human_approval_attestations,commands,workspace_counters,runs,gate_tasks,gates,task_dependencies,tasks,lanes,phases,workspaces,actors CASCADE"); err != nil {
		t.Fatal(err)
	}
	if err = repo.SeedDemo(ctx); err != nil {
		t.Fatal(err)
	}
	service := application.NewService(repo)
	handler := (&httpapi.API{Service: service, Repo: repo, AllowedOrigins: []string{"http://127.0.0.1:5173"}}).Handler()
	command := request("run.start", map[string]any{"workspaceId": postgres.DemoWorkspaceID, "taskId": 110, "clientRunId": "00000000-0000-4000-8000-000000000012", "kind": "detailed_planning"}, "run-http", 1)

	previewBody := serveCommand(t, handler, "/v1/commands/preview", command)
	var preview application.PreviewResult
	if err = json.Unmarshal(previewBody, &preview); err != nil || len(preview.Errors) != 0 || preview.RequiredCapability != "run:operate" {
		t.Fatalf("unexpected HTTP preview: %s", previewBody)
	}
	executeBody := serveCommand(t, handler, "/v1/commands/execute", command)
	var result application.ExecutionResult
	if err = json.Unmarshal(executeBody, &result); err != nil || result.LeaseToken == "" || result.WorkspaceRevision != 2 {
		t.Fatalf("unexpected HTTP execute: %s", executeBody)
	}
	snapshot, err := repo.LoadSnapshot(ctx, postgres.DemoWorkspaceID)
	if err != nil || len(snapshot.Runs) != 1 {
		t.Fatalf("Run projection unavailable: %#v %v", snapshot.Runs, err)
	}
	heartbeat := request("run.heartbeat", map[string]any{"workspaceId": postgres.DemoWorkspaceID, "runId": snapshot.Runs[0].ID, "leaseToken": result.LeaseToken, "expectedRunVersion": 1}, "run-http-heartbeat", 0)
	heartbeatBody := serveCommand(t, handler, "/v1/commands/execute", heartbeat)
	if err = json.Unmarshal(heartbeatBody, &result); err != nil || result.WorkspaceRevision != 2 || len(result.EventIDs) != 0 {
		t.Fatalf("unexpected heartbeat HTTP result: %s", heartbeatBody)
	}
	terminal := request("run.succeed", map[string]any{"workspaceId": postgres.DemoWorkspaceID, "runId": snapshot.Runs[0].ID, "expectedRunVersion": 2, "summary": "HTTP lifecycle complete"}, "run-http-succeed", 2)
	terminalBody := serveCommand(t, handler, "/v1/commands/execute", terminal)
	if err = json.Unmarshal(terminalBody, &result); err != nil || result.WorkspaceRevision != 3 || len(result.EventIDs) != 1 {
		t.Fatalf("unexpected terminal HTTP result: %s", terminalBody)
	}
	var runs []application.RunProjection
	if body := serveGET(t, handler, "/v1/workspaces/"+postgres.DemoWorkspaceID+"/runs"); json.Unmarshal(body, &runs) != nil || len(runs) != 1 || runs[0].Status != "succeeded" || runs[0].LeaseTokenHash != "" {
		t.Fatalf("unexpected Run list: %s", body)
	}
	var records []application.TaskRecordProjection
	if body := serveGET(t, handler, "/v1/workspaces/"+postgres.DemoWorkspaceID+"/records"); json.Unmarshal(body, &records) != nil || len(records) != 0 {
		t.Fatalf("unexpected Record list: %s", body)
	}
}

func serveCommand(t *testing.T, handler http.Handler, path string, command application.CommandRequest) []byte {
	t.Helper()
	raw, err := json.Marshal(command)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("HTTP %d: %s", res.Code, res.Body.String())
	}
	return res.Body.Bytes()
}

func serveGET(t *testing.T, handler http.Handler, path string) []byte {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("HTTP %d: %s", res.Code, res.Body.String())
	}
	return res.Body.Bytes()
}
