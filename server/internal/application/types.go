package application

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jazzcake/baley/server/internal/domain"
)

type CommandRequest struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
	Envelope  CommandEnvelope `json:"envelope"`
}

type CommandEnvelope struct {
	IdempotencyKey            string                    `json:"idempotencyKey"`
	ExpectedWorkspaceRevision int64                     `json:"expectedWorkspaceRevision,omitempty"`
	InitiatedByActorID        string                    `json:"initiatedByActorId,omitempty"`
	ExecutedByActorID         string                    `json:"executedByActorId"`
	AcknowledgedWarningCodes  []string                  `json:"acknowledgedWarningCodes,omitempty"`
	ProceedReason             string                    `json:"proceedReason,omitempty"`
	HumanApprovalAttestation  *HumanApprovalAttestation `json:"humanApprovalAttestation,omitempty"`
}

type HumanApprovalAttestation struct {
	ApprovedByActorID    string     `json:"approvedByActorId"`
	ApprovedCommandHash  string     `json:"approvedCommandHash"`
	DecisionSnapshotHash string     `json:"decisionSnapshotHash,omitempty"`
	StatementHash        string     `json:"statementHash,omitempty"`
	ConversationRef      string     `json:"conversationRef,omitempty"`
	ApprovedAt           *time.Time `json:"approvedAt,omitempty"`
}

type PhaseProjection struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	State    string `json:"state"`
	Position int    `json:"position"`
}
type LaneProjection struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Goal    string `json:"goal,omitempty"`
	Summary string `json:"summary,omitempty"`
	State   string `json:"state"`
}
type TaskProjection struct {
	ID                        string  `json:"id"`
	PublicID                  int     `json:"publicId"`
	LaneID                    string  `json:"laneId"`
	PhaseID                   string  `json:"phaseId"`
	ParentTaskID              string  `json:"parentTaskId,omitempty"`
	Title                     string  `json:"title"`
	Description               string  `json:"description"`
	CurrentSummary            string  `json:"currentSummary,omitempty"`
	NextAction                string  `json:"nextAction,omitempty"`
	Status                    string  `json:"status"`
	BlockerReason             *string `json:"blockerReason,omitempty"`
	TerminalReason            string  `json:"terminalReason,omitempty"`
	ImplementedAssessment     string  `json:"implementedAssessment,omitempty"`
	DecisionRequired          string  `json:"decisionRequired,omitempty"`
	ExpectedWorkspaceRevision int64   `json:"expectedWorkspaceRevision,omitempty"`
}
type DependencyProjection struct {
	FromTaskID string `json:"fromTaskId"`
	ToTaskID   string `json:"toTaskId"`
}
type GateTaskProjection struct {
	ID                 string     `json:"id"`
	GateID             string     `json:"gateId"`
	TaskID             string     `json:"taskId"`
	PassedAt           *time.Time `json:"passedAt,omitempty"`
	PassReason         *string    `json:"passReason,omitempty"`
	Satisfied          bool       `json:"satisfied"`
	SatisfactionReason string     `json:"satisfactionReason"`
}
type GateProjection struct {
	ID                        string               `json:"id"`
	Name                      string               `json:"name"`
	FromPhaseID               string               `json:"fromPhaseId"`
	ToPhaseID                 string               `json:"toPhaseId"`
	CriteriaRevision          int64                `json:"criteriaRevision"`
	PassedAt                  *time.Time           `json:"passedAt,omitempty"`
	Conditions                []GateTaskProjection `json:"conditions"`
	Status                    string               `json:"status"`
	DecisionRequired          string               `json:"decisionRequired,omitempty"`
	DecisionSnapshotHash      string               `json:"decisionSnapshotHash,omitempty"`
	ExpectedWorkspaceRevision int64                `json:"expectedWorkspaceRevision,omitempty"`
}
type WorkspaceProjection struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	State         string  `json:"state"`
	Revision      int64   `json:"revision"`
	ActivePhaseID *string `json:"activePhaseId,omitempty"`
}
type RunProjection struct {
	ID              string     `json:"id"`
	TaskID          string     `json:"taskId"`
	ClientRunID     string     `json:"clientRunId"`
	Kind            string     `json:"kind"`
	Status          string     `json:"status"`
	OperatorActorID string     `json:"operatorActorId"`
	SessionRef      string     `json:"sessionRef,omitempty"`
	ParentRunID     string     `json:"parentRunId,omitempty"`
	TargetRunID     string     `json:"targetRunId,omitempty"`
	LeaseTokenHash  string     `json:"-"`
	HeartbeatAt     time.Time  `json:"heartbeatAt"`
	LeaseExpiresAt  time.Time  `json:"leaseExpiresAt"`
	Version         int64      `json:"version"`
	StartedAt       time.Time  `json:"startedAt"`
	EndedAt         *time.Time `json:"endedAt,omitempty"`
	ResultSummary   string     `json:"resultSummary,omitempty"`
	ErrorSummary    string     `json:"errorSummary,omitempty"`
}
type RepositoryProjection struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	RemoteURL          string `json:"remoteUrl"`
	DefaultBranch      string `json:"defaultBranch,omitempty"`
	IsRecordRepository bool   `json:"isRecordRepository"`
	TaskRecordsRoot    string `json:"taskRecordsRoot,omitempty"`
}
type TaskRecordProjection struct {
	ID                 string `json:"id"`
	TaskID             string `json:"taskId"`
	RunID              string `json:"runId,omitempty"`
	Type               string `json:"recordType"`
	RepositoryID       string `json:"repositoryId"`
	RelativePath       string `json:"relativePath"`
	WorkingTreeHash    string `json:"workingTreeHash,omitempty"`
	CommitSHA          string `json:"commitSha,omitempty"`
	BlobSHA            string `json:"blobSha,omitempty"`
	State              string `json:"state"`
	ShortSummary       string `json:"shortSummary"`
	SupersedesRecordID string `json:"supersedesRecordId,omitempty"`
}
type CommitReferenceProjection struct {
	ID                string `json:"id"`
	TaskID            string `json:"taskId"`
	RunID             string `json:"runId,omitempty"`
	RepositoryID      string `json:"repositoryId"`
	CommitSHA         string `json:"commitSha"`
	Relation          string `json:"relation"`
	VerificationState string `json:"verificationState"`
}
type GitObservationProjection struct {
	ID            string    `json:"id"`
	RunID         string    `json:"runId"`
	RepositoryID  string    `json:"repositoryId"`
	ObservedAt    time.Time `json:"observedAt"`
	HeadCommitSHA string    `json:"headCommitSha,omitempty"`
	BranchHint    string    `json:"branchHint,omitempty"`
	WorktreeLabel string    `json:"worktreeLabel,omitempty"`
	Dirty         *bool     `json:"dirty,omitempty"`
}
type EventProjection struct {
	ID                 string          `json:"id"`
	CommandID          string          `json:"commandId"`
	EventType          string          `json:"eventType"`
	EntityType         string          `json:"entityType,omitempty"`
	EntityID           string          `json:"entityId,omitempty"`
	InitiatedByActorID string          `json:"initiatedByActorId,omitempty"`
	ExecutedByActorID  string          `json:"executedByActorId,omitempty"`
	ApprovedByActorID  string          `json:"approvedByActorId,omitempty"`
	WorkspaceRevision  int64           `json:"workspaceRevision"`
	Payload            json.RawMessage `json:"payload"`
	CreatedAt          time.Time       `json:"createdAt"`
}

