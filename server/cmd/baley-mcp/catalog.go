package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	mcpImplementationVersion = "0.2.0"
	mcpToolCatalogVersion    = "1.1.0"
	mcpCompactToolCount      = 15
	mcpCompactSchemaBytes    = 4700
	mcpFullToolCount         = 89
	mcpFullSchemaBytes       = 46217

	mcpToolProfileCompact mcpToolProfile = "compact"
	mcpToolProfileFull    mcpToolProfile = "full"

	commandClassOperator    commandClass = "operator"
	commandClassHuman       commandClass = "human_approval"
	commandClassConditional commandClass = "conditional_approval"

	bridgePreview         bridgeMode = "preview"
	bridgeExecuteOperator bridgeMode = "execute_operator"
	bridgeExecuteApproval bridgeMode = "execute_with_approval"
)

type mcpToolProfile string
type commandClass string
type bridgeMode string

type commandCatalogInput struct{}

type commandBridgeInput struct {
	Command   string         `json:"command"`
	Arguments map[string]any `json:"arguments"`
	Envelope  map[string]any `json:"envelope"`
}

type commandDescriptor struct {
	Name           string       `json:"name"`
	Classification commandClass `json:"classification"`
	HumanApproval  string       `json:"humanApproval"`
	ExecuteTool    string       `json:"executeTool"`
}

// commandDescriptors is the deterministic MCP bridge allow-list for commands
// accepted by POST /v1/commands/{preview,execute}. Command-specific argument,
// capability, revision, warning, idempotency, and approval checks remain in the
// HTTP command service; this list only prevents arbitrary endpoint tunnelling
// and selects the correctly annotated execution bridge.
var commandDescriptors = []commandDescriptor{
	{Name: "backlog.create"},
	{Name: "backlog.discard"},
	{Name: "backlog.move"},
	{Name: "backlog.promote"},
	{Name: "backlog.reorder"},
	{Name: "backlog.update"},
	{Name: "commit.attach"},
	{Name: "dependency.connect"},
	{Name: "dependency.disconnect"},
	{Name: "dependency.patch"},
	{Name: "gate.attach_entry_task"},
	{Name: "gate.attach_task", Classification: commandClassConditional, HumanApproval: "when_from_phase_active"},
	{Name: "gate.create"},
	{Name: "gate.detach_entry_task"},
	{Name: "gate.detach_task"},
	{Name: "gate.pass", Classification: commandClassHuman, HumanApproval: "always"},
	{Name: "gate.pass_task", Classification: commandClassHuman, HumanApproval: "always"},
	{Name: "gate.revoke_task_pass", Classification: commandClassHuman, HumanApproval: "always"},
	{Name: "git.observe"},
	{Name: "lane.close_out", Classification: commandClassHuman, HumanApproval: "always"},
	{Name: "lane.create"},
	{Name: "lane.discard", Classification: commandClassHuman, HumanApproval: "always"},
	{Name: "lane.update"},
	{Name: "phase.create"},
	{Name: "record.attach_commit"},
	{Name: "record.register"},
	{Name: "repository.register"},
	{Name: "run.cancel"},
	{Name: "run.correct"},
	{Name: "run.fail"},
	{Name: "run.heartbeat"},
	{Name: "run.interrupt"},
	{Name: "run.start"},
	{Name: "run.succeed"},
	{Name: "task.acceptance_mode.escalate", Classification: commandClassHuman, HumanApproval: "always"},
	{Name: "task.acceptance_policy.change", Classification: commandClassHuman, HumanApproval: "always"},
	{Name: "task.block"},
	{Name: "task.clear_terminal"},
	{Name: "task.confirm", Classification: commandClassHuman, HumanApproval: "always"},
	{Name: "task.create"},
	{Name: "task.discard", Classification: commandClassHuman, HumanApproval: "always"},
	{Name: "task.evidence.report"},
	{Name: "task.move"},
	{Name: "task.report_implemented"},
	{Name: "task.rework"},
	{Name: "task.set_terminal"},
	{Name: "task.unblock"},
	{Name: "task.update"},
	{Name: "workspace.close", Classification: commandClassHuman, HumanApproval: "always_owner"},
}

