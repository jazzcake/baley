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
type taskReportImplementedInput struct {
	WorkspaceID              string   `json:"workspaceId"`
	TaskID                   int      `json:"taskId"`
	Assessment               string   `json:"assessment"`
	ProceedReason            string   `json:"proceedReason,omitempty"`
	AcknowledgedWarningCodes []string `json:"acknowledgedWarningCodes,omitempty"`
	automaticEnvelope
}
type taskCreateFields struct {
	WorkspaceID        string `json:"workspaceId"`
	TaskUUID           string `json:"taskUuid"`
	LaneID             string `json:"laneId"`
	PhaseID            string `json:"phaseId"`
	ParentTaskID       int    `json:"parentTaskId,omitempty"`
	Title              string `json:"title"`
	Description        string `json:"description,omitempty"`
	PredecessorTaskIDs []int  `json:"predecessorTaskIds,omitempty"`
	SuccessorTaskIDs   []int  `json:"successorTaskIds,omitempty"`
	TerminalReason     string `json:"terminalReason,omitempty"`
}
type taskCreatePreviewInput struct {
	taskCreateFields
	previewEnvelope
}
type taskCreateExecuteInput struct {
	taskCreateFields
	AcknowledgedWarningCodes []string `json:"acknowledgedWarningCodes,omitempty"`
	ProceedReason            string   `json:"proceedReason,omitempty"`
	automaticEnvelope
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
	ExpectedWorkspaceRevision int64    `json:"expectedWorkspaceRevision"`
	IdempotencyKey            string   `json:"idempotencyKey"`
	ExecutedByActorID         string   `json:"executedByActorId"`
	AcknowledgedWarningCodes  []string `json:"acknowledgedWarningCodes,omitempty"`
	ProceedReason             string   `json:"proceedReason,omitempty"`
	ApprovedByActorID         string   `json:"approvedByActorId"`
	ApprovedCommandHash       string   `json:"approvedCommandHash"`
	DecisionSnapshotHash      string   `json:"decisionSnapshotHash,omitempty"`
	StatementHash             string   `json:"statementHash,omitempty"`
	ConversationRef           string   `json:"conversationRef,omitempty"`
}
type automaticEnvelope struct {
	ExpectedWorkspaceRevision int64  `json:"expectedWorkspaceRevision"`
	IdempotencyKey            string `json:"idempotencyKey"`
	ExecutedByActorID         string `json:"executedByActorId"`
}
type runStartInput struct {
	WorkspaceID string `json:"workspaceId"`
	TaskID      int    `json:"taskId"`
	ClientRunID string `json:"clientRunId"`
	Kind        string `json:"kind"`
	SessionRef  string `json:"sessionRef,omitempty"`
	ParentRunID string `json:"parentRunId,omitempty"`
	TargetRunID string `json:"targetRunId,omitempty"`
	automaticEnvelope
}
type runHeartbeatInput struct {
	WorkspaceID        string `json:"workspaceId"`
	RunID              string `json:"runId"`
	LeaseToken         string `json:"leaseToken"`
	ExpectedRunVersion int64  `json:"expectedRunVersion"`
	ExtensionSeconds   int64  `json:"extensionSeconds,omitempty"`
	IdempotencyKey     string `json:"idempotencyKey"`
	ExecutedByActorID  string `json:"executedByActorId"`
}
type runTerminalInput struct {
	WorkspaceID        string `json:"workspaceId"`
	RunID              string `json:"runId"`
	ExpectedRunVersion int64  `json:"expectedRunVersion"`
	Summary            string `json:"summary"`
	automaticEnvelope
}
type runCorrectInput struct {
	WorkspaceID        string `json:"workspaceId"`
	RunID              string `json:"runId"`
	ExpectedRunVersion int64  `json:"expectedRunVersion"`
	Status             string `json:"status"`
	Summary            string `json:"summary"`
	Reason             string `json:"reason"`
	automaticEnvelope
}
type repositoryRegisterInput struct {
	WorkspaceID        string `json:"workspaceId"`
	RepositoryID       string `json:"repositoryId"`
	Name               string `json:"name"`
	RemoteURL          string `json:"remoteUrl"`
	DefaultBranch      string `json:"defaultBranch,omitempty"`
	IsRecordRepository bool   `json:"isRecordRepository"`
	TaskRecordsRoot    string `json:"taskRecordsRoot,omitempty"`
	automaticEnvelope
}
type recordRegisterInput struct {
	WorkspaceID        string `json:"workspaceId"`
	RecordID           string `json:"recordId"`
	TaskID             int    `json:"taskId"`
	RunID              string `json:"runId,omitempty"`
	RecordType         string `json:"recordType"`
	RepositoryID       string `json:"repositoryId"`
	RelativePath       string `json:"relativePath"`
	WorkingTreeHash    string `json:"workingTreeHash,omitempty"`
	ShortSummary       string `json:"shortSummary"`
	SupersedesRecordID string `json:"supersedesRecordId,omitempty"`
	automaticEnvelope
}
type recordAttachCommitInput struct {
	WorkspaceID string `json:"workspaceId"`
	RecordID    string `json:"recordId"`
	CommitSHA   string `json:"commitSha"`
	BlobSHA     string `json:"blobSha"`
	automaticEnvelope
}
type commitAttachInput struct {
	WorkspaceID  string `json:"workspaceId"`
	CommitID     string `json:"commitId"`
	TaskID       int    `json:"taskId"`
	RunID        string `json:"runId,omitempty"`
	RepositoryID string `json:"repositoryId"`
	CommitSHA    string `json:"commitSha"`
	Relation     string `json:"relation"`
	automaticEnvelope
}
type gitObserveInput struct {
	WorkspaceID   string    `json:"workspaceId"`
	ObservationID string    `json:"observationId"`
	RunID         string    `json:"runId"`
	RepositoryID  string    `json:"repositoryId"`
	ObservedAt    time.Time `json:"observedAt"`
	HeadCommitSHA string    `json:"headCommitSha,omitempty"`
	BranchHint    string    `json:"branchHint,omitempty"`
	WorktreeLabel string    `json:"worktreeLabel,omitempty"`
	Dirty         *bool     `json:"dirty,omitempty"`
	automaticEnvelope
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
	mcp.AddTool(server, &mcp.Tool{Name: "baley_run_list", Description: "List Workspace Runs"}, c.runList)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_record_list", Description: "List Task Record indexes without loading document bodies"}, c.recordList)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_run_start", Description: "Start a Run and automatically start a pending Task"}, c.runStart)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_run_heartbeat", Description: "Extend a running Run lease using token and Run version CAS"}, c.runHeartbeat)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_run_succeed", Description: "Mark a Run succeeded using Run version CAS"}, c.runSucceed)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_run_fail", Description: "Mark a Run failed using Run version CAS"}, c.runFail)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_run_cancel", Description: "Cancel a Run using Run version CAS"}, c.runCancel)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_run_interrupt", Description: "Interrupt a Run using Run version CAS"}, c.runInterrupt)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_run_correct", Description: "Correct a terminal Run with an explicit reason"}, c.runCorrect)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_repository_register", Description: "Register a Git repository and optional Task Record root"}, c.repositoryRegister)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_record_register", Description: "Register a repository-relative Task Record index"}, c.recordRegister)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_record_attach_commit", Description: "Attach commit and blob evidence to a Task Record"}, c.recordAttachCommit)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_commit_attach", Description: "Attach a Git commit reference to a Task"}, c.commitAttach)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_git_observe", Description: "Record non-authoritative Run Git metadata"}, c.gitObserve)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_task_report_implemented", Description: "Report implementation complete with assessment and explicit warning acknowledgement"}, c.taskReportImplemented)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_task_create_preview", Description: "Preview atomic Task creation and initial relationships without writing"}, c.taskCreatePreview)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_task_create_execute", Description: "Create a Task and its initial relationships after reviewing the preview"}, c.taskCreateExecute)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_task_confirm_preview", Description: "Preview Task confirmation without writing"}, c.taskConfirmPreview)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_task_confirm_execute", Description: "Execute an explicitly approved Task confirmation with exact warning acknowledgement when preview returned warnings"}, c.taskConfirmExecute)
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
	return map[string]any{"expectedWorkspaceRevision": v.ExpectedWorkspaceRevision, "idempotencyKey": v.IdempotencyKey, "executedByActorId": v.ExecutedByActorID, "acknowledgedWarningCodes": v.AcknowledgedWarningCodes, "proceedReason": v.ProceedReason, "humanApprovalAttestation": map[string]any{"approvedByActorId": v.ApprovedByActorID, "approvedCommandHash": v.ApprovedCommandHash, "decisionSnapshotHash": v.DecisionSnapshotHash, "statementHash": v.StatementHash, "conversationRef": v.ConversationRef}}
}
func automaticEnv(v automaticEnvelope) map[string]any {
	return map[string]any{"expectedWorkspaceRevision": v.ExpectedWorkspaceRevision, "idempotencyKey": v.IdempotencyKey, "executedByActorId": v.ExecutedByActorID}
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
func (c *client) runList(ctx context.Context, _ *mcp.CallToolRequest, in workspaceInput) (*mcp.CallToolResult, any, error) {
	return c.get(ctx, "/v1/workspaces/"+url.PathEscape(in.WorkspaceID)+"/runs")
}
func (c *client) recordList(ctx context.Context, _ *mcp.CallToolRequest, in workspaceInput) (*mcp.CallToolResult, any, error) {
	return c.get(ctx, "/v1/workspaces/"+url.PathEscape(in.WorkspaceID)+"/records")
}
func (c *client) runStart(ctx context.Context, _ *mcp.CallToolRequest, in runStartInput) (*mcp.CallToolResult, any, error) {
	arguments := map[string]any{"workspaceId": in.WorkspaceID, "taskId": in.TaskID, "clientRunId": in.ClientRunID, "kind": in.Kind, "sessionRef": in.SessionRef, "parentRunId": in.ParentRunID, "targetRunId": in.TargetRunID}
	return c.call(ctx, "POST", "/v1/commands/execute", command("run.start", arguments, automaticEnv(in.automaticEnvelope)))
}
func (c *client) runHeartbeat(ctx context.Context, _ *mcp.CallToolRequest, in runHeartbeatInput) (*mcp.CallToolResult, any, error) {
	arguments := map[string]any{"workspaceId": in.WorkspaceID, "runId": in.RunID, "leaseToken": in.LeaseToken, "expectedRunVersion": in.ExpectedRunVersion, "extensionSeconds": in.ExtensionSeconds}
	envelope := map[string]any{"idempotencyKey": in.IdempotencyKey, "executedByActorId": in.ExecutedByActorID}
	return c.call(ctx, "POST", "/v1/commands/execute", command("run.heartbeat", arguments, envelope))
}
func (c *client) runTerminal(ctx context.Context, name string, in runTerminalInput) (*mcp.CallToolResult, any, error) {
	arguments := map[string]any{"workspaceId": in.WorkspaceID, "runId": in.RunID, "expectedRunVersion": in.ExpectedRunVersion, "summary": in.Summary}
	return c.call(ctx, "POST", "/v1/commands/execute", command(name, arguments, automaticEnv(in.automaticEnvelope)))
}
func (c *client) runSucceed(ctx context.Context, _ *mcp.CallToolRequest, in runTerminalInput) (*mcp.CallToolResult, any, error) {
	return c.runTerminal(ctx, "run.succeed", in)
}
func (c *client) runFail(ctx context.Context, _ *mcp.CallToolRequest, in runTerminalInput) (*mcp.CallToolResult, any, error) {
	return c.runTerminal(ctx, "run.fail", in)
}
func (c *client) runCancel(ctx context.Context, _ *mcp.CallToolRequest, in runTerminalInput) (*mcp.CallToolResult, any, error) {
	return c.runTerminal(ctx, "run.cancel", in)
}
func (c *client) runInterrupt(ctx context.Context, _ *mcp.CallToolRequest, in runTerminalInput) (*mcp.CallToolResult, any, error) {
	return c.runTerminal(ctx, "run.interrupt", in)
}
func (c *client) runCorrect(ctx context.Context, _ *mcp.CallToolRequest, in runCorrectInput) (*mcp.CallToolResult, any, error) {
	arguments := map[string]any{"workspaceId": in.WorkspaceID, "runId": in.RunID, "expectedRunVersion": in.ExpectedRunVersion, "status": in.Status, "summary": in.Summary, "reason": in.Reason}
	return c.call(ctx, "POST", "/v1/commands/execute", command("run.correct", arguments, automaticEnv(in.automaticEnvelope)))
}
func (c *client) repositoryRegister(ctx context.Context, _ *mcp.CallToolRequest, in repositoryRegisterInput) (*mcp.CallToolResult, any, error) {
	arguments := map[string]any{"workspaceId": in.WorkspaceID, "repositoryId": in.RepositoryID, "name": in.Name, "remoteUrl": in.RemoteURL, "defaultBranch": in.DefaultBranch, "isRecordRepository": in.IsRecordRepository, "taskRecordsRoot": in.TaskRecordsRoot}
	return c.call(ctx, "POST", "/v1/commands/execute", command("repository.register", arguments, automaticEnv(in.automaticEnvelope)))
}
func (c *client) recordRegister(ctx context.Context, _ *mcp.CallToolRequest, in recordRegisterInput) (*mcp.CallToolResult, any, error) {
	arguments := map[string]any{"workspaceId": in.WorkspaceID, "recordId": in.RecordID, "taskId": in.TaskID, "runId": in.RunID, "recordType": in.RecordType, "repositoryId": in.RepositoryID, "relativePath": in.RelativePath, "workingTreeHash": in.WorkingTreeHash, "shortSummary": in.ShortSummary, "supersedesRecordId": in.SupersedesRecordID}
	return c.call(ctx, "POST", "/v1/commands/execute", command("record.register", arguments, automaticEnv(in.automaticEnvelope)))
}
func (c *client) recordAttachCommit(ctx context.Context, _ *mcp.CallToolRequest, in recordAttachCommitInput) (*mcp.CallToolResult, any, error) {
	arguments := map[string]any{"workspaceId": in.WorkspaceID, "recordId": in.RecordID, "commitSha": in.CommitSHA, "blobSha": in.BlobSHA}
	return c.call(ctx, "POST", "/v1/commands/execute", command("record.attach_commit", arguments, automaticEnv(in.automaticEnvelope)))
}
func (c *client) commitAttach(ctx context.Context, _ *mcp.CallToolRequest, in commitAttachInput) (*mcp.CallToolResult, any, error) {
	arguments := map[string]any{"workspaceId": in.WorkspaceID, "commitId": in.CommitID, "taskId": in.TaskID, "runId": in.RunID, "repositoryId": in.RepositoryID, "commitSha": in.CommitSHA, "relation": in.Relation}
	return c.call(ctx, "POST", "/v1/commands/execute", command("commit.attach", arguments, automaticEnv(in.automaticEnvelope)))
}
func (c *client) gitObserve(ctx context.Context, _ *mcp.CallToolRequest, in gitObserveInput) (*mcp.CallToolResult, any, error) {
	arguments := map[string]any{"workspaceId": in.WorkspaceID, "observationId": in.ObservationID, "runId": in.RunID, "repositoryId": in.RepositoryID, "observedAt": in.ObservedAt, "headCommitSha": in.HeadCommitSHA, "branchHint": in.BranchHint, "worktreeLabel": in.WorktreeLabel, "dirty": in.Dirty}
	return c.call(ctx, "POST", "/v1/commands/execute", command("git.observe", arguments, automaticEnv(in.automaticEnvelope)))
}
func (c *client) taskReportImplemented(ctx context.Context, _ *mcp.CallToolRequest, in taskReportImplementedInput) (*mcp.CallToolResult, any, error) {
	arguments := map[string]any{"workspaceId": in.WorkspaceID, "taskId": in.TaskID, "assessment": in.Assessment}
	envelope := automaticEnv(in.automaticEnvelope)
	envelope["acknowledgedWarningCodes"] = in.AcknowledgedWarningCodes
	envelope["proceedReason"] = in.ProceedReason
	return c.call(ctx, "POST", "/v1/commands/execute", command("task.report_implemented", arguments, envelope))
}
func taskCreateArguments(in taskCreateFields) map[string]any {
	return map[string]any{
		"workspaceId": in.WorkspaceID, "taskUuid": in.TaskUUID, "laneId": in.LaneID, "phaseId": in.PhaseID,
		"parentTaskId": in.ParentTaskID, "title": in.Title, "description": in.Description,
		"predecessorTaskIds": in.PredecessorTaskIDs, "successorTaskIds": in.SuccessorTaskIDs,
		"terminalReason": in.TerminalReason,
	}
}
func (c *client) taskCreatePreview(ctx context.Context, _ *mcp.CallToolRequest, in taskCreatePreviewInput) (*mcp.CallToolResult, any, error) {
	return c.call(ctx, "POST", "/v1/commands/preview", command("task.create", taskCreateArguments(in.taskCreateFields), previewEnv(in.previewEnvelope)))
}
func (c *client) taskCreateExecute(ctx context.Context, _ *mcp.CallToolRequest, in taskCreateExecuteInput) (*mcp.CallToolResult, any, error) {
	envelope := automaticEnv(in.automaticEnvelope)
	if len(in.AcknowledgedWarningCodes) != 0 {
		envelope["acknowledgedWarningCodes"] = in.AcknowledgedWarningCodes
	}
	if in.ProceedReason != "" {
		envelope["proceedReason"] = in.ProceedReason
	}
	return c.call(ctx, "POST", "/v1/commands/execute", command("task.create", taskCreateArguments(in.taskCreateFields), envelope))
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
