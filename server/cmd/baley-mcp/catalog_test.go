package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	legacyCatalogToolCount    = 78
	legacyCatalogSchemaBytes  = 37800
	compactCatalogToolCount   = 14
	compactCatalogSchemaBytes = 4184
	fullCatalogToolCount      = 82
	fullCatalogSchemaBytes    = 40124
)

var expectedCompactToolNames = []string{
	"baley_backlog_get",
	"baley_backlog_list",
	"baley_command_catalog",
	"baley_command_execute",
	"baley_command_execute_with_approval",
	"baley_command_preview",
	"baley_gate_status",
	"baley_lane_brief",
	"baley_mcp_diagnostics",
	"baley_phase_tasks",
	"baley_task_acceptance_get",
	"baley_task_get",
	"baley_workspace_context",
	"baley_workspace_get",
}

var expectedLegacyToolNames = []string{
	"baley_backlog_create_execute",
	"baley_backlog_create_preview",
	"baley_backlog_discard_execute",
	"baley_backlog_discard_preview",
	"baley_backlog_get",
	"baley_backlog_list",
	"baley_backlog_move_execute",
	"baley_backlog_move_preview",
	"baley_backlog_promote_execute",
	"baley_backlog_promote_preview",
	"baley_backlog_reorder_execute",
	"baley_backlog_reorder_preview",
	"baley_backlog_update_execute",
	"baley_backlog_update_preview",
	"baley_commit_attach",
	"baley_decision_list",
	"baley_dependency_patch_execute",
	"baley_dependency_patch_preview",
	"baley_event_list",
	"baley_gate_attach_entry_task_execute",
	"baley_gate_attach_entry_task_preview",
	"baley_gate_attach_task_execute",
	"baley_gate_attach_task_preview",
	"baley_gate_create_execute",
	"baley_gate_create_preview",
	"baley_gate_detach_entry_task_execute",
	"baley_gate_detach_entry_task_preview",
	"baley_gate_pass_execute",
	"baley_gate_pass_preview",
	"baley_gate_pass_task_execute",
	"baley_gate_pass_task_preview",
	"baley_gate_revoke_task_pass_execute",
	"baley_gate_revoke_task_pass_preview",
	"baley_gate_status",
	"baley_git_observe",
	"baley_lane_brief",
	"baley_lane_create_execute",
	"baley_lane_create_preview",
	"baley_mcp_diagnostics",
	"baley_mutation_attempt_list",
	"baley_phase_create_execute",
	"baley_phase_create_preview",
	"baley_phase_tasks",
	"baley_record_attach_commit",
	"baley_record_list",
	"baley_record_register",
	"baley_repository_register",
	"baley_run_cancel",
	"baley_run_correct",
	"baley_run_fail",
	"baley_run_heartbeat",
	"baley_run_interrupt",
	"baley_run_list",
	"baley_run_start",
	"baley_run_succeed",
	"baley_task_acceptance_get",
	"baley_task_acceptance_mode_escalate_execute",
	"baley_task_acceptance_mode_escalate_preview",
	"baley_task_acceptance_policy_change_execute",
	"baley_task_acceptance_policy_change_preview",
	"baley_task_clear_terminal_execute",
	"baley_task_clear_terminal_preview",
	"baley_task_confirm_execute",
	"baley_task_confirm_preview",
	"baley_task_create_execute",
	"baley_task_create_preview",
	"baley_task_discard_execute",
	"baley_task_discard_preview",
	"baley_task_evidence_report",
	"baley_task_get",
	"baley_task_move_execute",
	"baley_task_move_preview",
	"baley_task_report_implemented",
	"baley_task_update_execute",
	"baley_task_update_preview",
	"baley_workspace_context",
	"baley_workspace_get",
	"baley_workspace_graph",
}

