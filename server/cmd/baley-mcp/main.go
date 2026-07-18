package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type client struct {
	base string
	http *http.Client
}
type workspaceInput struct {
	WorkspaceID string `json:"workspaceId" jsonschema:"Baley workspace ID"`
}
type taskInput struct {
	WorkspaceID string `json:"workspaceId"`
	TaskID      int    `json:"taskId"`
}
type gateInput struct {
	WorkspaceID string `json:"workspaceId"`
	GateID      string `json:"gateId"`
}
type previewEnvelope struct {
	ExpectedWorkspaceRevision int64  `json:"expectedWorkspaceRevision"`
	IdempotencyKey            string `json:"idempotencyKey"`
	ExecutedByActorID         string `json:"executedByActorId"`
}
type executeEnvelope struct {
	ExpectedWorkspaceRevision int64  `json:"expectedWorkspaceRevision"`
	IdempotencyKey            string `json:"idempotencyKey"`
	ExecutedByActorID         string `json:"executedByActorId"`
	ApprovedByActorID         string `json:"approvedByActorId"`
	ApprovedCommandHash       string `json:"approvedCommandHash"`
	DecisionSnapshotHash      string `json:"decisionSnapshotHash,omitempty"`
	StatementHash             string `json:"statementHash,omitempty"`
	ConversationRef           string `json:"conversationRef,omitempty"`
}
type taskConfirmPreviewInput struct {
	WorkspaceID string `json:"workspaceId"`
	TaskID      int    `json:"taskId"`
	previewEnvelope
}
type taskConfirmExecuteInput struct {
	WorkspaceID string `json:"workspaceId"`
	TaskID      int    `json:"taskId"`
	executeEnvelope
}
type gatePassPreviewInput struct {
	WorkspaceID string `json:"workspaceId"`
	GateID      string `json:"gateId"`
	previewEnvelope
}
type gatePassExecuteInput struct {
	WorkspaceID string `json:"workspaceId"`
	GateID      string `json:"gateId"`
	executeEnvelope
}
type gateTaskPreviewInput struct {
	WorkspaceID string `json:"workspaceId"`
	GateTaskID  string `json:"gateTaskId"`
	Reason      string `json:"reason"`
	previewEnvelope
}
type gateTaskExecuteInput struct {
	WorkspaceID string `json:"workspaceId"`
	GateTaskID  string `json:"gateTaskId"`
	Reason      string `json:"reason"`
	executeEnvelope
}

func main() {
	base := os.Getenv("BALEY_SERVER_URL")
	if base == "" {
		base = "http://127.0.0.1:8080"
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.Scheme != "http" || !(parsed.Hostname() == "127.0.0.1" || parsed.Hostname() == "localhost" || parsed.Hostname() == "::1") {
		log.Fatal("BALEY_SERVER_URL must be a loopback http URL")
	}
	c := &client{base: strings.TrimRight(base, "/"), http: &http.Client{Timeout: 15 * time.Second}}
	server := mcp.NewServer(&mcp.Implementation{Name: "baley", Version: "0.1.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_workspace_get", Description: "Read Workspace metadata"}, c.workspaceGet)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_workspace_graph", Description: "Read the current Workspace graph"}, c.workspaceGraph)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_task_get", Description: "Read one Task by public ID"}, c.taskGet)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_gate_status", Description: "Read Gate status and conditions"}, c.gateStatus)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_decision_list", Description: "List human decisions currently available"}, c.decisionList)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_event_list", Description: "List Workspace Events"}, c.eventList)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_task_confirm_preview", Description: "Preview Task confirmation without writing"}, c.taskConfirmPreview)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_task_confirm_execute", Description: "Execute an explicitly approved Task confirmation"}, c.taskConfirmExecute)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_gate_pass_task_preview", Description: "Preview explicit Gate Task pass without writing"}, c.gatePassTaskPreview)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_gate_pass_task_execute", Description: "Execute an explicitly approved Gate Task pass"}, c.gatePassTaskExecute)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_gate_revoke_task_pass_preview", Description: "Preview Gate Task pass revocation without writing"}, c.gateRevokePreview)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_gate_revoke_task_pass_execute", Description: "Execute an explicitly approved Gate Task pass revocation"}, c.gateRevokeExecute)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_gate_pass_preview", Description: "Preview Gate pass and Phase transition without writing"}, c.gatePassPreview)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_gate_pass_execute", Description: "Execute an explicitly approved Gate pass and Phase transition"}, c.gatePassExecute)
	if err = server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}

func (c *client) get(ctx context.Context, path string) (*mcp.CallToolResult, any, error) {
	return c.call(ctx, "GET", path, nil)
}
func (c *client) call(ctx context.Context, method, path string, payload any) (*mcp.CallToolResult, any, error) {
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, nil, err
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, body)
	if err != nil {
		return nil, nil, err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.http.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("Baley HTTP transport: %w", err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if err != nil {
		return nil, nil, err
	}
	var structured any
	if err = json.Unmarshal(raw, &structured); err != nil {
		structured = map[string]any{"httpStatus": res.StatusCode, "raw": string(raw)}
	}
	summary := fmt.Sprintf("Baley HTTP %d", res.StatusCode)
	if res.StatusCode >= 200 && res.StatusCode < 300 {
		summary = "Baley request succeeded"
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: summary}}, StructuredContent: structured, IsError: res.StatusCode >= 400}, structured, nil
}
func command(name string, args any, envelope any) map[string]any {
	return map[string]any{"name": name, "arguments": args, "envelope": envelope}
}
func previewEnv(v previewEnvelope) map[string]any {
	return map[string]any{"expectedWorkspaceRevision": v.ExpectedWorkspaceRevision, "idempotencyKey": v.IdempotencyKey, "executedByActorId": v.ExecutedByActorID}
}
func executeEnv(v executeEnvelope) map[string]any {
	return map[string]any{"expectedWorkspaceRevision": v.ExpectedWorkspaceRevision, "idempotencyKey": v.IdempotencyKey, "executedByActorId": v.ExecutedByActorID, "humanApprovalAttestation": map[string]any{"approvedByActorId": v.ApprovedByActorID, "approvedCommandHash": v.ApprovedCommandHash, "decisionSnapshotHash": v.DecisionSnapshotHash, "statementHash": v.StatementHash, "conversationRef": v.ConversationRef}}
}

