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
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type client struct {
	base                string
	http                *http.Client
	agentToken          string
	credentialStorePath string
	agentActorID        string
	connectionMu        sync.Mutex
	pendingConnections  map[string]pendingWorkspaceConnection
}
type workspaceInput struct {
	WorkspaceID string `json:"workspaceId" jsonschema:"Baley workspace ID"`
}
type taskInput struct {
	WorkspaceID string `json:"workspaceId"`
	TaskID      int    `json:"taskId"`
}
type laneBriefInput struct {
	WorkspaceID string `json:"workspaceId"`
	LaneID      string `json:"laneId"`
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
	WorkspaceID             string `json:"workspaceId"`
	TaskUUID                string `json:"taskUuid"`
	LaneID                  string `json:"laneId"`
	PhaseID                 string `json:"phaseId"`
	ParentTaskID            int    `json:"parentTaskId,omitempty"`
	Title                   string `json:"title"`
	Description             string `json:"description,omitempty"`
	PredecessorTaskIDs      []int  `json:"predecessorTaskIds,omitempty"`
	SuccessorTaskIDs        []int  `json:"successorTaskIds,omitempty"`
	TerminalReason          string `json:"terminalReason,omitempty"`
	RequestedAcceptanceMode string `json:"requestedAcceptanceMode,omitempty"`
	EvidenceProfileID       string `json:"evidenceProfileId,omitempty"`
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
type backlogInput struct {
	WorkspaceID     string `json:"workspaceId"`
	BacklogPublicID int    `json:"backlogPublicId"`
}
type backlogListInput struct {
	WorkspaceID string `json:"workspaceId"`
	LaneID      string `json:"laneId,omitempty"`
	Status      string `json:"status,omitempty"`
	Cursor      int    `json:"cursor,omitempty"`
	Limit       int    `json:"limit,omitempty"`
}
type mutationAttemptListInput struct {
	WorkspaceID string `json:"workspaceId"`
	Outcome     string `json:"outcome,omitempty"`
	CommandName string `json:"commandName,omitempty"`
	After       string `json:"after,omitempty" jsonschema:"RFC3339 timestamp cursor"`
	AfterID     string `json:"afterId,omitempty" jsonschema:"ID tie-breaker returned with the timestamp cursor"`
	Limit       int    `json:"limit,omitempty"`
}
type backlogMutationFields struct {
	WorkspaceID             string  `json:"workspaceId"`
	BacklogUUID             string  `json:"backlogUuid,omitempty"`
	BacklogPublicID         int     `json:"backlogPublicId,omitempty"`
	LaneID                  string  `json:"laneId,omitempty"`
	TargetLaneID            string  `json:"targetLaneId,omitempty"`
	Title                   *string `json:"title,omitempty"`
	Description             *string `json:"description,omitempty"`
	Reason                  string  `json:"reason,omitempty"`
	OrderedBacklogPublicIDs []int   `json:"orderedBacklogPublicIds,omitempty"`
	TaskUUID                string  `json:"taskUuid,omitempty"`
	PhaseID                 string  `json:"phaseId,omitempty"`
	ParentTaskID            int     `json:"parentTaskId,omitempty"`
	PredecessorTaskIDs      []int   `json:"predecessorTaskIds,omitempty"`
	SuccessorTaskIDs        []int   `json:"successorTaskIds,omitempty"`
	TerminalReason          string  `json:"terminalReason,omitempty"`
	RequestedAcceptanceMode string  `json:"requestedAcceptanceMode,omitempty"`
	EvidenceProfileID       string  `json:"evidenceProfileId,omitempty"`
}
type backlogCreateFields struct {
	WorkspaceID string  `json:"workspaceId"`
	BacklogUUID string  `json:"backlogUuid"`
	LaneID      string  `json:"laneId"`
	Title       string  `json:"title"`
	Description *string `json:"description,omitempty"`
}
type backlogUpdateFields struct {
	WorkspaceID     string  `json:"workspaceId"`
	BacklogPublicID int     `json:"backlogPublicId"`
	Title           *string `json:"title,omitempty"`
	Description     *string `json:"description,omitempty"`
}
type backlogMoveFields struct {
	WorkspaceID     string `json:"workspaceId"`
	BacklogPublicID int    `json:"backlogPublicId"`
	TargetLaneID    string `json:"targetLaneId"`
}
type backlogReorderFields struct {
	WorkspaceID             string `json:"workspaceId"`
	LaneID                  string `json:"laneId"`
	OrderedBacklogPublicIDs []int  `json:"orderedBacklogPublicIds"`
}
type backlogDiscardFields struct {
	WorkspaceID     string `json:"workspaceId"`
	BacklogPublicID int    `json:"backlogPublicId"`
	Reason          string `json:"reason"`
}
type backlogPromoteFields struct {
	WorkspaceID             string `json:"workspaceId"`
	BacklogPublicID         int    `json:"backlogPublicId"`
	TaskUUID                string `json:"taskUuid"`
	PhaseID                 string `json:"phaseId"`
	ParentTaskID            int    `json:"parentTaskId,omitempty"`
	PredecessorTaskIDs      []int  `json:"predecessorTaskIds,omitempty"`
	SuccessorTaskIDs        []int  `json:"successorTaskIds,omitempty"`
	TerminalReason          string `json:"terminalReason,omitempty"`
	RequestedAcceptanceMode string `json:"requestedAcceptanceMode,omitempty"`
	EvidenceProfileID       string `json:"evidenceProfileId,omitempty"`
}
type acceptancePolicyFields struct {
	WorkspaceID       string `json:"workspaceId"`
	PolicyVersion     string `json:"policyVersion"`
	DefaultMode       string `json:"defaultMode"`
	EvidenceProfileID string `json:"evidenceProfileId"`
}
type acceptancePolicyPreviewInput struct {
	acceptancePolicyFields
	previewEnvelope
}
type acceptancePolicyExecuteInput struct {
	acceptancePolicyFields
	executeEnvelope
}
type acceptanceEscalateFields struct {
	WorkspaceID       string `json:"workspaceId"`
	TaskID            int    `json:"taskId"`
	AssignmentID      string `json:"assignmentId"`
	Reason            string `json:"reason"`
	EvidenceReference string `json:"evidenceReference"`
	PolicyVersion     string `json:"policyVersion"`
}
type acceptanceEscalatePreviewInput struct {
	acceptanceEscalateFields
	previewEnvelope
}
type acceptanceEscalateExecuteInput struct {
	acceptanceEscalateFields
	executeEnvelope
}
type evidenceReportInput struct {
	WorkspaceID               string `json:"workspaceId"`
	TaskID                    int    `json:"taskId"`
	EvidenceID                string `json:"evidenceId"`
	CompletionReportRecordID  string `json:"completionReportRecordId"`
	VerificationVerdict       string `json:"verificationVerdict"`
	VerificationReference     string `json:"verificationReference,omitempty"`
	VerificationReferenceKind string `json:"verificationReferenceKind,omitempty"`
	IndependentReviewRecordID string `json:"independentReviewRecordId"`
	ReviewVerdict             string `json:"reviewVerdict"`
	UnresolvedBlockingCount   int    `json:"unresolvedBlockingCount"`
	CommitReferenceID         string `json:"commitReferenceId,omitempty"`
	automaticEnvelope
}
type backlogCreatePreviewInput struct {
	backlogCreateFields
	previewEnvelope
}
type backlogCreateExecuteInput struct {
	backlogCreateFields
	mutationExecuteEnvelope
}
type backlogUpdatePreviewInput struct {
	backlogUpdateFields
	previewEnvelope
}
type backlogUpdateExecuteInput struct {
	backlogUpdateFields
	mutationExecuteEnvelope
}
type backlogMovePreviewInput struct {
	backlogMoveFields
	previewEnvelope
}
type backlogMoveExecuteInput struct {
	backlogMoveFields
	mutationExecuteEnvelope
}
type backlogReorderPreviewInput struct {
	backlogReorderFields
	previewEnvelope
}
type backlogReorderExecuteInput struct {
	backlogReorderFields
	mutationExecuteEnvelope
}
type backlogDiscardPreviewInput struct {
	backlogDiscardFields
	previewEnvelope
}
type backlogDiscardExecuteInput struct {
	backlogDiscardFields
	mutationExecuteEnvelope
}
type backlogPromotePreviewInput struct {
	backlogPromoteFields
	previewEnvelope
}
type backlogPromoteExecuteInput struct {
	backlogPromoteFields
	mutationExecuteEnvelope
}
type phaseCreateFields struct {
	WorkspaceID string `json:"workspaceId"`
	PhaseID     string `json:"phaseId"`
	Name        string `json:"name"`
}
type phaseCreatePreviewInput struct {
	phaseCreateFields
	previewEnvelope
}
type phaseCreateExecuteInput struct {
	phaseCreateFields
	mutationExecuteEnvelope
}
type laneCreateFields struct {
	WorkspaceID string `json:"workspaceId"`
	LaneID      string `json:"laneId"`
	Name        string `json:"name"`
	Goal        string `json:"goal,omitempty"`
	Summary     string `json:"summary,omitempty"`
}
type laneCreatePreviewInput struct {
	laneCreateFields
	previewEnvelope
}
type laneCreateExecuteInput struct {
	laneCreateFields
	mutationExecuteEnvelope
}
type gateCreateFields struct {
	WorkspaceID string `json:"workspaceId"`
	GateID      string `json:"gateId"`
	Alias       string `json:"alias,omitempty"`
	Name        string `json:"name"`
	FromPhaseID string `json:"fromPhaseId"`
	ToPhaseID   string `json:"toPhaseId"`
}
type gateCreatePreviewInput struct {
	gateCreateFields
	previewEnvelope
}
type gateCreateExecuteInput struct {
	gateCreateFields
	mutationExecuteEnvelope
}
type gateAttachTaskFields struct {
	WorkspaceID   string `json:"workspaceId"`
	GateID        string `json:"gateId"`
	TaskID        int    `json:"taskId"`
	ClearTerminal bool   `json:"clearTerminal,omitempty"`
}
type gateAttachTaskPreviewInput struct {
	gateAttachTaskFields
	previewEnvelope
}
type gateAttachTaskExecuteInput struct {
	gateAttachTaskFields
	conditionalExecuteEnvelope
}
type gateEntryTaskFields struct {
	WorkspaceID string `json:"workspaceId"`
	GateID      string `json:"gateId"`
	TaskID      int    `json:"taskId"`
}
type gateEntryTaskPreviewInput struct {
	gateEntryTaskFields
	previewEnvelope
}
type gateEntryTaskExecuteInput struct {
	gateEntryTaskFields
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
	InitiatedByActorID        string `json:"initiatedByActorId,omitempty"`
}
type executeEnvelope struct {
	ExpectedWorkspaceRevision int64      `json:"expectedWorkspaceRevision"`
	IdempotencyKey            string     `json:"idempotencyKey"`
	ExecutedByActorID         string     `json:"executedByActorId"`
	InitiatedByActorID        string     `json:"initiatedByActorId,omitempty"`
	AcknowledgedWarningCodes  []string   `json:"acknowledgedWarningCodes,omitempty"`
	ProceedReason             string     `json:"proceedReason,omitempty"`
	ApprovedByActorID         string     `json:"approvedByActorId"`
	ApprovedCommandHash       string     `json:"approvedCommandHash"`
	DecisionSnapshotHash      string     `json:"decisionSnapshotHash,omitempty"`
	StatementHash             string     `json:"statementHash,omitempty"`
	ConversationRef           string     `json:"conversationRef,omitempty"`
	ApprovedAt                *time.Time `json:"approvedAt,omitempty"`
	ApprovalGrantToken        string     `json:"approvalGrantToken,omitempty"`
}
type automaticEnvelope struct {
	ExpectedWorkspaceRevision int64  `json:"expectedWorkspaceRevision"`
	IdempotencyKey            string `json:"idempotencyKey"`
	ExecutedByActorID         string `json:"executedByActorId"`
	InitiatedByActorID        string `json:"initiatedByActorId,omitempty"`
}
type mutationExecuteEnvelope struct {
	automaticEnvelope
	AcknowledgedWarningCodes []string `json:"acknowledgedWarningCodes,omitempty"`
	ProceedReason            string   `json:"proceedReason,omitempty"`
}
type conditionalExecuteEnvelope struct {
	mutationExecuteEnvelope
	ApprovedByActorID    string     `json:"approvedByActorId,omitempty"`
	ApprovedCommandHash  string     `json:"approvedCommandHash,omitempty"`
	DecisionSnapshotHash string     `json:"decisionSnapshotHash,omitempty"`
	StatementHash        string     `json:"statementHash,omitempty"`
	ConversationRef      string     `json:"conversationRef,omitempty"`
	ApprovedAt           *time.Time `json:"approvedAt,omitempty"`
	ApprovalGrantToken   string     `json:"approvalGrantToken,omitempty"`
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
	c := &client{
		base:                strings.TrimRight(base, "/"),
		http:                &http.Client{Timeout: 15 * time.Second},
		agentToken:          strings.TrimSpace(os.Getenv("BALEY_AGENT_TOKEN")),
		credentialStorePath: strings.TrimSpace(os.Getenv("BALEY_MCP_CREDENTIAL_STORE")),
		agentActorID:        strings.TrimSpace(os.Getenv("BALEY_AGENT_ACTOR_ID")),
		pendingConnections:  map[string]pendingWorkspaceConnection{},
	}
	if c.agentActorID == "" {
		c.agentActorID = "00000000-0000-4000-8000-000000000003"
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "baley", Version: "0.1.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_workspace_get", Description: "Read Workspace metadata"}, c.workspaceGet)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_workspace_graph", Description: "Read the current Workspace graph"}, c.workspaceGraph)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_task_get", Description: "Read one Task by public ID"}, c.taskGet)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_task_acceptance_get", Description: "Read a Task acceptance binding, policy/profile, assignments, and typed evidence"}, c.taskAcceptanceGet)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_lane_brief", Description: "Build a read-only active-Run-first lane recovery brief with evidence mismatch classification"}, c.laneBrief)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_backlog_list", Description: "List lane Backlog items with optional lane/status filters"}, c.backlogList)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_backlog_get", Description: "Read one Backlog item by B# public ID"}, c.backlogGet)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_gate_status", Description: "Read Gate status and conditions"}, c.gateStatus)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_decision_list", Description: "List human decisions currently available"}, c.decisionList)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_event_list", Description: "List Workspace Events"}, c.eventList)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_mutation_attempt_list", Description: "List append-only Workspace mutation attempts"}, c.mutationAttemptList)
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
	mcp.AddTool(server, &mcp.Tool{Name: "baley_task_evidence_report", Description: "Append typed acceptance evidence and atomically auto-confirm an eligible delegated Task"}, c.taskEvidenceReport)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_task_acceptance_policy_change_preview", Description: "Preview a human-approved future-Task acceptance policy change"}, c.acceptancePolicyChangePreview)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_task_acceptance_policy_change_execute", Description: "Execute an approved future-Task acceptance policy change"}, c.acceptancePolicyChangeExecute)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_task_acceptance_mode_escalate_preview", Description: "Preview monotonic delegated to human-required escalation"}, c.acceptanceModeEscalatePreview)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_task_acceptance_mode_escalate_execute", Description: "Execute an approved monotonic acceptance escalation"}, c.acceptanceModeEscalateExecute)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_task_create_preview", Description: "Preview atomic Task creation and initial relationships without writing"}, c.taskCreatePreview)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_task_create_execute", Description: "Create a Task and its initial relationships after reviewing the preview"}, c.taskCreateExecute)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_backlog_create_preview", Description: "Preview creating a phase-free lane Backlog item"}, c.backlogCreatePreview)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_backlog_create_execute", Description: "Create a phase-free lane Backlog item"}, c.backlogCreateExecute)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_backlog_update_preview", Description: "Preview updating an active Backlog item"}, c.backlogUpdatePreview)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_backlog_update_execute", Description: "Update an active Backlog item"}, c.backlogUpdateExecute)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_backlog_move_preview", Description: "Preview moving an active Backlog item to another lane"}, c.backlogMovePreview)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_backlog_move_execute", Description: "Move an active Backlog item to another lane"}, c.backlogMoveExecute)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_backlog_reorder_preview", Description: "Preview replacing one lane's complete active Backlog order"}, c.backlogReorderPreview)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_backlog_reorder_execute", Description: "Replace one lane's complete active Backlog order"}, c.backlogReorderExecute)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_backlog_discard_preview", Description: "Preview audited soft-discard of a Backlog item"}, c.backlogDiscardPreview)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_backlog_discard_execute", Description: "Soft-discard an active Backlog item"}, c.backlogDiscardExecute)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_backlog_promote_preview", Description: "Preview atomic Backlog promotion into a phase-targeted pending Task"}, c.backlogPromotePreview)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_backlog_promote_execute", Description: "Atomically promote Backlog into a pending Task with exact warning acknowledgement"}, c.backlogPromoteExecute)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_phase_create_preview", Description: "Preview appending a Phase without writing"}, c.phaseCreatePreview)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_phase_create_execute", Description: "Append a Phase after reviewing the preview"}, c.phaseCreateExecute)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_lane_create_preview", Description: "Preview creating a Lane without writing"}, c.laneCreatePreview)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_lane_create_execute", Description: "Create a Lane after reviewing the preview"}, c.laneCreateExecute)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_gate_create_preview", Description: "Preview creating a Phase Gate without writing"}, c.gateCreatePreview)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_gate_create_execute", Description: "Create a Phase Gate after reviewing the preview"}, c.gateCreateExecute)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_gate_attach_task_preview", Description: "Preview attaching a Task as a Gate condition without writing"}, c.gateAttachTaskPreview)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_gate_attach_task_execute", Description: "Attach a Task to a Gate; active Gates require fields from an explicitly approved fresh preview"}, c.gateAttachTaskExecute)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_gate_attach_entry_task_preview", Description: "Preview binding a to-Phase Task as work unlocked by a Gate"}, c.gateAttachEntryTaskPreview)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_gate_attach_entry_task_execute", Description: "Bind a to-Phase Task as work unlocked by a Gate"}, c.gateAttachEntryTaskExecute)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_gate_detach_entry_task_preview", Description: "Preview removing an explicit Gate entry Task binding"}, c.gateDetachEntryTaskPreview)
	mcp.AddTool(server, &mcp.Tool{Name: "baley_gate_detach_entry_task_execute", Description: "Remove an explicit Gate entry Task binding and restore automatic root selection when none remain"}, c.gateDetachEntryTaskExecute)
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
	workspaceID := requestWorkspaceID(path, payload)
	token := c.agentToken
	if c.credentialStorePath != "" && workspaceID != "" {
		var pending *mcp.CallToolResult
		var err error
		token, pending, err = c.workspaceCredential(ctx, workspaceID)
		if err != nil || pending != nil {
			return pending, pendingStructured(pending), err
		}
	}
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
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
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
	if (res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusNotFound) &&
		c.credentialStorePath != "" && workspaceID != "" && token != "" {
		if err = c.removeWorkspaceCredential(workspaceID); err != nil {
			return nil, nil, err
		}
		_, pending, connectErr := c.workspaceCredential(ctx, workspaceID)
		return pending, pendingStructured(pending), connectErr
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
	envelope := map[string]any{"expectedWorkspaceRevision": v.ExpectedWorkspaceRevision, "idempotencyKey": v.IdempotencyKey, "executedByActorId": v.ExecutedByActorID}
	if v.InitiatedByActorID != "" {
		envelope["initiatedByActorId"] = v.InitiatedByActorID
	}
	return envelope
}
func executeEnv(v executeEnvelope) map[string]any {
	envelope := map[string]any{"expectedWorkspaceRevision": v.ExpectedWorkspaceRevision, "idempotencyKey": v.IdempotencyKey, "executedByActorId": v.ExecutedByActorID, "acknowledgedWarningCodes": v.AcknowledgedWarningCodes, "proceedReason": v.ProceedReason}
	if v.InitiatedByActorID != "" {
		envelope["initiatedByActorId"] = v.InitiatedByActorID
	}
	envelope["humanApprovalAttestation"] = approvalAttestation(v.ApprovedByActorID, v.ApprovedCommandHash, v.DecisionSnapshotHash, v.StatementHash, v.ConversationRef, v.ApprovedAt)
	if v.ApprovalGrantToken != "" {
		envelope["approvalGrantToken"] = v.ApprovalGrantToken
	}
	return envelope
}
func automaticEnv(v automaticEnvelope) map[string]any {
	envelope := map[string]any{"expectedWorkspaceRevision": v.ExpectedWorkspaceRevision, "idempotencyKey": v.IdempotencyKey, "executedByActorId": v.ExecutedByActorID}
	if v.InitiatedByActorID != "" {
		envelope["initiatedByActorId"] = v.InitiatedByActorID
	}
	return envelope
}
func mutationExecuteEnv(v mutationExecuteEnvelope) map[string]any {
	envelope := automaticEnv(v.automaticEnvelope)
	if len(v.AcknowledgedWarningCodes) != 0 {
		envelope["acknowledgedWarningCodes"] = v.AcknowledgedWarningCodes
	}
	if v.ProceedReason != "" {
		envelope["proceedReason"] = v.ProceedReason
	}
	return envelope
}
func conditionalExecuteEnv(v conditionalExecuteEnvelope) map[string]any {
	envelope := mutationExecuteEnv(v.mutationExecuteEnvelope)
	if v.ApprovedByActorID != "" || v.ApprovedCommandHash != "" || v.DecisionSnapshotHash != "" || v.StatementHash != "" || v.ConversationRef != "" || v.ApprovedAt != nil {
		envelope["humanApprovalAttestation"] = approvalAttestation(v.ApprovedByActorID, v.ApprovedCommandHash, v.DecisionSnapshotHash, v.StatementHash, v.ConversationRef, v.ApprovedAt)
	}
	if v.ApprovalGrantToken != "" {
		envelope["approvalGrantToken"] = v.ApprovalGrantToken
	}
	return envelope
}
func approvalAttestation(approvedByActorID, approvedCommandHash, decisionSnapshotHash, statementHash, conversationRef string, approvedAt *time.Time) map[string]any {
	attestation := map[string]any{"approvedByActorId": approvedByActorID, "approvedCommandHash": approvedCommandHash}
	for key, value := range map[string]string{"decisionSnapshotHash": decisionSnapshotHash, "statementHash": statementHash, "conversationRef": conversationRef} {
		if value != "" {
			attestation[key] = value
		}
	}
	if approvedAt != nil {
		attestation["approvedAt"] = approvedAt
	}
	return attestation
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
func (c *client) taskAcceptanceGet(ctx context.Context, _ *mcp.CallToolRequest, in taskInput) (*mcp.CallToolResult, any, error) {
	return c.get(ctx, fmt.Sprintf("/v1/workspaces/%s/tasks/%d/acceptance", url.PathEscape(in.WorkspaceID), in.TaskID))
}
func (c *client) laneBrief(ctx context.Context, _ *mcp.CallToolRequest, in laneBriefInput) (*mcp.CallToolResult, any, error) {
	return c.get(ctx, "/v1/workspaces/"+url.PathEscape(in.WorkspaceID)+"/lanes/"+url.PathEscape(in.LaneID)+"/brief")
}
func (c *client) backlogGet(ctx context.Context, _ *mcp.CallToolRequest, in backlogInput) (*mcp.CallToolResult, any, error) {
	return c.get(ctx, fmt.Sprintf("/v1/workspaces/%s/backlog/%d", url.PathEscape(in.WorkspaceID), in.BacklogPublicID))
}
func (c *client) backlogList(ctx context.Context, _ *mcp.CallToolRequest, in backlogListInput) (*mcp.CallToolResult, any, error) {
	values := url.Values{}
	if in.LaneID != "" {
		values.Set("laneId", in.LaneID)
	}
	if in.Status != "" {
		values.Set("status", in.Status)
	}
	if in.Cursor > 0 {
		values.Set("cursor", fmt.Sprint(in.Cursor))
	}
	if in.Limit > 0 {
		values.Set("limit", fmt.Sprint(in.Limit))
	}
	path := "/v1/workspaces/" + url.PathEscape(in.WorkspaceID) + "/backlog"
	if encoded := values.Encode(); encoded != "" {
		path += "?" + encoded
	}
	return c.get(ctx, path)
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
func (c *client) mutationAttemptList(ctx context.Context, _ *mcp.CallToolRequest, in mutationAttemptListInput) (*mcp.CallToolResult, any, error) {
	values := url.Values{}
	if in.Outcome != "" {
		values.Set("outcome", in.Outcome)
	}
	if in.CommandName != "" {
		values.Set("commandName", in.CommandName)
	}
	if in.After != "" {
		values.Set("after", in.After)
		values.Set("afterId", in.AfterID)
	}
	if in.Limit > 0 {
		values.Set("limit", fmt.Sprintf("%d", in.Limit))
	}
	path := "/v1/workspaces/" + url.PathEscape(in.WorkspaceID) + "/mutation-attempts"
	if query := values.Encode(); query != "" {
		path += "?" + query
	}
	return c.get(ctx, path)
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
func acceptancePolicyArguments(in acceptancePolicyFields) map[string]any {
	return map[string]any{"workspaceId": in.WorkspaceID, "policyVersion": in.PolicyVersion, "defaultMode": in.DefaultMode, "evidenceProfileId": in.EvidenceProfileID}
}
func (c *client) acceptancePolicyChangePreview(ctx context.Context, _ *mcp.CallToolRequest, in acceptancePolicyPreviewInput) (*mcp.CallToolResult, any, error) {
	return c.call(ctx, "POST", "/v1/commands/preview", command("task.acceptance_policy.change", acceptancePolicyArguments(in.acceptancePolicyFields), previewEnv(in.previewEnvelope)))
}
func (c *client) acceptancePolicyChangeExecute(ctx context.Context, _ *mcp.CallToolRequest, in acceptancePolicyExecuteInput) (*mcp.CallToolResult, any, error) {
	return c.call(ctx, "POST", "/v1/commands/execute", command("task.acceptance_policy.change", acceptancePolicyArguments(in.acceptancePolicyFields), executeEnv(in.executeEnvelope)))
}
func acceptanceEscalateArguments(in acceptanceEscalateFields) map[string]any {
	return map[string]any{"workspaceId": in.WorkspaceID, "taskId": in.TaskID, "assignmentId": in.AssignmentID, "reason": in.Reason, "evidenceReference": in.EvidenceReference, "policyVersion": in.PolicyVersion}
}
func (c *client) acceptanceModeEscalatePreview(ctx context.Context, _ *mcp.CallToolRequest, in acceptanceEscalatePreviewInput) (*mcp.CallToolResult, any, error) {
	return c.call(ctx, "POST", "/v1/commands/preview", command("task.acceptance_mode.escalate", acceptanceEscalateArguments(in.acceptanceEscalateFields), previewEnv(in.previewEnvelope)))
}
func (c *client) acceptanceModeEscalateExecute(ctx context.Context, _ *mcp.CallToolRequest, in acceptanceEscalateExecuteInput) (*mcp.CallToolResult, any, error) {
	return c.call(ctx, "POST", "/v1/commands/execute", command("task.acceptance_mode.escalate", acceptanceEscalateArguments(in.acceptanceEscalateFields), executeEnv(in.executeEnvelope)))
}
func (c *client) taskEvidenceReport(ctx context.Context, _ *mcp.CallToolRequest, in evidenceReportInput) (*mcp.CallToolResult, any, error) {
	arguments := map[string]any{
		"workspaceId": in.WorkspaceID, "taskId": in.TaskID, "evidenceId": in.EvidenceID,
		"completionReportRecordId": in.CompletionReportRecordID, "verificationVerdict": in.VerificationVerdict,
		"verificationReference": in.VerificationReference, "verificationReferenceKind": in.VerificationReferenceKind,
		"independentReviewRecordId": in.IndependentReviewRecordID, "reviewVerdict": in.ReviewVerdict,
		"unresolvedBlockingCount": in.UnresolvedBlockingCount, "commitReferenceId": in.CommitReferenceID,
	}
	return c.call(ctx, "POST", "/v1/commands/execute", command("task.evidence.report", arguments, automaticEnv(in.automaticEnvelope)))
}
func taskCreateArguments(in taskCreateFields) map[string]any {
	return map[string]any{
		"workspaceId": in.WorkspaceID, "taskUuid": in.TaskUUID, "laneId": in.LaneID, "phaseId": in.PhaseID,
		"parentTaskId": in.ParentTaskID, "title": in.Title, "description": in.Description,
		"predecessorTaskIds": in.PredecessorTaskIDs, "successorTaskIds": in.SuccessorTaskIDs,
		"terminalReason": in.TerminalReason, "requestedAcceptanceMode": in.RequestedAcceptanceMode,
		"evidenceProfileId": in.EvidenceProfileID,
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
func backlogArguments(in backlogMutationFields) map[string]any {
	out := map[string]any{"workspaceId": in.WorkspaceID}
	if in.BacklogUUID != "" {
		out["backlogUuid"] = in.BacklogUUID
	}
	if in.BacklogPublicID != 0 {
		out["backlogPublicId"] = in.BacklogPublicID
	}
	if in.LaneID != "" {
		out["laneId"] = in.LaneID
	}
	if in.TargetLaneID != "" {
		out["targetLaneId"] = in.TargetLaneID
	}
	if in.Title != nil {
		out["title"] = *in.Title
	}
	if in.Description != nil {
		out["description"] = *in.Description
	}
	if in.Reason != "" {
		out["reason"] = in.Reason
	}
	if in.OrderedBacklogPublicIDs != nil {
		out["orderedBacklogPublicIds"] = in.OrderedBacklogPublicIDs
	}
	if in.TaskUUID != "" {
		out["taskUuid"] = in.TaskUUID
	}
	if in.PhaseID != "" {
		out["phaseId"] = in.PhaseID
	}
	if in.ParentTaskID != 0 {
		out["parentTaskId"] = in.ParentTaskID
	}
	if in.PredecessorTaskIDs != nil {
		out["predecessorTaskIds"] = in.PredecessorTaskIDs
	}
	if in.SuccessorTaskIDs != nil {
		out["successorTaskIds"] = in.SuccessorTaskIDs
	}
	if in.TerminalReason != "" {
		out["terminalReason"] = in.TerminalReason
	}
	return out
}
func (c *client) callBacklogPreview(ctx context.Context, name string, fields backlogMutationFields, envelope previewEnvelope) (*mcp.CallToolResult, any, error) {
	return c.call(ctx, "POST", "/v1/commands/preview", command(name, backlogArguments(fields), previewEnv(envelope)))
}
func (c *client) callBacklogExecute(ctx context.Context, name string, fields backlogMutationFields, envelope mutationExecuteEnvelope) (*mcp.CallToolResult, any, error) {
	return c.call(ctx, "POST", "/v1/commands/execute", command(name, backlogArguments(fields), mutationExecuteEnv(envelope)))
}
func (c *client) backlogCreatePreview(ctx context.Context, _ *mcp.CallToolRequest, in backlogCreatePreviewInput) (*mcp.CallToolResult, any, error) {
	return c.callBacklogPreview(ctx, "backlog.create", backlogMutationFields{WorkspaceID: in.WorkspaceID, BacklogUUID: in.BacklogUUID, LaneID: in.LaneID, Title: &in.Title, Description: in.Description}, in.previewEnvelope)
}
func (c *client) backlogCreateExecute(ctx context.Context, _ *mcp.CallToolRequest, in backlogCreateExecuteInput) (*mcp.CallToolResult, any, error) {
	return c.callBacklogExecute(ctx, "backlog.create", backlogMutationFields{WorkspaceID: in.WorkspaceID, BacklogUUID: in.BacklogUUID, LaneID: in.LaneID, Title: &in.Title, Description: in.Description}, in.mutationExecuteEnvelope)
}
func (c *client) backlogUpdatePreview(ctx context.Context, _ *mcp.CallToolRequest, in backlogUpdatePreviewInput) (*mcp.CallToolResult, any, error) {
	return c.callBacklogPreview(ctx, "backlog.update", backlogMutationFields{WorkspaceID: in.WorkspaceID, BacklogPublicID: in.BacklogPublicID, Title: in.Title, Description: in.Description}, in.previewEnvelope)
}
func (c *client) backlogUpdateExecute(ctx context.Context, _ *mcp.CallToolRequest, in backlogUpdateExecuteInput) (*mcp.CallToolResult, any, error) {
	return c.callBacklogExecute(ctx, "backlog.update", backlogMutationFields{WorkspaceID: in.WorkspaceID, BacklogPublicID: in.BacklogPublicID, Title: in.Title, Description: in.Description}, in.mutationExecuteEnvelope)
}
func (c *client) backlogMovePreview(ctx context.Context, _ *mcp.CallToolRequest, in backlogMovePreviewInput) (*mcp.CallToolResult, any, error) {
	return c.callBacklogPreview(ctx, "backlog.move", backlogMutationFields{WorkspaceID: in.WorkspaceID, BacklogPublicID: in.BacklogPublicID, TargetLaneID: in.TargetLaneID}, in.previewEnvelope)
}
func (c *client) backlogMoveExecute(ctx context.Context, _ *mcp.CallToolRequest, in backlogMoveExecuteInput) (*mcp.CallToolResult, any, error) {
	return c.callBacklogExecute(ctx, "backlog.move", backlogMutationFields{WorkspaceID: in.WorkspaceID, BacklogPublicID: in.BacklogPublicID, TargetLaneID: in.TargetLaneID}, in.mutationExecuteEnvelope)
}
func (c *client) backlogReorderPreview(ctx context.Context, _ *mcp.CallToolRequest, in backlogReorderPreviewInput) (*mcp.CallToolResult, any, error) {
	return c.callBacklogPreview(ctx, "backlog.reorder", backlogMutationFields{WorkspaceID: in.WorkspaceID, LaneID: in.LaneID, OrderedBacklogPublicIDs: in.OrderedBacklogPublicIDs}, in.previewEnvelope)
}
func (c *client) backlogReorderExecute(ctx context.Context, _ *mcp.CallToolRequest, in backlogReorderExecuteInput) (*mcp.CallToolResult, any, error) {
	return c.callBacklogExecute(ctx, "backlog.reorder", backlogMutationFields{WorkspaceID: in.WorkspaceID, LaneID: in.LaneID, OrderedBacklogPublicIDs: in.OrderedBacklogPublicIDs}, in.mutationExecuteEnvelope)
}
func (c *client) backlogDiscardPreview(ctx context.Context, _ *mcp.CallToolRequest, in backlogDiscardPreviewInput) (*mcp.CallToolResult, any, error) {
	return c.callBacklogPreview(ctx, "backlog.discard", backlogMutationFields{WorkspaceID: in.WorkspaceID, BacklogPublicID: in.BacklogPublicID, Reason: in.Reason}, in.previewEnvelope)
}
func (c *client) backlogDiscardExecute(ctx context.Context, _ *mcp.CallToolRequest, in backlogDiscardExecuteInput) (*mcp.CallToolResult, any, error) {
	return c.callBacklogExecute(ctx, "backlog.discard", backlogMutationFields{WorkspaceID: in.WorkspaceID, BacklogPublicID: in.BacklogPublicID, Reason: in.Reason}, in.mutationExecuteEnvelope)
}
func (c *client) backlogPromotePreview(ctx context.Context, _ *mcp.CallToolRequest, in backlogPromotePreviewInput) (*mcp.CallToolResult, any, error) {
	return c.callBacklogPreview(ctx, "backlog.promote", backlogMutationFields{WorkspaceID: in.WorkspaceID, BacklogPublicID: in.BacklogPublicID, TaskUUID: in.TaskUUID, PhaseID: in.PhaseID, ParentTaskID: in.ParentTaskID, PredecessorTaskIDs: in.PredecessorTaskIDs, SuccessorTaskIDs: in.SuccessorTaskIDs, TerminalReason: in.TerminalReason, RequestedAcceptanceMode: in.RequestedAcceptanceMode, EvidenceProfileID: in.EvidenceProfileID}, in.previewEnvelope)
}
func (c *client) backlogPromoteExecute(ctx context.Context, _ *mcp.CallToolRequest, in backlogPromoteExecuteInput) (*mcp.CallToolResult, any, error) {
	return c.callBacklogExecute(ctx, "backlog.promote", backlogMutationFields{WorkspaceID: in.WorkspaceID, BacklogPublicID: in.BacklogPublicID, TaskUUID: in.TaskUUID, PhaseID: in.PhaseID, ParentTaskID: in.ParentTaskID, PredecessorTaskIDs: in.PredecessorTaskIDs, SuccessorTaskIDs: in.SuccessorTaskIDs, TerminalReason: in.TerminalReason, RequestedAcceptanceMode: in.RequestedAcceptanceMode, EvidenceProfileID: in.EvidenceProfileID}, in.mutationExecuteEnvelope)
}
func phaseCreateArguments(in phaseCreateFields) map[string]any {
	return map[string]any{"workspaceId": in.WorkspaceID, "phaseId": in.PhaseID, "name": in.Name}
}
func (c *client) phaseCreatePreview(ctx context.Context, _ *mcp.CallToolRequest, in phaseCreatePreviewInput) (*mcp.CallToolResult, any, error) {
	return c.call(ctx, "POST", "/v1/commands/preview", command("phase.create", phaseCreateArguments(in.phaseCreateFields), previewEnv(in.previewEnvelope)))
}
func (c *client) phaseCreateExecute(ctx context.Context, _ *mcp.CallToolRequest, in phaseCreateExecuteInput) (*mcp.CallToolResult, any, error) {
	return c.call(ctx, "POST", "/v1/commands/execute", command("phase.create", phaseCreateArguments(in.phaseCreateFields), mutationExecuteEnv(in.mutationExecuteEnvelope)))
}
func laneCreateArguments(in laneCreateFields) map[string]any {
	arguments := map[string]any{"workspaceId": in.WorkspaceID, "laneId": in.LaneID, "name": in.Name}
	if in.Goal != "" {
		arguments["goal"] = in.Goal
	}
	if in.Summary != "" {
		arguments["summary"] = in.Summary
	}
	return arguments
}
func (c *client) laneCreatePreview(ctx context.Context, _ *mcp.CallToolRequest, in laneCreatePreviewInput) (*mcp.CallToolResult, any, error) {
	return c.call(ctx, "POST", "/v1/commands/preview", command("lane.create", laneCreateArguments(in.laneCreateFields), previewEnv(in.previewEnvelope)))
}
func (c *client) laneCreateExecute(ctx context.Context, _ *mcp.CallToolRequest, in laneCreateExecuteInput) (*mcp.CallToolResult, any, error) {
	return c.call(ctx, "POST", "/v1/commands/execute", command("lane.create", laneCreateArguments(in.laneCreateFields), mutationExecuteEnv(in.mutationExecuteEnvelope)))
}
func gateCreateArguments(in gateCreateFields) map[string]any {
	arguments := map[string]any{"workspaceId": in.WorkspaceID, "gateId": in.GateID, "name": in.Name, "fromPhaseId": in.FromPhaseID, "toPhaseId": in.ToPhaseID}
	if in.Alias != "" {
		arguments["alias"] = in.Alias
	}
	return arguments
}
func (c *client) gateCreatePreview(ctx context.Context, _ *mcp.CallToolRequest, in gateCreatePreviewInput) (*mcp.CallToolResult, any, error) {
	return c.call(ctx, "POST", "/v1/commands/preview", command("gate.create", gateCreateArguments(in.gateCreateFields), previewEnv(in.previewEnvelope)))
}
func (c *client) gateCreateExecute(ctx context.Context, _ *mcp.CallToolRequest, in gateCreateExecuteInput) (*mcp.CallToolResult, any, error) {
	return c.call(ctx, "POST", "/v1/commands/execute", command("gate.create", gateCreateArguments(in.gateCreateFields), mutationExecuteEnv(in.mutationExecuteEnvelope)))
}
func gateAttachTaskArguments(in gateAttachTaskFields) map[string]any {
	arguments := map[string]any{"workspaceId": in.WorkspaceID, "gateId": in.GateID, "taskId": in.TaskID}
	if in.ClearTerminal {
		arguments["clearTerminal"] = true
	}
	return arguments
}
func (c *client) gateAttachTaskPreview(ctx context.Context, _ *mcp.CallToolRequest, in gateAttachTaskPreviewInput) (*mcp.CallToolResult, any, error) {
	return c.call(ctx, "POST", "/v1/commands/preview", command("gate.attach_task", gateAttachTaskArguments(in.gateAttachTaskFields), previewEnv(in.previewEnvelope)))
}
func (c *client) gateAttachTaskExecute(ctx context.Context, _ *mcp.CallToolRequest, in gateAttachTaskExecuteInput) (*mcp.CallToolResult, any, error) {
	return c.call(ctx, "POST", "/v1/commands/execute", command("gate.attach_task", gateAttachTaskArguments(in.gateAttachTaskFields), conditionalExecuteEnv(in.conditionalExecuteEnvelope)))
}
func gateEntryTaskArguments(in gateEntryTaskFields) map[string]any {
	return map[string]any{"workspaceId": in.WorkspaceID, "gateId": in.GateID, "taskId": in.TaskID}
}
func (c *client) gateAttachEntryTaskPreview(ctx context.Context, _ *mcp.CallToolRequest, in gateEntryTaskPreviewInput) (*mcp.CallToolResult, any, error) {
	return c.call(ctx, "POST", "/v1/commands/preview", command("gate.attach_entry_task", gateEntryTaskArguments(in.gateEntryTaskFields), previewEnv(in.previewEnvelope)))
}
func (c *client) gateAttachEntryTaskExecute(ctx context.Context, _ *mcp.CallToolRequest, in gateEntryTaskExecuteInput) (*mcp.CallToolResult, any, error) {
	return c.call(ctx, "POST", "/v1/commands/execute", command("gate.attach_entry_task", gateEntryTaskArguments(in.gateEntryTaskFields), automaticEnv(in.automaticEnvelope)))
}
func (c *client) gateDetachEntryTaskPreview(ctx context.Context, _ *mcp.CallToolRequest, in gateEntryTaskPreviewInput) (*mcp.CallToolResult, any, error) {
	return c.call(ctx, "POST", "/v1/commands/preview", command("gate.detach_entry_task", gateEntryTaskArguments(in.gateEntryTaskFields), previewEnv(in.previewEnvelope)))
}
func (c *client) gateDetachEntryTaskExecute(ctx context.Context, _ *mcp.CallToolRequest, in gateEntryTaskExecuteInput) (*mcp.CallToolResult, any, error) {
	return c.call(ctx, "POST", "/v1/commands/execute", command("gate.detach_entry_task", gateEntryTaskArguments(in.gateEntryTaskFields), automaticEnv(in.automaticEnvelope)))
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