func TestMCPToolCatalogProfilesReduceSerializedSchemaCost(t *testing.T) {
	legacyCount, legacyBytes := catalogMetrics(t, newLegacyMCPServer(&client{}))
	if legacyCount != legacyCatalogToolCount || legacyBytes != legacyCatalogSchemaBytes {
		t.Fatalf("legacy baseline drifted: tools=%d bytes=%d, want tools=%d bytes=%d", legacyCount, legacyBytes, legacyCatalogToolCount, legacyCatalogSchemaBytes)
	}
	compactCount, compactBytes := catalogMetrics(t, newMCPServer(&client{}))
	if compactCount != compactCatalogToolCount || compactBytes != compactCatalogSchemaBytes {
		t.Fatalf("compact catalog metrics: tools=%d bytes=%d, want tools=%d bytes=%d", compactCount, compactBytes, compactCatalogToolCount, compactCatalogSchemaBytes)
	}
	fullCount, fullBytes := catalogMetrics(t, newMCPServerForProfile(&client{}, mcpToolProfileFull))
	if fullCount != fullCatalogToolCount || fullBytes != fullCatalogSchemaBytes {
		t.Fatalf("full catalog metrics: tools=%d bytes=%d, want tools=%d bytes=%d", fullCount, fullBytes, fullCatalogToolCount, fullCatalogSchemaBytes)
	}
	if compactCount > 15 {
		t.Fatalf("compact catalog has %d tools, want at most 15", compactCount)
	}
	if compactBytes*4 > legacyBytes {
		t.Fatalf("compact schema reduction is below 75%%: compact=%d legacy=%d", compactBytes, legacyBytes)
	}
	t.Logf("legacy=%d tools/%d bytes compact=%d tools/%d bytes full=%d tools/%d bytes reduction=%.2f%%", legacyCount, legacyBytes, compactCount, compactBytes, fullCount, fullBytes, 100*(1-float64(compactBytes)/float64(legacyBytes)))
}

func TestCompactCatalogHasExactDeterministicDefaultToolList(t *testing.T) {
	tools := listMCPTools(t, newMCPServer(&client{}))
	names := toolNames(tools)
	if !reflect.DeepEqual(names, expectedCompactToolNames) {
		t.Fatalf("compact tool names:\n got %v\nwant %v", names, expectedCompactToolNames)
	}
}

func TestFullCatalogPreservesEveryLegacyToolName(t *testing.T) {
	legacy := toolNames(listMCPTools(t, newLegacyMCPServer(&client{})))
	if !reflect.DeepEqual(legacy, expectedLegacyToolNames) {
		t.Fatalf("legacy catalog name baseline drifted:\n got %v\nwant %v", legacy, expectedLegacyToolNames)
	}
	fullSet := map[string]bool{}
	for _, name := range toolNames(listMCPTools(t, newMCPServerForProfile(&client{}, mcpToolProfileFull))) {
		fullSet[name] = true
	}
	for _, name := range expectedLegacyToolNames {
		if !fullSet[name] {
			t.Errorf("full profile is missing legacy tool %s", name)
		}
	}
}

func TestCommandCatalogAndProfileDiagnosticsAreExplicit(t *testing.T) {
	for _, profile := range []mcpToolProfile{mcpToolProfileCompact, mcpToolProfileFull} {
		session := connectMCPServer(t, newMCPServerForProfile(&client{}, profile))
		result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "baley_mcp_diagnostics", Arguments: map[string]any{}})
		if err != nil {
			t.Fatal(err)
		}
		structured, ok := result.StructuredContent.(map[string]any)
		wantToolCount, wantSchemaBytes := compactCatalogToolCount, compactCatalogSchemaBytes
		if profile == mcpToolProfileFull {
			wantToolCount, wantSchemaBytes = fullCatalogToolCount, fullCatalogSchemaBytes
		}
		if !ok || structured["toolProfile"] != string(profile) || structured["toolCatalogVersion"] != mcpToolCatalogVersion ||
			structured["mcpImplementationVersion"] != mcpImplementationVersion || structured["fullProfileOptInPath"] != "/mcp/full" ||
			structured["toolCount"] != float64(wantToolCount) || structured["serializedInputSchemaBytes"] != float64(wantSchemaBytes) {
			t.Fatalf("profile %s diagnostics=%#v", profile, result.StructuredContent)
		}
	}

	session := connectMCPServer(t, newMCPServer(&client{}))
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "baley_command_catalog", Arguments: map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	structured, ok := result.StructuredContent.(map[string]any)
	commands, commandsOK := structured["commands"].([]any)
	if !ok || !commandsOK || structured["catalogVersion"] != mcpToolCatalogVersion || len(commands) != len(commandDescriptors) {
		t.Fatalf("command catalog=%#v", result.StructuredContent)
	}
	assertCommandClassification(t, commands, "task.update", string(commandClassOperator), "baley_command_execute", "none")
	assertCommandClassification(t, commands, "task.confirm", string(commandClassHuman), "baley_command_execute_with_approval", "always")
	assertCommandClassification(t, commands, "gate.attach_task", string(commandClassConditional), "baley_command_execute_with_approval", "when_from_phase_active")
}

