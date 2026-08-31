package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
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
	gatewayToken        string
	secretStore         secretStore
	credentialStorePath string
	agentActorID        string
	connectionMu        sync.Mutex
	// sessionTokens are intentionally process-local. The persisted credential
	// store contains only a gateway registration and must never resurrect an
	// Agent token after the MCP process has restarted.
	sessionTokens map[string]string
}

type workspaceInput struct {
	WorkspaceID string `json:"workspaceId" jsonschema:"Baley workspace ID"`
}
type phaseTasksInput struct {
	WorkspaceID string `json:"workspaceId" jsonschema:"Baley workspace ID"`
	PhaseID     string `json:"phaseId" jsonschema:"Non-completed Phase ID to expand explicitly"`
	Cursor      int    `json:"cursor,omitempty" jsonschema:"Task public-ID cursor returned by a prior page"`
	Limit       int    `json:"limit,omitempty" jsonschema:"Page size from 1 to 100; default 50"`
}
type diagnosticsInput struct{}
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
	CurrentSummary          string `json:"currentSummary,omitempty" jsonschema:"Short, plain-language 1-2 sentence explanation for a human"`
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
type taskUpdateFields struct {
	WorkspaceID    string  `json:"workspaceId"`
	TaskID         int     `json:"taskId"`
	Title          *string `json:"title,omitempty"`
	Description    *string `json:"description,omitempty"`
	CurrentSummary *string `json:"currentSummary,omitempty" jsonschema:"Short, plain-language 1-2 sentence explanation for a human"`
}
type taskUpdatePreviewInput struct {
	taskUpdateFields
	previewEnvelope
}
type taskUpdateExecuteInput struct {
	taskUpdateFields
	mutationExecuteEnvelope
}
type taskMoveFields struct {
	WorkspaceID   string `json:"workspaceId"`
	TaskID        int    `json:"taskId"`
	TargetPhaseID string `json:"targetPhaseId"`
}
type taskMovePreviewInput struct {
	taskMoveFields
	previewEnvelope
}
type taskMoveExecuteInput struct {
	taskMoveFields
	mutationExecuteEnvelope
}
type taskClearTerminalFields struct {
	WorkspaceID string `json:"workspaceId"`
	TaskID      int    `json:"taskId"`
}
type taskClearTerminalPreviewInput struct {
	taskClearTerminalFields
	previewEnvelope
}
type taskClearTerminalExecuteInput struct {
	taskClearTerminalFields
	mutationExecuteEnvelope
}
type dependencyRefInput struct {
	PredecessorTaskID int `json:"predecessorTaskId"`
	SuccessorTaskID   int `json:"successorTaskId"`
}
type dependencyPatchFields struct {
	WorkspaceID string               `json:"workspaceId"`
	Add         []dependencyRefInput `json:"add,omitempty"`
	Remove      []dependencyRefInput `json:"remove,omitempty"`
}
type dependencyPatchPreviewInput struct {
	dependencyPatchFields
	previewEnvelope
}
type dependencyPatchExecuteInput struct {
	dependencyPatchFields
	mutationExecuteEnvelope
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
	ExpectedWorkspaceRevision int64    `json:"expectedWorkspaceRevision"`
	IdempotencyKey            string   `json:"idempotencyKey"`
	ExecutedByActorID         string   `json:"executedByActorId"`
	InitiatedByActorID        string   `json:"initiatedByActorId,omitempty"`
	AcknowledgedWarningCodes  []string `json:"acknowledgedWarningCodes,omitempty"`
	ProceedReason             string   `json:"proceedReason,omitempty"`
	ApprovalGrantID           string   `json:"approvalGrantId"`
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
	ApprovalGrantID string `json:"approvalGrantId,omitempty"`
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
type taskDiscardPreviewInput struct {
	WorkspaceID string `json:"workspaceId"`
	TaskID      int    `json:"taskId"`
	Reason      string `json:"reason"`
	previewEnvelope
}
type taskDiscardExecuteInput struct {
	WorkspaceID string `json:"workspaceId"`
	TaskID      int    `json:"taskId"`
	Reason      string `json:"reason"`
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
	mode := "stdio"
	if len(os.Args) > 2 || (len(os.Args) == 2 && os.Args[1] != "serve-http" && os.Args[1] != "migrate-legacy" && os.Args[1] != "rollback-legacy" && os.Args[1] != "diagnose") {
		log.Fatal("usage: baley-mcp [serve-http|migrate-legacy|rollback-legacy|diagnose]")
	}
	if len(os.Args) == 2 {
		mode = os.Args[1]
	}
	base := os.Getenv("BALEY_SERVER_URL")
	if base == "" {
		base = "http://127.0.0.1:8080"
	}
	validationMode := mode
	if mode != "serve-http" {
		validationMode = "stdio"
	}
	if _, err := validateServerURL(base, validationMode); err != nil {
		log.Fatal(err)
	}
	c := &client{
		base:                strings.TrimRight(base, "/"),
		http:                &http.Client{Timeout: 15 * time.Second},
		gatewayToken:        strings.TrimSpace(os.Getenv("BALEY_MCP_GATEWAY_TOKEN")),
		secretStore:         newOSSecretStore(),
		credentialStorePath: strings.TrimSpace(os.Getenv("BALEY_MCP_CREDENTIAL_STORE")),
		agentActorID:        strings.TrimSpace(os.Getenv("BALEY_AGENT_ACTOR_ID")),
	}
	if c.agentActorID == "" {
		c.agentActorID = "00000000-0000-4000-8000-000000000003"
	}
	switch mode {
	case "serve-http":
		serveHTTP(c)
		return
	case "migrate-legacy":
		if err := c.migrateLegacyCredentialStore(context.Background()); err != nil {
			log.Fatal(err)
		}
		fmt.Println("Baley legacy credentials migrated to the OS keychain and gateway registrations revalidated.")
		return
	case "rollback-legacy":
		if err := c.rollbackLegacyCredentialStore(); err != nil {
			log.Fatal(err)
		}
		fmt.Println("Baley local credential migration rolled back. Server-side revocation and membership rules remain enforced.")
		return
	case "diagnose":
		if err := json.NewEncoder(os.Stdout).Encode(c.localDiagnostics()); err != nil {
			log.Fatal(err)
		}
		return
	}
	server := newMCPServer(c)
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}

func toolHint(value bool) *bool {
	return &value
}

func readOnlyTool(name, description string) *mcp.Tool {
	return &mcp.Tool{
		Name:        name,
		Description: description,
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:    true,
			DestructiveHint: toolHint(false),
			OpenWorldHint:   toolHint(false),
		},
	}
}

