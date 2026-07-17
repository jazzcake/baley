package domain

import "testing"

func TestGateReadyUsesOnlyExplicitConditions(t *testing.T) {
	graph := mustNewDependencyGraph(t, []Task{
		{ID: "unrelated-build", WorkspaceID: "w1", PhasePosition: 1},
		{ID: "unrelated-validate", WorkspaceID: "w1", PhasePosition: 2},
	}, nil)
	if _, err := graph.Connect("unrelated-build", "unrelated-validate"); err != nil {
		t.Fatalf("cross-phase dependency should be valid: %v", err)
	}

	conditions := []GateTaskCondition{
		{TaskID: "explicit-condition", TaskStatus: "confirmed"},
	}
	if !GateReady(conditions) {
		t.Fatal("an unrelated cross-phase dependency must not affect Gate readiness")
	}
}

func TestGateReadyRequiresEveryExplicitCondition(t *testing.T) {
	conditions := []GateTaskCondition{
		{TaskID: "confirmed", TaskStatus: "confirmed"},
		{TaskID: "passed", TaskStatus: "implemented", Passed: true},
		{TaskID: "unresolved", TaskStatus: "implemented"},
	}
	if GateReady(conditions) {
		t.Fatal("unresolved explicit condition must keep Gate open")
	}
	if GateReady(nil) {
		t.Fatal("a Gate with no explicit conditions cannot be ready")
	}
}