func TestGenericCommandBridgeRejectsUnknownMalformedAndMisclassifiedCalls(t *testing.T) {
	var requests atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()
	session := connectMCPServer(t, newMCPServer(&client{base: upstream.URL, http: upstream.Client()}))
	envelope := map[string]any{"idempotencyKey": "test", "expectedWorkspaceRevision": 1, "executedByActorId": "agent"}

	tests := []struct {
		name      string
		tool      string
		command   string
		arguments any
		envelope  map[string]any
	}{
		{name: "unknown command", tool: "baley_command_preview", command: "database.drop", arguments: map[string]any{"workspaceId": "workspace"}, envelope: envelope},
		{name: "missing workspace", tool: "baley_command_preview", command: "task.update", arguments: map[string]any{"taskId": 1}, envelope: envelope},
		{name: "malformed arguments", tool: "baley_command_preview", command: "task.update", arguments: []any{"workspace"}, envelope: envelope},
		{name: "legacy approval authority", tool: "baley_command_execute_with_approval", command: "task.confirm", arguments: map[string]any{"workspaceId": "workspace", "taskId": 1}, envelope: map[string]any{"idempotencyKey": "test", "expectedWorkspaceRevision": 1, "executedByActorId": "agent", "humanApprovalAttestation": map[string]any{}}},
		{name: "human command on operator bridge", tool: "baley_command_execute", command: "task.confirm", arguments: map[string]any{"workspaceId": "workspace", "taskId": 1}, envelope: envelope},
		{name: "operator command on approval bridge", tool: "baley_command_execute_with_approval", command: "task.update", arguments: map[string]any{"workspaceId": "workspace", "taskId": 1}, envelope: envelope},
		{name: "missing browser grant", tool: "baley_command_execute_with_approval", command: "task.confirm", arguments: map[string]any{"workspaceId": "workspace", "taskId": 1}, envelope: envelope},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: test.tool, Arguments: map[string]any{"command": test.command, "arguments": test.arguments, "envelope": test.envelope}})
			if err != nil {
				t.Fatal(err)
			}
			if !result.IsError {
				t.Fatalf("malformed/misclassified call succeeded: %#v", result)
			}
		})
	}
	if requests.Load() != 0 {
		t.Fatalf("rejected bridge calls reached HTTP server %d times", requests.Load())
	}
}

func TestGenericCommandBridgeForwardsOnlyToFixedCommandEndpoints(t *testing.T) {
	type capturedRequest struct {
		Path string
		Body map[string]any
	}
	captured := make(chan capturedRequest, 4)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		captured <- capturedRequest{Path: r.URL.Path, Body: body}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()
	session := connectMCPServer(t, newMCPServer(&client{base: upstream.URL, http: upstream.Client()}))

	calls := []struct {
		tool, command, wantPath string
		envelope                map[string]any
	}{
		{"baley_command_preview", "task.update", "/v1/commands/preview", map[string]any{"idempotencyKey": "preview", "executedByActorId": "agent"}},
		{"baley_command_execute", "task.update", "/v1/commands/execute", map[string]any{"idempotencyKey": "execute", "expectedWorkspaceRevision": 2, "executedByActorId": "agent", "acknowledgedWarningCodes": []any{"dangling_path"}}},
		{"baley_command_execute_with_approval", "task.confirm", "/v1/commands/execute", map[string]any{"idempotencyKey": "human", "expectedWorkspaceRevision": 3, "executedByActorId": "agent", "approvalGrantId": "browser-grant"}},
		{"baley_command_execute_with_approval", "gate.attach_task", "/v1/commands/execute", map[string]any{"idempotencyKey": "conditional", "expectedWorkspaceRevision": 4, "executedByActorId": "agent"}},
	}
	for _, call := range calls {
		result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: call.tool, Arguments: map[string]any{
			"command": call.command, "arguments": map[string]any{"workspaceId": "workspace", "taskId": 7}, "envelope": call.envelope,
		}})
		if err != nil || result.IsError {
			t.Fatalf("%s %s failed: result=%#v err=%v", call.tool, call.command, result, err)
		}
		got := <-captured
		if got.Path != call.wantPath || got.Body["name"] != call.command {
			t.Fatalf("bridge target=%s body=%#v, want path=%s command=%s", got.Path, got.Body, call.wantPath, call.command)
		}
		arguments, argsOK := got.Body["arguments"].(map[string]any)
		envelope, envelopeOK := got.Body["envelope"].(map[string]any)
		if !argsOK || arguments["workspaceId"] != "workspace" || !envelopeOK || envelope["idempotencyKey"] != call.envelope["idempotencyKey"] {
			t.Fatalf("bridge changed payload: %#v", got.Body)
		}
	}
}

