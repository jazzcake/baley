package domain

import (
	"strings"
	"time"
)

// MutationContext is the transport-independent input used by the mutation
// registry. Command adapters populate only the fields used by their command.
type MutationContext struct {
	Mode      MutationPlanMode
	ProjectID string

	Workspace  Workspace
	Phase      Phase
	FromPhase  Phase
	ToPhase    Phase
	Phases     []Phase
	Lane       Lane
	Gate       Gate
	Gates      []Gate
	Condition  GateTaskCondition
	Conditions []GateTaskCondition
	Task       Task
	Tasks      []Task
	Graph      *WorkspaceGraph
	Lanes      []Lane

	InitialPatch    DependencyPatch
	DependencyPatch DependencyPatch
	RunningTaskIDs  map[string]bool
	Predecessors    []Task
	Records         []TaskRecord

	Repository      Repository
	Record          TaskRecord
	Registration    RecordRegistration
	ExistingRecords map[string]TaskRecord
	TaskRecordsRoot string
	Commit          CommitReference
	GitObservation  RunGitObservation

	Run             Run
	RunStartRequest RunStartRequest
	ExpectedVersion int64
	RunStatus       RunStatus
	LeaseToken      string
	LeaseExtension  time.Duration

	Title, Description, Summary, NextAction string
	Assessment, Reason                      string
	CommitSHA, BlobSHA                      string
	ClearTerminal                           bool
	Now                                     time.Time
	Acknowledgement                         WarningAcknowledgement
	InitiatedByActorID                      string
	ExecutedByActorID                       string
	Attestation                             HumanApprovalAttestation
	ActiveRunCount                          int
}

type MutationHandler func(MutationContext) DomainMutationPlan

type MutationPlanMode string

const (
	MutationPreview MutationPlanMode = "preview"
	MutationExecute MutationPlanMode = "execute"
)

type HumanApprovalAttestation struct {
	ID                   string
	Action               string
	EntityType           string
	EntityID             string
	WorkspaceRevision    int64
	ApprovedByActorID    string
	ApproverRole         string
	ApprovedCommandHash  string
	DecisionSnapshotHash string
}

// MutationHandlers is the executable counterpart to MutationPolicies. Keeping
// the registry total makes a contract addition fail tests until a planner is
// supplied, instead of silently accepting metadata-only coverage.
var MutationHandlers = map[string]MutationHandler{
	"project.bootstrap":       planProjectBootstrap,
	"repository.register":     planRepositoryRegister,
	"workspace.create":        planWorkspaceCreate,
	"workspace.activate":      planWorkspaceActivation,
	"workspace.close":         planWorkspaceClose,
	"phase.create":            planPhaseCreation,
	"lane.create":             planLaneCreation,
	"lane.update":             planLaneUpdateMutation,
	"lane.close_out":          planLaneTerminationMutation("lane.close_out"),
	"lane.discard":            planLaneTerminationMutation("lane.discard"),
	"gate.create":             planGateCreation,
	"task.create":             planTaskCreation,
	"task.update":             planTaskUpdateMutation,
	"task.set_terminal":       planTaskTerminalMutation(true),
	"task.clear_terminal":     planTaskTerminalMutation(false),
	"task.block":              planTaskLifecycleMutation("task.block"),
	"task.unblock":            planTaskLifecycleMutation("task.unblock"),
	"task.report_implemented": planTaskImplementedMutation,
	"task.confirm":            planTaskConfirm,
	"task.discard":            planTaskLifecycleMutation("task.discard"),
	"task.rework":             planTaskLifecycleMutation("task.rework"),
	"dependency.connect":      planDependencyMutation("dependency.connect"),
	"dependency.disconnect":   planDependencyMutation("dependency.disconnect"),
	"dependency.patch":        planDependencyMutation("dependency.patch"),
	"gate.attach_task":        planGateAttachmentMutation(true),
	"gate.detach_task":        planGateAttachmentMutation(false),
	"gate.pass_task":          planGateConditionMutation(true),
	"gate.revoke_task_pass":   planGateConditionMutation(false),
	"gate.pass":               planGatePassMutation,
	"run.start":               planRunStartMutation,
	"run.heartbeat":           planRunHeartbeatMutation,
	"run.succeed":             planRunTerminationMutation("run.succeed", RunSucceeded),
	"run.fail":                planRunTerminationMutation("run.fail", RunFailed),
	"run.cancel":              planRunTerminationMutation("run.cancel", RunCancelled),
	"run.interrupt":           planRunTerminationMutation("run.interrupt", RunInterrupted),
	"run.correct":             planRunCorrectionMutation,
	"record.register":         planRecordRegistration,
	"record.attach_commit":    planRecordCommitAttachment,
	"commit.attach":           planCommitAttachment,
	"git.observe":             planGitObservation,
}

