package domain

import (
	"sort"
	"strings"
)

type TaskActionability string

const (
	TaskExecutable      TaskActionability = "executable"
	TaskPlanningOnly    TaskActionability = "planning_only"
	TaskDecisionWaiting TaskActionability = "decision_waiting"
	TaskBlocked         TaskActionability = "blocked"
)

type ActionabilityReason string

const (
	ReasonReadyForExecution        ActionabilityReason = "ready_for_execution"
	ReasonFuturePhasePlanningOnly  ActionabilityReason = "future_phase_planning_only"
	ReasonTaskConfirmationRequired ActionabilityReason = "task_confirmation_required"
	ReasonGatePassRequired         ActionabilityReason = "gate_pass_required"
	ReasonWorkspaceInactive        ActionabilityReason = "workspace_inactive"
	ReasonWorkspaceClosed          ActionabilityReason = "workspace_closed"
	ReasonTaskTerminal             ActionabilityReason = "task_terminal"
	ReasonTaskBlocked              ActionabilityReason = "task_blocked"
	ReasonActiveRun                ActionabilityReason = "active_run"
	ReasonUnresolvedDependency     ActionabilityReason = "unresolved_dependency"
	ReasonPhaseInactive            ActionabilityReason = "phase_inactive"
)

var TaskActionabilities = []TaskActionability{TaskExecutable, TaskPlanningOnly, TaskDecisionWaiting, TaskBlocked}

type TaskSelection struct {
	TaskID            string
	PublicID          int
	WorkspaceID       string
	LaneID            string
	PhaseID           string
	PhasePosition     int
	Actionability     TaskActionability
	Reasons           []ActionabilityReason
	BlockingEntityIDs []string
	AllowedRunKinds   []RunKind
	DecisionRequired  string
}

type GateDecisionSelection struct {
	GateID           string
	WorkspaceID      string
	Actionability    TaskActionability
	Reasons          []ActionabilityReason
	DecisionRequired string
}

type ActionableSelectionInput struct {
	Workspace      Workspace
	Phases         []Phase
	Tasks          []Task
	Dependencies   []Dependency
	Runs           []Run
	Gates          []Gate
	GateConditions map[string][]GateTaskCondition
	LaneID         string
}

type ActionableSelection struct {
	Tasks         []TaskSelection
	GateDecisions []GateDecisionSelection
}

