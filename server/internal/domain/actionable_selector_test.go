package domain

import (
	"testing"
	"time"
)

func TestSelectActionableHandlesMultiplePredecessorsAndDisconnectedComponents(t *testing.T) {
	input := actionableFixture()
	input.Tasks = append(input.Tasks,
		Task{ID: "p2", PublicID: 2, WorkspaceID: "workspace", LaneID: "lane-a", PhaseID: "phase", Status: TaskConfirmed},
		Task{ID: "merge", PublicID: 3, WorkspaceID: "workspace", LaneID: "lane-a", PhaseID: "phase", Status: TaskPending},
		Task{ID: "disconnected", PublicID: 4, WorkspaceID: "workspace", LaneID: "lane-b", PhaseID: "phase", Status: TaskPending},
	)
	input.Dependencies = []Dependency{{FromTaskID: "p1", ToTaskID: "merge"}, {FromTaskID: "p2", ToTaskID: "merge"}}
	result, evaluation := SelectActionable(input)
	if evaluation.HasErrors() || selectionByID(result.Tasks, "merge").Actionability != TaskExecutable || selectionByID(result.Tasks, "disconnected").Actionability != TaskExecutable {
		t.Fatalf("valid components not actionable: %+v %+v", result, evaluation)
	}
}

func TestSelectActionableTreatsDiscardedPredecessorAsUnresolved(t *testing.T) {
	input := actionableFixture()
	input.Tasks[0].Status = TaskDiscarded
	input.Tasks = append(input.Tasks, Task{ID: "successor", PublicID: 2, WorkspaceID: "workspace", LaneID: "lane-a", PhaseID: "phase", Status: TaskPending})
	input.Dependencies = []Dependency{{FromTaskID: "p1", ToTaskID: "successor"}}
	result, evaluation := SelectActionable(input)
	selection := selectionByID(result.Tasks, "successor")
	if evaluation.HasErrors() || selection.Actionability != TaskPlanningOnly || !hasReason(selection.Reasons, ReasonUnresolvedDependency) || len(selection.BlockingEntityIDs) != 1 || len(selection.AllowedRunKinds) != 3 {
		t.Fatalf("discarded predecessor resolved dependency: %+v %+v", selection, evaluation)
	}
}

func TestSelectActionableAllowsNonImplementationRunsForBlockedTask(t *testing.T) {
	input := actionableFixture()
	input.Tasks[0].Status = TaskInProgress
	now := time.Now()
	input.Tasks[0].BlockedAt, input.Tasks[0].BlockerReason = &now, "waiting"
	result, evaluation := SelectActionable(input)
	if evaluation.HasErrors() || result.Tasks[0].Actionability != TaskBlocked || len(result.Tasks[0].AllowedRunKinds) != 3 {
		t.Fatalf("safe Run kinds hidden for blocker: %+v %+v", result, evaluation)
	}
}

func TestSelectActionableFuturePhaseAllowsDetailedPlanningOnly(t *testing.T) {
	input := actionableFixture()
	input.Phases = append(input.Phases, Phase{ID: "future", WorkspaceID: "workspace", Position: 2, State: PhasePlanned})
	input.Tasks = []Task{{ID: "future-task", PublicID: 2, WorkspaceID: "workspace", LaneID: "lane-a", PhaseID: "future", Status: TaskPending}}
	input.Runs = nil
	result, evaluation := SelectActionable(input)
	selection := result.Tasks[0]
	if evaluation.HasErrors() || selection.Actionability != TaskPlanningOnly || len(selection.AllowedRunKinds) != 1 || selection.AllowedRunKinds[0] != RunDetailedPlanning {
		t.Fatalf("future Task policy wrong: %+v %+v", selection, evaluation)
	}
}

func TestSelectActionableProjectsTaskAndReadyGateDecisions(t *testing.T) {
	input := actionableFixture()
	input.Tasks[0].Status = TaskImplemented
	input.Phases = append(input.Phases, Phase{ID: "future", WorkspaceID: "workspace", Position: 2, State: PhasePlanned})
	input.Gates = []Gate{{ID: "gate", WorkspaceID: "workspace", FromPhaseID: "phase", ToPhaseID: "future"}}
	input.GateConditions = map[string][]GateTaskCondition{"gate": {{WorkspaceID: "workspace", GateID: "gate", LinkID: "link", TaskID: "p1", TaskStatus: TaskImplemented, Passed: true, PassReason: "waived"}}}
	result, evaluation := SelectActionable(input)
	if evaluation.HasErrors() || result.Tasks[0].DecisionRequired != "task.confirm" || len(result.GateDecisions) != 1 || result.GateDecisions[0].DecisionRequired != "gate.pass" {
		t.Fatalf("decision projection missing: %+v %+v", result, evaluation)
	}
}