func PlanMutation(command string, context MutationContext) DomainMutationPlan {
	if context.Mode != MutationPreview && context.Mode != MutationExecute {
		return invalidMutationPlan(command, command, CodeInvalidStateTransition)
	}
	handler, ok := MutationHandlers[command]
	if !ok {
		return invalidMutationPlan(command, command, CodeInvalidStateTransition)
	}
	if context.Mode == MutationExecute {
		context.Acknowledgement.Enforce = true
	}
	plan := handler(context)
	if !plan.Evaluation.HasErrors() {
		plan.CommandHash = MutationCommandHash(plan, context.Workspace.Revision, canonicalMutationInput(command, context, plan))
	}
	if context.Mode == MutationExecute && !plan.Evaluation.HasErrors() {
		plan = finalizeExecutionPlan(plan, context)
	}
	return plan
}

func planProjectBootstrap(context MutationContext) DomainMutationPlan {
	plan := newDomainPlan("project.bootstrap", false)
	workspace := context.Workspace
	if strings.TrimSpace(context.ProjectID) == "" || workspace.ID == "" || strings.TrimSpace(workspace.Name) == "" || workspace.State != WorkspaceDraft || workspace.ActivePhaseID != "" || context.Repository.WorkspaceID != workspace.ID {
		return invalidPlan(plan, context.ProjectID, CodeInvalidStateTransition)
	}
	repository, err := NewRepository(context.Repository)
	if err != nil {
		return invalidPlan(plan, context.Repository.ID, violationCode(err))
	}
	plan.ProjectedDiff = map[string]any{"projectId": context.ProjectID, "workspace": workspace, "repository": repository}
	plan.Events = []PlannedEvent{
		{Type: "project.bootstrapped", EntityType: "project", EntityID: context.ProjectID, Payload: map[string]any{"projectId": context.ProjectID}},
		{Type: "workspace.created", EntityType: "workspace", EntityID: workspace.ID, Payload: map[string]any{"workspaceId": workspace.ID}},
		{Type: "repository.registered", EntityType: "repository", EntityID: repository.ID, Payload: map[string]any{"repositoryId": repository.ID}},
	}
	return plan
}

func planRepositoryRegister(context MutationContext) DomainMutationPlan {
	plan := newDomainPlan("repository.register", false)
	if context.Workspace.State == WorkspaceClosed || context.Repository.WorkspaceID != context.Workspace.ID {
		return invalidPlan(plan, context.Repository.ID, CodeInvalidStateTransition)
	}
	repository, err := NewRepository(context.Repository)
	if err != nil {
		return invalidPlan(plan, context.Repository.ID, violationCode(err))
	}
	plan.ProjectedDiff = repository
	plan.Events = []PlannedEvent{{Type: "repository.registered", EntityType: "repository", EntityID: repository.ID, Payload: map[string]any{"repositoryId": repository.ID}}}
	return plan
}

func planWorkspaceCreate(context MutationContext) DomainMutationPlan {
	plan := newDomainPlan("workspace.create", false)
	workspace := context.Workspace
	if workspace.ID == "" || strings.TrimSpace(workspace.Name) == "" || workspace.State != WorkspaceDraft || workspace.ActivePhaseID != "" {
		return invalidPlan(plan, workspace.ID, CodeInvalidStateTransition)
	}
	plan.ProjectedDiff = workspace
	plan.Events = []PlannedEvent{{Type: "workspace.created", EntityType: "workspace", EntityID: workspace.ID, Payload: map[string]any{"workspaceId": workspace.ID}}}
	return plan
}