func SelectActionable(input ActionableSelectionInput) (ActionableSelection, Evaluation) {
	evaluation := Evaluation{}
	if input.Workspace.ID == "" || !knownWorkspaceState(input.Workspace.State) {
		evaluation.Errors = append(evaluation.Errors, Diagnostic{Code: CodeInvalidStateTransition, EntityID: input.Workspace.ID})
	}
	phases := make(map[string]Phase, len(input.Phases))
	for _, phase := range input.Phases {
		if phase.ID == "" || phase.WorkspaceID != input.Workspace.ID || !knownPhaseState(phase.State) || phases[phase.ID].ID != "" {
			evaluation.Errors = append(evaluation.Errors, Diagnostic{Code: CodeInvalidStateTransition, EntityID: phase.ID})
			continue
		}
		phases[phase.ID] = phase
	}
	evaluation.Errors = append(evaluation.Errors, validateWorkspacePhaseTopology(input.Workspace, input.Phases)...)
	tasks := make(map[string]Task, len(input.Tasks))
	publicTaskIDs := make(map[int]string, len(input.Tasks))
	for _, task := range input.Tasks {
		_, duplicatePublicID := publicTaskIDs[task.PublicID]
		if task.ID == "" || task.PublicID <= 0 || duplicatePublicID || task.WorkspaceID != input.Workspace.ID || !knownTaskStatus(task.Status) || tasks[task.ID].ID != "" {
			evaluation.Errors = append(evaluation.Errors, Diagnostic{Code: CodeInvalidStateTransition, EntityID: task.ID})
			continue
		}
		if _, ok := phases[task.PhaseID]; !ok {
			evaluation.Errors = append(evaluation.Errors, Diagnostic{Code: CodeNotFound, EntityID: task.PhaseID})
		}
		tasks[task.ID] = task
		publicTaskIDs[task.PublicID] = task.ID
	}
	predecessors := make(map[string][]string)
	for _, dependency := range input.Dependencies {
		from, fromOK := tasks[dependency.FromTaskID]
		to, toOK := tasks[dependency.ToTaskID]
		if !fromOK || !toOK {
			evaluation.Errors = append(evaluation.Errors, Diagnostic{Code: CodeNotFound, EntityID: edgeID(dependency)})
			continue
		}
		if from.WorkspaceID != to.WorkspaceID {
			evaluation.Errors = append(evaluation.Errors, Diagnostic{Code: CodeCrossWorkspaceDependency, EntityID: edgeID(dependency)})
			continue
		}
		predecessors[to.ID] = append(predecessors[to.ID], from.ID)
	}
	_, graphEvaluation := NewWorkspaceGraph(input.Tasks, input.Dependencies, nil)
	evaluation.Errors = append(evaluation.Errors, graphEvaluation.Errors...)
	activeRuns := make(map[string]bool)
	for _, run := range input.Runs {
		if run.WorkspaceID != input.Workspace.ID || tasks[run.TaskID].ID == "" || !validSelectionRun(run) {
			evaluation.Errors = append(evaluation.Errors, Diagnostic{Code: CodeInvalidStateTransition, EntityID: run.ID})
			continue
		}
		if run.Status == RunRunning {
			activeRuns[run.TaskID] = true
		}
	}
	if evaluation.HasErrors() {
		evaluation.sort()
		return ActionableSelection{}, evaluation
	}

	result := ActionableSelection{}
	for _, task := range input.Tasks {
		if input.LaneID != "" && task.LaneID != input.LaneID {
			continue
		}
		selection := classifyTask(input.Workspace, phases[task.PhaseID], task, predecessors[task.ID], tasks, activeRuns[task.ID])
		result.Tasks = append(result.Tasks, selection)
	}
	gateIDs := make(map[string]bool, len(input.Gates))
	outgoingGates := make(map[string]string, len(input.Gates))
	incomingGates := make(map[string]string, len(input.Gates))
	gateLinkIDs := make(map[string]string)
	for _, gate := range input.Gates {
		from, fromOK := phases[gate.FromPhaseID]
		to, toOK := phases[gate.ToPhaseID]
		passedStateValid := gate.PassedAt == nil && from.State != PhaseCompleted || gate.PassedAt != nil && !gate.PassedAt.IsZero() && from.State == PhaseCompleted && (to.State == PhaseActive || to.State == PhaseCompleted)
		if gate.ID == "" || gateIDs[gate.ID] || outgoingGates[gate.FromPhaseID] != "" || incomingGates[gate.ToPhaseID] != "" || gate.WorkspaceID != input.Workspace.ID || !fromOK || !toOK || to.Position != from.Position+1 || !passedStateValid {
			evaluation.Errors = append(evaluation.Errors, Diagnostic{Code: CodeInvalidStateTransition, EntityID: gate.ID})
			continue
		}
		gateIDs[gate.ID] = true
		outgoingGates[gate.FromPhaseID] = gate.ID
		incomingGates[gate.ToPhaseID] = gate.ID
		conditions := append([]GateTaskCondition(nil), input.GateConditions[gate.ID]...)
		validConditions := true
		seenLinks, seenTasks := map[string]bool{}, map[string]bool{}
		for index := range conditions {
			task, ok := tasks[conditions[index].TaskID]
			invalidPassEvidence := conditions[index].Passed && strings.TrimSpace(conditions[index].PassReason) == "" || !conditions[index].Passed && strings.TrimSpace(conditions[index].PassReason) != ""
			if !ok || task.PhaseID != gate.FromPhaseID || task.TerminalReason != "" || conditions[index].WorkspaceID != input.Workspace.ID || conditions[index].GateID != gate.ID || conditions[index].LinkID == "" || seenLinks[conditions[index].LinkID] || gateLinkIDs[conditions[index].LinkID] != "" || seenTasks[conditions[index].TaskID] || invalidPassEvidence {
				evaluation.Errors = append(evaluation.Errors, Diagnostic{Code: CodeInvalidStateTransition, EntityID: conditions[index].LinkID})
				validConditions = false
				continue
			}
			seenLinks[conditions[index].LinkID], seenTasks[conditions[index].TaskID] = true, true
			gateLinkIDs[conditions[index].LinkID] = gate.ID
			conditions[index].TaskStatus = task.Status
		}
		if !validConditions {
			continue
		}
		if input.LaneID != "" && !gateTouchesLane(conditions, tasks, input.LaneID) {
			continue
		}
		if input.Workspace.State == WorkspaceActive && input.Workspace.ActivePhaseID == gate.FromPhaseID && GateStatusFor(gate, conditions) == GateReadyStatus {
			result.GateDecisions = append(result.GateDecisions, GateDecisionSelection{GateID: gate.ID, WorkspaceID: gate.WorkspaceID, Actionability: TaskDecisionWaiting, Reasons: []ActionabilityReason{ReasonGatePassRequired}, DecisionRequired: "gate.pass"})
		}
	}
	for gateID := range input.GateConditions {
		if !gateIDs[gateID] {
			evaluation.Errors = append(evaluation.Errors, Diagnostic{Code: CodeInvalidStateTransition, EntityID: gateID})
		}
	}
	evaluation.sort()
	if evaluation.HasErrors() {
		return ActionableSelection{}, evaluation
	}
	sort.Slice(result.Tasks, func(i, j int) bool {
		if result.Tasks[i].PhasePosition != result.Tasks[j].PhasePosition {
			return result.Tasks[i].PhasePosition < result.Tasks[j].PhasePosition
		}
		if result.Tasks[i].PublicID != result.Tasks[j].PublicID {
			return result.Tasks[i].PublicID < result.Tasks[j].PublicID
		}
		return result.Tasks[i].TaskID < result.Tasks[j].TaskID
	})
	sort.Slice(result.GateDecisions, func(i, j int) bool { return result.GateDecisions[i].GateID < result.GateDecisions[j].GateID })
	return result, evaluation
}