// phaseTasksTool makes the compact-context pagination boundary visible to MCP
// clients as well as enforcing it in phaseTasks. Keep this explicit rather
// than relying on a prose-only struct tag: clients can reject impossible page
// requests before invoking the tool.
func phaseTasksTool() *mcp.Tool {
	tool := readOnlyTool("baley_phase_tasks", "List one explicitly selected non-completed Phase's Tasks with a bounded cursor page")
	tool.InputSchema = json.RawMessage(`{
		"type":"object",
		"properties":{
			"workspaceId":{"type":"string","description":"Baley workspace ID"},
			"phaseId":{"type":"string","description":"Non-completed Phase ID to expand explicitly"},
			"cursor":{"type":"integer","minimum":0,"description":"Task public-ID cursor returned by a prior page"},
			"limit":{"type":"integer","minimum":1,"maximum":100,"default":50,"description":"Page size from 1 to 100; default 50"}
		},
		"required":["workspaceId","phaseId"],
		"additionalProperties":false
	}`)
	return tool
}

func operatorTool(name, description string) *mcp.Tool {
	return &mcp.Tool{
		Name:        name,
		Description: description,
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: toolHint(false),
			OpenWorldHint:   toolHint(false),
		},
	}
}

func humanApprovalTool(name, description string) *mcp.Tool {
	return &mcp.Tool{
		Name:        name,
		Description: description,
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: toolHint(true),
			OpenWorldHint:   toolHint(false),
		},
	}
}

func classifiedTool(name, description string) *mcp.Tool {
	if strings.HasSuffix(name, "_preview") {
		return readOnlyTool(name, description)
	}
	switch name {
	case "baley_task_acceptance_policy_change_execute",
		"baley_task_acceptance_mode_escalate_execute",
		"baley_gate_attach_task_execute",
		"baley_task_confirm_execute",
		"baley_task_discard_execute",
		"baley_gate_pass_task_execute",
		"baley_gate_revoke_task_pass_execute",
		"baley_gate_pass_execute":
		return humanApprovalTool(name, description)
	default:
		return operatorTool(name, description)
	}
}