func planWorkspaceActivation(context MutationContext) DomainMutationPlan {
	_, _, plan := PlanWorkspaceActivate(context.Workspace, context.Phases)
	return plan
}

func planWorkspaceClose(context MutationContext) DomainMutationPlan {
	plan := newDomainPlan("workspace.close", false)
	if context.Workspace.State != WorkspaceActive || context.Workspace.ActivePhaseID != context.Phase.ID || context.Phase.State != PhaseActive || context.Phase.WorkspaceID != context.Workspace.ID || context.ActiveRunCount != 0 {
		return invalidPlan(plan, context.Workspace.ID, CodeInvalidStateTransition)
	}
	for _, phase := range context.Phases {
		if phase.WorkspaceID != context.Workspace.ID || phase.Position > context.Phase.Position {
			return invalidPlan(plan, phase.ID, CodeInvalidStateTransition)
		}
	}
	for _, task := range context.Tasks {
		if task.WorkspaceID != context.Workspace.ID {
			return invalidPlan(plan, task.ID, CodeInvalidStateTransition)
		}
		if task.Status != TaskConfirmed && task.Status != TaskDiscarded {
			plan.Evaluation.Warnings = append(plan.Evaluation.Warnings, Diagnostic{Code: CodeWorkspaceCloseResidualTask, EntityID: task.ID})
		}
	}
	for _, lane := range context.Lanes {
		if lane.WorkspaceID != context.Workspace.ID {
			return invalidPlan(plan, lane.ID, CodeInvalidStateTransition)
		}
		if lane.State == LaneActive {
			plan.Evaluation.Warnings = append(plan.Evaluation.Warnings, Diagnostic{Code: CodeWorkspaceCloseActiveLane, EntityID: lane.ID})
		}
	}
	plan.Evaluation.sort()
	closed := context.Workspace
	closed.State, closed.ActivePhaseID = WorkspaceClosed, ""
	completed := context.Phase
	completed.State = PhaseCompleted
	plan.ProjectedDiff = map[string]any{"workspace": closed, "phase": completed}
	plan.Events = []PlannedEvent{
		{Type: "workspace.closed", EntityType: "workspace", EntityID: closed.ID, Payload: map[string]any{"workspaceId": closed.ID}},
		{Type: "phase.completed", EntityType: "phase", EntityID: completed.ID, Payload: map[string]any{"phaseId": completed.ID}},
	}
	return plan
}

func planPhaseCreation(context MutationContext) DomainMutationPlan {
	return PlanPhaseCreate(context.Workspace, context.Phases, context.Phase)
}

func planLaneCreation(context MutationContext) DomainMutationPlan {
	return PlanLaneCreate(context.Workspace, context.Lane)
}

func planLaneUpdateMutation(context MutationContext) DomainMutationPlan {
	_, plan := PlanLaneUpdate(context.Workspace, context.Lane, context.Title, context.Description, context.Summary)
	return plan
}

func planLaneTerminationMutation(command string) MutationHandler {
	return func(context MutationContext) DomainMutationPlan {
		_, plan := PlanLaneTermination(context.Workspace, command, context.Lane, context.Reason)
		return plan
	}
}

func planGateCreation(context MutationContext) DomainMutationPlan {
	return PlanGateCreate(context.Workspace, context.Gate, context.FromPhase, context.ToPhase, context.Gates)
}

func planTaskCreation(context MutationContext) DomainMutationPlan {
	return PlanTaskCreate(context.Workspace, context.Lane, context.Phase, context.Graph, context.Task, context.InitialPatch)
}

func planTaskUpdateMutation(context MutationContext) DomainMutationPlan {
	_, plan := PlanTaskUpdate(context.Workspace, context.Task, context.Title, context.Description, context.Summary, context.NextAction)
	return plan
}

