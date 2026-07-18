package domain

import (
	"testing"
	"time"
)

func TestGateStatusAndTransition(t *testing.T) {
	gate := Gate{ID: "g", WorkspaceID: "w", FromPhaseID: "p1", ToPhaseID: "p2"}
	conditions := []GateTaskCondition{{LinkID: "l1", TaskID: "t1", TaskStatus: TaskConfirmed}, {LinkID: "l2", TaskID: "t2", TaskStatus: TaskImplemented, Passed: true}}
	if got := GateStatusFor(gate, conditions); got != GateReadyStatus {
		t.Fatalf("status=%s", got)
	}
	now := time.Now()
	transition, err := PlanGatePass(gate, Phase{ID: "p1", Position: 0, State: PhaseActive}, Phase{ID: "p2", Position: 1, State: PhasePlanned}, conditions, now)
	if err != nil {
		t.Fatal(err)
	}
	if transition.From.State != PhaseCompleted || transition.To.State != PhaseActive {
		t.Fatalf("bad transition: %#v", transition)
	}
}

func TestGateWithNoConditionsIsNotReady(t *testing.T) {
	if GateReady(nil) {
		t.Fatal("empty gate must not be ready")
	}
	_, err := PlanGatePass(Gate{FromPhaseID: "p1", ToPhaseID: "p2"}, Phase{ID: "p1", Position: 0, State: PhaseActive}, Phase{ID: "p2", Position: 1, State: PhasePlanned}, nil, time.Now())
	if err == nil {
		t.Fatal("expected error")
	}
}