func validateServerURL(base, mode string) (*url.URL, error) {
	parsed, err := url.Parse(base)
	if err != nil || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Hostname() == "" {
		return nil, errors.New("BALEY_SERVER_URL must be an absolute http(s) URL without credentials, query, or fragment")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, errors.New("BALEY_SERVER_URL must use http or https")
	}
	if mode == "stdio" && parsed.Scheme == "http" && !(parsed.Hostname() == "127.0.0.1" || parsed.Hostname() == "localhost" || parsed.Hostname() == "::1") {
		return nil, errors.New("stdio BALEY_SERVER_URL must use HTTPS unless it is loopback HTTP")
	}
	return parsed, nil
}
func newMCPServer(c *client) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "baley", Version: "0.1.0"}, nil)
	mcp.AddTool(server, readOnlyTool("baley_workspace_get", "Read Workspace metadata"), c.workspaceGet)
	mcp.AddTool(server, readOnlyTool("baley_mcp_diagnostics", "Report tokenless credential-store, keychain, and local transport safety without exposing secrets"), c.diagnostics)
	mcp.AddTool(server, readOnlyTool("baley_workspace_context", "Read compact non-completed Phase and Lane status counts; expand a named Phase only when Task detail is needed"), c.workspaceContext)
	mcp.AddTool(server, readOnlyTool("baley_workspace_graph", "Read the current Workspace graph"), c.workspaceGraph)
	mcp.AddTool(server, phaseTasksTool(), c.phaseTasks)
	mcp.AddTool(server, readOnlyTool("baley_task_get", "Read one Task by public ID"), c.taskGet)
	mcp.AddTool(server, readOnlyTool("baley_task_acceptance_get", "Read a Task acceptance binding, policy/profile, assignments, and typed evidence"), c.taskAcceptanceGet)
	mcp.AddTool(server, readOnlyTool("baley_lane_brief", "Build a read-only active-Run-first lane recovery brief with evidence mismatch classification"), c.laneBrief)
	mcp.AddTool(server, readOnlyTool("baley_backlog_list", "List lane Backlog items with optional lane/status filters"), c.backlogList)
	mcp.AddTool(server, readOnlyTool("baley_backlog_get", "Read one Backlog item by B# public ID"), c.backlogGet)
	mcp.AddTool(server, readOnlyTool("baley_gate_status", "Read Gate status and conditions"), c.gateStatus)
	mcp.AddTool(server, readOnlyTool("baley_decision_list", "List human decisions currently available"), c.decisionList)
	mcp.AddTool(server, readOnlyTool("baley_event_list", "List Workspace Events"), c.eventList)
	mcp.AddTool(server, readOnlyTool("baley_mutation_attempt_list", "List append-only Workspace mutation attempts"), c.mutationAttemptList)
	mcp.AddTool(server, readOnlyTool("baley_run_list", "List Workspace Runs"), c.runList)
	mcp.AddTool(server, readOnlyTool("baley_record_list", "List Task Record indexes without loading document bodies"), c.recordList)
	mcp.AddTool(server, classifiedTool("baley_run_start", "Start a Run and automatically start a pending Task"), c.runStart)
	mcp.AddTool(server, classifiedTool("baley_run_heartbeat", "Extend a running Run lease using token and Run version CAS"), c.runHeartbeat)
	mcp.AddTool(server, classifiedTool("baley_run_succeed", "Mark a Run succeeded using Run version CAS"), c.runSucceed)
	mcp.AddTool(server, classifiedTool("baley_run_fail", "Mark a Run failed using Run version CAS"), c.runFail)
	mcp.AddTool(server, classifiedTool("baley_run_cancel", "Cancel a Run using Run version CAS"), c.runCancel)
	mcp.AddTool(server, classifiedTool("baley_run_interrupt", "Interrupt a Run using Run version CAS"), c.runInterrupt)
	mcp.AddTool(server, classifiedTool("baley_run_correct", "Correct a terminal Run with an explicit reason"), c.runCorrect)
	mcp.AddTool(server, classifiedTool("baley_repository_register", "Register a Git repository and optional Task Record root"), c.repositoryRegister)
	mcp.AddTool(server, classifiedTool("baley_record_register", "Register a repository-relative Task Record index"), c.recordRegister)
	mcp.AddTool(server, classifiedTool("baley_record_attach_commit", "Attach commit and blob evidence to a Task Record"), c.recordAttachCommit)
	mcp.AddTool(server, classifiedTool("baley_commit_attach", "Attach a Git commit reference to a Task"), c.commitAttach)
	mcp.AddTool(server, classifiedTool("baley_git_observe", "Record non-authoritative Run Git metadata"), c.gitObserve)
	mcp.AddTool(server, classifiedTool("baley_task_report_implemented", "Report implementation complete with assessment and explicit warning acknowledgement"), c.taskReportImplemented)
	mcp.AddTool(server, classifiedTool("baley_task_evidence_report", "Append typed acceptance evidence; evidence never confirms a Task"), c.taskEvidenceReport)
	mcp.AddTool(server, classifiedTool("baley_task_acceptance_policy_change_preview", "Preview a human-approved future-Task acceptance policy change"), c.acceptancePolicyChangePreview)
	mcp.AddTool(server, classifiedTool("baley_task_acceptance_policy_change_execute", "Execute an approved future-Task acceptance policy change"), c.acceptancePolicyChangeExecute)
	mcp.AddTool(server, classifiedTool("baley_task_acceptance_mode_escalate_preview", "Preview monotonic delegated to human-required escalation"), c.acceptanceModeEscalatePreview)
	mcp.AddTool(server, classifiedTool("baley_task_acceptance_mode_escalate_execute", "Execute an approved monotonic acceptance escalation"), c.acceptanceModeEscalateExecute)
	mcp.AddTool(server, classifiedTool("baley_task_create_preview", "Preview atomic Task creation and initial relationships without writing"), c.taskCreatePreview)
	mcp.AddTool(server, classifiedTool("baley_task_create_execute", "Create a Task and its initial relationships after reviewing the preview"), c.taskCreateExecute)
	mcp.AddTool(server, classifiedTool("baley_task_update_preview", "Preview changing a Task title and/or description without writing"), c.taskUpdatePreview)
	mcp.AddTool(server, classifiedTool("baley_task_update_execute", "Update a non-terminal Task title and/or description after preview"), c.taskUpdateExecute)
	mcp.AddTool(server, classifiedTool("baley_task_move_preview", "Preview moving a non-confirmed, non-discarded Task to another Phase without replacing it"), c.taskMovePreview)
	mcp.AddTool(server, classifiedTool("baley_task_move_execute", "Move a non-confirmed, non-discarded Task to another Phase after preview"), c.taskMoveExecute)
	mcp.AddTool(server, classifiedTool("baley_task_clear_terminal_preview", "Preview removing a Task terminal reason so a successor can be connected"), c.taskClearTerminalPreview)
	mcp.AddTool(server, classifiedTool("baley_task_clear_terminal_execute", "Remove a Task terminal reason after preview"), c.taskClearTerminalExecute)
	mcp.AddTool(server, classifiedTool("baley_dependency_patch_preview", "Preview an atomic dependency graph rewrite without writing"), c.dependencyPatchPreview)
	mcp.AddTool(server, classifiedTool("baley_dependency_patch_execute", "Atomically apply dependency additions and removals after preview"), c.dependencyPatchExecute)
	mcp.AddTool(server, classifiedTool("baley_backlog_create_preview", "Preview creating a phase-free lane Backlog item"), c.backlogCreatePreview)
	mcp.AddTool(server, classifiedTool("baley_backlog_create_execute", "Create a phase-free lane Backlog item"), c.backlogCreateExecute)
	mcp.AddTool(server, classifiedTool("baley_backlog_update_preview", "Preview updating an active Backlog item"), c.backlogUpdatePreview)
	mcp.AddTool(server, classifiedTool("baley_backlog_update_execute", "Update an active Backlog item"), c.backlogUpdateExecute)
	mcp.AddTool(server, classifiedTool("baley_backlog_move_preview", "Preview moving an active Backlog item to another lane"), c.backlogMovePreview)
	mcp.AddTool(server, classifiedTool("baley_backlog_move_execute", "Move an active Backlog item to another lane"), c.backlogMoveExecute)
	mcp.AddTool(server, classifiedTool("baley_backlog_reorder_preview", "Preview replacing one lane's complete active Backlog order"), c.backlogReorderPreview)
	mcp.AddTool(server, classifiedTool("baley_backlog_reorder_execute", "Replace one lane's complete active Backlog order"), c.backlogReorderExecute)
	mcp.AddTool(server, classifiedTool("baley_backlog_discard_preview", "Preview audited soft-discard of a Backlog item"), c.backlogDiscardPreview)
	mcp.AddTool(server, classifiedTool("baley_backlog_discard_execute", "Soft-discard an active Backlog item"), c.backlogDiscardExecute)
	mcp.AddTool(server, classifiedTool("baley_backlog_promote_preview", "Preview atomic Backlog promotion into a phase-targeted pending Task"), c.backlogPromotePreview)
	mcp.AddTool(server, classifiedTool("baley_backlog_promote_execute", "Atomically promote Backlog into a pending Task with exact warning acknowledgement"), c.backlogPromoteExecute)
	mcp.AddTool(server, classifiedTool("baley_phase_create_preview", "Preview appending a Phase without writing"), c.phaseCreatePreview)
	mcp.AddTool(server, classifiedTool("baley_phase_create_execute", "Append a Phase after reviewing the preview"), c.phaseCreateExecute)
	mcp.AddTool(server, classifiedTool("baley_lane_create_preview", "Preview creating a Lane without writing"), c.laneCreatePreview)
	mcp.AddTool(server, classifiedTool("baley_lane_create_execute", "Create a Lane after reviewing the preview"), c.laneCreateExecute)
	mcp.AddTool(server, classifiedTool("baley_gate_create_preview", "Preview creating a Phase Gate without writing"), c.gateCreatePreview)
	mcp.AddTool(server, classifiedTool("baley_gate_create_execute", "Create a Phase Gate after reviewing the preview"), c.gateCreateExecute)
	mcp.AddTool(server, classifiedTool("baley_gate_attach_task_preview", "Preview attaching a Task as a Gate condition without writing"), c.gateAttachTaskPreview)
	mcp.AddTool(server, classifiedTool("baley_gate_attach_task_execute", "Attach a Task to a Gate; active Gates require fields from an explicitly approved fresh preview"), c.gateAttachTaskExecute)
	mcp.AddTool(server, classifiedTool("baley_gate_attach_entry_task_preview", "Preview binding a to-Phase Task as work unlocked by a Gate"), c.gateAttachEntryTaskPreview)
	mcp.AddTool(server, classifiedTool("baley_gate_attach_entry_task_execute", "Bind a to-Phase Task as work unlocked by a Gate"), c.gateAttachEntryTaskExecute)
	mcp.AddTool(server, classifiedTool("baley_gate_detach_entry_task_preview", "Preview removing an explicit Gate entry Task binding"), c.gateDetachEntryTaskPreview)
	mcp.AddTool(server, classifiedTool("baley_gate_detach_entry_task_execute", "Remove an explicit Gate entry Task binding and restore automatic root selection when none remain"), c.gateDetachEntryTaskExecute)
	mcp.AddTool(server, classifiedTool("baley_task_confirm_preview", "Preview Task confirmation without writing"), c.taskConfirmPreview)
	mcp.AddTool(server, classifiedTool("baley_task_confirm_execute", "Execute an explicitly approved Task confirmation with exact warning acknowledgement when preview returned warnings"), c.taskConfirmExecute)
	mcp.AddTool(server, classifiedTool("baley_task_discard_preview", "Preview an explicitly approved audited Task discard without writing"), c.taskDiscardPreview)
	mcp.AddTool(server, classifiedTool("baley_task_discard_execute", "Execute an explicitly approved audited Task discard with a reason"), c.taskDiscardExecute)
	mcp.AddTool(server, classifiedTool("baley_gate_pass_task_preview", "Preview explicit Gate Task pass without writing"), c.gatePassTaskPreview)
	mcp.AddTool(server, classifiedTool("baley_gate_pass_task_execute", "Execute an explicitly approved Gate Task pass"), c.gatePassTaskExecute)
	mcp.AddTool(server, classifiedTool("baley_gate_revoke_task_pass_preview", "Preview Gate Task pass revocation without writing"), c.gateRevokePreview)
	mcp.AddTool(server, classifiedTool("baley_gate_revoke_task_pass_execute", "Execute an explicitly approved Gate Task pass revocation"), c.gateRevokeExecute)
	mcp.AddTool(server, classifiedTool("baley_gate_pass_preview", "Preview Gate pass and Phase transition without writing"), c.gatePassPreview)
	mcp.AddTool(server, classifiedTool("baley_gate_pass_execute", "Execute an explicitly approved Gate pass and Phase transition"), c.gatePassExecute)
	return server
}

