package domain

import (
	"testing"
	"time"
)

func TestPlanRunStartStartsPendingTaskAtomically(t *testing.T) {
	task := Task{ID: "task", WorkspaceID: "workspace", Status: TaskPending}
	request := validRunStartRequest(RunImplementation)
	plan, evaluation := PlanRunStart(WorkspaceActive, task, PhaseActive, []Task{{ID: "predecessor", Status: TaskImplemented}}, request)
	if evaluation.HasErrors() {
		t.Fatalf("start plan rejected: %+v", evaluation)
	}
	if plan.Task.Status != TaskInProgress || plan.Run.Status != RunRunning || plan.Run.TaskID != task.ID || plan.Run.Version != 1 {
		t.Fatalf("unexpected plan: %+v", plan)
	}
	if plan.Run.LeaseTokenHash == request.LeaseToken || plan.Run.LeaseTokenHash == "" {
		t.Fatal("plan must store only the lease token hash")
	}
	if len(plan.Events) != 2 || plan.Events[0].Type != "task.started" || plan.Events[1].Type != "run.started" {
		t.Fatalf("pending Task start must plan both events: %+v", plan.Events)
	}
}

func TestPlanRunStartLeavesInProgressTaskStateUnchanged(t *testing.T) {
	task := Task{ID: "task", WorkspaceID: "workspace", Status: TaskInProgress}
	plan, evaluation := PlanRunStart(WorkspaceActive, task, PhaseActive, nil, validRunStartRequest(RunCompletionReporting))
	if evaluation.HasErrors() {
		t.Fatalf("start plan rejected: %+v", evaluation)
	}
	if plan.Task.Status != TaskInProgress || len(plan.Events) != 1 || plan.Events[0].Type != "run.started" {
		t.Fatalf("existing task should only create Run event: %+v", plan)
	}
}

func TestPlanRunStartPhaseMatrix(t *testing.T) {
	task := Task{ID: "task", WorkspaceID: "workspace", Status: TaskInProgress}
	for _, kind := range RunKinds {
		for _, phase := range []PhaseState{PhaseActive, PhaseCompleted} {
			t.Run(string(kind)+"/"+string(phase), func(t *testing.T) {
				_, evaluation := PlanRunStart(WorkspaceActive, task, phase, nil, validRunStartRequest(kind))
				if evaluation.HasErrors() {
					t.Fatalf("current/completed phase rejected: %+v", evaluation)
				}
			})
		}
	}
	for _, kind := range RunKinds {
		t.Run(string(kind)+"/planned", func(t *testing.T) {
			_, evaluation := PlanRunStart(WorkspaceActive, task, PhasePlanned, nil, validRunStartRequest(kind))
			if kind == RunDetailedPlanning {
				if evaluation.HasErrors() {
					t.Fatalf("future detailed planning rejected: %+v", evaluation)
				}
				return
			}
			if !hasDiagnostic(evaluation.Errors, CodePhaseInactive) {
				t.Fatalf("future Run should be rejected: %+v", evaluation)
			}
		})
	}
}

func TestPlanRunStartPreservesOptionalReviewTarget(t *testing.T) {
	task := Task{ID: "task", WorkspaceID: "workspace", Status: TaskInProgress}
	request := validRunStartRequest(RunIndependentAgentReview)
	request.Identity.TargetRunID = "implementation-run"
	request.TargetRun = &Run{ID: "implementation-run", WorkspaceID: "workspace", TaskID: "task", Kind: RunImplementation}
	request.SessionRef = "review-session"
	plan, evaluation := PlanRunStart(WorkspaceActive, task, PhaseActive, nil, request)
	if evaluation.HasErrors() {
		t.Fatalf("review plan rejected: %+v", evaluation)
	}
	if plan.Run.TargetRunID != "implementation-run" || plan.Run.SessionRef != "review-session" {
		t.Fatalf("review provenance lost: %+v", plan.Run)
	}
	request.Identity.TargetRunID = ""
	request.TargetRun = nil
	if _, evaluation = PlanRunStart(WorkspaceActive, task, PhaseActive, nil, request); evaluation.HasErrors() {
		t.Fatalf("review target must remain optional: %+v", evaluation)
	}
}