type Snapshot struct {
	Workspace        WorkspaceProjection         `json:"workspace"`
	Phases           []PhaseProjection           `json:"phases"`
	Lanes            []LaneProjection            `json:"lanes"`
	Tasks            []TaskProjection            `json:"tasks"`
	Dependencies     []DependencyProjection      `json:"dependencies"`
	Gates            []GateProjection            `json:"gates"`
	Runs             []RunProjection             `json:"runs"`
	Repositories     []RepositoryProjection      `json:"repositories"`
	Records          []TaskRecordProjection      `json:"records"`
	Commits          []CommitReferenceProjection `json:"commits"`
	GitObservations  []GitObservationProjection  `json:"gitObservations"`
	HumanActorIDs    map[string]bool             `json:"-"`
	ActorIDs         map[string]bool             `json:"-"`
	NextTaskPublicID int                         `json:"-"`
}

type Diagnostic = domain.Diagnostic
type PreviewResult struct {
	CommandHash               string       `json:"commandHash"`
	ExpectedWorkspaceRevision int64        `json:"expectedWorkspaceRevision"`
	RequiredCapability        string       `json:"requiredCapability"`
	ProjectedDiff             any          `json:"projectedDiff"`
	Errors                    []Diagnostic `json:"errors"`
	Warnings                  []Diagnostic `json:"warnings"`
	Advisories                []Diagnostic `json:"advisories"`
	DecisionSnapshotHash      string       `json:"decisionSnapshotHash,omitempty"`
}

type EventWrite struct {
	Type       string
	EntityType string
	EntityID   string
	Payload    any
}
type MutationPlan struct {
	CommandName          string
	TaskID               string
	TaskStatus           string
	TaskUpdate           *domain.Task
	TaskCreate           *domain.Task
	ExpectedTaskPublicID int
	DependencyAdd        []domain.Dependency
	DependencyRemove     []domain.Dependency
	TerminalUpdates      []domain.TerminalUpdate
	LaneUpdate           *domain.Lane
	PhaseCreate          *domain.Phase
	PhaseName            string
	GateCreate           *domain.Gate
	GateName             string
	GateTaskCreate       *domain.GateTaskCondition
	GateTaskDeleteID     string
	GateCriteriaRevision int64
	ForceHumanApproval   bool
	GateTaskID           string
	GateTaskPassed       bool
	GateTaskReason       string
	GateID               string
	FromPhaseID          string
	ToPhaseID            string
	Events               []EventWrite
	Action               string
	EntityType           string
	EntityID             string
	Run                  *domain.Run
	RunUpdate            *domain.Run
	RunExpectedVersion   int64
	RunLeaseToken        string
	RunTaskStatus        string
	ExistingRunClientID  string
	Repository           *domain.Repository
	Record               *domain.TaskRecord
	CommitReference      *domain.CommitReference
	GitObservation       *domain.RunGitObservation
	NoWorkspaceRevision  bool
	IdempotentNoMutation bool
}

type ExecutionResult struct {
	CommandID         string   `json:"commandId"`
	WorkspaceRevision int64    `json:"workspaceRevision"`
	EventIDs          []string `json:"eventIds"`
	Projection        any      `json:"projection"`
	Idempotent        bool     `json:"idempotent"`
	ApprovalProtocol  string   `json:"approvalProtocol,omitempty"`
	LeaseToken        string   `json:"leaseToken,omitempty"`
}

type Repository interface {
	LoadSnapshot(context.Context, string) (Snapshot, error)
	WorkspaceIDs(context.Context) ([]string, error)
	RunLeaseToken(string) (string, error)
	Execute(context.Context, string, CommandRequest, string, func(Snapshot) (PreviewResult, MutationPlan, error)) (ExecutionResult, error)
	Task(context.Context, string, int) (TaskProjection, error)
	Events(context.Context, string) ([]EventProjection, error)
}

type CommandError struct {
	Code    string
	Message string
}

func (e *CommandError) Error() string { return e.Message }