func (c *client) workspaceGraph(ctx context.Context, _ *mcp.CallToolRequest, in workspaceInput) (*mcp.CallToolResult, any, error) {
	return c.get(ctx, "/v1/workspaces/"+url.PathEscape(in.WorkspaceID)+"/graph")
}
func (c *client) workspaceGet(ctx context.Context, _ *mcp.CallToolRequest, in workspaceInput) (*mcp.CallToolResult, any, error) {
	return c.get(ctx, "/v1/workspaces/"+url.PathEscape(in.WorkspaceID))
}
func (c *client) taskGet(ctx context.Context, _ *mcp.CallToolRequest, in taskInput) (*mcp.CallToolResult, any, error) {
	return c.get(ctx, fmt.Sprintf("/v1/workspaces/%s/tasks/%d", url.PathEscape(in.WorkspaceID), in.TaskID))
}
func (c *client) gateStatus(ctx context.Context, _ *mcp.CallToolRequest, in gateInput) (*mcp.CallToolResult, any, error) {
	return c.get(ctx, "/v1/workspaces/"+url.PathEscape(in.WorkspaceID)+"/gates/"+url.PathEscape(in.GateID)+"/status")
}
func (c *client) decisionList(ctx context.Context, _ *mcp.CallToolRequest, in workspaceInput) (*mcp.CallToolResult, any, error) {
	return c.get(ctx, "/v1/workspaces/"+url.PathEscape(in.WorkspaceID)+"/decisions")
}
func (c *client) eventList(ctx context.Context, _ *mcp.CallToolRequest, in workspaceInput) (*mcp.CallToolResult, any, error) {
	return c.get(ctx, "/v1/workspaces/"+url.PathEscape(in.WorkspaceID)+"/events")
}
func (c *client) taskConfirmPreview(ctx context.Context, _ *mcp.CallToolRequest, in taskConfirmPreviewInput) (*mcp.CallToolResult, any, error) {
	return c.call(ctx, "POST", "/v1/commands/preview", command("task.confirm", map[string]any{"workspaceId": in.WorkspaceID, "taskId": in.TaskID}, previewEnv(in.previewEnvelope)))
}
func (c *client) taskConfirmExecute(ctx context.Context, _ *mcp.CallToolRequest, in taskConfirmExecuteInput) (*mcp.CallToolResult, any, error) {
	return c.call(ctx, "POST", "/v1/commands/execute", command("task.confirm", map[string]any{"workspaceId": in.WorkspaceID, "taskId": in.TaskID}, executeEnv(in.executeEnvelope)))
}
func (c *client) gatePassPreview(ctx context.Context, _ *mcp.CallToolRequest, in gatePassPreviewInput) (*mcp.CallToolResult, any, error) {
	return c.call(ctx, "POST", "/v1/commands/preview", command("gate.pass", map[string]any{"workspaceId": in.WorkspaceID, "gateId": in.GateID}, previewEnv(in.previewEnvelope)))
}
func (c *client) gatePassExecute(ctx context.Context, _ *mcp.CallToolRequest, in gatePassExecuteInput) (*mcp.CallToolResult, any, error) {
	return c.call(ctx, "POST", "/v1/commands/execute", command("gate.pass", map[string]any{"workspaceId": in.WorkspaceID, "gateId": in.GateID}, executeEnv(in.executeEnvelope)))
}
func (c *client) gateTask(ctx context.Context, name, path string, in gateTaskPreviewInput) (*mcp.CallToolResult, any, error) {
	return c.call(ctx, "POST", path, command(name, map[string]any{"workspaceId": in.WorkspaceID, "gateTaskId": in.GateTaskID, "reason": in.Reason}, previewEnv(in.previewEnvelope)))
}
func (c *client) gateTaskExec(ctx context.Context, name string, in gateTaskExecuteInput) (*mcp.CallToolResult, any, error) {
	return c.call(ctx, "POST", "/v1/commands/execute", command(name, map[string]any{"workspaceId": in.WorkspaceID, "gateTaskId": in.GateTaskID, "reason": in.Reason}, executeEnv(in.executeEnvelope)))
}
func (c *client) gatePassTaskPreview(ctx context.Context, _ *mcp.CallToolRequest, in gateTaskPreviewInput) (*mcp.CallToolResult, any, error) {
	return c.gateTask(ctx, "gate.pass_task", "/v1/commands/preview", in)
}
func (c *client) gatePassTaskExecute(ctx context.Context, _ *mcp.CallToolRequest, in gateTaskExecuteInput) (*mcp.CallToolResult, any, error) {
	return c.gateTaskExec(ctx, "gate.pass_task", in)
}
func (c *client) gateRevokePreview(ctx context.Context, _ *mcp.CallToolRequest, in gateTaskPreviewInput) (*mcp.CallToolResult, any, error) {
	return c.gateTask(ctx, "gate.revoke_task_pass", "/v1/commands/preview", in)
}
func (c *client) gateRevokeExecute(ctx context.Context, _ *mcp.CallToolRequest, in gateTaskExecuteInput) (*mcp.CallToolResult, any, error) {
	return c.gateTaskExec(ctx, "gate.revoke_task_pass", in)
}
