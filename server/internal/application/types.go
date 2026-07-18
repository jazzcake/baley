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
	ID    string `json:"id"`
	Name  string `json:"name"`
	State string `json:"state"`
}
type TaskProjection struct {
	ID                        string  `json:"id"`
	PublicID                  int     `json:"publicId"`
	LaneID                    string  `json:"laneId"`
	PhaseID                   string  `json:"phaseId"`
	Title                     string  `json:"title"`
	Description               string  `json:"description"`
	Status                    string  `json:"status"`
	BlockerReason             *string `json:"blockerReason,omitempty"`
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
	Revision      int64   `json:"revision"`
	ActivePhaseID *string `json:"activePhaseId,omitempty"`
}
type EventProjection struct {
	ID                string          `json:"id"`
	CommandID         string          `json:"commandId"`
	EventType         string          `json:"eventType"`
	WorkspaceRevision int64           `json:"workspaceRevision"`
	Payload           json.RawMessage `json:"payload"`
	CreatedAt         time.Time       `json:"createdAt"`
}

type Snapshot struct {
	Workspace     WorkspaceProjection    `json:"workspace"`
	Phases        []PhaseProjection      `json:"phases"`
	Lanes         []LaneProjection       `json:"lanes"`
	Tasks         []TaskProjection       `json:"tasks"`
	Dependencies  []DependencyProjection `json:"dependencies"`
	Gates         []GateProjection       `json:"gates"`
	HumanActorIDs map[string]bool        `json:"-"`
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
	Type    string
	Payload any
}
type MutationPlan struct {
	CommandName    string
	TaskID         string
	TaskStatus     string
	GateTaskID     string
	GateTaskPassed bool
	GateTaskReason string
	GateID         string
	FromPhaseID    string
	ToPhaseID      string
	Events         []EventWrite
	Action         string
	EntityType     string
	EntityID       string
}

type ExecutionResult struct {
	CommandID         string   `json:"commandId"`
	WorkspaceRevision int64    `json:"workspaceRevision"`
	EventIDs          []string `json:"eventIds"`
	Projection        any      `json:"projection"`
	Idempotent        bool     `json:"idempotent"`
	ApprovalProtocol  string   `json:"approvalProtocol"`
}

type Repository interface {
	LoadSnapshot(context.Context, string) (Snapshot, error)
	Execute(context.Context, string, CommandRequest, string, func(Snapshot) (PreviewResult, MutationPlan, error)) (ExecutionResult, error)
	Task(context.Context, string, int) (TaskProjection, error)
	Events(context.Context, string) ([]EventProjection, error)
}

type CommandError struct {
	Code    string
	Message string
}

func (e *CommandError) Error() string { return e.Message }