func serveHTTP(c *client) {
	if strings.TrimSpace(c.credentialStorePath) == "" {
		log.Fatal("BALEY_MCP_CREDENTIAL_STORE is required for tokenless serve-http")
	}
	addr := strings.TrimSpace(os.Getenv("BALEY_MCP_HTTP_ADDR"))
	if addr == "" {
		addr = "127.0.0.1:8090"
	}
	if err := validateLoopbackMCPAddress(addr); err != nil {
		log.Fatal(err)
	}
	// Workspace credentials are scoped to this local gateway identity and the
	// target Workspace, not to an ephemeral MCP transport session. A new Codex
	// chat or the HTTP session timeout must not require a new gateway login.
	streamable := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return newMCPServer(c) }, &mcp.StreamableHTTPOptions{JSONResponse: true, SessionTimeout: 10 * time.Minute})
	mux := http.NewServeMux()
	// The endpoint is loopback-only. Credentials are never carried in Codex
	// configuration: the local Gateway reads the device binding from the OS
	// Keychain and obtains short-lived Workspace-scoped Agent tokens itself.
	mux.Handle("/mcp", streamable)
	httpServer := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 20 * time.Second, WriteTimeout: 35 * time.Second, IdleTimeout: 70 * time.Second, MaxHeaderBytes: 1 << 20}
	log.Printf("Baley Streamable HTTP MCP listening on http://%s/mcp", addr)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func validateLoopbackMCPAddress(addr string) error {
	host, _, err := net.SplitHostPort(strings.TrimSpace(addr))
	if err != nil || (host != "127.0.0.1" && host != "::1") {
		return errors.New("BALEY_MCP_HTTP_ADDR must bind to loopback")
	}
	return nil
}