func planTaskTerminalMutation(set bool) MutationHandler {
	return func(context MutationContext) DomainMutationPlan {
		command, eventType := "task.clear_terminal", "task.terminal_cleared"
		if set {
			command, eventType = "task.set_terminal", "task.terminal_set"
		}
		plan := newDomainPlan(command, false)
		if context.Workspace.State == WorkspaceClosed || context.Graph == nil || context.Task.WorkspaceID != context.Workspace.ID || set && strings.TrimSpace(context.Reason) == "" {
			return invalidPlan(plan, context.Task.ID, CodeInvalidStateTransition)
		}
		var reason *string
		if set {
			trimmed := strings.TrimSpace(context.Reason)
			reason = &trimmed
		}
		preview := context.Graph.PreviewPatch(DependencyPatch{TerminalUpdates: []TerminalUpdate{{TaskID: context.Task.ID, TerminalReason: reason}}})
		plan.ProjectedDiff, plan.Evaluation = preview.Diff, preview.Evaluation
		if !plan.Evaluation.HasErrors() {
			payload := map[string]any{"taskId": context.Task.ID}
			if set {
				payload["reason"] = strings.TrimSpace(context.Reason)
			}
			plan.Events = []PlannedEvent{{Type: eventType, EntityType: "task", EntityID: context.Task.ID, Payload: payload}}
		}
		return plan
	}
}

func planTaskLifecycleMutation(command string) MutationHandler {
	return func(context MutationContext) DomainMutationPlan {
		_, plan := PlanTaskMutation(context.Workspace, command, context.Task, context.Reason, context.Now)
		return plan
	}
}

func planTaskImplementedMutation(context MutationContext) DomainMutationPlan {
	if context.Workspace.State == WorkspaceClosed || context.Task.WorkspaceID != context.Workspace.ID || context.Graph == nil {
		return invalidMutationPlan("task.report_implemented", context.Task.ID, CodeInvalidStateTransition)
	}
	graphTask, exists := context.Graph.Tasks[context.Task.ID]
	if !exists || graphTask.WorkspaceID != context.Workspace.ID {
		return invalidMutationPlan("task.report_implemented", context.Task.ID, CodeInvalidStateTransition)
	}
	result := PlanTaskReportImplemented(context.Task, context.Assessment, context.Records, context.Graph.isDangling(context.Task.ID), context.Workspace.Revision+1, context.Acknowledgement)
	plan := newDomainPlan("task.report_implemented", false)
	plan.ProjectedDiff, plan.Evaluation = map[string]any{"task": result.Task, "decision": result.Decision}, result.Evaluation
	if result.Event.Type != "" {
		plan.Events = []PlannedEvent{result.Event}
	}
	return plan
}

func planTaskConfirm(context MutationContext) DomainMutationPlan {
	plan := newDomainPlan("task.confirm", false)
	if context.Workspace.State == WorkspaceClosed || context.Task.WorkspaceID != context.Workspace.ID {
		return invalidPlan(plan, context.Task.ID, CodeInvalidStateTransition)
	}
	task, err := context.Task.Confirm()
	if err != nil {
		return invalidPlan(plan, context.Task.ID, violationCode(err))
	}
	plan.ProjectedDiff = task
	plan.Events = []PlannedEvent{{Type: "task.confirmed", EntityType: "task", EntityID: task.ID, Payload: map[string]any{"taskId": task.ID}}}
	return plan
}

func planDependencyMutation(command string) MutationHandler {
	return func(context MutationContext) DomainMutationPlan {
		return PlanDependencyMutation(context.Workspace, command, context.Graph, context.DependencyPatch, context.RunningTaskIDs)
	}
}

func planGateAttachmentMutation(attach bool) MutationHandler {
	return func(context MutationContext) DomainMutationPlan {
		for _, condition := range context.Conditions {
			if condition.WorkspaceID != context.Workspace.ID || condition.GateID != context.Gate.ID {
				return invalidMutationPlan(map[bool]string{true: "gate.attach_task", false: "gate.detach_task"}[attach], condition.LinkID, CodeInvalidStateTransition)
			}
		}
		return PlanGateTaskAttachment(context.Workspace, context.Gate, context.FromPhase, context.Task, context.Conditions, attach, context.ClearTerminal)
	}
}

