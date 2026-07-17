package domain

import (
	"testing"
	"time"
)

func TestRunPolicyDistinguishesBlockingKinds(t *testing.T) {
	now := time.Now()
	task := Task{ID: "target", Status: TaskInProgress, BlockedAt: &now}
	predecessors := []Task{{ID: "pending", Status: TaskPending}}
	for _, kind := range []RunKind{RunImplementation, RunReviewResponse} {
		evaluation := EvaluateRunStart(task, kind, PhaseActive, predecessors)
		if !hasDiagnostic(evaluation.Errors, CodeBlockedTask) || !hasDiagnostic(evaluation.Errors, CodeUnresolvedDependency) {
			t.Errorf("%s should be blocked: %+v", kind, evaluation)
		}
	}
	for _, kind := range []RunKind{RunDetailedPlanning, RunIndependentAgentReview, RunCompletionReporting} {
		if evaluation := EvaluateRunStart(task, kind, PhaseActive, predecessors); evaluation.HasErrors() {
			t.Errorf("%s should ignore blocker/dependency: %+v", kind, evaluation)
		}
	}
}

func TestRunPolicyResolvesOnlyImplementedOrConfirmedPredecessors(t *testing.T) {
	task := Task{ID: "target", Status: TaskInProgress}
	resolved := []Task{{ID: "implemented", Status: TaskImplemented}, {ID: "confirmed", Status: TaskConfirmed}}
	if evaluation := EvaluateRunStart(task, RunImplementation, PhaseActive, resolved); evaluation.HasErrors() {
		t.Fatalf("resolved predecessors rejected: %+v", evaluation)
	}
	for _, status := range []TaskStatus{TaskPending, TaskInProgress, TaskDiscarded} {
		evaluation := EvaluateRunStart(task, RunImplementation, PhaseActive, []Task{{ID: string(status), Status: status}})
		if !hasDiagnostic(evaluation.Errors, CodeUnresolvedDependency) {
			t.Errorf("%s should be unresolved: %+v", status, evaluation)
		}
	}
}

func TestRunPolicyFuturePhaseAllowsOnlyDetailedPlanning(t *testing.T) {
	if evaluation := EvaluateRunStart(Task{ID: "task"}, RunDetailedPlanning, PhasePlanned, nil); evaluation.HasErrors() {
		t.Fatalf("planning rejected: %+v", evaluation)
	}
	if evaluation := EvaluateRunStart(Task{ID: "task"}, RunIndependentAgentReview, PhasePlanned, nil); !hasDiagnostic(evaluation.Errors, CodePhaseInactive) {
		t.Fatalf("future review should be rejected: %+v", evaluation)
	}
}

func TestRunPolicyAllowsImplementationAfterUnblock(t *testing.T) {
	now := time.Now()
	blocked := Task{ID: "task", Status: TaskInProgress, BlockedAt: &now, BlockerReason: "waiting"}
	unblocked, err := blocked.Unblock("resolved")
	if err != nil {
		t.Fatal(err)
	}
	if evaluation := EvaluateRunStart(unblocked, RunImplementation, PhaseActive, []Task{{ID: "predecessor", Status: TaskImplemented}}); evaluation.HasErrors() {
		t.Fatalf("unblocked task should be runnable: %+v", evaluation)
	}
}

func TestRunPolicyRejectsUnknownRunKind(t *testing.T) {
	evaluation := EvaluateRunStart(Task{ID: "task"}, RunKind("arbitrary"), PhaseActive, nil)
	if !hasDiagnostic(evaluation.Errors, CodeInvalidStateTransition) {
		t.Fatalf("unknown run kind accepted: %+v", evaluation)
	}
}

func hasDiagnostic(values []Diagnostic, code string) bool {
	for _, value := range values {
		if value.Code == code {
			return true
		}
	}
	return false
}