func (c *client) get(ctx context.Context, path string) (*mcp.CallToolResult, any, error) {
	return c.call(ctx, "GET", path, nil)
}
func (c *client) call(ctx context.Context, method, path string, payload any) (*mcp.CallToolResult, any, error) {
	workspaceID := requestWorkspaceID(path, payload)
	token := ""
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
	if c.credentialStorePath != "" && workspaceID != "" && token != "" {
		credentialRejected := res.StatusCode == http.StatusUnauthorized
		if res.StatusCode == http.StatusNotFound {
			// A 404 is also the normal result for a missing Task, Record, or other
			// Workspace resource. Do not turn those ordinary reads into another
			// gateway login. Validate the stored credential against the Workspace
			// root before treating a concealed Workspace read as a revoked token.
			credentialRejected = !c.workspaceCredentialValid(ctx, workspaceID, token)
		}
		if credentialRejected {
			if err = c.removeWorkspaceCredential(ctx, workspaceID); err != nil {
				return nil, nil, err
			}
			_, pending, connectErr := c.workspaceCredential(ctx, workspaceID)
			return pending, pendingStructured(pending), connectErr
		}
	}
	summary := fmt.Sprintf("Baley HTTP %d", res.StatusCode)
	if res.StatusCode >= 200 && res.StatusCode < 300 {
		summary = "Baley request succeeded"
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: summary}}, StructuredContent: structured, IsError: res.StatusCode >= 400}, structured, nil
}
func (c *client) workspaceCredentialValid(ctx context.Context, workspaceID, token string) bool {
	if workspaceID == "" || token == "" {
		return false
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/v1/workspaces/"+url.PathEscape(workspaceID), nil)
	if err != nil {
		return false
	}
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := c.http.Do(req)
	if err != nil {
		return false
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 64<<10))
	return res.StatusCode >= http.StatusOK && res.StatusCode < http.StatusMultipleChoices
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
	if v.ApprovalGrantID != "" {
		envelope["approvalGrantId"] = v.ApprovalGrantID
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
	if v.ApprovalGrantID != "" {
		envelope["approvalGrantId"] = v.ApprovalGrantID
	}
	return envelope
}

func (c *client) workspaceGraph(ctx context.Context, _ *mcp.CallToolRequest, in workspaceInput) (*mcp.CallToolResult, any, error) {
	return c.get(ctx, "/v1/workspaces/"+url.PathEscape(in.WorkspaceID)+"/graph")
}
func (c *client) workspaceContext(ctx context.Context, _ *mcp.CallToolRequest, in workspaceInput) (*mcp.CallToolResult, any, error) {
	return c.get(ctx, "/v1/workspaces/"+url.PathEscape(in.WorkspaceID)+"/context")
}
func (c *client) phaseTasks(ctx context.Context, _ *mcp.CallToolRequest, in phaseTasksInput) (*mcp.CallToolResult, any, error) {
	if in.Cursor < 0 {
		return nil, nil, errors.New("cursor must be a non-negative Task public ID")
	}
	if in.Limit < 0 || in.Limit > 100 {
		return nil, nil, errors.New("limit must be between 1 and 100")
	}
	values := url.Values{}
	if in.Cursor > 0 {
		values.Set("cursor", fmt.Sprint(in.Cursor))
	}
	if in.Limit > 0 {
		values.Set("limit", fmt.Sprint(in.Limit))
	}
	path := "/v1/workspaces/" + url.PathEscape(in.WorkspaceID) + "/phases/" + url.PathEscape(in.PhaseID) + "/tasks"
	if query := values.Encode(); query != "" {
		path += "?" + query
	}
	return c.get(ctx, path)
}
func (c *client) diagnostics(_ context.Context, _ *mcp.CallToolRequest, _ diagnosticsInput) (*mcp.CallToolResult, any, error) {
	result := c.localDiagnostics()
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "Baley MCP diagnostics collected without exposing credentials."}}, StructuredContent: result}, result, nil
}
func (c *client) localDiagnostics() map[string]any {
	result := map[string]any{
		"serverURL":                 c.base,
		"credentialStoreConfigured": c.credentialStorePath != "",
		"osKeychainConfigured":      c.secretStore != nil,
		"legacyTokenConfigured":     c.gatewayToken != "",
	}
	if c.credentialStorePath == "" {
		result["credentialStoreState"] = "not_configured"
	} else if raw, err := os.ReadFile(c.credentialStorePath); errors.Is(err, os.ErrNotExist) {
		result["credentialStoreState"] = "not_created"
	} else if err != nil {
		result["credentialStoreState"] = "unreadable"
	} else {
		var disk credentialStoreDisk
		if json.Unmarshal(raw, &disk) != nil {
			result["credentialStoreState"] = "invalid"
		} else if disk.KeyRef != "" {
			result["credentialStoreState"] = "keychain_backed"
			if c.secretStore != nil {
				_, keychainErr := c.secretStore.Get(credentialKeychainService, disk.KeyRef)
				result["keychainEntryAvailable"] = keychainErr == nil
			}
		} else if disk.Ciphertext != "" {
			result["credentialStoreState"] = "legacy_migration_required"
		} else {
			result["credentialStoreState"] = "legacy_plaintext_migration_required"
		}
	}
	if eligible, expiresAt := c.legacyRollbackEligible(); eligible {
		result["legacyRollbackEligible"] = true
		result["legacyRollbackExpiresAt"] = expiresAt
	} else {
		result["legacyRollbackEligible"] = false
	}
	return result
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
		"parentTaskId": in.ParentTaskID, "title": in.Title, "description": in.Description, "currentSummary": in.CurrentSummary,
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
func taskUpdateArguments(in taskUpdateFields) map[string]any {
	arguments := map[string]any{"workspaceId": in.WorkspaceID, "taskId": in.TaskID}
	if in.Title != nil {
		arguments["title"] = *in.Title
	}
	if in.Description != nil {
		arguments["description"] = *in.Description
	}
	if in.CurrentSummary != nil {
		arguments["currentSummary"] = *in.CurrentSummary
	}
	return arguments
}
func (c *client) taskUpdatePreview(ctx context.Context, _ *mcp.CallToolRequest, in taskUpdatePreviewInput) (*mcp.CallToolResult, any, error) {
	return c.call(ctx, "POST", "/v1/commands/preview", command("task.update", taskUpdateArguments(in.taskUpdateFields), previewEnv(in.previewEnvelope)))
}
func (c *client) taskUpdateExecute(ctx context.Context, _ *mcp.CallToolRequest, in taskUpdateExecuteInput) (*mcp.CallToolResult, any, error) {
	return c.call(ctx, "POST", "/v1/commands/execute", command("task.update", taskUpdateArguments(in.taskUpdateFields), mutationExecuteEnv(in.mutationExecuteEnvelope)))
}
func taskMoveArguments(in taskMoveFields) map[string]any {
	return map[string]any{"workspaceId": in.WorkspaceID, "taskId": in.TaskID, "targetPhaseId": in.TargetPhaseID}
}
func (c *client) taskMovePreview(ctx context.Context, _ *mcp.CallToolRequest, in taskMovePreviewInput) (*mcp.CallToolResult, any, error) {
	return c.call(ctx, "POST", "/v1/commands/preview", command("task.move", taskMoveArguments(in.taskMoveFields), previewEnv(in.previewEnvelope)))
}
func (c *client) taskMoveExecute(ctx context.Context, _ *mcp.CallToolRequest, in taskMoveExecuteInput) (*mcp.CallToolResult, any, error) {
	return c.call(ctx, "POST", "/v1/commands/execute", command("task.move", taskMoveArguments(in.taskMoveFields), mutationExecuteEnv(in.mutationExecuteEnvelope)))
}
func taskClearTerminalArguments(in taskClearTerminalFields) map[string]any {
	return map[string]any{"workspaceId": in.WorkspaceID, "taskId": in.TaskID}
}
func (c *client) taskClearTerminalPreview(ctx context.Context, _ *mcp.CallToolRequest, in taskClearTerminalPreviewInput) (*mcp.CallToolResult, any, error) {
	return c.call(ctx, "POST", "/v1/commands/preview", command("task.clear_terminal", taskClearTerminalArguments(in.taskClearTerminalFields), previewEnv(in.previewEnvelope)))
}
func (c *client) taskClearTerminalExecute(ctx context.Context, _ *mcp.CallToolRequest, in taskClearTerminalExecuteInput) (*mcp.CallToolResult, any, error) {
	return c.call(ctx, "POST", "/v1/commands/execute", command("task.clear_terminal", taskClearTerminalArguments(in.taskClearTerminalFields), mutationExecuteEnv(in.mutationExecuteEnvelope)))
}
func dependencyPatchArguments(in dependencyPatchFields) map[string]any {
	arguments := map[string]any{"workspaceId": in.WorkspaceID}
	if in.Add != nil {
		arguments["add"] = in.Add
	}
	if in.Remove != nil {
		arguments["remove"] = in.Remove
	}
	return arguments
}
func (c *client) dependencyPatchPreview(ctx context.Context, _ *mcp.CallToolRequest, in dependencyPatchPreviewInput) (*mcp.CallToolResult, any, error) {
	return c.call(ctx, "POST", "/v1/commands/preview", command("dependency.patch", dependencyPatchArguments(in.dependencyPatchFields), previewEnv(in.previewEnvelope)))
}
func (c *client) dependencyPatchExecute(ctx context.Context, _ *mcp.CallToolRequest, in dependencyPatchExecuteInput) (*mcp.CallToolResult, any, error) {
	return c.call(ctx, "POST", "/v1/commands/execute", command("dependency.patch", dependencyPatchArguments(in.dependencyPatchFields), mutationExecuteEnv(in.mutationExecuteEnvelope)))
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
func (c *client) taskDiscardPreview(ctx context.Context, _ *mcp.CallToolRequest, in taskDiscardPreviewInput) (*mcp.CallToolResult, any, error) {
	return c.call(ctx, "POST", "/v1/commands/preview", command("task.discard", map[string]any{"workspaceId": in.WorkspaceID, "taskId": in.TaskID, "reason": in.Reason}, previewEnv(in.previewEnvelope)))
}
func (c *client) taskDiscardExecute(ctx context.Context, _ *mcp.CallToolRequest, in taskDiscardExecuteInput) (*mcp.CallToolResult, any, error) {
	return c.call(ctx, "POST", "/v1/commands/execute", command("task.discard", map[string]any{"workspaceId": in.WorkspaceID, "taskId": in.TaskID, "reason": in.Reason}, executeEnv(in.executeEnvelope)))
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