func planGateConditionMutation(pass bool) MutationHandler {
	return func(context MutationContext) DomainMutationPlan {
		command, eventType := "gate.revoke_task_pass", "gate.task_pass_revoked"
		if pass {
			command, eventType = "gate.pass_task", "gate.task_passed"
		}
		plan := newDomainPlan(command, false)
		if context.Workspace.State != WorkspaceActive || context.Workspace.ActivePhaseID != context.FromPhase.ID || context.Gate.PassedAt != nil || context.Gate.WorkspaceID != context.Workspace.ID || context.FromPhase.WorkspaceID != context.Workspace.ID || context.Gate.FromPhaseID != context.FromPhase.ID || context.FromPhase.State != PhaseActive || context.Condition.GateID != context.Gate.ID || context.Condition.WorkspaceID != context.Workspace.ID {
			return invalidPlan(plan, context.Condition.LinkID, CodeGateNotCurrent)
		}
		var condition GateTaskCondition
		var err error
		if pass {
			condition, err = PassGateTask(context.Condition, context.Reason)
		} else {
			condition, err = RevokeGateTaskPass(context.Condition, context.Reason)
		}
		if err != nil {
			return invalidPlan(plan, context.Condition.LinkID, violationCode(err))
		}
		plan.ProjectedDiff = condition
		plan.Events = []PlannedEvent{{Type: eventType, EntityType: "gate_task", EntityID: condition.LinkID, Payload: map[string]any{"gateTaskId": condition.LinkID, "reason": strings.TrimSpace(context.Reason)}}}
		return plan
	}
}

func planGatePassMutation(context MutationContext) DomainMutationPlan {
	plan := newDomainPlan("gate.pass", false)
	if context.Workspace.State != WorkspaceActive || context.Workspace.ActivePhaseID != context.FromPhase.ID || context.Gate.WorkspaceID != context.Workspace.ID || context.FromPhase.WorkspaceID != context.Workspace.ID || context.ToPhase.WorkspaceID != context.Workspace.ID {
		return invalidPlan(plan, context.Gate.ID, CodeGateNotCurrent)
	}
	for _, condition := range context.Conditions {
		if condition.WorkspaceID != context.Workspace.ID || condition.GateID != context.Gate.ID {
			return invalidPlan(plan, condition.LinkID, CodeGateTaskWrongPhase)
		}
	}
	transition, err := PlanGatePass(context.Gate, context.FromPhase, context.ToPhase, context.Conditions, context.Now)
	if err != nil {
		return invalidPlan(plan, context.Gate.ID, violationCode(err))
	}
	decisionSnapshotHash := GateDecisionSnapshotHash(context.Workspace, context.Gate, context.Conditions)
	var evidence GatePassEvidence
	if context.Mode == MutationExecute {
		evidence, err = ProjectGatePassEvidence(context.Gate.ID, context.Workspace.Revision+1, context.Attestation.ID, context.Conditions)
		if err != nil {
			return invalidPlan(plan, context.Gate.ID, violationCode(err))
		}
	} else {
		conditionEvidence, projectionErr := projectGatePassConditions(context.Gate.ID, context.Conditions)
		if projectionErr != nil {
			return invalidPlan(plan, context.Gate.ID, violationCode(projectionErr))
		}
		evidence = GatePassEvidence{GateID: context.Gate.ID, WorkspaceRevision: context.Workspace.Revision + 1, Conditions: conditionEvidence}
	}
	plan.DecisionSnapshotHash = decisionSnapshotHash
	plan.ProjectedDiff = map[string]any{"transition": transition, "decisionSnapshotHash": decisionSnapshotHash}
	plan.Events = []PlannedEvent{
		{Type: "gate.passed", EntityType: "gate", EntityID: context.Gate.ID, Payload: map[string]any{"gateId": evidence.GateID, "conditions": evidence.Conditions, "humanApprovalAttestationId": evidence.HumanApprovalAttestationID, "workspaceRevision": evidence.WorkspaceRevision, "decisionSnapshotHash": decisionSnapshotHash}},
		{Type: "phase.completed", EntityType: "phase", EntityID: transition.From.ID, Payload: map[string]any{"phaseId": transition.From.ID}},
		{Type: "phase.activated", EntityType: "phase", EntityID: transition.To.ID, Payload: map[string]any{"phaseId": transition.To.ID}},
	}
	return plan
}

func planRunStartMutation(context MutationContext) DomainMutationPlan {
	result, evaluation := PlanRunStart(context.Workspace.State, context.Task, context.Phase.State, context.Predecessors, context.RunStartRequest)
	plan := newDomainPlan("run.start", false)
	plan.ProjectedDiff, plan.Evaluation, plan.Events = result, evaluation, result.Events
	return plan
}