func TestPlanRunStartRejectsPolicyAndRequestErrorsWithoutPlan(t *testing.T) {
	now := testRunTime()
	blocked := Task{ID: "task", WorkspaceID: "workspace", Status: TaskInProgress, BlockedAt: &now, BlockerReason: "blocked"}
	plan, evaluation := PlanRunStart(WorkspaceActive, blocked, PhaseActive, []Task{{ID: "pending", Status: TaskPending}}, validRunStartRequest(RunImplementation))
	if !hasDiagnostic(evaluation.Errors, CodeBlockedTask) || !hasDiagnostic(evaluation.Errors, CodeUnresolvedDependency) || plan.Run.ID != "" {
		t.Fatalf("policy errors should return no plan: plan=%+v evaluation=%+v", plan, evaluation)
	}
	terminal := Task{ID: "task", WorkspaceID: "workspace", Status: TaskConfirmed}
	_, evaluation = PlanRunStart(WorkspaceActive, terminal, PhaseCompleted, nil, validRunStartRequest(RunDetailedPlanning))
	if !hasDiagnostic(evaluation.Errors, CodeInvalidStateTransition) {
		t.Fatalf("terminal Task accepted: %+v", evaluation)
	}
	invalid := validRunStartRequest(RunDetailedPlanning)
	invalid.LeaseToken = ""
	_, evaluation = PlanRunStart(WorkspaceActive, Task{ID: "task", WorkspaceID: "workspace", Status: TaskPending}, PhaseActive, nil, invalid)
	if !hasDiagnostic(evaluation.Errors, CodeInvalidStateTransition) {
		t.Fatalf("invalid request accepted: %+v", evaluation)
	}
}

func TestPlanRunStartRejectsClosedWorkspace(t *testing.T) {
	task := Task{ID: "task", WorkspaceID: "workspace", Status: TaskInProgress}
	for _, state := range []WorkspaceState{WorkspaceDraft, WorkspaceClosed} {
		_, evaluation := PlanRunStart(state, task, PhaseCompleted, nil, validRunStartRequest(RunDetailedPlanning))
		if !hasDiagnostic(evaluation.Errors, CodeInvalidStateTransition) {
			t.Fatalf("Workspace %s accepted: %+v", state, evaluation)
		}
	}
}

func TestPlanRunStartValidatesIndependentReviewTarget(t *testing.T) {
	task := Task{ID: "task", WorkspaceID: "workspace", Status: TaskInProgress}
	base := validRunStartRequest(RunIndependentAgentReview)
	base.Identity.TargetRunID = "target"
	tests := map[string]struct {
		targetID string
		target   *Run
	}{
		"missing":         {targetID: "target", target: nil},
		"self":            {targetID: "run", target: &Run{ID: "run", WorkspaceID: "workspace", TaskID: "task", Kind: RunImplementation}},
		"other workspace": {targetID: "target", target: &Run{ID: "target", WorkspaceID: "other", TaskID: "task", Kind: RunImplementation}},
		"other task":      {targetID: "target", target: &Run{ID: "target", WorkspaceID: "workspace", TaskID: "other", Kind: RunImplementation}},
		"wrong kind":      {targetID: "target", target: &Run{ID: "target", WorkspaceID: "workspace", TaskID: "task", Kind: RunDetailedPlanning}},
	}
	for name, testCase := range tests {
		t.Run(name, func(t *testing.T) {
			request := base
			request.Identity.TargetRunID = testCase.targetID
			request.TargetRun = testCase.target
			_, evaluation := PlanRunStart(WorkspaceActive, task, PhaseActive, nil, request)
			if !hasDiagnostic(evaluation.Errors, CodeInvalidStateTransition) {
				t.Fatalf("invalid target accepted: %+v", evaluation)
			}
		})
	}
}

func validRunStartRequest(kind RunKind) RunStartRequest {
	return RunStartRequest{
		RunID: "run",
		Identity: RunStartIdentity{
			WorkspaceID: "workspace",
			TaskID:      "task",
			ClientRunID: testClientRunID,
			Kind:        kind,
		},
		OperatorActorID: "operator",
		LeaseToken:      "lease-token",
		LeaseDuration:   5 * time.Minute,
		Now:             testRunTime(),
	}
}
