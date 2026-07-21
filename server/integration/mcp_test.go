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

	"github.com/jazzcake/baley/server/internal/persistence/postgres"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMCPStdioListsAndCallsTools(t *testing.T) {
	if os.Getenv("BALEY_MCP_E2E") == "" {
		t.Skip("BALEY_MCP_E2E is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	testDatabaseURL := os.Getenv("BALEY_TEST_DATABASE_URL")
	if testDatabaseURL == "" {
		t.Fatal("BALEY_TEST_DATABASE_URL is required when BALEY_MCP_E2E is set")
	}
	t.Setenv("BALEY_LEASE_TOKEN_SECRET", "mcp-stdio-e2e-audit-secret")
	auditRepo, err := postgres.Open(ctx, testDatabaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer auditRepo.Pool.Close()
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
	if len(tools.Tools) != 39 {
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
		"baley_task_create_preview":     true, "baley_task_create_execute": true,
		"baley_phase_create_preview": true, "baley_phase_create_execute": true,
		"baley_lane_create_preview": true, "baley_lane_create_execute": true,
		"baley_gate_create_preview": true, "baley_gate_create_execute": true,
		"baley_gate_attach_task_preview": true, "baley_gate_attach_task_execute": true,
		"baley_task_confirm_preview": true, "baley_task_confirm_execute": true,
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
		if tool.Name == "baley_task_create_preview" || tool.Name == "baley_task_create_execute" {
			schema, marshalErr := json.Marshal(tool.InputSchema)
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			assertTaskCreateSchema(t, tool.Name, schema, tool.Name == "baley_task_create_execute")
		}
		if fields, ok := structuralToolRequiredFields(tool.Name); ok {
			schema, marshalErr := json.Marshal(tool.InputSchema)
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			assertStructuralCreateSchema(t, tool.Name, schema, fields)
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
		WorkspaceRevision int64 `json:"workspaceRevision"`
		Projection        struct {
			Status struct {
				After string `json:"after"`
			} `json:"status"`
		} `json:"projection"`
	}
	if json.Unmarshal(raw, &confirmed) != nil || confirmed.Projection.Status.After != "confirmed" || confirmed.WorkspaceRevision == 0 {
		t.Fatalf("task.confirm execute did not return confirmed projection: %s", raw)
	}

	createKey := testMCPUUID(t)
	taskUUID := testMCPUUID(t)
	createArguments := map[string]any{
		"workspaceId": postgresDemoWorkspaceID, "taskUuid": taskUUID,
		"laneId": "client", "phaseId": "validate", "parentTaskId": 110,
		"title": "MCP-created operational checkpoint", "description": "Verify typed task.create stdio flow.",
		"predecessorTaskIds": []int{106}, "successorTaskIds": []int{101},
		"expectedWorkspaceRevision": confirmed.WorkspaceRevision, "idempotencyKey": createKey,
		"executedByActorId": "00000000-0000-4000-8000-000000000003",
	}
	var tasksBeforePreview, commandsBeforePreview, eventsBeforePreview int
	if err = auditRepo.Pool.QueryRow(ctx, "SELECT count(*) FROM tasks WHERE workspace_id=$1", postgresDemoWorkspaceID).Scan(&tasksBeforePreview); err != nil {
		t.Fatal(err)
	}
	if err = auditRepo.Pool.QueryRow(ctx, "SELECT count(*) FROM commands WHERE workspace_id=$1", postgresDemoWorkspaceID).Scan(&commandsBeforePreview); err != nil {
		t.Fatal(err)
	}
	if err = auditRepo.Pool.QueryRow(ctx, "SELECT count(*) FROM events WHERE workspace_id=$1", postgresDemoWorkspaceID).Scan(&eventsBeforePreview); err != nil {
		t.Fatal(err)
	}
	result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "baley_task_create_preview", Arguments: createArguments})
	if err != nil || result.IsError {
		t.Fatalf("task.create preview tool failed: %#v %v", result.StructuredContent, err)
	}
	raw, _ = json.Marshal(result.StructuredContent)
	var createPreview struct {
		CommandHash   string `json:"commandHash"`
		ProjectedDiff struct {
			Task struct {
				PublicID int `json:"PublicID"`
			} `json:"task"`
		} `json:"projectedDiff"`
		Warnings []struct {
			Code string `json:"code"`
		} `json:"warnings"`
	}
	if json.Unmarshal(raw, &createPreview) != nil || createPreview.CommandHash == "" || createPreview.ProjectedDiff.Task.PublicID != 111 || len(createPreview.Warnings) != 1 || createPreview.Warnings[0].Code != "phase_order_inversion" {
		t.Fatalf("task.create preview projection invalid: %#v", result.StructuredContent)
	}
	result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "baley_workspace_graph", Arguments: map[string]any{"workspaceId": postgresDemoWorkspaceID}})
	raw, _ = json.Marshal(result.StructuredContent)
	var afterPreview struct {
		Workspace struct {
			Revision int64 `json:"revision"`
		} `json:"workspace"`
	}
	if err != nil || result.IsError || json.Unmarshal(raw, &afterPreview) != nil || afterPreview.Workspace.Revision != confirmed.WorkspaceRevision {
		t.Fatalf("task.create preview wrote state: %#v %v", result.StructuredContent, err)
	}
	var tasksAfterPreview, commandsAfterPreview, eventsAfterPreview int
	if err = auditRepo.Pool.QueryRow(ctx, "SELECT count(*) FROM tasks WHERE workspace_id=$1", postgresDemoWorkspaceID).Scan(&tasksAfterPreview); err != nil {
		t.Fatal(err)
	}
	if err = auditRepo.Pool.QueryRow(ctx, "SELECT count(*) FROM commands WHERE workspace_id=$1", postgresDemoWorkspaceID).Scan(&commandsAfterPreview); err != nil {
		t.Fatal(err)
	}
	if err = auditRepo.Pool.QueryRow(ctx, "SELECT count(*) FROM events WHERE workspace_id=$1", postgresDemoWorkspaceID).Scan(&eventsAfterPreview); err != nil {
		t.Fatal(err)
	}
	if tasksAfterPreview != tasksBeforePreview || commandsAfterPreview != commandsBeforePreview || eventsAfterPreview != eventsBeforePreview {
		t.Fatalf("task.create preview wrote non-revision state: tasks %d->%d commands %d->%d events %d->%d", tasksBeforePreview, tasksAfterPreview, commandsBeforePreview, commandsAfterPreview, eventsBeforePreview, eventsAfterPreview)
	}
	executeCreateArguments := make(map[string]any, len(createArguments)+2)
	for key, value := range createArguments {
		executeCreateArguments[key] = value
	}
	executeCreateArguments["acknowledgedWarningCodes"] = []string{"phase_order_inversion"}
	executeCreateArguments["proceedReason"] = "The E2E fixture intentionally verifies a later-phase to earlier-phase relationship warning."
	result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "baley_task_create_execute", Arguments: executeCreateArguments})
	if err != nil || result.IsError {
		t.Fatalf("task.create execute tool failed: %#v %v", result.StructuredContent, err)
	}
	raw, _ = json.Marshal(result.StructuredContent)
	var createResult struct {
		CommandID         string `json:"commandId"`
		WorkspaceRevision int64  `json:"workspaceRevision"`
		Idempotent        bool   `json:"idempotent"`
	}
	if json.Unmarshal(raw, &createResult) != nil || createResult.CommandID == "" || createResult.WorkspaceRevision != confirmed.WorkspaceRevision+1 || createResult.Idempotent {
		t.Fatalf("task.create execute result invalid: %s", raw)
	}
	result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "baley_task_create_execute", Arguments: executeCreateArguments})
	raw, _ = json.Marshal(result.StructuredContent)
	var createRetry struct {
		CommandID  string `json:"commandId"`
		Idempotent bool   `json:"idempotent"`
	}
	if err != nil || result.IsError || json.Unmarshal(raw, &createRetry) != nil || !createRetry.Idempotent || createRetry.CommandID != createResult.CommandID {
		t.Fatalf("task.create idempotent retry failed: %s %v", raw, err)
	}
	result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "baley_task_get", Arguments: map[string]any{"workspaceId": postgresDemoWorkspaceID, "taskId": 111}})
	raw, _ = json.Marshal(result.StructuredContent)
	var created struct {
		PublicID int    `json:"publicId"`
		Status   string `json:"status"`
	}
	if err != nil || result.IsError || json.Unmarshal(raw, &created) != nil || created.PublicID != 111 || created.Status != "pending" {
		t.Fatalf("task.create task projection unavailable: %s %v", raw, err)
	}
	result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "baley_workspace_graph", Arguments: map[string]any{"workspaceId": postgresDemoWorkspaceID}})
	raw, _ = json.Marshal(result.StructuredContent)
	if err != nil || result.IsError || !strings.Contains(string(raw), `"fromTaskId":"assets","toTaskId":"`+taskUUID+`"`) || !strings.Contains(string(raw), `"fromTaskId":"`+taskUUID+`","toTaskId":"api"`) {
		t.Fatalf("task.create relationships missing: %s %v", raw, err)
	}
	result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "baley_event_list", Arguments: map[string]any{"workspaceId": postgresDemoWorkspaceID}})
	raw, _ = json.Marshal(result.StructuredContent)
	if err != nil || result.IsError || !strings.Contains(string(raw), `"eventType":"task.created"`) {
		t.Fatalf("task.create Event evidence missing: %s %v", raw, err)
	}
	events, err := auditRepo.Events(ctx, postgresDemoWorkspaceID)
	if err != nil {
		t.Fatal(err)
	}
	foundBoundEvent := false
	for _, event := range events {
		if event.EventType != "task.created" || event.CommandID != createResult.CommandID || event.EntityID != taskUUID || event.WorkspaceRevision != createResult.WorkspaceRevision {
			continue
		}
		var payload map[string]any
		if json.Unmarshal(event.Payload, &payload) != nil || payload["proceedReason"] != executeCreateArguments["proceedReason"] {
			t.Fatalf("task.created bound Event payload invalid: %s", event.Payload)
		}
		codes, ok := payload["acknowledgedWarningCodes"].([]any)
		if !ok || len(codes) != 1 || codes[0] != "phase_order_inversion" {
			t.Fatalf("task.created bound Event warning evidence invalid: %s", event.Payload)
		}
		foundBoundEvent = true
	}
	if !foundBoundEvent {
		t.Fatalf("task.created Event is not bound to command=%s task=%s revision=%d", createResult.CommandID, taskUUID, createResult.WorkspaceRevision)
	}

	structuralRevision := createResult.WorkspaceRevision
	structuralSuffix := strings.ReplaceAll(testMCPUUID(t), "-", "")[:8]
	phaseID := "mcp-embedding-" + structuralSuffix
	laneID := "mcp-adoption-" + structuralSuffix
	gateID := "mcp-embedding-gate-" + structuralSuffix
	type structuralMutation struct {
		previewTool string
		executeTool string
		eventType   string
		entityID    string
		arguments   map[string]any
	}
	structuralMutations := []structuralMutation{
		{
			previewTool: "baley_phase_create_preview", executeTool: "baley_phase_create_execute",
			eventType: "phase.created", entityID: phaseID,
			arguments: map[string]any{"workspaceId": postgresDemoWorkspaceID, "phaseId": phaseID, "name": "Embedding Contract"},
		},
		{
			previewTool: "baley_lane_create_preview", executeTool: "baley_lane_create_execute",
			eventType: "lane.created", entityID: laneID,
			arguments: map[string]any{"workspaceId": postgresDemoWorkspaceID, "laneId": laneID, "name": "Adoption", "goal": "Adopt typed Baley structures", "summary": "MCP structural fixture"},
		},
		{
			previewTool: "baley_gate_create_preview", executeTool: "baley_gate_create_execute",
			eventType: "gate.created", entityID: gateID,
			arguments: map[string]any{"workspaceId": postgresDemoWorkspaceID, "gateId": gateID, "name": "Embedding Entry", "fromPhaseId": "validate", "toPhaseId": phaseID},
		},
		{
			previewTool: "baley_gate_attach_task_preview", executeTool: "baley_gate_attach_task_execute",
			eventType: "gate.task_attached", entityID: gateID,
			arguments: map[string]any{"workspaceId": postgresDemoWorkspaceID, "gateId": gateID, "taskId": 110},
		},
	}
	for _, mutation := range structuralMutations {
		key := testMCPUUID(t)
		arguments := make(map[string]any, len(mutation.arguments)+4)
		for name, value := range mutation.arguments {
			arguments[name] = value
		}
		arguments["expectedWorkspaceRevision"] = structuralRevision
		arguments["idempotencyKey"] = key
		arguments["executedByActorId"] = "00000000-0000-4000-8000-000000000003"

		var commandsBefore, eventsBefore int
		if err = auditRepo.Pool.QueryRow(ctx, "SELECT count(*) FROM commands WHERE workspace_id=$1", postgresDemoWorkspaceID).Scan(&commandsBefore); err != nil {
			t.Fatal(err)
		}
		if err = auditRepo.Pool.QueryRow(ctx, "SELECT count(*) FROM events WHERE workspace_id=$1", postgresDemoWorkspaceID).Scan(&eventsBefore); err != nil {
			t.Fatal(err)
		}
		result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: mutation.previewTool, Arguments: arguments})
		if err != nil || result.IsError {
			t.Fatalf("%s failed: %#v %v", mutation.previewTool, result.StructuredContent, err)
		}
		raw, _ = json.Marshal(result.StructuredContent)
		var previewResult struct {
			CommandHash               string `json:"commandHash"`
			ExpectedWorkspaceRevision int64  `json:"expectedWorkspaceRevision"`
			Errors                    []any  `json:"errors"`
		}
		if json.Unmarshal(raw, &previewResult) != nil || previewResult.CommandHash == "" || previewResult.ExpectedWorkspaceRevision != structuralRevision || len(previewResult.Errors) != 0 || !strings.Contains(string(raw), mutation.entityID) {
			t.Fatalf("%s preview projection invalid: %s", mutation.previewTool, raw)
		}
		var commandsAfter, eventsAfter int
		if err = auditRepo.Pool.QueryRow(ctx, "SELECT count(*) FROM commands WHERE workspace_id=$1", postgresDemoWorkspaceID).Scan(&commandsAfter); err != nil {
			t.Fatal(err)
		}
		if err = auditRepo.Pool.QueryRow(ctx, "SELECT count(*) FROM events WHERE workspace_id=$1", postgresDemoWorkspaceID).Scan(&eventsAfter); err != nil {
			t.Fatal(err)
		}
		if commandsAfter != commandsBefore || eventsAfter != eventsBefore {
			t.Fatalf("%s preview wrote state: commands %d->%d events %d->%d", mutation.previewTool, commandsBefore, commandsAfter, eventsBefore, eventsAfter)
		}

		result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: mutation.executeTool, Arguments: arguments})
		if err != nil || result.IsError {
			t.Fatalf("%s failed: %#v %v", mutation.executeTool, result.StructuredContent, err)
		}
		raw, _ = json.Marshal(result.StructuredContent)
		var executeResult struct {
			CommandID         string `json:"commandId"`
			WorkspaceRevision int64  `json:"workspaceRevision"`
			Idempotent        bool   `json:"idempotent"`
		}
		if json.Unmarshal(raw, &executeResult) != nil || executeResult.CommandID == "" || executeResult.WorkspaceRevision != structuralRevision+1 || executeResult.Idempotent || !strings.Contains(string(raw), mutation.entityID) {
			t.Fatalf("%s execute result invalid: %s", mutation.executeTool, raw)
		}
		result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: mutation.executeTool, Arguments: arguments})
		raw, _ = json.Marshal(result.StructuredContent)
		var retryResult struct {
			CommandID  string `json:"commandId"`
			Idempotent bool   `json:"idempotent"`
		}
		if err != nil || result.IsError || json.Unmarshal(raw, &retryResult) != nil || !retryResult.Idempotent || retryResult.CommandID != executeResult.CommandID {
			t.Fatalf("%s idempotent retry failed: %s %v", mutation.executeTool, raw, err)
		}
		structuralEvents, eventErr := auditRepo.Events(ctx, postgresDemoWorkspaceID)
		if eventErr != nil {
			t.Fatal(eventErr)
		}
		foundBoundStructuralEvent := false
		for _, event := range structuralEvents {
			if event.EventType == mutation.eventType && event.CommandID == executeResult.CommandID && event.EntityID == mutation.entityID && event.WorkspaceRevision == executeResult.WorkspaceRevision {
				foundBoundStructuralEvent = true
				break
			}
		}
		if !foundBoundStructuralEvent {
			t.Fatalf("%s Event is not bound to command=%s entity=%s revision=%d", mutation.eventType, executeResult.CommandID, mutation.entityID, executeResult.WorkspaceRevision)
		}
		structuralRevision = executeResult.WorkspaceRevision
	}

	activeGateTaskUUID := testMCPUUID(t)
	activeGateTaskKey := testMCPUUID(t)
	activeGateTaskArguments := map[string]any{
		"workspaceId": postgresDemoWorkspaceID, "taskUuid": activeGateTaskUUID,
		"laneId": "server", "phaseId": "build", "title": "Active Gate typed MCP approval fixture",
		"expectedWorkspaceRevision": structuralRevision, "idempotencyKey": activeGateTaskKey,
		"executedByActorId": "00000000-0000-4000-8000-000000000003",
	}
	result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "baley_task_create_preview", Arguments: activeGateTaskArguments})
	if err != nil || result.IsError {
		t.Fatalf("active Gate fixture Task preview failed: %#v %v", result.StructuredContent, err)
	}
	result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "baley_task_create_execute", Arguments: activeGateTaskArguments})
	raw, _ = json.Marshal(result.StructuredContent)
	var activeGateTaskResult struct {
		WorkspaceRevision int64 `json:"workspaceRevision"`
		Projection        struct {
			Task struct {
				PublicID int `json:"PublicID"`
			} `json:"task"`
		} `json:"projection"`
	}
	if err != nil || result.IsError || json.Unmarshal(raw, &activeGateTaskResult) != nil || activeGateTaskResult.WorkspaceRevision != structuralRevision+1 || activeGateTaskResult.Projection.Task.PublicID == 0 {
		t.Fatalf("active Gate fixture Task execute failed: %s %v", raw, err)
	}
	structuralRevision = activeGateTaskResult.WorkspaceRevision

	activeAttachPreviewKey := testMCPUUID(t)
	activeAttachPreviewArguments := map[string]any{
		"workspaceId": postgresDemoWorkspaceID, "gateId": "pilot-ready", "taskId": activeGateTaskResult.Projection.Task.PublicID,
		"expectedWorkspaceRevision": structuralRevision, "idempotencyKey": activeAttachPreviewKey,
		"executedByActorId": "00000000-0000-4000-8000-000000000003", "initiatedByActorId": "00000000-0000-4000-8000-000000000002",
	}
	result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "baley_gate_attach_task_preview", Arguments: activeAttachPreviewArguments})
	raw, _ = json.Marshal(result.StructuredContent)
	var activeAttachPreview struct {
		CommandHash        string `json:"commandHash"`
		RequiredCapability string `json:"requiredCapability"`
		Errors             []struct {
			Code string `json:"code"`
		} `json:"errors"`
	}
	if err != nil || result.IsError || json.Unmarshal(raw, &activeAttachPreview) != nil || activeAttachPreview.CommandHash == "" || activeAttachPreview.RequiredCapability != "gate:approve" || len(activeAttachPreview.Errors) != 1 || activeAttachPreview.Errors[0].Code != "human_approval_required" {
		t.Fatalf("active Gate attach preview approval evidence invalid: %s %v", raw, err)
	}

	unapprovedArguments := make(map[string]any, len(activeAttachPreviewArguments))
	for name, value := range activeAttachPreviewArguments {
		unapprovedArguments[name] = value
	}
	unapprovedArguments["idempotencyKey"] = testMCPUUID(t)
	result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "baley_gate_attach_task_execute", Arguments: unapprovedArguments})
	raw, _ = json.Marshal(result.StructuredContent)
	if err != nil || !result.IsError || !strings.Contains(string(raw), `"code":"human_approval_mismatch"`) {
		t.Fatalf("active Gate approval-less execute was not rejected: %s %v", raw, err)
	}

	approvedArguments := make(map[string]any, len(activeAttachPreviewArguments)+4)
	for name, value := range activeAttachPreviewArguments {
		approvedArguments[name] = value
	}
	approvedArguments["idempotencyKey"] = testMCPUUID(t)
	approvedArguments["approvedByActorId"] = "00000000-0000-4000-8000-000000000002"
	approvedArguments["approvedCommandHash"] = activeAttachPreview.CommandHash
	approvedArguments["conversationRef"] = "mcp-e2e-active-gate-approval"
	result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "baley_gate_attach_task_execute", Arguments: approvedArguments})
	raw, _ = json.Marshal(result.StructuredContent)
	var activeAttachResult struct {
		CommandID         string `json:"commandId"`
		WorkspaceRevision int64  `json:"workspaceRevision"`
	}
	if err != nil || result.IsError || json.Unmarshal(raw, &activeAttachResult) != nil || activeAttachResult.CommandID == "" || activeAttachResult.WorkspaceRevision != structuralRevision+1 {
		t.Fatalf("active Gate approved execute failed: %s %v", raw, err)
	}
	activeAttachEvents, eventErr := auditRepo.Events(ctx, postgresDemoWorkspaceID)
	if eventErr != nil {
		t.Fatal(eventErr)
	}
	foundActiveAttachEvent := false
	for _, event := range activeAttachEvents {
		if event.EventType == "gate.task_attached" && event.CommandID == activeAttachResult.CommandID && event.EntityID == "pilot-ready" && event.WorkspaceRevision == activeAttachResult.WorkspaceRevision {
			foundActiveAttachEvent = true
			break
		}
	}
	if !foundActiveAttachEvent {
		t.Fatalf("active gate.task_attached Event is not bound to command=%s gate=pilot-ready revision=%d", activeAttachResult.CommandID, activeAttachResult.WorkspaceRevision)
	}
	result, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "baley_event_list", Arguments: map[string]any{"workspaceId": postgresDemoWorkspaceID}})
	raw, _ = json.Marshal(result.StructuredContent)
	if err != nil || result.IsError {
		t.Fatalf("structural Event query failed: %s %v", raw, err)
	}
	for _, mutation := range structuralMutations {
		if !strings.Contains(string(raw), `"eventType":"`+mutation.eventType+`"`) {
			t.Fatalf("missing %s Event evidence: %s", mutation.eventType, raw)
		}
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

func assertTaskCreateSchema(t *testing.T, name string, raw []byte, execute bool) {
	t.Helper()
	var schema struct {
		Properties map[string]struct {
			Type  json.RawMessage `json:"type"`
			Items *struct {
				Type json.RawMessage `json:"type"`
			} `json:"items"`
		} `json:"properties"`
		Required []string `json:"required"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("%s schema decode failed: %v", name, err)
	}
	required := make(map[string]bool, len(schema.Required))
	for _, field := range schema.Required {
		required[field] = true
	}
	for _, field := range []string{"workspaceId", "taskUuid", "laneId", "phaseId", "title", "expectedWorkspaceRevision", "idempotencyKey", "executedByActorId"} {
		if !required[field] {
			t.Fatalf("%s schema does not require %s: %s", name, field, raw)
		}
	}
	for _, field := range []string{"parentTaskId", "description", "predecessorTaskIds", "successorTaskIds", "terminalReason", "acknowledgedWarningCodes", "proceedReason"} {
		if required[field] {
			t.Fatalf("%s schema unexpectedly requires %s: %s", name, field, raw)
		}
	}
	for _, field := range []string{"predecessorTaskIds", "successorTaskIds"} {
		property, ok := schema.Properties[field]
		if !ok || !schemaTypeIncludes(property.Type, "array") || property.Items == nil || !schemaTypeIncludes(property.Items.Type, "integer") {
			t.Fatalf("%s schema has invalid %s items: %s", name, field, raw)
		}
	}
	_, hasAck := schema.Properties["acknowledgedWarningCodes"]
	_, hasReason := schema.Properties["proceedReason"]
	if execute != (hasAck && hasReason) {
		t.Fatalf("%s warning evidence field boundary is invalid: %s", name, raw)
	}
}

func structuralToolRequiredFields(name string) ([]string, bool) {
	common := []string{"workspaceId", "expectedWorkspaceRevision", "idempotencyKey", "executedByActorId"}
	var fields []string
	switch {
	case strings.HasPrefix(name, "baley_phase_create_"):
		fields = []string{"phaseId", "name"}
	case strings.HasPrefix(name, "baley_lane_create_"):
		fields = []string{"laneId", "name"}
	case strings.HasPrefix(name, "baley_gate_create_"):
		fields = []string{"gateId", "name", "fromPhaseId", "toPhaseId"}
	case strings.HasPrefix(name, "baley_gate_attach_task_"):
		fields = []string{"gateId", "taskId"}
	default:
		return nil, false
	}
	return append(common, fields...), true
}

func assertStructuralCreateSchema(t *testing.T, name string, raw []byte, requiredFields []string) {
	t.Helper()
	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
		Required   []string                   `json:"required"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("%s schema decode failed: %v", name, err)
	}
	required := make(map[string]bool, len(schema.Required))
	for _, field := range schema.Required {
		required[field] = true
	}
	for _, field := range requiredFields {
		if !required[field] {
			t.Fatalf("%s schema does not require %s: %s", name, field, raw)
		}
	}
	for _, field := range []string{"goal", "summary", "clearTerminal", "initiatedByActorId", "acknowledgedWarningCodes", "proceedReason", "approvedByActorId", "approvedCommandHash", "decisionSnapshotHash", "statementHash", "conversationRef", "approvedAt"} {
		if required[field] {
			t.Fatalf("%s schema unexpectedly requires %s: %s", name, field, raw)
		}
	}
	isExecute := strings.HasSuffix(name, "_execute")
	_, hasAcknowledgements := schema.Properties["acknowledgedWarningCodes"]
	_, hasProceedReason := schema.Properties["proceedReason"]
	if isExecute != (hasAcknowledgements && hasProceedReason) {
		t.Fatalf("%s warning evidence field boundary is invalid: %s", name, raw)
	}
	if _, ok := schema.Properties["initiatedByActorId"]; !ok {
		t.Fatalf("%s schema lacks optional initiatedByActorId: %s", name, raw)
	}
	_, hasApprovedBy := schema.Properties["approvedByActorId"]
	_, hasApprovedHash := schema.Properties["approvedCommandHash"]
	_, hasApprovedAt := schema.Properties["approvedAt"]
	wantsConditionalApproval := name == "baley_gate_attach_task_execute"
	if wantsConditionalApproval != (hasApprovedBy && hasApprovedHash && hasApprovedAt) {
		t.Fatalf("%s conditional approval field boundary is invalid: %s", name, raw)
	}
}

func schemaTypeIncludes(raw json.RawMessage, expected string) bool {
	var single string
	if json.Unmarshal(raw, &single) == nil {
		return single == expected
	}
	var multiple []string
	if json.Unmarshal(raw, &multiple) != nil {
		return false
	}
	for _, value := range multiple {
		if value == expected {
			return true
		}
	}
	return false
}