func knownWorkspaceState(value WorkspaceState) bool {
	for _, candidate := range WorkspaceStates {
		if value == candidate {
			return true
		}
	}
	return false
}

func knownPhaseState(value PhaseState) bool {
	return value == PhasePlanned || value == PhaseActive || value == PhaseCompleted
}

func knownTaskStatus(value TaskStatus) bool {
	for _, candidate := range TaskStatuses {
		if value == candidate {
			return true
		}
	}
	return false
}

func knownRunStatus(value RunStatus) bool { return value == RunRunning || isTerminalRunStatus(value) }

func validSelectionRun(run Run) bool {
	if run.ID == "" || run.TaskID == "" || !validRunKind(run.Kind) || run.StartedAt.IsZero() || !knownRunStatus(run.Status) {
		return false
	}
	if run.Status == RunRunning {
		return run.EndedAt == nil
	}
	return run.EndedAt != nil && !run.EndedAt.Before(run.StartedAt)
}

func validateWorkspacePhaseTopology(workspace Workspace, phases []Phase) []Diagnostic {
	if len(phases) == 0 {
		if workspace.State == WorkspaceActive || workspace.State == WorkspaceClosed {
			return []Diagnostic{{Code: CodeInvalidStateTransition, EntityID: workspace.ID}}
		}
		return nil
	}
	ordered := append([]Phase(nil), phases...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Position < ordered[j].Position })
	for index, phase := range ordered {
		if phase.Position != index+1 {
			return []Diagnostic{{Code: CodeInvalidStateTransition, EntityID: phase.ID}}
		}
	}
	switch workspace.State {
	case WorkspaceDraft:
		if workspace.ActivePhaseID != "" {
			return []Diagnostic{{Code: CodeInvalidStateTransition, EntityID: workspace.ID}}
		}
		for _, phase := range ordered {
			if phase.State != PhasePlanned {
				return []Diagnostic{{Code: CodeInvalidStateTransition, EntityID: phase.ID}}
			}
		}
	case WorkspaceActive:
		activeIndex := -1
		for index, phase := range ordered {
			if phase.ID == workspace.ActivePhaseID {
				activeIndex = index
			}
		}
		if activeIndex < 0 {
			return []Diagnostic{{Code: CodeInvalidStateTransition, EntityID: workspace.ActivePhaseID}}
		}
		for index, phase := range ordered {
			expected := PhasePlanned
			if index < activeIndex {
				expected = PhaseCompleted
			} else if index == activeIndex {
				expected = PhaseActive
			}
			if phase.State != expected {
				return []Diagnostic{{Code: CodeInvalidStateTransition, EntityID: phase.ID}}
			}
		}
	case WorkspaceClosed:
		if workspace.ActivePhaseID != "" {
			return []Diagnostic{{Code: CodeInvalidStateTransition, EntityID: workspace.ID}}
		}
		for _, phase := range ordered {
			if phase.State != PhaseCompleted {
				return []Diagnostic{{Code: CodeInvalidStateTransition, EntityID: phase.ID}}
			}
		}
	}
	return nil
}