func catalogMetrics(t *testing.T, server *mcp.Server) (int, int) {
	t.Helper()
	tools := listMCPTools(t, server)
	schemaBytes := 0
	for _, tool := range tools {
		raw, err := json.Marshal(tool.InputSchema)
		if err != nil {
			t.Fatalf("marshal %s input schema: %v", tool.Name, err)
		}
		schemaBytes += len(raw)
	}
	return len(tools), schemaBytes
}

func toolNames(tools []*mcp.Tool) []string {
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool.Name)
	}
	return names
}

func listMCPTools(t *testing.T, server *mcp.Server) []*mcp.Tool {
	t.Helper()
	session := connectMCPServer(t, server)
	listed, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	return listed.Tools
}

func connectMCPServer(t *testing.T, server *mcp.Server) *mcp.ClientSession {
	t.Helper()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(context.Background(), serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = serverSession.Close() })
	clientSession, err := mcp.NewClient(&mcp.Implementation{Name: "catalog-test", Version: "test"}, nil).Connect(context.Background(), clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = clientSession.Close() })
	return clientSession
}

func assertCommandClassification(t *testing.T, commands []any, name, classification, executeTool, humanApproval string) {
	t.Helper()
	for _, value := range commands {
		command, ok := value.(map[string]any)
		if ok && command["name"] == name {
			if command["classification"] != classification || command["executeTool"] != executeTool || command["humanApproval"] != humanApproval {
				t.Fatalf("command %s classification=%#v", name, command)
			}
			return
		}
	}
	t.Fatalf("command catalog is missing %s", name)
}

func TestCommandDescriptorsAreUniqueAndSorted(t *testing.T) {
	names := make([]string, 0, len(commandDescriptors))
	for _, descriptor := range commandDescriptors {
		names = append(names, descriptor.Name)
	}
	sorted := append([]string(nil), names...)
	sort.Strings(sorted)
	if !reflect.DeepEqual(names, sorted) {
		t.Fatalf("command descriptors are not sorted: %v", names)
	}
	for index := 1; index < len(names); index++ {
		if names[index] == names[index-1] {
			t.Fatalf("duplicate command descriptor %s", names[index])
		}
	}
	if len(commandDescriptors) != 49 {
		t.Fatalf("command descriptor count=%d, want 49 HTTP commands", len(commandDescriptors))
	}
	for _, descriptor := range commandDescriptors {
		if strings.TrimSpace(descriptor.Name) != descriptor.Name {
			t.Fatalf("non-canonical command name %q", descriptor.Name)
		}
	}
}