func init() {
	for i := range commandDescriptors {
		descriptor := &commandDescriptors[i]
		if descriptor.Classification == "" {
			descriptor.Classification = commandClassOperator
		}
		if descriptor.HumanApproval == "" {
			descriptor.HumanApproval = "none"
		}
		if descriptor.Classification == commandClassOperator {
			descriptor.ExecuteTool = "baley_command_execute"
		} else {
			descriptor.ExecuteTool = "baley_command_execute_with_approval"
		}
	}
}

func newMCPServer(c *client) *mcp.Server {
	return newMCPServerForProfile(c, mcpToolProfileCompact)
}

func newMCPServerForProfile(c *client, profile mcpToolProfile) *mcp.Server {
	var server *mcp.Server
	if profile == mcpToolProfileFull {
		server = newLegacyMCPServer(c)
		addFullTaskLifecycleTools(server, c)
	} else {
		server = mcp.NewServer(&mcp.Implementation{Name: "baley", Version: mcpImplementationVersion}, nil)
		addCompactReadTools(server, c)
	}
	mcp.AddTool(server, taskJournalTool(), c.taskJournal)
	addCommandBridgeTools(server, c)
	return server
}

func addFullTaskLifecycleTools(server *mcp.Server, c *client) {
	mcp.AddTool(server, classifiedTool("baley_task_rework_preview", "Preview returning an implemented Task to active work"), c.taskReworkPreview)
	mcp.AddTool(server, classifiedTool("baley_task_rework_execute", "Return an implemented Task to active work with an explicit reason"), c.taskReworkExecute)
	mcp.AddTool(server, classifiedTool("baley_task_block_preview", "Preview blocking an active Task"), c.taskBlockPreview)
	mcp.AddTool(server, classifiedTool("baley_task_block_execute", "Block an active Task with an explicit reason"), c.taskBlockExecute)
	mcp.AddTool(server, classifiedTool("baley_task_unblock_preview", "Preview unblocking a Task"), c.taskUnblockPreview)
	mcp.AddTool(server, classifiedTool("baley_task_unblock_execute", "Unblock a Task with an explicit reason"), c.taskUnblockExecute)
}

func taskJournalTool() *mcp.Tool {
	tool := readOnlyTool("baley_task_journal", "Read the append-only Task lifecycle context journal for one Task or an authorized Workspace")
	tool.InputSchema = json.RawMessage(`{
		"type":"object",
		"properties":{
			"workspaceId":{"type":"string","minLength":1},
			"taskId":{"type":"integer","minimum":1,"description":"Optional Task public ID; omit for the Workspace journal"},
			"after":{"type":"string","format":"date-time","description":"RFC3339 timestamp cursor returned by a prior page"},
			"afterId":{"type":"string","minLength":1,"description":"ID tie-breaker paired with after"},
			"limit":{"type":"integer","minimum":1,"maximum":100,"default":50}
		},
		"required":["workspaceId"],
		"additionalProperties":false
	}`)
	return tool
}

func addCompactReadTools(server *mcp.Server, c *client) {
	mcp.AddTool(server, readOnlyTool("baley_workspace_get", "Read Workspace metadata"), c.workspaceGet)
	mcp.AddTool(server, readOnlyTool("baley_mcp_diagnostics", "Report redacted local safety plus the compact MCP catalog profile and version"), c.diagnostics)
	mcp.AddTool(server, readOnlyTool("baley_workspace_context", "Read compact non-completed Phase and Lane status counts; expand a named Phase only when Task detail is needed"), c.workspaceContext)
	mcp.AddTool(server, phaseTasksTool(), c.phaseTasks)
	mcp.AddTool(server, readOnlyTool("baley_task_get", "Read one Task by public ID"), c.taskGet)
	mcp.AddTool(server, readOnlyTool("baley_task_acceptance_get", "Read a Task acceptance binding, policy/profile, assignments, and typed evidence"), c.taskAcceptanceGet)
	mcp.AddTool(server, readOnlyTool("baley_lane_brief", "Build a read-only active-Run-first lane recovery brief with evidence mismatch classification"), c.laneBrief)
	mcp.AddTool(server, readOnlyTool("baley_backlog_list", "List lane Backlog items with optional lane/status filters"), c.backlogList)
	mcp.AddTool(server, readOnlyTool("baley_backlog_get", "Read one Backlog item by B# public ID"), c.backlogGet)
	mcp.AddTool(server, readOnlyTool("baley_gate_status", "Read Gate status and conditions"), c.gateStatus)
}