func classifyTask(workspace Workspace, phase Phase, task Task, predecessorIDs []string, tasks map[string]Task, activeRun bool) TaskSelection {
	selection := TaskSelection{TaskID: task.ID, PublicID: task.PublicID, WorkspaceID: task.WorkspaceID, LaneID: task.LaneID, PhaseID: task.PhaseID, PhasePosition: phase.Position}
	if workspace.State != WorkspaceActive {
		reason := ReasonWorkspaceInactive
		if workspace.State == WorkspaceClosed {
			reason = ReasonWorkspaceClosed
		}
		selection.Actionability, selection.Reasons = TaskBlocked, []ActionabilityReason{reason}
		return selection
	}
	if task.Status == TaskConfirmed || task.Status == TaskDiscarded {
		selection.Actionability, selection.Reasons = TaskBlocked, []ActionabilityReason{ReasonTaskTerminal}
		return selection
	}
	if activeRun {
		selection.Actionability, selection.Reasons = TaskBlocked, []ActionabilityReason{ReasonActiveRun}
		return selection
	}
	if task.BlockedAt != nil {
		selection.Actionability, selection.Reasons = TaskBlocked, []ActionabilityReason{ReasonTaskBlocked}
		selection.AllowedRunKinds = nonImplementationRunKinds()
		return selection
	}
	if task.Status == TaskImplemented {
		selection.Actionability = TaskDecisionWaiting
		selection.Reasons = []ActionabilityReason{ReasonTaskConfirmationRequired}
		selection.DecisionRequired = "task.confirm"
		return selection
	}
	for _, predecessorID := range predecessorIDs {
		status := tasks[predecessorID].Status
		if status != TaskImplemented && status != TaskConfirmed {
			selection.BlockingEntityIDs = append(selection.BlockingEntityIDs, predecessorID)
		}
	}
	sort.Strings(selection.BlockingEntityIDs)
	if phase.State == PhasePlanned {
		selection.Actionability = TaskPlanningOnly
		selection.Reasons = []ActionabilityReason{ReasonFuturePhasePlanningOnly}
		if len(selection.BlockingEntityIDs) != 0 {
			selection.Reasons = append(selection.Reasons, ReasonUnresolvedDependency)
		}
		selection.AllowedRunKinds = []RunKind{RunDetailedPlanning}
		return selection
	}
	if phase.State != PhaseActive && phase.State != PhaseCompleted {
		selection.Actionability, selection.Reasons = TaskBlocked, []ActionabilityReason{ReasonPhaseInactive}
		return selection
	}
	if len(selection.BlockingEntityIDs) != 0 {
		selection.Actionability = TaskPlanningOnly
		selection.Reasons = []ActionabilityReason{ReasonUnresolvedDependency}
		selection.AllowedRunKinds = nonImplementationRunKinds()
		return selection
	}
	selection.Actionability = TaskExecutable
	selection.Reasons = []ActionabilityReason{ReasonReadyForExecution}
	selection.AllowedRunKinds = []RunKind{RunDetailedPlanning, RunImplementation, RunIndependentAgentReview, RunCompletionReporting}
	if task.Status == TaskInProgress {
		selection.AllowedRunKinds = append(selection.AllowedRunKinds, RunReviewResponse)
	}
	return selection
}

func nonImplementationRunKinds() []RunKind {
	return []RunKind{RunDetailedPlanning, RunIndependentAgentReview, RunCompletionReporting}
}

func gateTouchesLane(conditions []GateTaskCondition, tasks map[string]Task, laneID string) bool {
	for _, condition := range conditions {
		if tasks[condition.TaskID].LaneID == laneID {
			return true
		}
	}
	return false
}
