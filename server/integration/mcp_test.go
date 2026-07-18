package integration_test

import (
	"context"
	"os"
	"os/exec"
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
	cmd := exec.CommandContext(ctx, "go", "run", "./cmd/baley-mcp")
	cmd.Dir = ".."
	cmd.Env = append(os.Environ(), "BALEY_SERVER_URL=http://127.0.0.1:8080")
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
	if len(tools.Tools) != 14 {
		t.Fatalf("tool count=%d", len(tools.Tools))
	}
	want := map[string]bool{"baley_workspace_get": true, "baley_workspace_graph": true, "baley_task_get": true, "baley_gate_status": true, "baley_decision_list": true, "baley_event_list": true, "baley_task_confirm_preview": true, "baley_task_confirm_execute": true, "baley_gate_pass_task_preview": true, "baley_gate_pass_task_execute": true, "baley_gate_revoke_task_pass_preview": true, "baley_gate_revoke_task_pass_execute": true, "baley_gate_pass_preview": true, "baley_gate_pass_execute": true}
	for _, tool := range tools.Tools {
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
}

const postgresDemoWorkspaceID = "00000000-0000-4000-8000-000000000001"
