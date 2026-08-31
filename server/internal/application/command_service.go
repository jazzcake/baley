package application

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jazzcake/baley/server/internal/domain"
)

type Service struct {
	repo Repository
	now  func() time.Time
}

func NewService(repo Repository) *Service { return &Service{repo: repo, now: time.Now} }

type workspaceMutationArgs struct {
	WorkspaceID string `json:"workspaceId"`
}
type taskConfirmArgs struct {
	WorkspaceID string `json:"workspaceId"`
	TaskID      int    `json:"taskId"`
}
type taskReportImplementedArgs struct {
	WorkspaceID string `json:"workspaceId"`
	TaskID      int    `json:"taskId"`
	Assessment  string `json:"assessment"`
}
type taskMutationArgs struct {
	WorkspaceID    string  `json:"workspaceId"`
	TaskID         int     `json:"taskId"`
	Title          *string `json:"title,omitempty"`
	Description    *string `json:"description,omitempty"`
	CurrentSummary *string `json:"currentSummary,omitempty"`
	Reason         string  `json:"reason,omitempty"`
	TargetPhaseID  string  `json:"targetPhaseId,omitempty"`
}
type taskCreateArgs struct {
	WorkspaceID             string `json:"workspaceId"`
	TaskUUID                string `json:"taskUuid"`
	LaneID                  string `json:"laneId"`
	PhaseID                 string `json:"phaseId"`
	ParentTaskID            int    `json:"parentTaskId,omitempty"`
	Title                   string `json:"title"`
	Description             string `json:"description,omitempty"`
	CurrentSummary          string `json:"currentSummary,omitempty"`
	PredecessorTaskIDs      []int  `json:"predecessorTaskIds,omitempty"`
	SuccessorTaskIDs        []int  `json:"successorTaskIds,omitempty"`
	TerminalReason          string `json:"terminalReason,omitempty"`
	RequestedAcceptanceMode string `json:"requestedAcceptanceMode,omitempty"`
	EvidenceProfileID       string `json:"evidenceProfileId,omitempty"`
}
type backlogMutationArgs struct {
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
type backlogCreateArgs struct {
	WorkspaceID string `json:"workspaceId"`
	BacklogUUID string `json:"backlogUuid"`
	LaneID      string `json:"laneId"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
}
type backlogUpdateArgs struct {
	WorkspaceID     string  `json:"workspaceId"`
	BacklogPublicID int     `json:"backlogPublicId"`
	Title           *string `json:"title,omitempty"`
	Description     *string `json:"description,omitempty"`
}
type backlogMoveArgs struct {
	WorkspaceID     string `json:"workspaceId"`
	BacklogPublicID int    `json:"backlogPublicId"`
	TargetLaneID    string `json:"targetLaneId"`
}
type backlogReorderArgs struct {
	WorkspaceID             string `json:"workspaceId"`
	LaneID                  string `json:"laneId"`
	OrderedBacklogPublicIDs []int  `json:"orderedBacklogPublicIds"`
}
type backlogDiscardArgs struct {
	WorkspaceID     string `json:"workspaceId"`
	BacklogPublicID int    `json:"backlogPublicId"`
	Reason          string `json:"reason"`
}
type backlogPromoteArgs struct {
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
type acceptancePolicyChangeArgs struct {
	WorkspaceID       string `json:"workspaceId"`
	PolicyVersion     string `json:"policyVersion"`
	DefaultMode       string `json:"defaultMode"`
	EvidenceProfileID string `json:"evidenceProfileId"`
}
type acceptanceModeEscalateArgs struct {
	WorkspaceID       string `json:"workspaceId"`
	TaskID            int    `json:"taskId"`
	AssignmentID      string `json:"assignmentId"`
	Reason            string `json:"reason"`
	EvidenceReference string `json:"evidenceReference"`
	PolicyVersion     string `json:"policyVersion"`
}
type taskEvidenceReportArgs struct {
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
}
type phaseCreateArgs struct {
	WorkspaceID string `json:"workspaceId"`
	PhaseID     string `json:"phaseId"`
	Name        string `json:"name"`
}
type dependencyRefArgs struct {
	PredecessorTaskID int `json:"predecessorTaskId"`
	SuccessorTaskID   int `json:"successorTaskId"`
}
type terminalUpdateArgs struct {
	TaskID         int     `json:"taskId"`
	TerminalReason *string `json:"terminalReason"`
}
type dependencyMutationArgs struct {
	WorkspaceID       string               `json:"workspaceId"`
	PredecessorTaskID int                  `json:"predecessorTaskId,omitempty"`
	SuccessorTaskID   int                  `json:"successorTaskId,omitempty"`
	Remove            []dependencyRefArgs  `json:"remove,omitempty"`
	Add               []dependencyRefArgs  `json:"add,omitempty"`
	TerminalUpdates   []terminalUpdateArgs `json:"terminalUpdates,omitempty"`
}
type laneMutationArgs struct {
	WorkspaceID string `json:"workspaceId"`
	LaneID      string `json:"laneId"`
	Name        string `json:"name,omitempty"`
	Goal        string `json:"goal,omitempty"`
	Summary     string `json:"summary,omitempty"`
	Reason      string `json:"reason,omitempty"`
}
type gateMutationArgs struct {
	WorkspaceID   string `json:"workspaceId"`
	GateID        string `json:"gateId"`
	Alias         string `json:"alias,omitempty"`
	Name          string `json:"name,omitempty"`
	FromPhaseID   string `json:"fromPhaseId,omitempty"`
	ToPhaseID     string `json:"toPhaseId,omitempty"`
	TaskID        int    `json:"taskId,omitempty"`
	ClearTerminal bool   `json:"clearTerminal,omitempty"`
}

func (v gateMutationArgs) TaskIDString() string { return fmt.Sprint(v.TaskID) }

type gateTaskArgs struct {
	WorkspaceID string `json:"workspaceId"`
	GateTaskID  string `json:"gateTaskId"`
	Reason      string `json:"reason"`
}
type gatePassArgs struct {
	WorkspaceID string `json:"workspaceId"`
	GateID      string `json:"gateId"`
}
type runStartArgs struct {
	WorkspaceID string         `json:"workspaceId"`
	TaskID      int            `json:"taskId"`
	ClientRunID string         `json:"clientRunId"`
	Kind        domain.RunKind `json:"kind"`
	SessionRef  string         `json:"sessionRef,omitempty"`
	ParentRunID string         `json:"parentRunId,omitempty"`
	TargetRunID string         `json:"targetRunId,omitempty"`
}
type runHeartbeatArgs struct {
	WorkspaceID        string `json:"workspaceId"`
	RunID              string `json:"runId"`
	LeaseToken         string `json:"leaseToken"`
	ExpectedRunVersion int64  `json:"expectedRunVersion"`
	ExtensionSeconds   int64  `json:"extensionSeconds,omitempty"`
}
type runTerminalArgs struct {
	WorkspaceID        string `json:"workspaceId"`
	RunID              string `json:"runId"`
	ExpectedRunVersion int64  `json:"expectedRunVersion"`
	Summary            string `json:"summary"`
}
type runCorrectArgs struct {
	WorkspaceID        string           `json:"workspaceId"`
	RunID              string           `json:"runId"`
	ExpectedRunVersion int64            `json:"expectedRunVersion"`
	Status             domain.RunStatus `json:"status"`
	Summary            string           `json:"summary"`
	Reason             string           `json:"reason"`
}
type repositoryRegisterArgs struct {
	WorkspaceID        string `json:"workspaceId"`
	RepositoryID       string `json:"repositoryId"`
	Name               string `json:"name"`
	RemoteURL          string `json:"remoteUrl"`
	DefaultBranch      string `json:"defaultBranch,omitempty"`
	IsRecordRepository bool   `json:"isRecordRepository"`
	TaskRecordsRoot    string `json:"taskRecordsRoot,omitempty"`
}
type recordRegisterArgs struct {
	WorkspaceID        string            `json:"workspaceId"`
	RecordID           string            `json:"recordId"`
	TaskID             int               `json:"taskId"`
	RunID              string            `json:"runId,omitempty"`
	RecordType         domain.RecordType `json:"recordType"`
	RepositoryID       string            `json:"repositoryId"`
	RelativePath       string            `json:"relativePath"`
	WorkingTreeHash    string            `json:"workingTreeHash,omitempty"`
	ShortSummary       string            `json:"shortSummary"`
	SupersedesRecordID string            `json:"supersedesRecordId,omitempty"`
}
type recordAttachCommitArgs struct {
	WorkspaceID string `json:"workspaceId"`
	RecordID    string `json:"recordId"`
	CommitSHA   string `json:"commitSha"`
	BlobSHA     string `json:"blobSha"`
}
type commitAttachArgs struct {
	WorkspaceID  string                `json:"workspaceId"`
	CommitID     string                `json:"commitId"`
	TaskID       int                   `json:"taskId"`
	RunID        string                `json:"runId,omitempty"`
	RepositoryID string                `json:"repositoryId"`
	CommitSHA    string                `json:"commitSha"`
	Relation     domain.CommitRelation `json:"relation"`
}
type gitObserveArgs struct {
	WorkspaceID   string    `json:"workspaceId"`
	ObservationID string    `json:"observationId"`
	RunID         string    `json:"runId"`
	RepositoryID  string    `json:"repositoryId"`
	ObservedAt    time.Time `json:"observedAt"`
	HeadCommitSHA string    `json:"headCommitSha,omitempty"`
	BranchHint    string    `json:"branchHint,omitempty"`
	WorktreeLabel string    `json:"worktreeLabel,omitempty"`
	Dirty         *bool     `json:"dirty,omitempty"`
}

func (s *Service) Preview(ctx context.Context, request CommandRequest) (PreviewResult, error) {
	wid, typed, err := decodeArguments(request.Name, request.Arguments)
	if err != nil {
		return PreviewResult{}, err
	}
	snapshot, err := s.repo.LoadSnapshot(ctx, wid)
	if err != nil {
		return PreviewResult{}, err
	}
	if request.Envelope.ExpectedWorkspaceRevision == 0 {
		request.Envelope.ExpectedWorkspaceRevision = snapshot.Workspace.Revision
	}
	result, _, err := s.evaluate(ctx, request, typed, snapshot, false)
	return result, err
}

func (s *Service) Execute(ctx context.Context, request CommandRequest) (result ExecutionResult, err error) {
	started := s.now()
	attemptID := randomUUID()
	request.Envelope.AttemptID = attemptID
	wid := extractWorkspaceID(request.Arguments)
	fingerprint := ""
	defer func() {
		recorder, ok := s.repo.(MutationAttemptRecorder)
		if !ok || wid == "" || attemptID == "" || request.Principal != nil && !request.TenantAuditAuthorized {
			return
		}
		outcome := "succeeded"
		codes := []string{}
		if result.Idempotent {
			outcome = "idempotent"
		} else if err != nil {
			outcome = "failed"
			if commandErr, isCommandErr := err.(*CommandError); isCommandErr {
				outcome = "rejected"
				codes = append(codes, commandErr.Code)
			}
		}
		idempotencyHash := sha256.Sum256([]byte(request.Envelope.IdempotencyKey))
		argumentHash := sha256.Sum256(request.Arguments)
		entityType, entityID := extractAttemptTarget(request.Arguments)
		attempt := MutationAttemptProjection{
			ID: attemptID, WorkspaceID: wid, CommandName: request.Name, Source: "command_service",
			Outcome: outcome, EntityType: entityType, EntityID: entityID,
			InitiatedByActorID: request.Envelope.InitiatedByActorID,
			ExecutedByActorID:  request.Envelope.ExecutedByActorID,
			IdempotencyKeyHash: hex.EncodeToString(idempotencyHash[:]), ArgumentDigest: hex.EncodeToString(argumentHash[:]),
			RequestFingerprint: fingerprint, CommandID: result.CommandID, EventIDs: result.EventIDs,
			CommandHash:               result.CommandHash,
			ExpectedWorkspaceRevision: request.Envelope.ExpectedWorkspaceRevision,
			ObservedWorkspaceRevision: result.WorkspaceRevision, DiagnosticCodes: codes,
			DurationMS: s.now().Sub(started).Milliseconds(), OccurredAt: s.now().UTC(),
		}
		if recordErr := recorder.RecordMutationAttempt(context.WithoutCancel(ctx), attempt); recordErr != nil {
			fmt.Printf("mutation attempt audit failed: %v\n", recordErr)
		}
	}()
	var typed any
	wid, typed, err = decodeArguments(request.Name, request.Arguments)
	if err != nil {
		return ExecutionResult{}, err
	}
	if request.Principal != nil {
		authenticatedActor := request.Principal.Subject.ActorID
		if authenticatedActor == "" || request.Envelope.ExecutedByActorID != "" && request.Envelope.ExecutedByActorID != authenticatedActor {
			return ExecutionResult{}, &CommandError{Code: "forbidden", Message: "command Actor does not match authenticated principal"}
		}
		request.Envelope.ExecutedByActorID = authenticatedActor
		request.Envelope.InitiatedByActorID = authenticatedActor
		if request.Envelope.HumanApprovalAttestation != nil {
			return ExecutionResult{}, &CommandError{Code: domain.CodeHumanApprovalMismatch, Message: "legacy human approval fields are not authority in enforced mode"}
		}
	}
	workspaceRevisionRequired := request.Name != "run.heartbeat"
	if strings.TrimSpace(request.Envelope.IdempotencyKey) == "" || strings.TrimSpace(request.Envelope.ExecutedByActorID) == "" || workspaceRevisionRequired && request.Envelope.ExpectedWorkspaceRevision == 0 {
		return ExecutionResult{}, &CommandError{Code: "invalid_request", Message: "execute requires idempotencyKey, executedByActorId and expectedWorkspaceRevision"}
	}
	// Hash is finalized in the locked callback because Gate commands bind the current condition snapshot.
	fingerprint = hashRequestFingerprint(request.Name, typed, request.Envelope.ExpectedWorkspaceRevision)
	if request.Principal != nil {
		securityDigest := sha256.Sum256([]byte(strings.Join([]string{
			request.Principal.Subject.ActorID,
			request.Principal.CredentialID,
			request.Principal.SessionID,
			strings.TrimSpace(request.Envelope.ApprovalGrantID),
			strings.Join(request.Envelope.AcknowledgedWarningCodes, "\x1f"),
			strings.TrimSpace(request.Envelope.ProceedReason),
		}, "\x00")))
		fingerprint += ":" + hex.EncodeToString(securityDigest[:])
	}
	return s.repo.Execute(ctx, wid, request, fingerprint, func(snapshot Snapshot) (PreviewResult, MutationPlan, error) {
		return s.evaluate(ctx, request, typed, snapshot, true)
	})
}

func extractWorkspaceID(arguments json.RawMessage) string {
	var value struct {
		WorkspaceID string `json:"workspaceId"`
	}
	_ = json.Unmarshal(arguments, &value)
	return strings.TrimSpace(value.WorkspaceID)
}

func extractAttemptTarget(arguments json.RawMessage) (string, string) {
	var values map[string]any
	if json.Unmarshal(arguments, &values) != nil {
		return "", ""
	}
	for _, candidate := range []struct {
		key, kind string
	}{
		{"taskUuid", "task"}, {"taskId", "task"}, {"backlogUuid", "backlog_item"},
		{"backlogPublicId", "backlog_item"}, {"gateId", "gate"}, {"laneId", "lane"},
		{"phaseId", "phase"}, {"runId", "run"}, {"recordId", "task_record"},
		{"repositoryId", "repository"}, {"commitId", "commit"}, {"observationId", "git_observation"},
	} {
		if value, ok := values[candidate.key]; ok {
			switch typed := value.(type) {
			case string:
				if strings.TrimSpace(typed) != "" {
					return candidate.kind, strings.TrimSpace(typed)
				}
			case float64:
				return candidate.kind, fmt.Sprintf("%.0f", typed)
			}
		}
	}
	return "", ""
}

// InterruptExpiredRuns performs one deterministic sweep. The command path is
// reused so timeout interruption has the same CAS, Event and idempotency rules
// as an explicit Run terminal transition.
func (s *Service) InterruptExpiredRuns(ctx context.Context) ([]ExecutionResult, error) {
	workspaceIDs, err := s.repo.WorkspaceIDs(ctx)
	if err != nil {
		return nil, err
	}
	results := []ExecutionResult{}
	for _, workspaceID := range workspaceIDs {
		for attempts := 0; attempts < 128; attempts++ {
			snapshot, loadErr := s.repo.LoadSnapshot(ctx, workspaceID)
			if loadErr != nil {
				return results, loadErr
			}
			var expired *RunProjection
			now := s.now().UTC()
			for index := range snapshot.Runs {
				candidate := snapshot.Runs[index]
				if candidate.Status == string(domain.RunRunning) && !now.Before(candidate.LeaseExpiresAt) {
					expired = &candidate
					break
				}
			}
			if expired == nil {
				break
			}
			arguments, _ := json.Marshal(runTerminalArgs{WorkspaceID: workspaceID, RunID: expired.ID, ExpectedRunVersion: expired.Version, Summary: "Run lease expired"})
			internalCommandID := randomUUID()
			if internalCommandID == "" {
				return results, fmt.Errorf("secure timeout command identity generation failed")
			}
			request := CommandRequest{Name: "run.interrupt", Arguments: arguments, Envelope: CommandEnvelope{IdempotencyKey: "run-timeout:" + internalCommandID, ExpectedWorkspaceRevision: snapshot.Workspace.Revision, ExecutedByActorID: expired.OperatorActorID}}
			result, executeErr := s.Execute(ctx, request)
			if executeErr == nil {
				results = append(results, result)
				continue
			}
			if commandErr, ok := executeErr.(*CommandError); ok && (commandErr.Code == domain.CodeStaleRevision || commandErr.Code == domain.CodeStaleRunVersion || commandErr.Code == domain.CodeInvalidStateTransition || commandErr.Code == domain.CodeIdempotencyConflict) {
				continue
			}
			return results, executeErr
		}
	}
	return results, nil
}

func (s *Service) evaluate(ctx context.Context, request CommandRequest, typed any, snapshot Snapshot, executing bool) (PreviewResult, MutationPlan, error) {
	result := PreviewResult{ExpectedWorkspaceRevision: request.Envelope.ExpectedWorkspaceRevision, Errors: []Diagnostic{}, Warnings: []Diagnostic{}, Advisories: []Diagnostic{}}
	plan := MutationPlan{CommandName: request.Name, Action: request.Name}
	if request.Name != "run.heartbeat" && request.Envelope.ExpectedWorkspaceRevision != snapshot.Workspace.Revision {
		result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeStaleRevision, EntityID: snapshot.Workspace.ID})
		if executing {
			return result, plan, &CommandError{Code: domain.CodeStaleRevision, Message: "command evaluation failed: " + domain.CodeStaleRevision}
		}
	}
	if executing && (!snapshot.ActorIDs[request.Envelope.ExecutedByActorID] || request.Envelope.InitiatedByActorID != "" && !snapshot.ActorIDs[request.Envelope.InitiatedByActorID]) {
		result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeNotFound, EntityID: request.Envelope.ExecutedByActorID})
	}
	var decisionHash string
	switch args := typed.(type) {
	case workspaceMutationArgs:
		result.RequiredCapability = "workspace:close"
		plan.EntityType, plan.EntityID = "workspace", args.WorkspaceID
		var activePhase *PhaseProjection
		phases := make([]domain.Phase, 0, len(snapshot.Phases))
		for index := range snapshot.Phases {
			phases = append(phases, domainPhase(snapshot.Phases[index], args.WorkspaceID))
			if snapshot.Workspace.ActivePhaseID != nil && snapshot.Phases[index].ID == *snapshot.Workspace.ActivePhaseID {
				activePhase = &snapshot.Phases[index]
			}
		}
		if activePhase == nil {
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeInvalidStateTransition, EntityID: args.WorkspaceID})
			break
		}
		tasks := make([]domain.Task, 0, len(snapshot.Tasks))
		for _, value := range snapshot.Tasks {
			tasks = append(tasks, domainTask(value, args.WorkspaceID))
		}
		lanes := make([]domain.Lane, 0, len(snapshot.Lanes))
		for _, value := range snapshot.Lanes {
			lanes = append(lanes, domainLane(value, args.WorkspaceID))
		}
		activeRunCount := 0
		for _, value := range snapshot.Runs {
			if value.Status == string(domain.RunRunning) {
				activeRunCount++
			}
		}
		domainPlan := domain.PlanMutation("workspace.close", domain.MutationContext{
			Mode: domain.MutationPreview, Workspace: domainWorkspace(snapshot),
			Phase: domainPhase(*activePhase, args.WorkspaceID), Phases: phases,
			Tasks: tasks, Lanes: lanes, BacklogItems: domainBacklogItems(snapshot.BacklogItems, args.WorkspaceID),
			ActiveRunCount: activeRunCount,
		})
		result.Errors = append(result.Errors, domainPlan.Evaluation.Errors...)
		result.Warnings = append(result.Warnings, domainPlan.Evaluation.Warnings...)
		result.ProjectedDiff = domainPlan.ProjectedDiff
		if executing && !domainPlan.Evaluation.HasErrors() && !sameDiagnosticCodes(domainPlan.Evaluation.Warnings, request.Envelope.AcknowledgedWarningCodes) {
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeInvalidStateTransition, EntityID: "warnings"})
		}
		if len(result.Errors) > 0 {
			break
		}
		closedPhase := domainPhase(*activePhase, args.WorkspaceID)
		closedPhase.State = domain.PhaseCompleted
		plan.WorkspaceState, plan.PhaseUpdate = domain.WorkspaceClosed, &closedPhase
		for _, event := range domainPlan.Events {
			payload := event.Payload
			if event.Type == "workspace.closed" && len(domainPlan.Evaluation.Warnings) > 0 {
				payload["warnings"] = request.Envelope.AcknowledgedWarningCodes
				payload["acknowledgedWarningCodes"] = request.Envelope.AcknowledgedWarningCodes
				payload["proceedReason"] = strings.TrimSpace(request.Envelope.ProceedReason)
			}
			plan.Events = append(plan.Events, EventWrite{Type: event.Type, EntityType: event.EntityType, EntityID: event.EntityID, Payload: payload})
		}
	case phaseCreateArgs:
		result.RequiredCapability = "workspace:operate"
		plan.EntityType, plan.EntityID = "phase", args.PhaseID
		position := 1
		existing := make([]domain.Phase, 0, len(snapshot.Phases))
		for _, projected := range snapshot.Phases {
			existing = append(existing, domainPhase(projected, args.WorkspaceID))
			if projected.Position >= position {
				position = projected.Position + 1
			}
		}
		phase := domain.Phase{ID: args.PhaseID, WorkspaceID: args.WorkspaceID, Position: position, State: domain.PhasePlanned}
		domainPlan := domain.PlanPhaseCreate(domainWorkspace(snapshot), existing, phase)
		if strings.TrimSpace(args.Name) == "" {
			domainPlan.Evaluation.Errors = append(domainPlan.Evaluation.Errors, Diagnostic{Code: domain.CodeInvalidStateTransition, EntityID: args.PhaseID})
		}
		result.Errors = append(result.Errors, domainPlan.Evaluation.Errors...)
		result.ProjectedDiff = domainPlan.ProjectedDiff
		if len(result.Errors) > 0 {
			break
		}
		plan.PhaseCreate, plan.PhaseName = &phase, strings.TrimSpace(args.Name)
		for _, event := range domainPlan.Events {
			plan.Events = append(plan.Events, EventWrite{Type: event.Type, EntityType: event.EntityType, EntityID: event.EntityID, Payload: event.Payload})
		}
	case taskCreateArgs:
		result.RequiredCapability = "workspace:operate"
		plan.EntityType, plan.EntityID = "task", args.TaskUUID
		if !isUUID(args.TaskUUID) {
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeInvalidStateTransition, EntityID: args.TaskUUID})
			break
		}
		var lane *LaneProjection
		for index := range snapshot.Lanes {
			if snapshot.Lanes[index].ID == args.LaneID {
				lane = &snapshot.Lanes[index]
				break
			}
		}
		phase := findPhase(snapshot.Phases, args.PhaseID)
		if lane == nil || phase == nil {
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeNotFound, EntityID: args.TaskUUID})
			break
		}
		publicID := snapshot.NextTaskPublicID
		if publicID <= 0 {
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeInvalidStateTransition, EntityID: "workspace_counter"})
			break
		}
		parentID := ""
		if args.ParentTaskID != 0 {
			parent := findTaskByPublicID(snapshot.Tasks, args.ParentTaskID)
			if parent == nil {
				result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeNotFound, EntityID: fmt.Sprint(args.ParentTaskID)})
				break
			}
			parentID = parent.ID
		}
		task := domain.Task{ID: args.TaskUUID, PublicID: publicID, WorkspaceID: args.WorkspaceID, LaneID: args.LaneID, PhaseID: args.PhaseID, ParentTaskID: parentID, Title: args.Title, Description: args.Description, CurrentSummary: strings.TrimSpace(args.CurrentSummary), Status: domain.TaskPending, TerminalReason: strings.TrimSpace(args.TerminalReason), PhasePosition: phase.Position}
		graph, graphEvaluation := workspaceGraph(snapshot)
		result.Errors = append(result.Errors, graphEvaluation.Errors...)
		if graphEvaluation.HasErrors() {
			break
		}
		patch := domain.DependencyPatch{}
		for _, predecessorID := range args.PredecessorTaskIDs {
			predecessor := findTaskByPublicID(snapshot.Tasks, predecessorID)
			if predecessor == nil {
				result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeNotFound, EntityID: fmt.Sprint(predecessorID)})
				break
			}
			patch.Add = append(patch.Add, domain.Dependency{FromTaskID: predecessor.ID, ToTaskID: task.ID})
		}
		for _, successorID := range args.SuccessorTaskIDs {
			successor := findTaskByPublicID(snapshot.Tasks, successorID)
			if successor == nil {
				result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeNotFound, EntityID: fmt.Sprint(successorID)})
				break
			}
			patch.Add = append(patch.Add, domain.Dependency{FromTaskID: task.ID, ToTaskID: successor.ID})
		}
		if len(result.Errors) > 0 {
			break
		}
		patch = graph.InsertTaskIntoRoutes(task.ID, patch)
		domainPlan := domain.PlanTaskCreate(domainWorkspace(snapshot), domain.Lane{ID: lane.ID, WorkspaceID: args.WorkspaceID, Name: lane.Name, Goal: lane.Goal, Summary: lane.Summary, State: domain.LaneState(lane.State)}, domainPhase(*phase, args.WorkspaceID), graph, task, patch)
		result.Errors = append(result.Errors, domainPlan.Evaluation.Errors...)
		result.Warnings = append(result.Warnings, domainPlan.Evaluation.Warnings...)
		result.ProjectedDiff = domainPlan.ProjectedDiff
		if executing && !domainPlan.Evaluation.HasErrors() && !sameDiagnosticCodes(domainPlan.Evaluation.Warnings, request.Envelope.AcknowledgedWarningCodes) {
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeInvalidStateTransition, EntityID: "warnings"})
		}
		if domainPlan.Evaluation.HasErrors() {
			break
		}
		assignment, assignmentErr := resolveTaskAcceptance(snapshot, task.ID, args.RequestedAcceptanceMode, args.EvidenceProfileID, executing)
		if assignmentErr != nil {
			result.Errors = append(result.Errors, Diagnostic{Code: violationCode(assignmentErr), EntityID: task.ID})
			break
		}
		plan.TaskCreate, plan.ExpectedTaskPublicID, plan.DependencyAdd, plan.DependencyRemove = &task, publicID, patch.Add, patch.Remove
		plan.AcceptanceAssignment = &assignment
		result.ProjectedDiff = map[string]any{"task": domainPlan.ProjectedDiff, "acceptanceAssignment": assignment}
		for _, event := range domainPlan.Events {
			if len(domainPlan.Evaluation.Warnings) > 0 {
				event.Payload["warnings"] = request.Envelope.AcknowledgedWarningCodes
				event.Payload["acknowledgedWarningCodes"] = request.Envelope.AcknowledgedWarningCodes
				event.Payload["proceedReason"] = strings.TrimSpace(request.Envelope.ProceedReason)
			}
			plan.Events = append(plan.Events, EventWrite{Type: event.Type, EntityType: event.EntityType, EntityID: event.EntityID, Payload: event.Payload})
		}
	case backlogMutationArgs:
		result.RequiredCapability = "workspace:operate"
		if snapshot.Workspace.State != string(domain.WorkspaceActive) {
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeInvalidStateTransition, EntityID: snapshot.Workspace.ID})
			break
		}
		item := findBacklogByPublicID(snapshot.BacklogItems, args.BacklogPublicID)
		lane := findLane(snapshot.Lanes, args.LaneID)
		if request.Name == "backlog.create" {
			if !isUUID(args.BacklogUUID) || lane == nil || args.Title == nil || snapshot.NextBacklogPublicID <= 0 {
				result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeInvalidStateTransition, EntityID: args.BacklogUUID})
				break
			}
			position := 1
			for _, existing := range snapshot.BacklogItems {
				if existing.Status == string(domain.BacklogActive) && existing.LaneID == args.LaneID && existing.Position != nil && *existing.Position >= position {
					position = *existing.Position + 1
				}
			}
			created, createErr := domain.NewBacklogItem(domain.BacklogItem{ID: args.BacklogUUID, WorkspaceID: args.WorkspaceID, PublicID: snapshot.NextBacklogPublicID, LaneID: args.LaneID, Title: *args.Title, Description: stringPointerValue(args.Description), Status: domain.BacklogActive}, domainLane(*lane, args.WorkspaceID), position)
			if createErr != nil {
				result.Errors = append(result.Errors, Diagnostic{Code: violationCode(createErr), EntityID: args.BacklogUUID})
				break
			}
			plan.EntityType, plan.EntityID = "backlog_item", fmt.Sprint(created.PublicID)
			plan.BacklogCreate, plan.ExpectedBacklogPublicID = &created, created.PublicID
			result.ProjectedDiff = backlogProjection(created)
			plan.Events = []EventWrite{{Type: "backlog.created", EntityType: "backlog_item", EntityID: plan.EntityID, Payload: map[string]any{"backlogPublicId": created.PublicID, "laneId": created.LaneID, "position": position}}}
			break
		}
		if request.Name == "backlog.reorder" {
			lane = findLane(snapshot.Lanes, args.LaneID)
			if lane == nil {
				result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeNotFound, EntityID: args.LaneID})
				break
			}
			values, reorderErr := domain.ReorderBacklog(domainBacklogItems(snapshot.BacklogItems, args.WorkspaceID), args.LaneID, args.OrderedBacklogPublicIDs)
			if reorderErr != nil {
				result.Errors = append(result.Errors, Diagnostic{Code: violationCode(reorderErr), EntityID: args.LaneID})
				break
			}
			plan.EntityType, plan.EntityID, plan.BacklogPositions = "backlog_item", args.LaneID, values
			result.ProjectedDiff = map[string]any{"laneId": args.LaneID, "orderedBacklogPublicIds": args.OrderedBacklogPublicIDs}
			plan.Events = []EventWrite{{Type: "backlog.reordered", EntityType: "backlog_item", EntityID: args.LaneID, Payload: result.ProjectedDiff}}
			break
		}
		if item == nil {
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeNotFound, EntityID: fmt.Sprint(args.BacklogPublicID)})
			break
		}
		current := domainBacklog(*item, args.WorkspaceID)
		plan.EntityType, plan.EntityID = "backlog_item", fmt.Sprint(item.PublicID)
		switch request.Name {
		case "backlog.update":
			next, updateErr := current.Update(args.Title, args.Description)
			if updateErr != nil {
				result.Errors = append(result.Errors, Diagnostic{Code: violationCode(updateErr), EntityID: plan.EntityID})
				break
			}
			plan.BacklogUpdate = &next
			result.ProjectedDiff = map[string]any{"before": item, "after": backlogProjection(next)}
			plan.Events = []EventWrite{{Type: "backlog.updated", EntityType: "backlog_item", EntityID: plan.EntityID, Payload: map[string]any{"backlogPublicId": item.PublicID}}}
		case "backlog.move":
			target := findLane(snapshot.Lanes, args.TargetLaneID)
			if current.Status != domain.BacklogActive || target == nil || target.State != string(domain.LaneActive) {
				result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeInvalidStateTransition, EntityID: plan.EntityID})
				break
			}
			if current.LaneID == args.TargetLaneID {
				result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeBacklogLaneUnchanged, EntityID: plan.EntityID})
				break
			}
			source := current.LaneID
			nextPosition := 1
			for _, existing := range snapshot.BacklogItems {
				if existing.Status == string(domain.BacklogActive) && existing.LaneID == args.TargetLaneID && existing.Position != nil && *existing.Position >= nextPosition {
					nextPosition = *existing.Position + 1
				}
			}
			current.LaneID = args.TargetLaneID
			current.Position = &nextPosition
			values := replaceDomainBacklog(snapshot.BacklogItems, current, args.WorkspaceID)
			values = domain.CompactBacklog(values, source)
			plan.BacklogUpdate, plan.BacklogPositions = &current, values
			result.ProjectedDiff = map[string]any{"backlogPublicId": current.PublicID, "sourceLaneId": source, "targetLaneId": current.LaneID, "position": nextPosition}
			plan.Events = []EventWrite{{Type: "backlog.moved", EntityType: "backlog_item", EntityID: plan.EntityID, Payload: result.ProjectedDiff}}
		case "backlog.discard":
			next, discardErr := current.Discard(args.Reason)
			if discardErr != nil {
				result.Errors = append(result.Errors, Diagnostic{Code: violationCode(discardErr), EntityID: plan.EntityID})
				break
			}
			values := replaceDomainBacklog(snapshot.BacklogItems, next, args.WorkspaceID)
			values = domain.CompactBacklog(values, current.LaneID)
			plan.BacklogUpdate, plan.BacklogPositions = &next, values
			result.ProjectedDiff = map[string]any{"backlogPublicId": item.PublicID, "reason": next.DiscardReason}
			plan.Events = []EventWrite{{Type: "backlog.discarded", EntityType: "backlog_item", EntityID: plan.EntityID, Payload: result.ProjectedDiff}}
		case "backlog.promote":
			phase := findPhase(snapshot.Phases, args.PhaseID)
			lane = findLane(snapshot.Lanes, current.LaneID)
			if !isUUID(args.TaskUUID) || phase == nil || lane == nil || current.Status != domain.BacklogActive {
				result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeInvalidStateTransition, EntityID: plan.EntityID})
				break
			}
			parentID := ""
			if args.ParentTaskID != 0 {
				parent := findTaskByPublicID(snapshot.Tasks, args.ParentTaskID)
				if parent == nil {
					result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeNotFound, EntityID: fmt.Sprint(args.ParentTaskID)})
					break
				}
				parentID = parent.ID
			}
			task := domain.Task{ID: args.TaskUUID, PublicID: snapshot.NextTaskPublicID, WorkspaceID: args.WorkspaceID, LaneID: current.LaneID, PhaseID: args.PhaseID, ParentTaskID: parentID, Title: current.Title, Description: current.Description, Status: domain.TaskPending, TerminalReason: strings.TrimSpace(args.TerminalReason), PhasePosition: phase.Position}
			graph, graphEvaluation := workspaceGraph(snapshot)
			result.Errors = append(result.Errors, graphEvaluation.Errors...)
			patch := domain.DependencyPatch{}
			for _, publicID := range args.PredecessorTaskIDs {
				predecessor := findTaskByPublicID(snapshot.Tasks, publicID)
				if predecessor == nil {
					result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeNotFound, EntityID: fmt.Sprint(publicID)})
					break
				}
				patch.Add = append(patch.Add, domain.Dependency{FromTaskID: predecessor.ID, ToTaskID: task.ID})
			}
			for _, publicID := range args.SuccessorTaskIDs {
				successor := findTaskByPublicID(snapshot.Tasks, publicID)
				if successor == nil {
					result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeNotFound, EntityID: fmt.Sprint(publicID)})
					break
				}
				patch.Add = append(patch.Add, domain.Dependency{FromTaskID: task.ID, ToTaskID: successor.ID})
			}
			if len(result.Errors) > 0 {
				break
			}
			patch = graph.InsertTaskIntoRoutes(task.ID, patch)
			taskPlan := domain.PlanTaskCreate(domainWorkspace(snapshot), domainLane(*lane, args.WorkspaceID), domainPhase(*phase, args.WorkspaceID), graph, task, patch)
			result.Errors = append(result.Errors, taskPlan.Evaluation.Errors...)
			result.Warnings = append(result.Warnings, taskPlan.Evaluation.Warnings...)
			if executing && !taskPlan.Evaluation.HasErrors() && !sameDiagnosticCodes(taskPlan.Evaluation.Warnings, request.Envelope.AcknowledgedWarningCodes) {
				result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeInvalidStateTransition, EntityID: "warnings"})
			}
			if len(result.Errors) > 0 {
				break
			}
			assignment, assignmentErr := resolveTaskAcceptance(snapshot, task.ID, args.RequestedAcceptanceMode, args.EvidenceProfileID, executing)
			if assignmentErr != nil {
				result.Errors = append(result.Errors, Diagnostic{Code: violationCode(assignmentErr), EntityID: task.ID})
				break
			}
			promoted, promoteErr := current.Promote(task.ID)
			if promoteErr != nil {
				result.Errors = append(result.Errors, Diagnostic{Code: violationCode(promoteErr), EntityID: plan.EntityID})
				break
			}
			values := domain.CompactBacklog(replaceDomainBacklog(snapshot.BacklogItems, promoted, args.WorkspaceID), current.LaneID)
			plan.TaskCreate, plan.ExpectedTaskPublicID, plan.DependencyAdd, plan.DependencyRemove = &task, task.PublicID, patch.Add, patch.Remove
			plan.AcceptanceAssignment = &assignment
			plan.BacklogUpdate, plan.BacklogPositions = &promoted, values
			result.ProjectedDiff = map[string]any{"backlogPublicId": current.PublicID, "laneId": current.LaneID, "phaseId": args.PhaseID, "taskId": task.ID, "taskPublicId": task.PublicID, "relations": taskPlan.ProjectedDiff, "acceptanceAssignment": assignment}
			payload := result.ProjectedDiff.(map[string]any)
			if len(taskPlan.Evaluation.Warnings) > 0 {
				payload["warnings"], payload["acknowledgedWarningCodes"], payload["proceedReason"] = request.Envelope.AcknowledgedWarningCodes, request.Envelope.AcknowledgedWarningCodes, strings.TrimSpace(request.Envelope.ProceedReason)
			}
			plan.Events = []EventWrite{
				{Type: "backlog.promoted", EntityType: "backlog_item", EntityID: plan.EntityID, Payload: payload},
				{Type: "task.created", EntityType: "task", EntityID: task.ID, Payload: map[string]any{"task": task, "relations": taskPlan.ProjectedDiff}},
			}
		}
	case gateMutationArgs:
		plan.EntityType, plan.EntityID = "gate", args.GateID
		if request.Name == "gate.create" {
			result.RequiredCapability = "workspace:operate"
			from, to := findPhase(snapshot.Phases, args.FromPhaseID), findPhase(snapshot.Phases, args.ToPhaseID)
			alias, aliasErr := domain.NormalizeGateAlias(args.Alias)
			if from == nil || to == nil {
				result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeNotFound, EntityID: args.GateID})
				break
			}
			if snapshot.NextGatePublicID <= 0 || domain.IsReservedGatePublicReference(args.GateID) || FindGateByReference(snapshot.Gates, args.GateID) != nil || strings.TrimSpace(args.Name) == "" || aliasErr != nil {
				result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeInvalidStateTransition, EntityID: args.GateID})
				break
			}
			gate := domain.Gate{ID: args.GateID, PublicID: snapshot.NextGatePublicID, Alias: alias, WorkspaceID: args.WorkspaceID, FromPhaseID: from.ID, ToPhaseID: to.ID}
			existing := make([]domain.Gate, 0, len(snapshot.Gates))
			for _, value := range snapshot.Gates {
				existing = append(existing, domainGate(value, args.WorkspaceID))
			}
			domainPlan := domain.PlanGateCreate(domainWorkspace(snapshot), gate, domainPhase(*from, args.WorkspaceID), domainPhase(*to, args.WorkspaceID), existing)
			result.Errors = append(result.Errors, domainPlan.Evaluation.Errors...)
			result.ProjectedDiff = domainPlan.ProjectedDiff
			if domainPlan.Evaluation.HasErrors() {
				break
			}
			plan.GateCreate, plan.ExpectedGatePublicID, plan.GateName = &gate, gate.PublicID, strings.TrimSpace(args.Name)
			for _, event := range domainPlan.Events {
				plan.Events = append(plan.Events, EventWrite{Type: event.Type, EntityType: event.EntityType, EntityID: event.EntityID, Payload: event.Payload})
			}
			break
		}
		gate := FindGateByReference(snapshot.Gates, args.GateID)
		task := findTaskByPublicID(snapshot.Tasks, args.TaskID)
		if gate == nil || task == nil {
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeNotFound, EntityID: args.GateID})
			break
		}
		plan.EntityID = gate.ID
		from := findPhase(snapshot.Phases, gate.FromPhaseID)
		to := findPhase(snapshot.Phases, gate.ToPhaseID)
		if from == nil || to == nil {
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeNotFound, EntityID: gate.FromPhaseID})
			break
		}
		if request.Name == "gate.attach_entry_task" || request.Name == "gate.detach_entry_task" {
			if task.PhaseID != gate.ToPhaseID || gate.PassedAt != nil {
				result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeInvalidStateTransition, EntityID: args.TaskIDString()})
				break
			}
			attached := false
			for _, entry := range gate.EntryTasks {
				attached = attached || (entry.TaskID == task.ID && entry.SelectionSource == "explicit")
			}
			attach := request.Name == "gate.attach_entry_task"
			if attach == attached {
				result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeInvalidStateTransition, EntityID: task.ID})
				break
			}
			result.RequiredCapability = "workspace:operate"
			plan.GateID = gate.ID
			plan.GateCriteriaRevision = gate.CriteriaRevision + 1
			entry := domain.GateEntryTask{WorkspaceID: args.WorkspaceID, GateID: gate.ID, TaskID: task.ID, SelectionSource: "explicit"}
			if attach {
				plan.GateEntryTaskCreate = &entry
			} else {
				plan.GateEntryTaskDelete = &entry
			}
			action := "attached"
			if !attach {
				action = "detached"
			}
			result.ProjectedDiff = map[string]any{"gateId": gate.ID, "entryTaskId": task.ID, "after": action, "criteriaRevisionAfter": plan.GateCriteriaRevision}
			plan.Events = []EventWrite{{Type: "gate.entry_task_" + action, Payload: map[string]any{"gateId": gate.ID, "taskId": task.ID, "selectionSource": "explicit", "criteriaRevisionAfter": plan.GateCriteriaRevision}}}
			break
		}
		conditions := make([]domain.GateTaskCondition, 0, len(gate.Conditions))
		var matching *GateTaskProjection
		for index := range gate.Conditions {
			condition := gate.Conditions[index]
			conditions = append(conditions, domain.GateTaskCondition{LinkID: condition.ID, WorkspaceID: args.WorkspaceID, GateID: gate.ID, TaskID: condition.TaskID, Passed: condition.PassedAt != nil, PassReason: stringValue(condition.PassReason)})
			if condition.TaskID == task.ID {
				matching = &gate.Conditions[index]
			}
		}
		attach := request.Name == "gate.attach_task"
		domainPlan := domain.PlanGateTaskAttachment(domainWorkspace(snapshot), domainGate(*gate, args.WorkspaceID), domainPhase(*from, args.WorkspaceID), domainTask(*task, args.WorkspaceID), conditions, attach, args.ClearTerminal)
		result.RequiredCapability = domainPlan.RequiredCapability
		result.Errors = append(result.Errors, domainPlan.Evaluation.Errors...)
		result.ProjectedDiff = domainPlan.ProjectedDiff
		if domainPlan.Evaluation.HasErrors() {
			break
		}
		plan.GateID, plan.GateCriteriaRevision = gate.ID, gate.CriteriaRevision+1
		active := snapshot.Workspace.ActivePhaseID != nil && *snapshot.Workspace.ActivePhaseID == gate.FromPhaseID
		plan.ForceHumanApproval = attach && active
		if attach {
			linkID := "preview-gate-task"
			if executing {
				linkID = randomUUID()
			}
			created := domain.GateTaskCondition{LinkID: linkID, WorkspaceID: args.WorkspaceID, GateID: gate.ID, TaskID: task.ID}
			plan.GateTaskCreate = &created
			if args.ClearTerminal {
				updated := domainTask(*task, args.WorkspaceID)
				updated.TerminalReason = ""
				plan.TaskUpdate = &updated
			}
		} else if matching != nil {
			plan.GateTaskDeleteID = matching.ID
		}
		for _, event := range domainPlan.Events {
			plan.Events = append(plan.Events, EventWrite{Type: event.Type, EntityType: event.EntityType, EntityID: event.EntityID, Payload: event.Payload})
		}
	case laneMutationArgs:
		plan.EntityType, plan.EntityID = "lane", args.LaneID
		var lane domain.Lane
		var existing *LaneProjection
		for index := range snapshot.Lanes {
			if snapshot.Lanes[index].ID == args.LaneID {
				existing = &snapshot.Lanes[index]
				break
			}
		}
		var domainPlan domain.DomainMutationPlan
		if request.Name == "lane.create" {
			result.RequiredCapability = "workspace:operate"
			if existing != nil {
				result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeInvalidStateTransition, EntityID: args.LaneID})
				break
			}
			lane = domain.Lane{ID: args.LaneID, WorkspaceID: args.WorkspaceID, Name: args.Name, Goal: args.Goal, Summary: args.Summary, State: domain.LaneActive}
			domainPlan = domain.PlanLaneCreate(domainWorkspace(snapshot), lane)
		} else {
			if existing == nil {
				result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeNotFound, EntityID: args.LaneID})
				break
			}
			lane = domain.Lane{ID: existing.ID, WorkspaceID: args.WorkspaceID, Name: existing.Name, Goal: existing.Goal, Summary: existing.Summary, State: domain.LaneState(existing.State)}
			if request.Name == "lane.update" {
				result.RequiredCapability = "workspace:operate"
				lane, domainPlan = domain.PlanLaneUpdate(domainWorkspace(snapshot), lane, args.Name, args.Goal, args.Summary)
			} else {
				result.RequiredCapability = "lane:approve"
				lane, domainPlan = domain.PlanLaneTermination(domainWorkspace(snapshot), request.Name, lane, args.Reason)
				for _, backlog := range snapshot.BacklogItems {
					if backlog.Status == string(domain.BacklogActive) && backlog.LaneID == args.LaneID {
						domainPlan.Evaluation.Errors = append(domainPlan.Evaluation.Errors, Diagnostic{Code: domain.CodeLaneHasActiveBacklog, EntityID: fmt.Sprint(backlog.PublicID)})
					}
				}
			}
		}
		result.Errors = append(result.Errors, domainPlan.Evaluation.Errors...)
		result.Warnings = append(result.Warnings, domainPlan.Evaluation.Warnings...)
		result.ProjectedDiff = domainPlan.ProjectedDiff
		if domainPlan.Evaluation.HasErrors() {
			break
		}
		plan.LaneUpdate = &lane
		for _, event := range domainPlan.Events {
			plan.Events = append(plan.Events, EventWrite{Type: event.Type, EntityType: event.EntityType, EntityID: event.EntityID, Payload: event.Payload})
		}
	case taskMutationArgs:
		result.RequiredCapability = "workspace:operate"
		if request.Name == "task.discard" {
			result.RequiredCapability = "task:approve"
		}
		plan.EntityType = "task"
		task := findTaskByPublicID(snapshot.Tasks, args.TaskID)
		if task == nil {
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeNotFound, EntityID: fmt.Sprint(args.TaskID)})
			break
		}
		plan.EntityID, plan.TaskID = task.ID, task.ID
		current := domainTask(*task, args.WorkspaceID)
		var next domain.Task
		var domainPlan domain.DomainMutationPlan
		switch request.Name {
		case "task.update":
			next, domainPlan = domain.PlanTaskUpdate(domainWorkspace(snapshot), current, args.Title, args.Description, args.CurrentSummary)
		case "task.move":
			target := findPhase(snapshot.Phases, args.TargetPhaseID)
			if target == nil {
				domainPlan = invalidDomainPlan(request.Name, current.ID, domain.CodeNotFound)
				break
			}
			for _, run := range snapshot.Runs {
				if run.TaskID == current.ID && run.Status == string(domain.RunRunning) {
					domainPlan = invalidDomainPlan(request.Name, current.ID, domain.CodeInvalidStateTransition)
					break
				}
			}
			if domainPlan.Evaluation.HasErrors() {
				break
			}
			for _, related := range snapshot.Tasks {
				if (related.ID == current.ParentTaskID || related.ParentTaskID == current.ID) && related.PhaseID != target.ID {
					domainPlan = invalidDomainPlan(request.Name, current.ID, domain.CodeInvalidStateTransition)
					break
				}
			}
			if domainPlan.Evaluation.HasErrors() {
				break
			}
			if taskMoveViolatesGateBinding(snapshot.Gates, current.ID, target.ID) {
				domainPlan = invalidDomainPlan(request.Name, current.ID, domain.CodeInvalidStateTransition)
			}
			if domainPlan.Evaluation.HasErrors() {
				break
			}
			next, domainPlan = domain.PlanTaskMove(domainWorkspace(snapshot), current, domainPhase(*target, args.WorkspaceID))
			if !domainPlan.Evaluation.HasErrors() {
				moved := snapshot
				moved.Tasks = append([]TaskProjection(nil), snapshot.Tasks...)
				for index := range moved.Tasks {
					if moved.Tasks[index].ID == current.ID {
						moved.Tasks[index].PhaseID = target.ID
					}
				}
				_, graphEvaluation := workspaceGraph(moved)
				domainPlan.Evaluation.Warnings = append(domainPlan.Evaluation.Warnings, graphEvaluation.Warnings...)
			}
		case "task.block", "task.unblock", "task.rework", "task.discard":
			next, domainPlan = domain.PlanTaskMutation(domainWorkspace(snapshot), request.Name, current, args.Reason, s.now().UTC())
		case "task.set_terminal", "task.clear_terminal":
			graph, graphEvaluation := workspaceGraph(snapshot)
			result.Errors = append(result.Errors, graphEvaluation.Errors...)
			if graphEvaluation.HasErrors() {
				break
			}
			var reason *string
			if request.Name == "task.set_terminal" {
				trimmed := strings.TrimSpace(args.Reason)
				reason = &trimmed
			}
			patch := domain.DependencyPatch{TerminalUpdates: []domain.TerminalUpdate{{TaskID: task.ID, TerminalReason: reason}}}
			preview := graph.PreviewPatch(patch)
			domainPlan = domain.DomainMutationPlan{Command: request.Name, RequiredCapability: "workspace:operate", ProjectedDiff: preview.Diff, Evaluation: preview.Evaluation}
			if !preview.Evaluation.HasErrors() {
				next = current
				if reason == nil {
					next.TerminalReason = ""
				} else {
					next.TerminalReason = *reason
				}
				payload := map[string]any{"taskId": task.ID}
				eventType := "task.terminal_cleared"
				if reason != nil {
					eventType, payload["reason"] = "task.terminal_set", *reason
				}
				domainPlan.Events = []domain.PlannedEvent{{Type: eventType, EntityType: "task", EntityID: task.ID, Payload: payload}}
			}
		}
		result.Errors = append(result.Errors, domainPlan.Evaluation.Errors...)
		result.Warnings = append(result.Warnings, domainPlan.Evaluation.Warnings...)
		result.Advisories = append(result.Advisories, domainPlan.Evaluation.Advisories...)
		result.ProjectedDiff = domainPlan.ProjectedDiff
		if executing && !domainPlan.Evaluation.HasErrors() && !sameDiagnosticCodes(domainPlan.Evaluation.Warnings, request.Envelope.AcknowledgedWarningCodes) {
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeInvalidStateTransition, EntityID: "warnings"})
		}
		if domainPlan.Evaluation.HasErrors() {
			break
		}
		plan.TaskUpdate = &next
		for _, event := range domainPlan.Events {
			if len(domainPlan.Evaluation.Warnings) > 0 {
				event.Payload["warnings"] = request.Envelope.AcknowledgedWarningCodes
				event.Payload["acknowledgedWarningCodes"] = request.Envelope.AcknowledgedWarningCodes
				event.Payload["proceedReason"] = strings.TrimSpace(request.Envelope.ProceedReason)
			}
			plan.Events = append(plan.Events, EventWrite{Type: event.Type, EntityType: event.EntityType, EntityID: event.EntityID, Payload: event.Payload})
		}
	case dependencyMutationArgs:
		result.RequiredCapability = "workspace:operate"
		plan.EntityType, plan.EntityID = "workspace_graph", "workspace_graph"
		graph, graphEvaluation := workspaceGraph(snapshot)
		result.Errors = append(result.Errors, graphEvaluation.Errors...)
		if graphEvaluation.HasErrors() {
			break
		}
		patch, patchErr := dependencyPatchFromArgs(snapshot.Tasks, args, request.Name)
		if patchErr != nil {
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeNotFound, EntityID: "dependency"})
			break
		}
		running := map[string]bool{}
		for _, run := range snapshot.Runs {
			if run.Status == string(domain.RunRunning) {
				running[run.TaskID] = true
			}
		}
		domainPlan := domain.PlanDependencyMutation(domainWorkspace(snapshot), request.Name, graph, patch, running)
		result.Errors = append(result.Errors, domainPlan.Evaluation.Errors...)
		result.Warnings = append(result.Warnings, domainPlan.Evaluation.Warnings...)
		result.Advisories = append(result.Advisories, domainPlan.Evaluation.Advisories...)
		result.ProjectedDiff = domainPlan.ProjectedDiff
		if executing && !domainPlan.Evaluation.HasErrors() && !sameDiagnosticCodes(domainPlan.Evaluation.Warnings, request.Envelope.AcknowledgedWarningCodes) {
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeInvalidStateTransition, EntityID: "warnings"})
		}
		if len(result.Errors) > 0 {
			break
		}
		plan.DependencyAdd, plan.DependencyRemove, plan.TerminalUpdates = patch.Add, patch.Remove, patch.TerminalUpdates
		for _, event := range domainPlan.Events {
			payload := event.Payload
			if len(domainPlan.Evaluation.Warnings) > 0 {
				payload["warnings"] = request.Envelope.AcknowledgedWarningCodes
				payload["acknowledgedWarningCodes"] = request.Envelope.AcknowledgedWarningCodes
				payload["proceedReason"] = strings.TrimSpace(request.Envelope.ProceedReason)
			}
			plan.Events = append(plan.Events, EventWrite{Type: event.Type, EntityType: event.EntityType, EntityID: event.EntityID, Payload: payload})
		}
	case taskReportImplementedArgs:
		result.RequiredCapability = "workspace:operate"
		plan.EntityType = "task"
		plan.EntityID = fmt.Sprint(args.TaskID)
		task := findTaskByPublicID(snapshot.Tasks, args.TaskID)
		if task == nil {
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeNotFound, EntityID: fmt.Sprint(args.TaskID)})
			break
		}
		plan.EntityID = task.ID
		tasks := make([]domain.Task, 0, len(snapshot.Tasks))
		for _, projected := range snapshot.Tasks {
			converted := domainTask(projected, args.WorkspaceID)
			if phase := findPhase(snapshot.Phases, projected.PhaseID); phase != nil {
				converted.PhasePosition = phase.Position
			}
			tasks = append(tasks, converted)
		}
		dependencies := make([]domain.Dependency, 0, len(snapshot.Dependencies))
		for _, projected := range snapshot.Dependencies {
			dependencies = append(dependencies, domain.Dependency{FromTaskID: projected.FromTaskID, ToTaskID: projected.ToTaskID})
		}
		gateTaskIDs := []string{}
		for _, gate := range snapshot.Gates {
			for _, condition := range gate.Conditions {
				gateTaskIDs = append(gateTaskIDs, condition.TaskID)
			}
		}
		graph, graphEvaluation := domain.NewWorkspaceGraph(tasks, dependencies, gateTaskIDs)
		result.Errors = append(result.Errors, graphEvaluation.Errors...)
		if graphEvaluation.HasErrors() {
			break
		}
		records := make([]domain.TaskRecord, 0, len(snapshot.Records))
		for _, projected := range snapshot.Records {
			records = append(records, domainRecord(projected, args.WorkspaceID))
		}
		mode := domain.MutationPreview
		if executing {
			mode = domain.MutationExecute
		}
		domainPlan := domain.PlanMutation("task.report_implemented", domain.MutationContext{
			Mode:      mode,
			Workspace: domain.Workspace{ID: snapshot.Workspace.ID, Name: snapshot.Workspace.Name, State: domain.WorkspaceState(snapshot.Workspace.State), Revision: snapshot.Workspace.Revision},
			Task:      domainTask(*task, args.WorkspaceID), Graph: graph, Records: records, Assessment: args.Assessment,
			Acknowledgement:    domain.WarningAcknowledgement{Codes: append([]string(nil), request.Envelope.AcknowledgedWarningCodes...), ProceedReason: request.Envelope.ProceedReason},
			InitiatedByActorID: request.Envelope.InitiatedByActorID, ExecutedByActorID: request.Envelope.ExecutedByActorID,
		})
		result.Errors = append(result.Errors, domainPlan.Evaluation.Errors...)
		result.Warnings = append(result.Warnings, domainPlan.Evaluation.Warnings...)
		result.Advisories = append(result.Advisories, domainPlan.Evaluation.Advisories...)
		result.ProjectedDiff = domainPlan.ProjectedDiff
		if domainPlan.Evaluation.HasErrors() {
			break
		}
		projected, ok := domainPlan.ProjectedDiff.(map[string]any)
		updated, okTask := projected["task"].(domain.Task)
		if !ok || !okTask {
			return result, plan, fmt.Errorf("task.report_implemented domain projection is invalid")
		}
		plan.TaskID = task.ID
		plan.TaskUpdate = &updated
		for _, event := range domainPlan.Events {
			plan.Events = append(plan.Events, EventWrite{Type: event.Type, EntityType: event.EntityType, EntityID: event.EntityID, Payload: event.Payload})
		}
	case taskConfirmArgs:
		result.RequiredCapability = "task:approve"
		plan.EntityType = "task"
		plan.EntityID = fmt.Sprint(args.TaskID)
		task := findTaskByPublicID(snapshot.Tasks, args.TaskID)
		if task == nil {
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeNotFound, EntityID: fmt.Sprint(args.TaskID)})
		} else if task.Status != "implemented" {
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeInvalidStateTransition, EntityID: task.ID})
		} else {
			plan.TaskID, plan.EntityID = task.ID, task.ID
			plan.TaskStatus = "confirmed"
			result.ProjectedDiff = map[string]any{"taskId": args.TaskID, "status": map[string]string{"before": task.Status, "after": "confirmed"}}
			payload := map[string]any{"taskId": task.ID}
			if isTaskDangling(snapshot, task.ID) {
				result.Warnings = append(result.Warnings, Diagnostic{Code: domain.CodeDanglingPath, EntityID: task.ID})
			}
			if executing && !sameDiagnosticCodes(result.Warnings, request.Envelope.AcknowledgedWarningCodes) {
				result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeInvalidStateTransition, EntityID: "warnings"})
			}
			if len(result.Warnings) > 0 {
				payload["warnings"] = request.Envelope.AcknowledgedWarningCodes
				payload["acknowledgedWarningCodes"] = request.Envelope.AcknowledgedWarningCodes
				payload["proceedReason"] = strings.TrimSpace(request.Envelope.ProceedReason)
			}
			plan.Events = []EventWrite{{Type: "task.confirmed", EntityType: "task", EntityID: task.ID, Payload: payload}}
		}
	case acceptancePolicyChangeArgs:
		result.RequiredCapability = "task:approve"
		plan.EntityType, plan.EntityID = "acceptance_policy", args.WorkspaceID
		mode := domain.AcceptanceMode(args.DefaultMode)
		foundProfile := false
		for _, profile := range snapshot.EvidenceProfiles {
			if profile.ID == args.EvidenceProfileID {
				foundProfile = true
				break
			}
		}
		if strings.TrimSpace(args.PolicyVersion) == "" || !foundProfile || mode != domain.AcceptanceHumanRequired {
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeInvalidStateTransition, EntityID: args.WorkspaceID})
			break
		}
		if args.PolicyVersion == snapshot.AcceptancePolicy.PolicyVersion {
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeIdempotencyConflict, EntityID: args.WorkspaceID})
			break
		}
		next := domain.AcceptancePolicy{WorkspaceID: args.WorkspaceID, PolicyVersion: strings.TrimSpace(args.PolicyVersion), DefaultMode: mode, EvidenceProfileID: args.EvidenceProfileID}
		plan.AcceptancePolicy = &next
		result.ProjectedDiff = map[string]any{"before": snapshot.AcceptancePolicy, "after": next, "existingTaskAssignmentsChanged": false}
		plan.Events = []EventWrite{{Type: "task.acceptance_policy_changed", EntityType: "acceptance_policy", EntityID: args.WorkspaceID, Payload: map[string]any{"workspaceId": args.WorkspaceID, "policy": next, "existingTaskAssignmentsChanged": false}}}
	case acceptanceModeEscalateArgs:
		result.RequiredCapability = "task:approve"
		plan.EntityType = "task"
		task := findTaskByPublicID(snapshot.Tasks, args.TaskID)
		if task == nil || !isUUID(args.AssignmentID) {
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeNotFound, EntityID: fmt.Sprint(args.TaskID)})
			break
		}
		plan.EntityID, plan.TaskID = task.ID, task.ID
		current := latestAcceptanceAssignment(snapshot.AcceptanceAssignments, task.ID)
		if current == nil {
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeNotFound, EntityID: task.ID})
			break
		}
		approvedBy := "preview-human"
		if executing && request.Envelope.HumanApprovalAttestation != nil {
			approvedBy = request.Envelope.HumanApprovalAttestation.ApprovedByActorID
		}
		next, escalationErr := domain.EscalateAcceptance(domainTask(*task, args.WorkspaceID), *current, args.AssignmentID, args.Reason, args.EvidenceReference, args.PolicyVersion, approvedBy)
		if escalationErr != nil {
			result.Errors = append(result.Errors, Diagnostic{Code: violationCode(escalationErr), EntityID: task.ID})
			break
		}
		plan.AcceptanceAssignment = &next
		result.ProjectedDiff = map[string]any{"taskId": args.TaskID, "before": current, "after": next}
		plan.Events = []EventWrite{{Type: "task.acceptance_mode_escalated", EntityType: "task", EntityID: task.ID, Payload: map[string]any{"taskId": task.ID, "assignment": next}}}
	case taskEvidenceReportArgs:
		result.RequiredCapability = "workspace:operate"
		plan.EntityType = "task"
		task := findTaskByPublicID(snapshot.Tasks, args.TaskID)
		if task == nil || !isUUID(args.EvidenceID) {
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeNotFound, EntityID: fmt.Sprint(args.TaskID)})
			break
		}
		plan.EntityID, plan.TaskID = task.ID, task.ID
		if task.Status != string(domain.TaskImplemented) {
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeInvalidStateTransition, EntityID: task.ID})
			break
		}
		for _, existing := range snapshot.AcceptanceEvidence {
			if existing.ID == args.EvidenceID {
				result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeIdempotencyConflict, EntityID: args.EvidenceID})
				break
			}
		}
		if len(result.Errors) > 0 {
			break
		}
		completion := findRecord(snapshot.Records, args.CompletionReportRecordID)
		review := findRecord(snapshot.Records, args.IndependentReviewRecordID)
		if completion == nil || review == nil || completion.TaskID != task.ID || review.TaskID != task.ID ||
			completion.Type != string(domain.RecordCompletionReport) || review.Type != string(domain.RecordIndependentReview) {
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeInvalidStateTransition, EntityID: args.EvidenceID})
			break
		}
		if args.CommitReferenceID != "" {
			commit := findCommit(snapshot.Commits, args.CommitReferenceID)
			if commit == nil || commit.TaskID != task.ID {
				result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeInvalidStateTransition, EntityID: args.CommitReferenceID})
				break
			}
		}
		version := 1
		for _, existing := range snapshot.AcceptanceEvidence {
			if existing.TaskID == task.ID && existing.Version >= version {
				version = existing.Version + 1
			}
		}
		evidence := domain.TaskAcceptanceEvidence{
			ID: args.EvidenceID, WorkspaceID: args.WorkspaceID, TaskID: task.ID, Version: version,
			CompletionReportRecordID: args.CompletionReportRecordID,
			VerificationVerdict:      domain.EvidenceVerdict(args.VerificationVerdict),
			VerificationReference:    args.VerificationReference, VerificationReferenceKind: args.VerificationReferenceKind,
			IndependentReviewRecordID: args.IndependentReviewRecordID,
			ReviewVerdict:             domain.ReviewVerdict(args.ReviewVerdict), UnresolvedBlockingCount: args.UnresolvedBlockingCount,
			CommitReferenceID: args.CommitReferenceID, ReportedByActorID: request.Envelope.ExecutedByActorID,
		}
		if evidence.VerificationReference != "" &&
			!validVerificationReference(snapshot, task.ID, evidence.VerificationReferenceKind, evidence.VerificationReference) {
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeInvalidStateTransition, EntityID: args.EvidenceID})
			break
		}
		profile := findEvidenceProfile(snapshot.EvidenceProfiles, task.EvidenceProfileID)
		if profile == nil || args.UnresolvedBlockingCount < 0 ||
			(evidence.VerificationVerdict != domain.EvidencePassed && evidence.VerificationVerdict != domain.EvidenceFailed && evidence.VerificationVerdict != domain.EvidenceUnavailable) ||
			(evidence.ReviewVerdict != domain.ReviewPass && evidence.ReviewVerdict != domain.ReviewFail && evidence.ReviewVerdict != domain.ReviewUnavailable) {
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeInvalidStateTransition, EntityID: args.EvidenceID})
			break
		}
		evaluation := domain.EvaluateAcceptance(*profile, evidence)
		plan.AcceptanceEvidence = &evidence
		result.ProjectedDiff = map[string]any{"taskId": args.TaskID, "evidence": evidence, "evaluation": evaluation, "humanApprovalRequired": true, "autoConfirmed": false}
		plan.Events = []EventWrite{{Type: "task.acceptance_evidence_reported", EntityType: "task", EntityID: task.ID, Payload: map[string]any{"taskId": task.ID, "evidenceId": evidence.ID, "evaluation": evaluation}}}
	case gateTaskArgs:
		result.RequiredCapability = "gate:approve"
		plan.EntityType = "gate_task"
		plan.EntityID = args.GateTaskID
		gate, condition := findGateTask(snapshot.Gates, args.GateTaskID)
		if gate == nil || condition == nil {
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeNotFound, EntityID: args.GateTaskID})
			break
		}
		if snapshot.Workspace.ActivePhaseID == nil || *snapshot.Workspace.ActivePhaseID != gate.FromPhaseID {
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeGateNotCurrent, EntityID: gate.ID})
			break
		}
		decisionHash = hashDecision(snapshot, *gate)
		if gate.PassedAt != nil {
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeInvalidStateTransition, EntityID: gate.ID})
			break
		}
		if strings.TrimSpace(args.Reason) == "" {
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeInvalidStateTransition, EntityID: condition.ID})
			break
		}
		isPass := request.Name == "gate.pass_task"
		if isPass == (condition.PassedAt != nil) {
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeInvalidStateTransition, EntityID: condition.ID})
			break
		}
		plan.GateTaskID = condition.ID
		plan.GateTaskPassed = isPass
		plan.GateTaskReason = strings.TrimSpace(args.Reason)
		plan.GateID = gate.ID
		after := "passed"
		event := "gate.task_passed"
		if !isPass {
			after = "required"
			event = "gate.task_pass_revoked"
		}
		result.ProjectedDiff = map[string]any{"gateTaskId": condition.ID, "condition": map[string]string{"after": after}}
		plan.Events = []EventWrite{{Type: event, Payload: map[string]any{"gateTaskId": condition.ID, "reason": plan.GateTaskReason}}}
	case gatePassArgs:
		result.RequiredCapability = "gate:approve"
		plan.EntityType = "gate"
		plan.EntityID = args.GateID
		gate := FindGateByReference(snapshot.Gates, args.GateID)
		if gate == nil {
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeNotFound, EntityID: args.GateID})
			break
		}
		plan.EntityID = gate.ID
		decisionHash = hashDecision(snapshot, *gate)
		conditions := make([]domain.GateTaskCondition, 0, len(gate.Conditions))
		for _, c := range gate.Conditions {
			task := findTask(snapshot.Tasks, c.TaskID)
			status := domain.TaskPending
			if task != nil {
				status = domain.TaskStatus(task.Status)
			}
			conditions = append(conditions, domain.GateTaskCondition{LinkID: c.ID, TaskID: c.TaskID, TaskStatus: status, Passed: c.PassedAt != nil})
		}
		from := findPhase(snapshot.Phases, gate.FromPhaseID)
		to := findPhase(snapshot.Phases, gate.ToPhaseID)
		if from == nil || to == nil {
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeNotFound, EntityID: gate.ID})
			break
		}
		_, transitionErr := domain.PlanGatePass(domain.Gate{ID: gate.ID, FromPhaseID: gate.FromPhaseID, ToPhaseID: gate.ToPhaseID, PassedAt: gate.PassedAt}, domain.Phase{ID: from.ID, Position: from.Position, State: domain.PhaseState(from.State)}, domain.Phase{ID: to.ID, Position: to.Position, State: domain.PhaseState(to.State)}, conditions, s.now())
		if transitionErr != nil {
			result.Errors = append(result.Errors, Diagnostic{Code: transitionErr.(*domain.Violation).Code, EntityID: gate.ID})
			break
		}
		plan.GateID = gate.ID
		plan.FromPhaseID = from.ID
		plan.ToPhaseID = to.ID
		evidence := make([]domain.GatePassConditionEvidence, 0, len(gate.Conditions))
		for _, c := range gate.Conditions {
			task := findTask(snapshot.Tasks, c.TaskID)
			status := ""
			if task != nil {
				status = task.Status
			}
			passReason := ""
			if c.PassReason != nil {
				passReason = *c.PassReason
			}
			evidence = append(evidence, domain.GatePassConditionEvidence{LinkID: c.ID, TaskID: c.TaskID, TaskStatus: domain.TaskStatus(status), Passed: c.PassedAt != nil, PassReason: passReason})
		}
		entries := append([]GateEntryTaskProjection(nil), gate.EntryTasks...)
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].TaskID == entries[j].TaskID {
				return entries[i].SelectionSource < entries[j].SelectionSource
			}
			return entries[i].TaskID < entries[j].TaskID
		})
		plan.Events = []EventWrite{{Type: "gate.passed", Payload: map[string]any{"gateId": gate.ID, "conditions": evidence, "entryTasks": entries}}, {Type: "phase.completed", Payload: map[string]any{"phaseId": from.ID}}, {Type: "phase.activated", Payload: map[string]any{"phaseId": to.ID}}}
		result.ProjectedDiff = map[string]any{"gate": map[string]string{"id": gate.ID, "after": "passed"}, "phases": []map[string]string{{"id": from.ID, "after": "completed"}, {"id": to.ID, "after": "active"}}, "entryTasks": entries}
	case runStartArgs:
		result.RequiredCapability = "run:operate"
		plan.EntityType = "run"
		plan.EntityID = args.ClientRunID
		task := findTaskByPublicID(snapshot.Tasks, args.TaskID)
		if task == nil {
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeNotFound, EntityID: fmt.Sprint(args.TaskID)})
			break
		}
		phase := findPhase(snapshot.Phases, task.PhaseID)
		if phase == nil {
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeNotFound, EntityID: task.PhaseID})
			break
		}
		identity := domain.RunStartIdentity{WorkspaceID: args.WorkspaceID, TaskID: task.ID, ClientRunID: args.ClientRunID, Kind: args.Kind, ParentRunID: args.ParentRunID, TargetRunID: args.TargetRunID}
		var existingRun *RunProjection
		for _, existing := range snapshot.Runs {
			if existing.ClientRunID != args.ClientRunID {
				continue
			}
			existingIdentity := domain.RunStartIdentity{WorkspaceID: args.WorkspaceID, TaskID: existing.TaskID, ClientRunID: existing.ClientRunID, Kind: domain.RunKind(existing.Kind), ParentRunID: existing.ParentRunID, TargetRunID: existing.TargetRunID}
			if err := domain.CompareRunStartIdentity(existingIdentity, identity); err != nil {
				result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeIdempotencyConflict, EntityID: args.ClientRunID})
			} else {
				copy := existing
				existingRun = &copy
				plan.ExistingRunClientID = args.ClientRunID
			}
			break
		}
		if len(result.Errors) > 0 {
			break
		}
		if existingRun != nil {
			result.ProjectedDiff = map[string]any{"run": existingRun, "idempotent": true}
			break
		}
		if args.ParentRunID != "" {
			parent := findRun(snapshot.Runs, args.ParentRunID)
			if parent == nil {
				result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeNotFound, EntityID: args.ParentRunID})
				break
			}
			if parent.TaskID != task.ID {
				result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeInvalidStateTransition, EntityID: args.ParentRunID})
				break
			}
		}
		if args.TargetRunID != "" && args.Kind != domain.RunIndependentAgentReview {
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeInvalidStateTransition, EntityID: args.TargetRunID})
			break
		}
		runID, leaseToken := "preview-run", "preview-lease-token"
		if executing {
			runID = randomUUID()
			if runID == "" {
				return result, plan, fmt.Errorf("secure Run identity generation failed")
			}
			var leaseErr error
			leaseToken, leaseErr = s.repo.RunLeaseToken(runID)
			if leaseErr != nil || leaseToken == "" {
				return result, plan, fmt.Errorf("secure Run lease generation failed: %w", leaseErr)
			}
		}
		plan.EntityID = runID
		var targetRun *domain.Run
		if args.TargetRunID != "" {
			if found := findRun(snapshot.Runs, args.TargetRunID); found != nil {
				converted := domainRun(*found, args.WorkspaceID)
				targetRun = &converted
			}
		}
		predecessors := make([]domain.Task, 0)
		for _, edge := range snapshot.Dependencies {
			if edge.ToTaskID == task.ID {
				if predecessor := findTask(snapshot.Tasks, edge.FromTaskID); predecessor != nil {
					predecessors = append(predecessors, domainTask(*predecessor, args.WorkspaceID))
				}
			}
		}
		startPlan, evaluation := domain.PlanRunStart(domain.WorkspaceState(snapshot.Workspace.State), domainTask(*task, args.WorkspaceID), domain.PhaseState(phase.State), predecessors, domain.RunStartRequest{RunID: runID, Identity: identity, OperatorActorID: request.Envelope.ExecutedByActorID, SessionRef: args.SessionRef, LeaseToken: leaseToken, LeaseDuration: 2 * time.Minute, Now: s.now().UTC(), TargetRun: targetRun})
		result.Errors = append(result.Errors, evaluation.Errors...)
		result.Warnings = append(result.Warnings, evaluation.Warnings...)
		result.Advisories = append(result.Advisories, evaluation.Advisories...)
		if evaluation.HasErrors() {
			break
		}
		plan.Run = &startPlan.Run
		plan.RunLeaseToken = leaseToken
		if startPlan.Task.Status != domain.TaskStatus(task.Status) {
			plan.RunTaskStatus = string(startPlan.Task.Status)
		}
		for _, event := range startPlan.Events {
			plan.Events = append(plan.Events, EventWrite{Type: event.Type, Payload: event.Payload})
		}
		result.ProjectedDiff = map[string]any{"run": map[string]any{"id": runID, "clientRunId": args.ClientRunID, "taskId": args.TaskID, "kind": args.Kind, "status": domain.RunRunning, "version": 1}, "taskStatus": map[string]string{"before": task.Status, "after": string(startPlan.Task.Status)}}
	case runHeartbeatArgs:
		result.RequiredCapability = "run:operate"
		plan.EntityType, plan.EntityID = "run", args.RunID
		existing := findRun(snapshot.Runs, args.RunID)
		if existing == nil {
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeNotFound, EntityID: args.RunID})
			break
		}
		extension := time.Duration(args.ExtensionSeconds) * time.Second
		if args.ExtensionSeconds == 0 {
			extension = 2 * time.Minute
		}
		next, heartbeatErr := domainRun(*existing, args.WorkspaceID).Heartbeat(args.LeaseToken, args.ExpectedRunVersion, s.now().UTC(), extension)
		if heartbeatErr != nil {
			result.Errors = append(result.Errors, Diagnostic{Code: violationCode(heartbeatErr), EntityID: args.RunID})
			break
		}
		plan.RunUpdate = &next
		plan.RunExpectedVersion = args.ExpectedRunVersion
		plan.NoWorkspaceRevision = true
		result.ProjectedDiff = map[string]any{"run": safeRunProjection(next)}
	case runTerminalArgs:
		result.RequiredCapability = "run:operate"
		plan.EntityType, plan.EntityID = "run", args.RunID
		existing := findRun(snapshot.Runs, args.RunID)
		if existing == nil {
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeNotFound, EntityID: args.RunID})
			break
		}
		status := terminalStatusForCommand(request.Name)
		next, outcome, terminalErr := domainRun(*existing, args.WorkspaceID).Terminate(status, args.ExpectedRunVersion, s.now().UTC(), args.Summary)
		if terminalErr != nil {
			result.Errors = append(result.Errors, Diagnostic{Code: violationCode(terminalErr), EntityID: args.RunID})
			break
		}
		result.ProjectedDiff = map[string]any{"run": safeRunProjection(next), "outcome": outcome}
		if outcome == domain.RunTransitionIdempotent {
			plan.NoWorkspaceRevision = true
			plan.IdempotentNoMutation = true
			break
		}
		plan.RunUpdate = &next
		plan.RunExpectedVersion = args.ExpectedRunVersion
		payload := map[string]any{"runId": next.ID}
		if status == domain.RunSucceeded {
			payload["resultSummary"] = next.ResultSummary
		} else {
			payload["errorSummary"] = next.ErrorSummary
		}
		plan.Events = []EventWrite{{Type: runTerminalEvent(request.Name), Payload: payload}}
	case runCorrectArgs:
		result.RequiredCapability = "run:operate"
		plan.EntityType, plan.EntityID = "run", args.RunID
		existing := findRun(snapshot.Runs, args.RunID)
		if existing == nil {
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeNotFound, EntityID: args.RunID})
			break
		}
		before := domainRun(*existing, args.WorkspaceID)
		next, correction, correctionErr := before.CorrectTerminal(args.Status, args.ExpectedRunVersion, s.now().UTC(), args.Summary, args.Reason)
		if correctionErr != nil {
			result.Errors = append(result.Errors, Diagnostic{Code: violationCode(correctionErr), EntityID: args.RunID})
			break
		}
		plan.RunUpdate = &next
		plan.RunExpectedVersion = args.ExpectedRunVersion
		plan.Events = []EventWrite{{Type: "run.corrected", Payload: map[string]any{"runId": next.ID, "previousStatus": correction.PreviousStatus, "previousResultSummary": correction.PreviousResultSummary, "previousErrorSummary": correction.PreviousErrorSummary, "previousEndedAt": correction.PreviousEndedAt, "newStatus": correction.NewStatus, "newResultSummary": correction.NewResultSummary, "newErrorSummary": correction.NewErrorSummary, "newEndedAt": correction.NewEndedAt, "reason": correction.Reason}}}
		result.ProjectedDiff = map[string]any{"run": safeRunProjection(next)}
	case repositoryRegisterArgs:
		result.RequiredCapability = "workspace:operate"
		plan.EntityType, plan.EntityID = "repository", args.RepositoryID
		if snapshot.Workspace.State == string(domain.WorkspaceClosed) {
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeInvalidStateTransition, EntityID: args.RepositoryID})
			break
		}
		if !isUUID(args.RepositoryID) {
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeInvalidStateTransition, EntityID: args.RepositoryID})
			break
		}
		requested, repositoryErr := domain.NewRepository(domain.Repository{ID: args.RepositoryID, WorkspaceID: args.WorkspaceID, Name: args.Name, RemoteURL: args.RemoteURL, DefaultBranch: args.DefaultBranch, IsRecordRepository: args.IsRecordRepository, TaskRecordsRoot: args.TaskRecordsRoot})
		if repositoryErr != nil {
			result.Errors = append(result.Errors, Diagnostic{Code: violationCode(repositoryErr), EntityID: args.RepositoryID})
			break
		}
		if existing := findRepository(snapshot.Repositories, args.RepositoryID); existing != nil {
			if domainRepository(*existing, args.WorkspaceID) != requested {
				result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeIdempotencyConflict, EntityID: args.RepositoryID})
				break
			}
			plan.NoWorkspaceRevision, plan.IdempotentNoMutation = true, true
			result.ProjectedDiff = map[string]any{"repository": existing, "outcome": domain.RunTransitionIdempotent}
			break
		}
		if requested.IsRecordRepository {
			for _, existing := range snapshot.Repositories {
				if existing.IsRecordRepository {
					result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeIdempotencyConflict, EntityID: args.RepositoryID})
					break
				}
			}
			if len(result.Errors) > 0 {
				break
			}
		}
		plan.Repository = &requested
		plan.Events = []EventWrite{{Type: "repository.registered", Payload: map[string]any{"repositoryId": requested.ID}}}
		result.ProjectedDiff = map[string]any{"repository": repositoryProjection(requested), "outcome": domain.RunTransitionApplied}
	case recordRegisterArgs:
		result.RequiredCapability = "record:operate"
		plan.EntityType, plan.EntityID = "task_record", args.RecordID
		if snapshot.Workspace.State == string(domain.WorkspaceClosed) {
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeInvalidStateTransition, EntityID: args.RecordID})
			break
		}
		if !isUUID(args.RecordID) || !isUUID(args.RepositoryID) || args.SupersedesRecordID != "" && !isUUID(args.SupersedesRecordID) {
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeInvalidStateTransition, EntityID: args.RecordID})
			break
		}
		task := findTaskByPublicID(snapshot.Tasks, args.TaskID)
		repository := findRepository(snapshot.Repositories, args.RepositoryID)
		if task == nil || repository == nil {
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeNotFound, EntityID: args.RecordID})
			break
		}
		if args.RunID != "" {
			run := findRun(snapshot.Runs, args.RunID)
			if run == nil || run.TaskID != task.ID {
				result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeInvalidStateTransition, EntityID: args.RunID})
				break
			}
		}
		registration := domain.RecordRegistration{ID: args.RecordID, WorkspaceID: args.WorkspaceID, TaskID: task.ID, RunID: args.RunID, Type: args.RecordType, RepositoryID: args.RepositoryID, RelativePath: args.RelativePath, WorkingTreeHash: args.WorkingTreeHash, ShortSummary: args.ShortSummary, SupersedesRecordID: args.SupersedesRecordID}
		existingRecords := make(map[string]domain.TaskRecord, len(snapshot.Records))
		for _, existing := range snapshot.Records {
			existingRecords[existing.ID] = domainRecord(existing, args.WorkspaceID)
		}
		if existing, ok := existingRecords[args.RecordID]; ok {
			if compareErr := domain.CompareRecordRegistration(repository.TaskRecordsRoot, existing, registration); compareErr != nil {
				result.Errors = append(result.Errors, Diagnostic{Code: violationCode(compareErr), EntityID: args.RecordID})
				break
			}
			plan.NoWorkspaceRevision, plan.IdempotentNoMutation = true, true
			result.ProjectedDiff = map[string]any{"record": recordProjection(existing), "outcome": domain.RunTransitionIdempotent}
			break
		}
		record, recordErr := domain.NewTaskRecord(repository.TaskRecordsRoot, registration, existingRecords)
		if recordErr != nil {
			result.Errors = append(result.Errors, Diagnostic{Code: violationCode(recordErr), EntityID: args.RecordID})
			break
		}
		for _, existing := range snapshot.Records {
			if existing.RepositoryID == record.RepositoryID && existing.RelativePath == record.RelativePath {
				result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeIdempotencyConflict, EntityID: args.RecordID})
				break
			}
		}
		if len(result.Errors) > 0 {
			break
		}
		plan.Record = &record
		plan.Events = []EventWrite{{Type: "record.registered", Payload: map[string]any{"recordId": record.ID, "taskId": record.TaskID, "repositoryId": record.RepositoryID, "relativePath": record.RelativePath}}}
		result.ProjectedDiff = map[string]any{"record": recordProjection(record), "outcome": domain.RunTransitionApplied}
	case recordAttachCommitArgs:
		result.RequiredCapability = "record:operate"
		plan.EntityType, plan.EntityID = "task_record", args.RecordID
		if snapshot.Workspace.State == string(domain.WorkspaceClosed) {
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeInvalidStateTransition, EntityID: args.RecordID})
			break
		}
		existing := findRecord(snapshot.Records, args.RecordID)
		if existing == nil {
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeNotFound, EntityID: args.RecordID})
			break
		}
		next, outcome, attachErr := domainRecord(*existing, args.WorkspaceID).AttachCommit(args.CommitSHA, args.BlobSHA)
		if attachErr != nil {
			result.Errors = append(result.Errors, Diagnostic{Code: violationCode(attachErr), EntityID: args.RecordID})
			break
		}
		result.ProjectedDiff = map[string]any{"record": recordProjection(next), "outcome": outcome}
		if outcome == domain.RunTransitionIdempotent {
			plan.NoWorkspaceRevision, plan.IdempotentNoMutation = true, true
			break
		}
		plan.Record = &next
		plan.Events = []EventWrite{{Type: "record.commit_attached", Payload: map[string]any{"recordId": next.ID, "commitSha": next.CommitSHA, "blobSha": next.BlobSHA}}}
	case commitAttachArgs:
		result.RequiredCapability = "record:operate"
		plan.EntityType, plan.EntityID = "commit_reference", args.CommitID
		if snapshot.Workspace.State == string(domain.WorkspaceClosed) {
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeInvalidStateTransition, EntityID: args.CommitID})
			break
		}
		if !isUUID(args.CommitID) || !isUUID(args.RepositoryID) {
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeInvalidStateTransition, EntityID: args.CommitID})
			break
		}
		task := findTaskByPublicID(snapshot.Tasks, args.TaskID)
		if task == nil || findRepository(snapshot.Repositories, args.RepositoryID) == nil {
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeNotFound, EntityID: args.CommitID})
			break
		}
		if args.RunID != "" {
			run := findRun(snapshot.Runs, args.RunID)
			if run == nil || run.TaskID != task.ID {
				result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeInvalidStateTransition, EntityID: args.RunID})
				break
			}
		}
		for _, existing := range snapshot.Commits {
			if existing.ID == args.CommitID {
				requested := CommitReferenceProjection{ID: args.CommitID, TaskID: task.ID, RunID: args.RunID, RepositoryID: args.RepositoryID, CommitSHA: strings.ToLower(strings.TrimSpace(args.CommitSHA)), Relation: string(args.Relation), VerificationState: string(domain.CommitReported)}
				if equalCommitReference(existing, requested) {
					plan.NoWorkspaceRevision, plan.IdempotentNoMutation = true, true
					result.ProjectedDiff = map[string]any{"commit": existing, "outcome": domain.RunTransitionIdempotent}
				} else {
					result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeIdempotencyConflict, EntityID: args.CommitID})
				}
				break
			}
			if existing.TaskID == task.ID && existing.RepositoryID == args.RepositoryID && existing.CommitSHA == strings.ToLower(strings.TrimSpace(args.CommitSHA)) && existing.Relation == string(args.Relation) {
				result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeIdempotencyConflict, EntityID: args.CommitID})
				break
			}
		}
		if plan.IdempotentNoMutation {
			break
		}
		if len(result.Errors) > 0 {
			break
		}
		commit, commitErr := domain.NewCommitReference(domain.CommitReference{ID: args.CommitID, WorkspaceID: args.WorkspaceID, TaskID: task.ID, RunID: args.RunID, RepositoryID: args.RepositoryID, CommitSHA: args.CommitSHA, Relation: args.Relation})
		if commitErr != nil {
			result.Errors = append(result.Errors, Diagnostic{Code: violationCode(commitErr), EntityID: args.CommitID})
			break
		}
		plan.CommitReference = &commit
		plan.Events = []EventWrite{{Type: "commit.attached", Payload: map[string]any{"commitId": commit.ID, "taskId": commit.TaskID, "repositoryId": commit.RepositoryID, "commitSha": commit.CommitSHA, "relation": commit.Relation}}}
		result.ProjectedDiff = map[string]any{"commit": commitProjection(commit), "outcome": domain.RunTransitionApplied}
	case gitObserveArgs:
		result.RequiredCapability = "record:operate"
		plan.EntityType, plan.EntityID = "run_git_observation", args.ObservationID
		if snapshot.Workspace.State == string(domain.WorkspaceClosed) {
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeInvalidStateTransition, EntityID: args.ObservationID})
			break
		}
		if !isUUID(args.ObservationID) || !isUUID(args.RepositoryID) || findRepository(snapshot.Repositories, args.RepositoryID) == nil || findRun(snapshot.Runs, args.RunID) == nil {
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeNotFound, EntityID: args.ObservationID})
			break
		}
		observation, observationErr := domain.NewRunGitObservation(domain.RunGitObservation{ID: args.ObservationID, WorkspaceID: args.WorkspaceID, RunID: args.RunID, RepositoryID: args.RepositoryID, ObservedAt: args.ObservedAt, HeadCommitSHA: args.HeadCommitSHA, BranchHint: args.BranchHint, WorktreeLabel: args.WorktreeLabel, Dirty: args.Dirty})
		if observationErr != nil {
			result.Errors = append(result.Errors, Diagnostic{Code: violationCode(observationErr), EntityID: args.ObservationID})
			break
		}
		for _, existing := range snapshot.GitObservations {
			if existing.ID == args.ObservationID {
				if equalGitObservation(existing, gitObservationProjection(observation)) {
					plan.NoWorkspaceRevision, plan.IdempotentNoMutation = true, true
					result.ProjectedDiff = map[string]any{"observation": existing, "outcome": domain.RunTransitionIdempotent}
				} else {
					result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeIdempotencyConflict, EntityID: args.ObservationID})
				}
				break
			}
		}
		if plan.IdempotentNoMutation {
			break
		}
		if len(result.Errors) > 0 {
			break
		}
		plan.GitObservation = &observation
		plan.Events = []EventWrite{{Type: "git.observed", Payload: map[string]any{"observationId": observation.ID, "runId": observation.RunID, "repositoryId": observation.RepositoryID, "observedAt": observation.ObservedAt}}}
		result.ProjectedDiff = map[string]any{"observation": gitObservationProjection(observation), "outcome": domain.RunTransitionApplied}
	default:
		return result, plan, &CommandError{Code: "invalid_request", Message: "unsupported command"}
	}
	result.DecisionSnapshotHash = decisionHash
	result.EntityType, result.EntityID = plan.EntityType, plan.EntityID
	result.CommandHash = hashCommand(request.Name, typed, request.Envelope.ExpectedWorkspaceRevision, decisionHash)
	requiresHumanApproval := requiresHumanApproval(request.Name) || plan.ForceHumanApproval
	if executing && !requiresHumanApproval && (request.Envelope.HumanApprovalAttestation != nil || request.Envelope.ApprovalGrantID != "") {
		result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeHumanApprovalMismatch, EntityID: plan.EntityID})
	}
	if requiresHumanApproval && (!executing || request.Envelope.HumanApprovalAttestation == nil && request.Envelope.ApprovalGrantID == "") {
		result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeHumanApprovalRequired, EntityID: plan.EntityID})
	}
	if executing && requiresHumanApproval && request.Envelope.ApprovalGrantID == "" {
		att := request.Envelope.HumanApprovalAttestation
		valid := att != nil && att.ApprovedCommandHash == result.CommandHash && (decisionHash == "" || att.DecisionSnapshotHash == decisionHash)
		if valid {
			valid = snapshot.HumanActorIDs[att.ApprovedByActorID]
		}
		if !valid {
			result.Errors = removeDiagnostic(result.Errors, domain.CodeHumanApprovalRequired)
			result.Errors = append(result.Errors, Diagnostic{Code: domain.CodeHumanApprovalMismatch, EntityID: plan.EntityID})
		}
	}
	sort.Slice(result.Errors, func(i, j int) bool { return result.Errors[i].Code < result.Errors[j].Code })
	if executing && len(result.Errors) > 0 {
		return result, plan, &CommandError{Code: result.Errors[0].Code, Message: "command evaluation failed: " + result.Errors[0].Code}
	}
	return result, plan, nil
}

func approvalFingerprint(attestation *HumanApprovalAttestation) string {
	if attestation == nil {
		return "absent"
	}
	approvedAt := ""
	if attestation.ApprovedAt != nil {
		approvedAt = attestation.ApprovedAt.UTC().Format(time.RFC3339Nano)
	}
	return strings.Join([]string{
		"present",
		strings.TrimSpace(attestation.ApprovedByActorID),
		strings.TrimSpace(attestation.ApprovedCommandHash),
		strings.TrimSpace(attestation.DecisionSnapshotHash),
		strings.TrimSpace(attestation.StatementHash),
		strings.TrimSpace(attestation.ConversationRef),
		approvedAt,
	}, "\x1e")
}

func decodeArguments(name string, raw json.RawMessage) (string, any, error) {
	var target any
	switch name {
	case "workspace.close":
		target = &workspaceMutationArgs{}
	case "phase.create":
		target = &phaseCreateArgs{}
	case "task.create":
		target = &taskCreateArgs{}
	case "backlog.create":
		target = &backlogCreateArgs{}
	case "backlog.update":
		target = &backlogUpdateArgs{}
	case "backlog.move":
		target = &backlogMoveArgs{}
	case "backlog.reorder":
		target = &backlogReorderArgs{}
	case "backlog.discard":
		target = &backlogDiscardArgs{}
	case "backlog.promote":
		target = &backlogPromoteArgs{}
	case "gate.create", "gate.attach_task", "gate.detach_task", "gate.attach_entry_task", "gate.detach_entry_task":
		target = &gateMutationArgs{}
	case "lane.create", "lane.update", "lane.close_out", "lane.discard":
		target = &laneMutationArgs{}
	case "task.update", "task.move", "task.set_terminal", "task.clear_terminal", "task.block", "task.unblock", "task.discard", "task.rework":
		target = &taskMutationArgs{}
	case "dependency.connect", "dependency.disconnect", "dependency.patch":
		target = &dependencyMutationArgs{}
	case "task.report_implemented":
		target = &taskReportImplementedArgs{}
	case "task.confirm":
		target = &taskConfirmArgs{}
	case "task.acceptance_policy.change":
		target = &acceptancePolicyChangeArgs{}
	case "task.acceptance_mode.escalate":
		target = &acceptanceModeEscalateArgs{}
	case "task.evidence.report":
		target = &taskEvidenceReportArgs{}
	case "gate.pass_task", "gate.revoke_task_pass":
		target = &gateTaskArgs{}
	case "gate.pass":
		target = &gatePassArgs{}
	case "run.start":
		target = &runStartArgs{}
	case "run.heartbeat":
		target = &runHeartbeatArgs{}
	case "run.succeed", "run.fail", "run.cancel", "run.interrupt":
		target = &runTerminalArgs{}
	case "run.correct":
		target = &runCorrectArgs{}
	case "repository.register":
		target = &repositoryRegisterArgs{}
	case "record.register":
		target = &recordRegisterArgs{}
	case "record.attach_commit":
		target = &recordAttachCommitArgs{}
	case "commit.attach":
		target = &commitAttachArgs{}
	case "git.observe":
		target = &gitObserveArgs{}
	default:
		return "", nil, &CommandError{Code: "invalid_request", Message: "unsupported command: " + name}
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return "", nil, &CommandError{Code: "invalid_request", Message: err.Error()}
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("arguments must contain exactly one JSON value")
		}
		return "", nil, &CommandError{Code: "invalid_request", Message: err.Error()}
	}
	switch v := target.(type) {
	case *workspaceMutationArgs:
		return v.WorkspaceID, *v, nil
	case *phaseCreateArgs:
		return v.WorkspaceID, *v, nil
	case *taskCreateArgs:
		return v.WorkspaceID, *v, nil
	case *backlogCreateArgs:
		title, description := v.Title, v.Description
		return v.WorkspaceID, backlogMutationArgs{
			WorkspaceID: v.WorkspaceID, BacklogUUID: v.BacklogUUID, LaneID: v.LaneID,
			Title: &title, Description: &description,
		}, nil
	case *backlogUpdateArgs:
		return v.WorkspaceID, backlogMutationArgs{
			WorkspaceID: v.WorkspaceID, BacklogPublicID: v.BacklogPublicID,
			Title: v.Title, Description: v.Description,
		}, nil
	case *backlogMoveArgs:
		return v.WorkspaceID, backlogMutationArgs{
			WorkspaceID: v.WorkspaceID, BacklogPublicID: v.BacklogPublicID, TargetLaneID: v.TargetLaneID,
		}, nil
	case *backlogReorderArgs:
		return v.WorkspaceID, backlogMutationArgs{
			WorkspaceID: v.WorkspaceID, LaneID: v.LaneID, OrderedBacklogPublicIDs: v.OrderedBacklogPublicIDs,
		}, nil
	case *backlogDiscardArgs:
		return v.WorkspaceID, backlogMutationArgs{
			WorkspaceID: v.WorkspaceID, BacklogPublicID: v.BacklogPublicID, Reason: v.Reason,
		}, nil
	case *backlogPromoteArgs:
		return v.WorkspaceID, backlogMutationArgs{
			WorkspaceID: v.WorkspaceID, BacklogPublicID: v.BacklogPublicID,
			TaskUUID: v.TaskUUID, PhaseID: v.PhaseID, ParentTaskID: v.ParentTaskID,
			PredecessorTaskIDs: v.PredecessorTaskIDs, SuccessorTaskIDs: v.SuccessorTaskIDs,
			TerminalReason: v.TerminalReason, RequestedAcceptanceMode: v.RequestedAcceptanceMode,
			EvidenceProfileID: v.EvidenceProfileID,
		}, nil
	case *gateMutationArgs:
		return v.WorkspaceID, *v, nil
	case *laneMutationArgs:
		return v.WorkspaceID, *v, nil
	case *taskMutationArgs:
		return v.WorkspaceID, *v, nil
	case *dependencyMutationArgs:
		return v.WorkspaceID, *v, nil
	case *taskReportImplementedArgs:
		return v.WorkspaceID, *v, nil
	case *taskConfirmArgs:
		return v.WorkspaceID, *v, nil
	case *acceptancePolicyChangeArgs:
		return v.WorkspaceID, *v, nil
	case *acceptanceModeEscalateArgs:
		return v.WorkspaceID, *v, nil
	case *taskEvidenceReportArgs:
		return v.WorkspaceID, *v, nil
	case *gateTaskArgs:
		return v.WorkspaceID, *v, nil
	case *gatePassArgs:
		return v.WorkspaceID, *v, nil
	case *runStartArgs:
		return v.WorkspaceID, *v, nil
	case *runHeartbeatArgs:
		return v.WorkspaceID, *v, nil
	case *runTerminalArgs:
		return v.WorkspaceID, *v, nil
	case *runCorrectArgs:
		return v.WorkspaceID, *v, nil
	case *repositoryRegisterArgs:
		return v.WorkspaceID, *v, nil
	case *recordRegisterArgs:
		return v.WorkspaceID, *v, nil
	case *recordAttachCommitArgs:
		return v.WorkspaceID, *v, nil
	case *commitAttachArgs:
		return v.WorkspaceID, *v, nil
	case *gitObserveArgs:
		return v.WorkspaceID, *v, nil
	}
	panic("unreachable")
}

func requiresHumanApproval(name string) bool {
	switch name {
	case "workspace.close", "task.confirm", "task.discard", "task.acceptance_policy.change", "task.acceptance_mode.escalate", "lane.close_out", "lane.discard", "gate.pass_task", "gate.revoke_task_pass", "gate.pass":
		return true
	default:
		return false
	}
}

func terminalStatusForCommand(name string) domain.RunStatus {
	switch name {
	case "run.succeed":
		return domain.RunSucceeded
	case "run.fail":
		return domain.RunFailed
	case "run.cancel":
		return domain.RunCancelled
	case "run.interrupt":
		return domain.RunInterrupted
	default:
		return ""
	}
}

func runTerminalEvent(name string) string {
	return map[string]string{"run.succeed": "run.succeeded", "run.fail": "run.failed", "run.cancel": "run.cancelled", "run.interrupt": "run.interrupted"}[name]
}

func safeRunProjection(run domain.Run) RunProjection {
	return RunProjection{ID: run.ID, TaskID: run.TaskID, ClientRunID: run.ClientRunID, Kind: string(run.Kind), Status: string(run.Status), OperatorActorID: run.OperatorActorID, SessionRef: run.SessionRef, ParentRunID: run.ParentRunID, TargetRunID: run.TargetRunID, HeartbeatAt: run.HeartbeatAt, LeaseExpiresAt: run.LeaseExpiresAt, Version: run.Version, StartedAt: run.StartedAt, EndedAt: run.EndedAt, ResultSummary: run.ResultSummary, ErrorSummary: run.ErrorSummary}
}

func findRepository(values []RepositoryProjection, id string) *RepositoryProjection {
	for index := range values {
		if values[index].ID == id {
			return &values[index]
		}
	}
	return nil
}

func findRecord(values []TaskRecordProjection, id string) *TaskRecordProjection {
	for index := range values {
		if values[index].ID == id {
			return &values[index]
		}
	}
	return nil
}
func findCommit(values []CommitReferenceProjection, id string) *CommitReferenceProjection {
	for index := range values {
		if values[index].ID == id {
			return &values[index]
		}
	}
	return nil
}

func validVerificationReference(snapshot Snapshot, taskID, kind, reference string) bool {
	reference = strings.TrimSpace(reference)
	switch kind {
	case "task_record":
		record := findRecord(snapshot.Records, reference)
		return record != nil && record.TaskID == taskID
	case "run":
		run := findRun(snapshot.Runs, reference)
		return run != nil && run.TaskID == taskID && run.Status == string(domain.RunSucceeded)
	case "commit_reference":
		commit := findCommit(snapshot.Commits, reference)
		return commit != nil && commit.TaskID == taskID
	case "artifact":
		parts := strings.SplitN(reference, "#", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
			return false
		}
		binding := strings.SplitN(parts[0], ":", 2)
		if len(binding) != 2 {
			return false
		}
		switch binding[0] {
		case "record":
			record := findRecord(snapshot.Records, binding[1])
			return record != nil && record.TaskID == taskID
		case "run":
			run := findRun(snapshot.Runs, binding[1])
			return run != nil && run.TaskID == taskID && run.Status == string(domain.RunSucceeded)
		}
	}
	return false
}
func latestAcceptanceAssignment(values []domain.TaskAcceptanceAssignment, taskID string) *domain.TaskAcceptanceAssignment {
	var latest *domain.TaskAcceptanceAssignment
	for index := range values {
		value := &values[index]
		if value.TaskID == taskID && (latest == nil || value.Version > latest.Version) {
			latest = value
		}
	}
	return latest
}
func findEvidenceProfile(values []domain.EvidenceProfile, id string) *domain.EvidenceProfile {
	for index := range values {
		if values[index].ID == id {
			return &values[index]
		}
	}
	return nil
}

func resolveTaskAcceptance(snapshot Snapshot, taskID, requestedMode, profileID string, executing bool) (domain.TaskAcceptanceAssignment, error) {
	policy := snapshot.AcceptancePolicy
	if policy.WorkspaceID == "" {
		policy = domain.AcceptancePolicy{WorkspaceID: snapshot.Workspace.ID, PolicyVersion: "migration-v1", DefaultMode: domain.AcceptanceHumanRequired, EvidenceProfileID: "technical-v1"}
	}
	assignment, err := domain.ResolveAcceptance(policy, domain.AcceptanceMode(requestedMode), profileID)
	if err != nil {
		return domain.TaskAcceptanceAssignment{}, err
	}
	foundProfile := false
	for _, profile := range snapshot.EvidenceProfiles {
		if profile.ID == assignment.EvidenceProfileID {
			foundProfile = true
			break
		}
	}
	if !foundProfile && len(snapshot.EvidenceProfiles) > 0 {
		return domain.TaskAcceptanceAssignment{}, &domain.Violation{Code: domain.CodeNotFound}
	}
	assignment.ID = "preview-acceptance-assignment"
	if executing {
		assignment.ID = randomUUID()
		if assignment.ID == "" {
			return domain.TaskAcceptanceAssignment{}, fmt.Errorf("secure acceptance assignment identity generation failed")
		}
	}
	assignment.WorkspaceID = snapshot.Workspace.ID
	assignment.TaskID = taskID
	return assignment, nil
}

func domainRepository(value RepositoryProjection, workspaceID string) domain.Repository {
	return domain.Repository{ID: value.ID, WorkspaceID: workspaceID, Name: value.Name, RemoteURL: value.RemoteURL, DefaultBranch: value.DefaultBranch, IsRecordRepository: value.IsRecordRepository, TaskRecordsRoot: value.TaskRecordsRoot}
}

func domainRecord(value TaskRecordProjection, workspaceID string) domain.TaskRecord {
	return domain.TaskRecord{ID: value.ID, WorkspaceID: workspaceID, TaskID: value.TaskID, RunID: value.RunID, Type: domain.RecordType(value.Type), RepositoryID: value.RepositoryID, RelativePath: value.RelativePath, WorkingTreeHash: value.WorkingTreeHash, CommitSHA: value.CommitSHA, BlobSHA: value.BlobSHA, State: domain.RecordState(value.State), ShortSummary: value.ShortSummary, SupersedesRecordID: value.SupersedesRecordID}
}

func repositoryProjection(value domain.Repository) RepositoryProjection {
	return RepositoryProjection{ID: value.ID, Name: value.Name, RemoteURL: value.RemoteURL, DefaultBranch: value.DefaultBranch, IsRecordRepository: value.IsRecordRepository, TaskRecordsRoot: value.TaskRecordsRoot}
}

func recordProjection(value domain.TaskRecord) TaskRecordProjection {
	return TaskRecordProjection{ID: value.ID, TaskID: value.TaskID, RunID: value.RunID, Type: string(value.Type), RepositoryID: value.RepositoryID, RelativePath: value.RelativePath, WorkingTreeHash: value.WorkingTreeHash, CommitSHA: value.CommitSHA, BlobSHA: value.BlobSHA, State: string(value.State), ShortSummary: value.ShortSummary, SupersedesRecordID: value.SupersedesRecordID}
}

func commitProjection(value domain.CommitReference) CommitReferenceProjection {
	return CommitReferenceProjection{ID: value.ID, TaskID: value.TaskID, RunID: value.RunID, RepositoryID: value.RepositoryID, CommitSHA: value.CommitSHA, Relation: string(value.Relation), VerificationState: string(value.VerificationState)}
}

func gitObservationProjection(value domain.RunGitObservation) GitObservationProjection {
	return GitObservationProjection{ID: value.ID, RunID: value.RunID, RepositoryID: value.RepositoryID, ObservedAt: value.ObservedAt, HeadCommitSHA: value.HeadCommitSHA, BranchHint: value.BranchHint, WorktreeLabel: value.WorktreeLabel, Dirty: value.Dirty}
}

func equalGitObservation(left, right GitObservationProjection) bool {
	if left.ID != right.ID || left.RunID != right.RunID || left.RepositoryID != right.RepositoryID || !left.ObservedAt.Equal(right.ObservedAt) || left.HeadCommitSHA != right.HeadCommitSHA || left.BranchHint != right.BranchHint || left.WorktreeLabel != right.WorktreeLabel {
		return false
	}
	if left.Dirty == nil || right.Dirty == nil {
		return left.Dirty == nil && right.Dirty == nil
	}
	return *left.Dirty == *right.Dirty
}

func equalCommitReference(left, right CommitReferenceProjection) bool {
	return left.ID == right.ID &&
		left.TaskID == right.TaskID &&
		left.RunID == right.RunID &&
		left.RepositoryID == right.RepositoryID &&
		left.CommitSHA == right.CommitSHA &&
		left.Relation == right.Relation &&
		left.VerificationState == right.VerificationState
}

func isUUID(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return false
	}
	_, err := hex.DecodeString(strings.ReplaceAll(value, "-", ""))
	return err == nil
}

func violationCode(err error) string {
	if violation, ok := err.(*domain.Violation); ok {
		return violation.Code
	}
	return domain.CodeInvalidStateTransition
}

func hashCommand(name string, args any, revision int64, snapshot string) string {
	payload, _ := json.Marshal(struct {
		ContractVersion string `json:"contractVersion"`
		Name            string `json:"commandName"`
		Arguments       any    `json:"arguments"`
		Revision        int64  `json:"expectedWorkspaceRevision"`
		Snapshot        string `json:"decisionSnapshotHash,omitempty"`
	}{"1.0.0", name, args, revision, snapshot})
	return sha(payload)
}
func hashRequestFingerprint(name string, args any, revision int64) string {
	payload, _ := json.Marshal(struct {
		ContractVersion string `json:"contractVersion"`
		Name            string `json:"commandName"`
		Arguments       any    `json:"arguments"`
		Revision        int64  `json:"expectedWorkspaceRevision"`
	}{"1.0.0", name, args, revision})
	return sha(payload)
}
func hashDecision(snapshot Snapshot, gate GateProjection) string {
	sort.Slice(gate.Conditions, func(i, j int) bool { return gate.Conditions[i].ID < gate.Conditions[j].ID })
	rows := make([]any, 0, len(gate.Conditions))
	for _, c := range gate.Conditions {
		t := findTask(snapshot.Tasks, c.TaskID)
		status := ""
		if t != nil {
			status = t.Status
		}
		rows = append(rows, []any{c.ID, c.TaskID, status, c.PassedAt != nil, c.PassReason != nil})
	}
	entries := append([]GateEntryTaskProjection(nil), gate.EntryTasks...)
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].TaskID == entries[j].TaskID {
			return entries[i].SelectionSource < entries[j].SelectionSource
		}
		return entries[i].TaskID < entries[j].TaskID
	})
	entryRows := make([]any, 0, len(entries))
	for _, entry := range entries {
		entryRows = append(entryRows, []any{entry.TaskID, entry.SelectionSource})
	}
	payload, _ := json.Marshal([]any{gate.ID, gate.CriteriaRevision, gate.FromPhaseID, gate.ToPhaseID, rows, entryRows, snapshot.Workspace.Revision})
	return sha(payload)
}

func DecisionSnapshotHash(snapshot Snapshot, gate GateProjection) string {
	return hashDecision(snapshot, gate)
}
func sha(value []byte) string {
	sum := sha256.Sum256(value)
	return "sha256:" + hex.EncodeToString(sum[:])
}
func removeDiagnostic(values []Diagnostic, code string) []Diagnostic {
	out := values[:0]
	for _, v := range values {
		if v.Code != code {
			out = append(out, v)
		}
	}
	return out
}
func findTaskByPublicID(v []TaskProjection, id int) *TaskProjection {
	for i := range v {
		if v[i].PublicID == id {
			return &v[i]
		}
	}
	return nil
}
func findBacklogByPublicID(values []BacklogItemProjection, id int) *BacklogItemProjection {
	for i := range values {
		if values[i].PublicID == id {
			return &values[i]
		}
	}
	return nil
}
func findLane(values []LaneProjection, id string) *LaneProjection {
	for i := range values {
		if values[i].ID == id {
			return &values[i]
		}
	}
	return nil
}
func domainLane(value LaneProjection, workspaceID string) domain.Lane {
	return domain.Lane{ID: value.ID, WorkspaceID: workspaceID, Name: value.Name, Goal: value.Goal, Summary: value.Summary, State: domain.LaneState(value.State)}
}
func domainBacklog(value BacklogItemProjection, workspaceID string) domain.BacklogItem {
	item := domain.BacklogItem{ID: value.ID, WorkspaceID: workspaceID, PublicID: value.PublicID, LaneID: value.LaneID, Title: value.Title, Description: value.Description, Status: domain.BacklogItemStatus(value.Status), Position: value.Position}
	if value.PromotedTaskID != nil {
		item.PromotedTaskID = *value.PromotedTaskID
	}
	if value.DiscardReason != nil {
		item.DiscardReason = *value.DiscardReason
	}
	return item
}
func domainBacklogItems(values []BacklogItemProjection, workspaceID string) []domain.BacklogItem {
	out := make([]domain.BacklogItem, 0, len(values))
	for _, value := range values {
		out = append(out, domainBacklog(value, workspaceID))
	}
	return out
}
func replaceDomainBacklog(values []BacklogItemProjection, replacement domain.BacklogItem, workspaceID string) []domain.BacklogItem {
	out := domainBacklogItems(values, workspaceID)
	for i := range out {
		if out[i].ID == replacement.ID {
			out[i] = replacement
		}
	}
	return out
}
func backlogProjection(value domain.BacklogItem) BacklogItemProjection {
	projected := BacklogItemProjection{ID: value.ID, PublicID: value.PublicID, LaneID: value.LaneID, Title: value.Title, Description: value.Description, Status: string(value.Status), Position: value.Position}
	if value.PromotedTaskID != "" {
		projected.PromotedTaskID = &value.PromotedTaskID
	}
	if value.DiscardReason != "" {
		projected.DiscardReason = &value.DiscardReason
	}
	return projected
}
func stringPointerValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
func findTask(v []TaskProjection, id string) *TaskProjection {
	for i := range v {
		if v[i].ID == id {
			return &v[i]
		}
	}
	return nil
}
func findGate(v []GateProjection, id string) *GateProjection {
	for i := range v {
		if v[i].ID == id {
			return &v[i]
		}
	}
	return nil
}
func FindGateByReference(values []GateProjection, reference string) *GateProjection {
	trimmed := strings.TrimSpace(reference)
	if domain.IsReservedGatePublicReference(trimmed) {
		publicID, _ := strconv.Atoi(trimmed[2:])
		for index := range values {
			if values[index].PublicID == publicID {
				return &values[index]
			}
		}
		return nil
	}
	if exact := findGate(values, reference); exact != nil {
		return exact
	}
	for index := range values {
		if values[index].Alias != "" && strings.EqualFold(values[index].Alias, trimmed) {
			return &values[index]
		}
	}
	return nil
}
func findGateTask(v []GateProjection, id string) (*GateProjection, *GateTaskProjection) {
	for i := range v {
		for j := range v[i].Conditions {
			if v[i].Conditions[j].ID == id {
				return &v[i], &v[i].Conditions[j]
			}
		}
	}
	return nil, nil
}

// taskMoveViolatesGateBinding protects explicit Gate decisions while allowing
// derived automatic entry tasks to follow a Task's new Phase.
func taskMoveViolatesGateBinding(gates []GateProjection, taskID, targetPhaseID string) bool {
	for _, gate := range gates {
		for _, condition := range gate.Conditions {
			if condition.TaskID == taskID && gate.FromPhaseID != targetPhaseID {
				return true
			}
		}
		for _, entry := range gate.EntryTasks {
			if entry.TaskID == taskID && entry.SelectionSource == "explicit" && gate.ToPhaseID != targetPhaseID {
				return true
			}
		}
	}
	return false
}
func findPhase(v []PhaseProjection, id string) *PhaseProjection {
	for i := range v {
		if v[i].ID == id {
			return &v[i]
		}
	}
	return nil
}

func findRun(v []RunProjection, id string) *RunProjection {
	for i := range v {
		if v[i].ID == id {
			return &v[i]
		}
	}
	return nil
}

func domainTask(v TaskProjection, workspaceID string) domain.Task {
	task := domain.Task{ID: v.ID, PublicID: v.PublicID, WorkspaceID: workspaceID, LaneID: v.LaneID, PhaseID: v.PhaseID, ParentTaskID: v.ParentTaskID, Title: v.Title, Description: v.Description, CurrentSummary: v.CurrentSummary, NextAction: v.NextAction, Status: domain.TaskStatus(v.Status), TerminalReason: v.TerminalReason, ImplementedAssessment: v.ImplementedAssessment}
	if v.BlockerReason != nil {
		now := time.Unix(1, 0)
		task.BlockedAt = &now
		task.BlockerReason = *v.BlockerReason
	}
	return task
}

func domainWorkspace(snapshot Snapshot) domain.Workspace {
	active := ""
	if snapshot.Workspace.ActivePhaseID != nil {
		active = *snapshot.Workspace.ActivePhaseID
	}
	return domain.Workspace{ID: snapshot.Workspace.ID, Name: snapshot.Workspace.Name, State: domain.WorkspaceState(snapshot.Workspace.State), ActivePhaseID: active, Revision: snapshot.Workspace.Revision}
}

func invalidDomainPlan(command, entityID, code string) domain.DomainMutationPlan {
	return domain.DomainMutationPlan{Command: command, Evaluation: domain.Evaluation{Errors: []domain.Diagnostic{{Code: code, EntityID: entityID}}}}
}
func domainPhase(value PhaseProjection, workspaceID string) domain.Phase {
	return domain.Phase{ID: value.ID, WorkspaceID: workspaceID, Position: value.Position, State: domain.PhaseState(value.State)}
}

func domainGate(value GateProjection, workspaceID string) domain.Gate {
	return domain.Gate{ID: value.ID, PublicID: value.PublicID, Alias: value.Alias, WorkspaceID: workspaceID, FromPhaseID: value.FromPhaseID, ToPhaseID: value.ToPhaseID, PassedAt: value.PassedAt, CriteriaRevision: value.CriteriaRevision}
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func workspaceGraph(snapshot Snapshot) (*domain.WorkspaceGraph, domain.Evaluation) {
	tasks := make([]domain.Task, 0, len(snapshot.Tasks))
	for _, projected := range snapshot.Tasks {
		task := domainTask(projected, snapshot.Workspace.ID)
		if phase := findPhase(snapshot.Phases, projected.PhaseID); phase != nil {
			task.PhasePosition = phase.Position
		}
		tasks = append(tasks, task)
	}
	edges := make([]domain.Dependency, 0, len(snapshot.Dependencies))
	for _, edge := range snapshot.Dependencies {
		edges = append(edges, domain.Dependency{FromTaskID: edge.FromTaskID, ToTaskID: edge.ToTaskID})
	}
	gateTaskIDs := []string{}
	for _, gate := range snapshot.Gates {
		for _, condition := range gate.Conditions {
			gateTaskIDs = append(gateTaskIDs, condition.TaskID)
		}
	}
	return domain.NewWorkspaceGraph(tasks, edges, gateTaskIDs)
}

func isTaskDangling(snapshot Snapshot, taskID string) bool {
	for _, task := range snapshot.Tasks {
		if task.ID == taskID && task.TerminalReason != "" {
			return false
		}
	}
	for _, edge := range snapshot.Dependencies {
		if edge.FromTaskID == taskID {
			return false
		}
	}
	for _, gate := range snapshot.Gates {
		for _, condition := range gate.Conditions {
			if condition.TaskID == taskID {
				return false
			}
		}
	}
	return true
}

func dependencyPatchFromArgs(tasks []TaskProjection, args dependencyMutationArgs, command string) (domain.DependencyPatch, error) {
	resolve := func(publicID int) (string, error) {
		task := findTaskByPublicID(tasks, publicID)
		if task == nil {
			return "", fmt.Errorf("Task #%d not found", publicID)
		}
		return task.ID, nil
	}
	convert := func(values []dependencyRefArgs) ([]domain.Dependency, error) {
		out := make([]domain.Dependency, 0, len(values))
		for _, value := range values {
			from, err := resolve(value.PredecessorTaskID)
			if err != nil {
				return nil, err
			}
			to, err := resolve(value.SuccessorTaskID)
			if err != nil {
				return nil, err
			}
			out = append(out, domain.Dependency{FromTaskID: from, ToTaskID: to})
		}
		return out, nil
	}
	if command == "dependency.connect" || command == "dependency.disconnect" {
		ref := dependencyRefArgs{PredecessorTaskID: args.PredecessorTaskID, SuccessorTaskID: args.SuccessorTaskID}
		if command == "dependency.connect" {
			args.Add = []dependencyRefArgs{ref}
		} else {
			args.Remove = []dependencyRefArgs{ref}
		}
	}
	add, err := convert(args.Add)
	if err != nil {
		return domain.DependencyPatch{}, err
	}
	remove, err := convert(args.Remove)
	if err != nil {
		return domain.DependencyPatch{}, err
	}
	updates := make([]domain.TerminalUpdate, 0, len(args.TerminalUpdates))
	for _, value := range args.TerminalUpdates {
		id, resolveErr := resolve(value.TaskID)
		if resolveErr != nil {
			return domain.DependencyPatch{}, resolveErr
		}
		updates = append(updates, domain.TerminalUpdate{TaskID: id, TerminalReason: value.TerminalReason})
	}
	return domain.DependencyPatch{Add: add, Remove: remove, TerminalUpdates: updates}, nil
}

func sameDiagnosticCodes(diagnostics []domain.Diagnostic, codes []string) bool {
	want := make([]string, 0, len(diagnostics))
	seen := map[string]bool{}
	for _, diagnostic := range diagnostics {
		if !seen[diagnostic.Code] {
			want = append(want, diagnostic.Code)
			seen[diagnostic.Code] = true
		}
	}
	left, right := append([]string(nil), want...), append([]string(nil), codes...)
	sort.Strings(left)
	sort.Strings(right)
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func domainRun(v RunProjection, workspaceID string) domain.Run {
	return domain.Run{ID: v.ID, WorkspaceID: workspaceID, TaskID: v.TaskID, ClientRunID: v.ClientRunID, Kind: domain.RunKind(v.Kind), Status: domain.RunStatus(v.Status), OperatorActorID: v.OperatorActorID, SessionRef: v.SessionRef, ParentRunID: v.ParentRunID, TargetRunID: v.TargetRunID, LeaseTokenHash: v.LeaseTokenHash, HeartbeatAt: v.HeartbeatAt, LeaseExpiresAt: v.LeaseExpiresAt, Version: v.Version, StartedAt: v.StartedAt, EndedAt: v.EndedAt, ResultSummary: v.ResultSummary, ErrorSummary: v.ErrorSummary}
}

func randomUUID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return ""
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	raw := hex.EncodeToString(value[:])
	return raw[:8] + "-" + raw[8:12] + "-" + raw[12:16] + "-" + raw[16:20] + "-" + raw[20:]
}