func TestSelectActionableAppliesLaneFilterAndActiveRun(t *testing.T) {
	input := actionableFixture()
	input.Tasks[0].Status = TaskPending
	input.Tasks = append(input.Tasks, Task{ID: "lane-b", PublicID: 2, WorkspaceID: "workspace", LaneID: "lane-b", PhaseID: "phase", Status: TaskPending})
	input.Runs = []Run{{ID: "run", WorkspaceID: "workspace", TaskID: "p1", Kind: RunImplementation, Status: RunRunning, StartedAt: time.Now()}}
	input.LaneID = "lane-a"
	result, evaluation := SelectActionable(input)
	if evaluation.HasErrors() || len(result.Tasks) != 1 || result.Tasks[0].Actionability != TaskBlocked || result.Tasks[0].Reasons[0] != ReasonActiveRun {
		t.Fatalf("filter or active Run policy wrong: %+v %+v", result, evaluation)
	}
}

func TestSelectActionableRejectsCrossWorkspaceInput(t *testing.T) {
	input := actionableFixture()
	input.Tasks[0].WorkspaceID = "other"
	if _, evaluation := SelectActionable(input); !evaluation.HasErrors() {
		t.Fatal("cross-Workspace Task accepted")
	}
}

func TestSelectActionableRejectsInvalidGraphAndSpoofedGateEvidence(t *testing.T) {
	input := actionableFixture()
	input.Tasks = append(input.Tasks, Task{ID: "other", PublicID: 2, WorkspaceID: "workspace", LaneID: "lane-a", PhaseID: "phase", Status: TaskPending})
	input.Dependencies = []Dependency{{FromTaskID: "p1", ToTaskID: "other"}, {FromTaskID: "other", ToTaskID: "p1"}}
	if _, evaluation := SelectActionable(input); !hasDiagnostic(evaluation.Errors, CodeDependencyCycle) {
		t.Fatalf("cyclic selector input accepted: %+v", evaluation)
	}
	input.Dependencies = nil
	input.Phases = append(input.Phases, Phase{ID: "future", WorkspaceID: "workspace", Position: 2, State: PhasePlanned})
	input.Gates = []Gate{{ID: "gate", WorkspaceID: "workspace", FromPhaseID: "phase", ToPhaseID: "future"}}
	input.GateConditions = map[string][]GateTaskCondition{"gate": {{WorkspaceID: "workspace", GateID: "gate", LinkID: "link", TaskID: "other", Passed: true}}}
	if _, evaluation := SelectActionable(input); !evaluation.HasErrors() {
		t.Fatal("Gate pass without reason accepted by selector")
	}
}

func TestSelectActionableRejectsWrongPhaseGateConditionAndTerminalTask(t *testing.T) {
	input := actionableFixture()
	input.Phases = append(input.Phases, Phase{ID: "future", WorkspaceID: "workspace", Position: 2, State: PhasePlanned})
	input.Tasks = append(input.Tasks, Task{ID: "future-task", PublicID: 2, WorkspaceID: "workspace", LaneID: "lane-a", PhaseID: "future", Status: TaskConfirmed})
	input.Gates = []Gate{{ID: "gate", WorkspaceID: "workspace", FromPhaseID: "phase", ToPhaseID: "future"}}
	input.GateConditions = map[string][]GateTaskCondition{"gate": {{WorkspaceID: "workspace", GateID: "gate", LinkID: "link", TaskID: "future-task"}}}
	if _, evaluation := SelectActionable(input); !evaluation.HasErrors() {
		t.Fatal("wrong-Phase Gate condition accepted")
	}
	input.GateConditions["gate"][0].TaskID = "p1"
	input.Tasks[0].TerminalReason = "leaf"
	if _, evaluation := SelectActionable(input); !evaluation.HasErrors() {
		t.Fatal("terminal Task Gate condition accepted")
	}
}

func TestSelectActionableValidatesWorkspacePhaseTopology(t *testing.T) {
	input := actionableFixture()
	input.Workspace.State, input.Workspace.ActivePhaseID = WorkspaceDraft, ""
	if _, evaluation := SelectActionable(input); !evaluation.HasErrors() {
		t.Fatal("draft Workspace with active Phase accepted")
	}
	input = actionableFixture()
	input.Phases = append(input.Phases, Phase{ID: "duplicate-position", WorkspaceID: "workspace", Position: 1, State: PhasePlanned})
	if _, evaluation := SelectActionable(input); !evaluation.HasErrors() {
		t.Fatal("duplicate Phase position accepted")
	}
	input = actionableFixture()
	input.Phases = append(input.Phases, Phase{ID: "past-planned", WorkspaceID: "workspace", Position: 2, State: PhaseCompleted}, Phase{ID: "future", WorkspaceID: "workspace", Position: 3, State: PhasePlanned})
	if _, evaluation := SelectActionable(input); !evaluation.HasErrors() {
		t.Fatal("completed Phase after active Phase accepted")
	}
	input = actionableFixture()
	input.Workspace.State, input.Workspace.ActivePhaseID, input.Phases, input.Tasks, input.Runs = WorkspaceClosed, "", nil, nil, nil
	if _, evaluation := SelectActionable(input); !evaluation.HasErrors() {
		t.Fatal("closed Workspace without Phases accepted")
	}
}