func planRunHeartbeatMutation(context MutationContext) DomainMutationPlan {
	plan := newDomainPlan("run.heartbeat", false)
	if context.Workspace.State == WorkspaceClosed || context.Run.WorkspaceID != context.Workspace.ID {
		return invalidPlan(plan, context.Run.ID, CodeInvalidStateTransition)
	}
	run, err := context.Run.Heartbeat(context.LeaseToken, context.ExpectedVersion, context.Now, context.LeaseExtension)
	if err != nil {
		return invalidPlan(plan, context.Run.ID, violationCode(err))
	}
	plan.ProjectedDiff = run
	return plan
}

func planRunTerminationMutation(command string, status RunStatus) MutationHandler {
	return func(context MutationContext) DomainMutationPlan {
		plan := newDomainPlan(command, false)
		if context.Workspace.State == WorkspaceClosed || context.Run.WorkspaceID != context.Workspace.ID {
			return invalidPlan(plan, context.Run.ID, CodeInvalidStateTransition)
		}
		run, outcome, err := context.Run.Terminate(status, context.ExpectedVersion, context.Now, context.Summary)
		if err != nil {
			return invalidPlan(plan, context.Run.ID, violationCode(err))
		}
		plan.ProjectedDiff = map[string]any{"run": run, "outcome": outcome}
		if outcome == RunTransitionApplied {
			policy, _ := policyFor(command, false)
			payload := map[string]any{"runId": run.ID}
			if status == RunSucceeded {
				payload["resultSummary"] = run.ResultSummary
			} else {
				payload["errorSummary"] = run.ErrorSummary
			}
			plan.Events = []PlannedEvent{{Type: policy.EventType, EntityType: "run", EntityID: run.ID, Payload: payload}}
		}
		return plan
	}
}

func planRunCorrectionMutation(context MutationContext) DomainMutationPlan {
	plan := newDomainPlan("run.correct", false)
	if context.Workspace.State == WorkspaceClosed || context.Run.WorkspaceID != context.Workspace.ID {
		return invalidPlan(plan, context.Run.ID, CodeInvalidStateTransition)
	}
	run, correction, err := context.Run.CorrectTerminal(context.RunStatus, context.ExpectedVersion, context.Now, context.Summary, context.Reason)
	if err != nil {
		return invalidPlan(plan, context.Run.ID, violationCode(err))
	}
	plan.ProjectedDiff = run
	plan.Events = []PlannedEvent{{Type: "run.corrected", EntityType: "run", EntityID: run.ID, Payload: map[string]any{"runId": run.ID, "previousStatus": correction.PreviousStatus, "newStatus": correction.NewStatus, "reason": correction.Reason}}}
	return plan
}

func planRecordRegistration(context MutationContext) DomainMutationPlan {
	plan := newDomainPlan("record.register", false)
	if context.Workspace.State == WorkspaceClosed || context.Registration.WorkspaceID != context.Workspace.ID {
		return invalidPlan(plan, context.Registration.ID, CodeInvalidStateTransition)
	}
	record, err := NewTaskRecord(context.TaskRecordsRoot, context.Registration, context.ExistingRecords)
	if err != nil {
		return invalidPlan(plan, context.Registration.ID, violationCode(err))
	}
	plan.ProjectedDiff = record
	plan.Events = []PlannedEvent{{Type: "record.registered", EntityType: "task_record", EntityID: record.ID, Payload: map[string]any{"recordId": record.ID, "taskId": record.TaskID, "repositoryId": record.RepositoryID, "relativePath": record.RelativePath}}}
	return plan
}

func planRecordCommitAttachment(context MutationContext) DomainMutationPlan {
	plan := newDomainPlan("record.attach_commit", false)
	if context.Workspace.State == WorkspaceClosed || context.Record.WorkspaceID != context.Workspace.ID {
		return invalidPlan(plan, context.Record.ID, CodeInvalidStateTransition)
	}
	record, outcome, err := context.Record.AttachCommit(context.CommitSHA, context.BlobSHA)
	if err != nil {
		return invalidPlan(plan, context.Record.ID, violationCode(err))
	}
	plan.ProjectedDiff = record
	if outcome == RunTransitionApplied {
		plan.Events = []PlannedEvent{{Type: "record.commit_attached", EntityType: "task_record", EntityID: record.ID, Payload: map[string]any{"recordId": record.ID, "commitSha": record.CommitSHA, "blobSha": record.BlobSHA}}}
	}
	return plan
}