func addCommandBridgeTools(server *mcp.Server, c *client) {
	mcp.AddTool(server, readOnlyTool("baley_command_catalog", "List supported domain commands and the correctly classified execution bridge on demand"), c.commandCatalog)
	mcp.AddTool(server, commandBridgeTool("baley_command_preview", "Preview one supported domain command without writing", bridgePreview), func(ctx context.Context, req *mcp.CallToolRequest, in commandBridgeInput) (*mcp.CallToolResult, any, error) {
		return c.commandBridge(ctx, in, bridgePreview)
	})
	mcp.AddTool(server, commandBridgeTool("baley_command_execute", "Execute one routine Operator command after preview; human-approval commands are rejected", bridgeExecuteOperator), func(ctx context.Context, req *mcp.CallToolRequest, in commandBridgeInput) (*mcp.CallToolResult, any, error) {
		return c.commandBridge(ctx, in, bridgeExecuteOperator)
	})
	mcp.AddTool(server, commandBridgeTool("baley_command_execute_with_approval", "Execute one human or conditionally approved command; the HTTP server validates any required fresh browser grant", bridgeExecuteApproval), func(ctx context.Context, req *mcp.CallToolRequest, in commandBridgeInput) (*mcp.CallToolResult, any, error) {
		return c.commandBridge(ctx, in, bridgeExecuteApproval)
	})
}

func commandBridgeTool(name, description string, mode bridgeMode) *mcp.Tool {
	var tool *mcp.Tool
	switch mode {
	case bridgePreview:
		tool = readOnlyTool(name, description)
	case bridgeExecuteApproval:
		tool = humanApprovalTool(name, description)
	default:
		tool = operatorTool(name, description)
	}
	tool.InputSchema = commandBridgeSchema(mode)
	return tool
}

func commandBridgeSchema(mode bridgeMode) json.RawMessage {
	envelopeProperties := `
		"idempotencyKey":{"type":"string","minLength":1},
		"expectedWorkspaceRevision":{"type":"integer","minimum":0},
		"executedByActorId":{"type":"string","minLength":1},
		"initiatedByActorId":{"type":"string"}`
	if mode != bridgePreview {
		envelopeProperties += `,
		"acknowledgedWarningCodes":{"type":"array","items":{"type":"string"}},
		"proceedReason":{"type":"string"}`
	}
	if mode == bridgeExecuteApproval {
		envelopeProperties += `,
		"approvalGrantId":{"type":"string"}`
	}
	return json.RawMessage(fmt.Sprintf(`{
		"type":"object",
		"properties":{
			"command":{"type":"string","minLength":1,"description":"Exact name returned by baley_command_catalog"},
			"arguments":{"type":"object","properties":{"workspaceId":{"type":"string","minLength":1}},"required":["workspaceId"],"additionalProperties":true},
			"envelope":{"type":"object","properties":{%s},"required":["idempotencyKey","executedByActorId"],"additionalProperties":false}
		},
		"required":["command","arguments","envelope"],
		"additionalProperties":false
	}`, envelopeProperties))
}

func (c *client) commandCatalog(_ context.Context, _ *mcp.CallToolRequest, _ commandCatalogInput) (*mcp.CallToolResult, any, error) {
	result := map[string]any{
		"catalogVersion": mcpToolCatalogVersion,
		"commandCount":   len(commandDescriptors),
		"commands":       commandDescriptors,
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "Baley command catalog returned on demand; command-specific validation remains server-side."}}, StructuredContent: result}, result, nil
}

