package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMCPStreamableHTTPListsAndCallsTools(t *testing.T) {
	if os.Getenv("BALEY_MCP_E2E") == "" {
		t.Skip("BALEY_MCP_E2E is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	binary := strings.TrimSpace(os.Getenv("BALEY_MCP_BINARY"))
	if binary == "" {
		t.Fatal("BALEY_MCP_BINARY must point to a prebuilt baley-mcp executable")
	}
	serverURL := strings.TrimRight(strings.TrimSpace(os.Getenv("BALEY_SERVER_URL")), "/")
	if serverURL == "" {
		t.Fatal("BALEY_SERVER_URL is required when BALEY_MCP_E2E is set")
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	_ = listener.Close()

	cmd := exec.CommandContext(ctx, binary, "serve-http")
	cmd.Env = append(os.Environ(),
		"BALEY_SERVER_URL="+serverURL,
		"BALEY_MCP_HTTP_ADDR="+address,
		"BALEY_MCP_CREDENTIAL_STORE="+filepath.Join(t.TempDir(), "credentials.json"),
	)
	var processOutput strings.Builder
	cmd.Stdout, cmd.Stderr = &processOutput, &processOutput
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})

	deadline := time.Now().Add(5 * time.Second)
	for {
		connection, dialErr := net.DialTimeout("tcp", address, 100*time.Millisecond)
		if dialErr == nil {
			_ = connection.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("Baley MCP Gateway did not start: %v\n%s", dialErr, processOutput.String())
		}
		time.Sleep(25 * time.Millisecond)
	}

	client := mcp.NewClient(&mcp.Implementation{Name: "baley-integration-test", Version: "0.1.0"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: fmt.Sprintf("http://%s/mcp", address)}, nil)
	if err != nil {
		t.Fatalf("Streamable HTTP initialize failed: %v\n%s", err, processOutput.String())
	}
	defer session.Close()

	listed, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(listed.Tools), 15; got != want {
		t.Fatalf("compact MCP catalog contains %d tools, want %d", got, want)
	}
	toolsByName := make(map[string]*mcp.Tool, len(listed.Tools))
	for _, tool := range listed.Tools {
		toolsByName[tool.Name] = tool
	}
	for _, name := range []string{
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
		"baley_task_rework_execute",
		"baley_task_rework_preview",
		"baley_workspace_context",
	} {
		if toolsByName[name] == nil {
			t.Fatalf("missing MCP tool %s", name)
		}
	}
	if toolsByName["baley_task_create_execute"] != nil {
		t.Fatal("compact MCP catalog unexpectedly exposes a legacy typed command tool")
	}

	fullSession, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: fmt.Sprintf("http://%s/mcp/full", address)}, nil)
	if err != nil {
		t.Fatalf("full-profile Streamable HTTP initialize failed: %v\n%s", err, processOutput.String())
	}
	defer fullSession.Close()
	fullListed, err := fullSession.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(fullListed.Tools), 84; got != want {
		t.Fatalf("full MCP catalog contains %d tools, want %d", got, want)
	}
	fullToolsByName := make(map[string]*mcp.Tool, len(fullListed.Tools))
	for _, tool := range fullListed.Tools {
		fullToolsByName[tool.Name] = tool
	}
	for _, name := range []string{
		"baley_task_create_preview",
		"baley_task_create_execute",
		"baley_task_confirm_execute",
		"baley_task_rework_execute",
		"baley_task_rework_preview",
		"baley_gate_pass_execute",
		"baley_command_catalog",
	} {
		if fullToolsByName[name] == nil {
			t.Fatalf("full MCP catalog is missing %s", name)
		}
	}

	for _, name := range []string{
		"baley_task_acceptance_policy_change_execute",
		"baley_task_acceptance_mode_escalate_execute",
		"baley_gate_attach_task_execute",
		"baley_task_confirm_execute",
		"baley_task_discard_execute",
		"baley_gate_pass_task_execute",
		"baley_gate_revoke_task_pass_execute",
		"baley_gate_pass_execute",
	} {
		tool := fullToolsByName[name]
		if tool == nil {
			t.Fatalf("missing human-only MCP tool %s", name)
		}
		raw, marshalErr := json.Marshal(tool.InputSchema)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		schema := string(raw)
		if !strings.Contains(schema, `"approvalGrantId"`) {
			t.Fatalf("%s lacks browser grant reference: %s", name, schema)
		}
		for _, removed := range []string{"approvedByActorId", "approvedCommandHash", "approvedAt", "approvalGrantToken"} {
			if strings.Contains(schema, `"`+removed+`"`) {
				t.Fatalf("%s exposes removed approval field %s: %s", name, removed, schema)
			}
		}
	}
	if os.Getenv("BALEY_MCP_E2E_LIST_ONLY") != "" {
		return
	}

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "baley_workspace_context",
		Arguments: map[string]any{"workspaceId": "00000000-0000-4000-8000-000000000001"},
	})
	if err != nil {
		t.Fatal(err)
	}
	structured, ok := result.StructuredContent.(map[string]any)
	loginURL, _ := structured["loginUrl"].(string)
	if !result.IsError || !ok || structured["code"] != "workspace_login_required" || !strings.HasPrefix(loginURL, "http://"+address+"/mcp-login/start?") {
		t.Fatalf("first unregistered device did not return its loopback login URL: %#v", result.StructuredContent)
	}
}