func planCommitAttachment(context MutationContext) DomainMutationPlan {
	plan := newDomainPlan("commit.attach", false)
	if context.Workspace.State == WorkspaceClosed || context.Commit.WorkspaceID != context.Workspace.ID {
		return invalidPlan(plan, context.Commit.ID, CodeInvalidStateTransition)
	}
	commit, err := NewCommitReference(context.Commit)
	if err != nil {
		return invalidPlan(plan, context.Commit.ID, violationCode(err))
	}
	plan.ProjectedDiff = commit
	plan.Events = []PlannedEvent{{Type: "commit.attached", EntityType: "commit_reference", EntityID: commit.ID, Payload: map[string]any{"commitId": commit.ID, "taskId": commit.TaskID, "repositoryId": commit.RepositoryID, "commitSha": commit.CommitSHA, "relation": commit.Relation}}}
	return plan
}

func planGitObservation(context MutationContext) DomainMutationPlan {
	plan := newDomainPlan("git.observe", false)
	if context.Workspace.State == WorkspaceClosed || context.GitObservation.WorkspaceID != context.Workspace.ID {
		return invalidPlan(plan, context.GitObservation.ID, CodeInvalidStateTransition)
	}
	observation, err := NewRunGitObservation(context.GitObservation)
	if err != nil {
		return invalidPlan(plan, context.GitObservation.ID, violationCode(err))
	}
	plan.ProjectedDiff = observation
	plan.Events = []PlannedEvent{{Type: "git.observed", EntityType: "run_git_observation", EntityID: observation.ID, Payload: map[string]any{"observationId": observation.ID, "runId": observation.RunID, "repositoryId": observation.RepositoryID, "observedAt": observation.ObservedAt}}}
	return plan
}

func invalidMutationPlan(command, entityID, code string) DomainMutationPlan {
	return invalidPlan(newDomainPlan(command, false), entityID, code)
}

func invalidPlan(plan DomainMutationPlan, entityID, code string) DomainMutationPlan {
	plan.ProjectedDiff, plan.Events = nil, nil
	plan.Evaluation.Errors = []Diagnostic{{Code: code, EntityID: entityID}}
	plan.Evaluation.sort()
	return plan
}