func commandDescriptorFor(name string) (commandDescriptor, bool) {
	for _, descriptor := range commandDescriptors {
		if descriptor.Name == name {
			return descriptor, true
		}
	}
	return commandDescriptor{}, false
}

func (c *client) commandBridge(ctx context.Context, in commandBridgeInput, mode bridgeMode) (*mcp.CallToolResult, any, error) {
	if in.Command == "" || strings.TrimSpace(in.Command) != in.Command {
		return nil, nil, errors.New("command must be an exact non-empty name from baley_command_catalog")
	}
	descriptor, ok := commandDescriptorFor(in.Command)
	if !ok {
		return nil, nil, fmt.Errorf("unknown Baley command %q", in.Command)
	}
	if in.Arguments == nil || in.Envelope == nil {
		return nil, nil, errors.New("arguments and envelope must be JSON objects")
	}
	if value, ok := in.Arguments["workspaceId"].(string); !ok || strings.TrimSpace(value) == "" {
		return nil, nil, errors.New("arguments.workspaceId must be a non-empty string")
	}
	for _, field := range []string{"idempotencyKey", "executedByActorId"} {
		if value, ok := in.Envelope[field].(string); !ok || strings.TrimSpace(value) == "" {
			return nil, nil, fmt.Errorf("envelope.%s must be a non-empty string", field)
		}
	}
	switch mode {
	case bridgePreview:
		return c.call(ctx, http.MethodPost, "/v1/commands/preview", command(in.Command, in.Arguments, in.Envelope))
	case bridgeExecuteOperator:
		if descriptor.Classification != commandClassOperator {
			return nil, nil, fmt.Errorf("command %q is classified %s; use baley_command_execute_with_approval", in.Command, descriptor.Classification)
		}
		if _, present := in.Envelope["approvalGrantId"]; present {
			return nil, nil, errors.New("routine Operator commands must not carry approvalGrantId")
		}
	case bridgeExecuteApproval:
		if descriptor.Classification == commandClassOperator {
			return nil, nil, fmt.Errorf("command %q is classified operator; use baley_command_execute", in.Command)
		}
		if descriptor.Classification == commandClassHuman {
			if value, ok := in.Envelope["approvalGrantId"].(string); !ok || strings.TrimSpace(value) == "" {
				return nil, nil, fmt.Errorf("command %q requires a fresh browser-issued approvalGrantId", in.Command)
			}
		}
	default:
		return nil, nil, errors.New("unsupported command bridge mode")
	}
	return c.call(ctx, http.MethodPost, "/v1/commands/execute", command(in.Command, in.Arguments, in.Envelope))
}

func (c *client) diagnosticsForProfile(profile mcpToolProfile) (*mcp.CallToolResult, any, error) {
	result := c.localDiagnostics()
	toolCount, schemaBytes := mcpCompactToolCount, mcpCompactSchemaBytes
	if profile == mcpToolProfileFull {
		toolCount, schemaBytes = mcpFullToolCount, mcpFullSchemaBytes
	}
	result["mcpImplementationVersion"] = mcpImplementationVersion
	result["toolCatalogVersion"] = mcpToolCatalogVersion
	result["toolProfile"] = string(profile)
	result["toolCount"] = toolCount
	result["serializedInputSchemaBytes"] = schemaBytes
	result["defaultToolProfile"] = string(mcpToolProfileCompact)
	result["fullProfileOptInPath"] = "/mcp/full"
	result["toolListMode"] = "static"
	result["commandDiscoveryTool"] = "baley_command_catalog"
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "Baley MCP diagnostics collected without exposing credentials."}}, StructuredContent: result}, result, nil
}

func (c *client) fullDiagnostics(_ context.Context, _ *mcp.CallToolRequest, _ diagnosticsInput) (*mcp.CallToolResult, any, error) {
	return c.diagnosticsForProfile(mcpToolProfileFull)
}
