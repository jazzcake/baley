package integration_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMCPStdioListsAndCallsTools(t *testing.T) {
	if os.Getenv("BALEY_MCP_E2E") == "" {
		t.Skip("BALEY_MCP_E2E is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	serverURL := os.Getenv("BALEY_SERVER_URL")
	if serverURL == "" {
		serverURL = "http://127.0.0.1:8080"
	}
	cmd := exec.CommandContext(ctx, "go", "run", "./cmd/baley-mcp")
	cmd.Dir = ".."
	cmd.Env = append(os.Environ(), "BALEY_SERVER_URL="+serverURL)
	client := mcp.NewClient(&mcp.Implementation{Name: "baley-integration-test", Version: "0.1.0"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(tools.Tools) != 29 {
		t.Fatalf("tool count=%d", len(tools.Tools))
	}
	want := map[string]bool{
		"baley_workspace_get": true, "baley_workspace_graph": true, "baley_task_get": true,
		"baley_gate_status": true, "baley_decision_list": true, "baley_event_list": true,
		"baley_run_list": true, "baley_record_list": true,
		"baley_run_start": true, "baley_run_heartbeat": true, "baley_run_succeed": true,
		"baley_run_fail": true, "baley_run_cancel": true, "baley_run_interrupt": true,
		"baley_run_correct": true, "baley_repository_register": true, "baley_record_register": true,
		"baley_record_attach_commit": true, "baley_commit_attach": true, "baley_git_observe": true,
		"baley_task_report_implemented": true,
		"baley_task_confirm_preview":    true, "baley_task_confirm_execute": true,
		"baley_gate_pass_task_preview": true, "baley_gate_pass_task_execute": true,
		"baley_gate_revoke_task_pass_preview": true, "baley_gate_revoke_task_pass_execute": true,
		"baley_gate_pass_preview": true, "baley_gate_pass_execute": true,
	}
	for _, tool := range tools.Tools {
		if tool.Name == "baley_task_confirm_execute" {
			schema, marshalErr := json.Marshal(tool.InputSchema)
			if marshalErr != nil || !strings.Contains(string(schema), `"acknowledgedWarningCodes"`) || !strings.Contains(string(schema), `"proceedReason"`) {
				t.Fatalf("task confirm execute schema lacks warning evidence fields: %s %v", schema, marshalErr)
			}
		}
		delete(want, tool.Name)
	}
	if len(want) != 0 {
		t.Fatalf("missing tools: %#v", want)
	}
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "baley_workspace_graph", Arguments: map[string]any{"workspaceId": postgresDemoWorkspaceID}})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("graph tool error: %#v", result.StructuredContent)
	}
	var graph struct {
		Workspace struct {
			Revision int64 `json:"revision"`
		} `json:"workspace"`
	}
	raw, err := json.Marshal(result.StructuredContent)
	if err != nil || json.Unmarshal(raw, &graph) != nil || graph.Workspace.Revision == 0 {
		t.Fatalf("graph revision unavailable: %#v", result.StructuredContent)
	}
	for _, toolName := range []string{"baley_run_list", "baley_record_list"} {
		result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: toolName, Arguments: map[string]any{"workspaceId": postgresDemoWorkspaceID}})
		if err != nil || result.IsError {
			t.Fatalf("%s failed: %#v %v", toolName, result.StructuredContent, err)
		}
	}
	clientRunID := testMCPUUID(t)
	result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "baley_run_start", Arguments: map[string]any{"workspaceId": postgresDemoWorkspaceID, "taskId": 110, "clientRunId": clientRunID, "kind": "detailed_planning", "sessionRef": "mcp-e2e", "expectedWorkspaceRevision": graph.Workspace.Revision, "idempotencyKey": clientRunID, "executedByActorId": "00000000-0000-4000-8000-000000000003"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("run.start tool error: %#v", result.StructuredContent)
	}
	raw, err = json.Marshal(result.StructuredContent)
	var started struct {
		LeaseToken        string `json:"leaseToken"`
		WorkspaceRevision int64  `json:"workspaceRevision"`
		Projection        struct {
			Run struct {
				ID      string `json:"id"`
				Version int64  `json:"version"`
			} `json:"run"`
		} `json:"projection"`
	}
	if err != nil || json.Unmarshal(raw, &started) != nil || started.LeaseToken == "" || started.Projection.Run.ID == "" || started.Projection.Run.Version != 1 {
		t.Fatalf("run.start lease token unavailable: %#v", result.StructuredContent)
	}
	heartbeatKey := testMCPUUID(t)
	result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "baley_run_heartbeat", Arguments: map[string]any{"workspaceId": postgresDemoWorkspaceID, "runId": started.Projection.Run.ID, "leaseToken": started.LeaseToken, "expectedRunVersion": 1, "extensionSeconds": 60, "idempotencyKey": heartbeatKey, "executedByActorId": "00000000-0000-4000-8000-000000000003"}})
	if err != nil || result.IsError {
		t.Fatalf("run.heartbeat tool failed: %#v %v", result.StructuredContent, err)
	}
	terminalKey := testMCPUUID(t)
	result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "baley_run_succeed", Arguments: map[string]any{"workspaceId": postgresDemoWorkspaceID, "runId": started.Projection.Run.ID, "expectedRunVersion": 2, "summary": "MCP lifecycle verified", "expectedWorkspaceRevision": started.WorkspaceRevision, "idempotencyKey": terminalKey, "executedByActorId": "00000000-0000-4000-8000-000000000003"}})
	if err != nil || result.IsError {
		t.Fatalf("run.succeed tool failed: %#v %v", result.StructuredContent, err)
	}
	raw, _ = json.Marshal(result.StructuredContent)
	var terminal struct {
		WorkspaceRevision int64 `json:"workspaceRevision"`
	}
	if json.Unmarshal(raw, &terminal) != nil || terminal.WorkspaceRevision == 0 {
		t.Fatalf("run.succeed revision unavailable: %#v", result.StructuredContent)
	}
	recordID := testMCPUUID(t)
	recordKey := testMCPUUID(t)
	result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "baley_record_register", Arguments: map[string]any{"workspaceId": postgresDemoWorkspaceID, "recordId": recordID, "taskId": 110, "runId": started.Projection.Run.ID, "recordType": "completion-report", "repositoryId": "00000000-0000-4000-8000-000000000004", "relativePath": "task-records/mcp/" + recordID + ".md", "workingTreeHash": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "shortSummary": "MCP Record integration evidence", "expectedWorkspaceRevision": terminal.WorkspaceRevision, "idempotencyKey": recordKey, "executedByActorId": "00000000-0000-4000-8000-000000000003"}})
	if err != nil || result.IsError {
		t.Fatalf("record.register tool failed: %#v %v", result.StructuredContent, err)
	}
	raw, _ = json.Marshal(result.StructuredContent)
	var recorded struct {
		WorkspaceRevision int64 `json:"workspaceRevision"`
	}
	if json.Unmarshal(raw, &recorded) != nil || recorded.WorkspaceRevision == 0 {
		t.Fatalf("record.register revision unavailable: %#v", result.StructuredContent)
	}
	result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "baley_record_list", Arguments: map[string]any{"workspaceId": postgresDemoWorkspaceID}})
	raw, _ = json.Marshal(result.StructuredContent)
	if err != nil || result.IsError || !strings.Contains(string(raw), recordID) {
		t.Fatalf("record.list does not contain registered Record: %s %v", raw, err)
	}

	reportKey := testMCPUUID(t)
	reportWarnings := []string{"dangling_path", "missing_detailed_plan_record", "missing_independent_review_record"}
	result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "baley_task_report_implemented", Arguments: map[string]any{
		"workspaceId": postgresDemoWorkspaceID, "taskId": 110,
		"assessment":                "MCP stdio warning acknowledgement flow verified.",
		"acknowledgedWarningCodes":  reportWarnings,
		"proceedReason":             "The E2E fixture intentionally leaves Task #110 terminal.",
		"expectedWorkspaceRevision": recorded.WorkspaceRevision, "idempotencyKey": reportKey,
		"executedByActorId": "00000000-0000-4000-8000-000000000003",
	}})
	if err != nil || result.IsError {
		t.Fatalf("task.report_implemented tool failed: %#v %v", result.StructuredContent, err)
	}
	raw, _ = json.Marshal(result.StructuredContent)
	var reported struct {
		WorkspaceRevision int64 `json:"workspaceRevision"`
	}
	if json.Unmarshal(raw, &reported) != nil || reported.WorkspaceRevision == 0 {
		t.Fatalf("task.report_implemented revision unavailable: %#v", result.StructuredContent)
	}

	confirmKey := testMCPUUID(t)
	result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "baley_task_confirm_preview", Arguments: map[string]any{
		"workspaceId": postgresDemoWorkspaceID, "taskId": 110,
		"expectedWorkspaceRevision": reported.WorkspaceRevision, "idempotencyKey": confirmKey,
		"executedByActorId": "00000000-0000-4000-8000-000000000003",
	}})
	if err != nil || result.IsError {
		t.Fatalf("task.confirm preview tool failed: %#v %v", result.StructuredContent, err)
	}
	raw, _ = json.Marshal(result.StructuredContent)
	var confirmPreview struct {
		CommandHash string `json:"commandHash"`
		Warnings    []struct {
			Code string `json:"code"`
		} `json:"warnings"`
	}
	if json.Unmarshal(raw, &confirmPreview) != nil || confirmPreview.CommandHash == "" || len(confirmPreview.Warnings) != 1 || confirmPreview.Warnings[0].Code != "dangling_path" {
		t.Fatalf("task.confirm preview evidence invalid: %#v", result.StructuredContent)
	}
	result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "baley_task_confirm_execute", Arguments: map[string]any{
		"workspaceId": postgresDemoWorkspaceID, "taskId": 110,
		"expectedWorkspaceRevision": reported.WorkspaceRevision, "idempotencyKey": confirmKey,
		"executedByActorId":        "00000000-0000-4000-8000-000000000003",
		"acknowledgedWarningCodes": []string{"dangling_path"},
		"proceedReason":            "Task #110 is the intentional terminal MCP E2E validation task.",
		"approvedByActorId":        "00000000-0000-4000-8000-000000000002",
		"approvedCommandHash":      confirmPreview.CommandHash,
	}})
	if err != nil || result.IsError {
		t.Fatalf("task.confirm execute tool failed: %#v %v", result.StructuredContent, err)
	}
	raw, _ = json.Marshal(result.StructuredContent)
	var confirmed struct {
		Projection struct {
			Status struct {
				After string `json:"after"`
			} `json:"status"`
		} `json:"projection"`
	}
	if json.Unmarshal(raw, &confirmed) != nil || confirmed.Projection.Status.After != "confirmed" {
		t.Fatalf("task.confirm execute did not return confirmed projection: %s", raw)
	}
}

const postgresDemoWorkspaceID = "00000000-0000-4000-8000-000000000001"

func testMCPUUID(t *testing.T) string {
	t.Helper()
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		t.Fatal(err)
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	raw := hex.EncodeToString(value[:])
	return raw[:8] + "-" + raw[8:12] + "-" + raw[12:16] + "-" + raw[16:20] + "-" + raw[20:]
}