func TestSelectActionableRejectsAmbiguousTaskPublicIDsAndInvalidRuns(t *testing.T) {
	input := actionableFixture()
	input.Tasks = append(input.Tasks, Task{ID: "duplicate", PublicID: 1, WorkspaceID: "workspace", LaneID: "lane-a", PhaseID: "phase", Status: TaskPending})
	if _, evaluation := SelectActionable(input); !evaluation.HasErrors() {
		t.Fatal("duplicate public Task ID accepted")
	}
	input = actionableFixture()
	input.Tasks[0].PublicID = 0
	if _, evaluation := SelectActionable(input); !evaluation.HasErrors() {
		t.Fatal("non-positive public Task ID accepted")
	}
	input = actionableFixture()
	input.Runs = []Run{{ID: "invalid", WorkspaceID: "workspace", TaskID: "p1", Status: RunRunning}}
	if _, evaluation := SelectActionable(input); !evaluation.HasErrors() {
		t.Fatal("invalid active Run blocked Task")
	}
}

func TestSelectActionableRejectsAmbiguousOrInconsistentGateTopology(t *testing.T) {
	fixture := func() ActionableSelectionInput {
		input := actionableFixture()
		input.Phases = append(input.Phases, Phase{ID: "future", WorkspaceID: "workspace", Position: 2, State: PhasePlanned})
		input.Gates = []Gate{{ID: "gate-a", WorkspaceID: "workspace", FromPhaseID: "phase", ToPhaseID: "future"}}
		input.GateConditions = map[string][]GateTaskCondition{"gate-a": {{WorkspaceID: "workspace", GateID: "gate-a", LinkID: "link", TaskID: "p1"}}}
		return input
	}
	input := fixture()
	input.Gates = append(input.Gates, Gate{ID: "gate-b", WorkspaceID: "workspace", FromPhaseID: "phase", ToPhaseID: "future"})
	input.GateConditions["gate-b"] = []GateTaskCondition{{WorkspaceID: "workspace", GateID: "gate-b", LinkID: "other-link", TaskID: "p1"}}
	if _, evaluation := SelectActionable(input); !evaluation.HasErrors() {
		t.Fatal("multiple outgoing/incoming Gates accepted")
	}
	input = fixture()
	input.Phases = append(input.Phases, Phase{ID: "third", WorkspaceID: "workspace", Position: 3, State: PhasePlanned})
	input.Tasks = append(input.Tasks, Task{ID: "future-task", PublicID: 2, WorkspaceID: "workspace", LaneID: "lane-b", PhaseID: "future", Status: TaskPending})
	input.Gates = append(input.Gates, Gate{ID: "gate-b", WorkspaceID: "workspace", FromPhaseID: "future", ToPhaseID: "third"})
	input.GateConditions["gate-b"] = []GateTaskCondition{{WorkspaceID: "workspace", GateID: "gate-b", LinkID: "link", TaskID: "future-task"}}
	if _, evaluation := SelectActionable(input); !evaluation.HasErrors() {
		t.Fatal("Gate-global duplicate Link ID accepted")
	}
	input = fixture()
	passedAt := time.Now()
	input.Gates[0].PassedAt = &passedAt
	if _, evaluation := SelectActionable(input); !evaluation.HasErrors() {
		t.Fatal("passed Gate with active from Phase accepted")
	}
}

func actionableFixture() ActionableSelectionInput {
	return ActionableSelectionInput{
		Workspace: Workspace{ID: "workspace", State: WorkspaceActive, ActivePhaseID: "phase"},
		Phases:    []Phase{{ID: "phase", WorkspaceID: "workspace", Position: 1, State: PhaseActive}},
		Tasks:     []Task{{ID: "p1", PublicID: 1, WorkspaceID: "workspace", LaneID: "lane-a", PhaseID: "phase", Status: TaskConfirmed}},
		Runs:      []Run{{ID: "old", WorkspaceID: "workspace", TaskID: "p1", Kind: RunImplementation, Status: RunSucceeded, StartedAt: time.Now().Add(-time.Minute), EndedAt: timePointer(time.Now())}},
	}
}

func selectionByID(selections []TaskSelection, id string) TaskSelection {
	for _, selection := range selections {
		if selection.TaskID == id {
			return selection
		}
	}
	return TaskSelection{}
}

func hasReason(reasons []ActionabilityReason, expected ActionabilityReason) bool {
	for _, reason := range reasons {
		if reason == expected {
			return true
		}
	}
	return false
}