func TestMCPToolCatalogMatchesLiteralCommandContract(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "contracts", "v1", "commands.json"))
	if err != nil {
		t.Fatal(err)
	}
	var contract struct {
		MCPCatalog struct {
			CatalogVersion string `json:"catalogVersion"`
			DefaultProfile string `json:"defaultProfile"`
			Profiles       struct {
				Compact struct {
					Path                       string   `json:"path"`
					MaximumToolCount           int      `json:"maximumToolCount"`
					ToolCount                  int      `json:"toolCount"`
					SerializedInputSchemaBytes int      `json:"serializedInputSchemaBytes"`
					Tools                      []string `json:"tools"`
				} `json:"compact"`
				Full struct {
					Path                       string `json:"path"`
					ToolCount                  int    `json:"toolCount"`
					SerializedInputSchemaBytes int    `json:"serializedInputSchemaBytes"`
					LegacyToolCount            int    `json:"legacyToolCount"`
					LegacyToolParity           bool   `json:"legacyToolParity"`
				} `json:"full"`
			} `json:"profiles"`
			Baseline struct {
				ToolCount                  int    `json:"toolCount"`
				SerializedInputSchemaBytes int    `json:"serializedInputSchemaBytes"`
				MinimumReductionPercent    int    `json:"minimumReductionPercent"`
				Measurement                string `json:"measurement"`
			} `json:"baseline"`
			CommandBridge struct {
				CatalogTool                string `json:"catalogTool"`
				PreviewTool                string `json:"previewTool"`
				OperatorExecuteTool        string `json:"operatorExecuteTool"`
				ApprovalExecuteTool        string `json:"approvalExecuteTool"`
				HTTPCommandCount           int    `json:"httpCommandCount"`
				UnknownCommandHandling     string `json:"unknownCommandHandling"`
				CommandValidationAuthority string `json:"commandValidationAuthority"`
			} `json:"commandBridge"`
		} `json:"mcpCatalog"`
		Mutations map[string]struct {
			HumanApproval string `json:"humanApproval"`
		} `json:"mutations"`
	}
	if err := json.Unmarshal(raw, &contract); err != nil {
		t.Fatal(err)
	}
	mcpContract := contract.MCPCatalog
	if mcpContract.CatalogVersion != mcpToolCatalogVersion || mcpContract.DefaultProfile != string(mcpToolProfileCompact) ||
		mcpContract.Profiles.Compact.Path != "/mcp" || mcpContract.Profiles.Full.Path != "/mcp/full" || !mcpContract.Profiles.Full.LegacyToolParity ||
		mcpContract.Profiles.Compact.MaximumToolCount != 15 || mcpContract.Profiles.Compact.ToolCount != compactCatalogToolCount ||
		mcpContract.Profiles.Compact.SerializedInputSchemaBytes != compactCatalogSchemaBytes || mcpContract.Profiles.Full.ToolCount != fullCatalogToolCount ||
		mcpContract.Profiles.Full.SerializedInputSchemaBytes != fullCatalogSchemaBytes || mcpContract.Profiles.Full.LegacyToolCount != legacyCatalogToolCount ||
		mcpContract.Baseline.ToolCount != legacyCatalogToolCount || mcpContract.Baseline.SerializedInputSchemaBytes != legacyCatalogSchemaBytes ||
		mcpContract.Baseline.MinimumReductionPercent != 75 || mcpContract.Baseline.Measurement != "sum of JSON-serialized inputSchema bytes" ||
		!reflect.DeepEqual(mcpContract.Profiles.Compact.Tools, expectedCompactToolNames) {
		t.Fatalf("MCP catalog contract drifted: %+v", mcpContract)
	}
	bridge := mcpContract.CommandBridge
	if bridge.CatalogTool != "baley_command_catalog" || bridge.PreviewTool != "baley_command_preview" || bridge.OperatorExecuteTool != "baley_command_execute" ||
		bridge.ApprovalExecuteTool != "baley_command_execute_with_approval" || bridge.HTTPCommandCount != len(commandDescriptors) ||
		bridge.UnknownCommandHandling != "reject_before_http" || bridge.CommandValidationAuthority != "server" {
		t.Fatalf("MCP command bridge contract drifted: %+v", bridge)
	}
	bridged := make(map[string]bool, len(commandDescriptors))
	for _, descriptor := range commandDescriptors {
		bridged[descriptor.Name] = true
		mutation, ok := contract.Mutations[descriptor.Name]
		if !ok {
			t.Errorf("bridge command %s is missing from mutation contract", descriptor.Name)
			continue
		}
		wantClass := commandClassOperator
		if mutation.HumanApproval == "when_from_phase_active" {
			wantClass = commandClassConditional
		} else if mutation.HumanApproval != "none" {
			wantClass = commandClassHuman
		}
		if descriptor.Classification != wantClass || descriptor.HumanApproval != mutation.HumanApproval {
			t.Errorf("command %s classification=%s/%s, want %s/%s", descriptor.Name, descriptor.Classification, descriptor.HumanApproval, wantClass, mutation.HumanApproval)
		}
	}
	notBridged := make([]string, 0, len(contract.Mutations)-len(bridged))
	for name := range contract.Mutations {
		if !bridged[name] {
			notBridged = append(notBridged, name)
		}
	}
	sort.Strings(notBridged)
	if want := []string{"project.bootstrap", "workspace.activate", "workspace.create"}; !reflect.DeepEqual(notBridged, want) {
		t.Fatalf("commands outside the generic HTTP bridge=%v, want domain/bootstrap-only %v", notBridged, want)
	}
}
