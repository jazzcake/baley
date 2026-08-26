package application

import "testing"

func TestTaskMoveAllowsDerivedAutomaticGateEntryToRecalculate(t *testing.T) {
	gates := []GateProjection{{
		ID: "phase-exit", FromPhaseID: "from", ToPhaseID: "to",
		EntryTasks: []GateEntryTaskProjection{{GateID: "phase-exit", TaskID: "task", SelectionSource: "automatic"}},
	}}
	if taskMoveViolatesGateBinding(gates, "task", "from") {
		t.Fatal("automatic entry task must not prevent moving back to the source Phase")
	}
}

func TestTaskMovePreservesExplicitGateBindings(t *testing.T) {
	tests := []struct {
		name  string
		gates []GateProjection
	}{
		{
			name: "explicit entry",
			gates: []GateProjection{{
				ID: "phase-exit", FromPhaseID: "from", ToPhaseID: "to",
				EntryTasks: []GateEntryTaskProjection{{GateID: "phase-exit", TaskID: "task", SelectionSource: "explicit"}},
			}},
		},
		{
			name: "gate condition",
			gates: []GateProjection{{
				ID: "phase-exit", FromPhaseID: "from", ToPhaseID: "to",
				Conditions: []GateTaskProjection{{GateID: "phase-exit", TaskID: "task"}},
			}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if !taskMoveViolatesGateBinding(test.gates, "task", "another-phase") {
				t.Fatal("explicit Gate binding must prevent cross-Phase move")
			}
		})
	}
}