func finalizeExecutionPlan(plan DomainMutationPlan, context MutationContext) DomainMutationPlan {
	warningCodes := make([]string, 0, len(plan.Evaluation.Warnings))
	warningSet := make(map[string]bool, len(plan.Evaluation.Warnings))
	for _, warning := range plan.Evaluation.Warnings {
		if !warningSet[warning.Code] {
			warningSet[warning.Code] = true
			warningCodes = append(warningCodes, warning.Code)
		}
	}
	if !sameUniqueStrings(warningCodes, context.Acknowledgement.Codes) {
		return invalidPlan(plan, plan.Command, CodeInvalidStateTransition)
	}
	if len(warningCodes) > 0 {
		for index := range plan.Events {
			if plan.Events[index].Type == mutationEventType(plan.Command) {
				plan.Events[index].Payload["warnings"] = append([]string{}, warningCodes...)
				plan.Events[index].Payload["acknowledgedWarningCodes"] = append([]string{}, context.Acknowledgement.Codes...)
				plan.Events[index].Payload["proceedReason"] = strings.TrimSpace(context.Acknowledgement.ProceedReason)
			}
		}
	}
	primary, ok := primaryMutationEvent(plan)
	if plan.Command == "run.heartbeat" {
		primary = PlannedEvent{EntityType: "run", EntityID: context.Run.ID}
		ok = true
	}
	if !ok {
		return invalidPlan(plan, plan.Command, CodeInvalidStateTransition)
	}
	if plan.HumanApproval != ApprovalNone {
		attestation := context.Attestation
		if attestation.ID == "" || attestation.ApprovedByActorID == "" || attestation.ApprovedCommandHash == "" {
			return invalidPlan(plan, primary.EntityID, CodeHumanApprovalRequired)
		}
		if attestation.Action != canonicalApprovalAction(plan.Command) || attestation.EntityType != primary.EntityType || attestation.EntityID != primary.EntityID || attestation.WorkspaceRevision != context.Workspace.Revision || attestation.ApprovedCommandHash != plan.CommandHash {
			return invalidPlan(plan, primary.EntityID, CodeHumanApprovalMismatch)
		}
		if plan.HumanApproval == ApprovalAlwaysOwner && attestation.ApproverRole != "owner" {
			return invalidPlan(plan, primary.EntityID, CodeHumanApprovalMismatch)
		}
		if plan.Command == "gate.pass" && (plan.DecisionSnapshotHash == "" || attestation.DecisionSnapshotHash != plan.DecisionSnapshotHash) {
			return invalidPlan(plan, primary.EntityID, CodeHumanApprovalMismatch)
		}
		plan.Events = append(plan.Events, PlannedEvent{Type: "human_approval_attestation.recorded", EntityType: "attestation", EntityID: attestation.ID, Payload: map[string]any{
			"action": attestation.Action, "entityType": attestation.EntityType, "entityId": attestation.EntityID,
			"workspaceRevision": attestation.WorkspaceRevision, "approvedByActorId": attestation.ApprovedByActorID,
			"approvedCommandHash": attestation.ApprovedCommandHash, "decisionSnapshotHash": attestation.DecisionSnapshotHash,
		}})
	}
	expectation := AuditExpectation{
		Command: plan.Command, EntityType: primary.EntityType, EntityID: primary.EntityID,
		WorkspaceRevision: context.Workspace.Revision, CommandHash: plan.CommandHash,
		DecisionSnapshotHash: plan.DecisionSnapshotHash, ActiveGate: plan.HumanApproval == ApprovalWhenFromPhaseActive,
	}
	provenance := ActorProvenance{InitiatedBy: context.InitiatedByActorID, ExecutedBy: context.ExecutedByActorID, ApprovedBy: context.Attestation.ApprovedByActorID}
	audit := ValidateCommandAudit(expectation, plan.Events, provenance)
	if audit.HasErrors() {
		plan.ProjectedDiff, plan.Events = nil, nil
		plan.Evaluation.Errors = append(plan.Evaluation.Errors, audit.Errors...)
		plan.Evaluation.sort()
	}
	return plan
}

func primaryMutationEvent(plan DomainMutationPlan) (PlannedEvent, bool) {
	want := mutationEventType(plan.Command)
	for _, event := range plan.Events {
		if event.Type == want {
			return event, true
		}
	}
	return PlannedEvent{}, false
}

func mutationEventType(command string) string {
	policy, _ := policyFor(command, false)
	return policy.EventType
}

func sameUniqueStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	seen := make(map[string]bool, len(left))
	for _, value := range left {
		if value == "" || seen[value] {
			return false
		}
		seen[value] = true
	}
	for _, value := range right {
		if !seen[value] {
			return false
		}
		delete(seen, value)
	}
	return len(seen) == 0
}

func canonicalMutationInput(command string, context MutationContext, plan DomainMutationPlan) any {
	switch command {
	case "workspace.close":
		return map[string]any{"workspaceId": context.Workspace.ID}
	case "lane.close_out", "lane.discard":
		return map[string]any{"laneId": context.Lane.ID, "reason": strings.TrimSpace(context.Reason)}
	case "task.confirm":
		return map[string]any{"taskId": context.Task.ID}
	case "task.discard":
		return map[string]any{"taskId": context.Task.ID, "reason": strings.TrimSpace(context.Reason)}
	case "gate.attach_task":
		return map[string]any{"gateId": context.Gate.ID, "taskId": context.Task.ID, "clearTerminalReason": context.ClearTerminal}
	case "gate.pass_task", "gate.revoke_task_pass":
		return map[string]any{"gateTaskId": context.Condition.LinkID, "reason": strings.TrimSpace(context.Reason)}
	case "gate.pass":
		return map[string]any{"gateId": context.Gate.ID}
	default:
		return plan.ProjectedDiff
	}
}
