package postgres

import (
	"reflect"
	"testing"

	"github.com/jazzcake/baley/server/internal/application"
)

func TestFocusWorkspaceUsesExplicitSemanticBindings(t *testing.T) {
	snapshot := application.Snapshot{
		Workspace: application.WorkspaceProjection{ID: "workspace"},
		Phases: []application.PhaseProjection{
			{ID: "intake", Position: 1}, {ID: "proof", Position: 2}, {ID: "map", Position: 3},
		},
		Lanes: []application.LaneProjection{{ID: "data"}, {ID: "web"}, {ID: "unrelated"}},
		Tasks: []application.TaskProjection{
			{ID: "audit", PublicID: 18, PhaseID: "intake", LaneID: "data"},
			{ID: "identity", PublicID: 13, PhaseID: "intake", LaneID: "data"},
			{ID: "baseline", PublicID: 2, PhaseID: "proof", LaneID: "data"},
			{ID: "unrelated", PublicID: 34, PhaseID: "proof", LaneID: "unrelated"},
		},
		Dependencies: []application.DependencyProjection{
			{FromTaskID: "audit", ToTaskID: "identity"},
			{FromTaskID: "identity", ToTaskID: "baseline"},
			{FromTaskID: "baseline", ToTaskID: "unrelated"},
		},
		Gates: []application.GateProjection{
			{ID: "g1", FromPhaseID: "intake", ToPhaseID: "proof", Conditions: []application.GateTaskProjection{{TaskID: "identity"}}},
			{ID: "g2", FromPhaseID: "proof", ToPhaseID: "map", Conditions: []application.GateTaskProjection{{TaskID: "baseline"}}},
		},
	}
	bindings := []application.BirdViewBindingProjection{
		{TargetType: "phase", TargetID: "intake"},
		{TargetType: "gate", TargetID: "g1"},
		{TargetType: "task", TargetID: "audit"},
		{TargetType: "task", TargetID: "identity"},
	}

	focus := focusWorkspace(snapshot, bindings)
	publicIDs := make([]int, 0, len(focus.Tasks))
	for _, task := range focus.Tasks {
		publicIDs = append(publicIDs, task.PublicID)
	}
	if !reflect.DeepEqual(publicIDs, []int{18, 13}) {
		t.Fatalf("focused Tasks = %v, want exact explicit set [18 13]", publicIDs)
	}
	if len(focus.Dependencies) != 1 || focus.Dependencies[0].FromTaskID != "audit" || focus.Dependencies[0].ToTaskID != "identity" {
		t.Fatalf("focused dependencies = %#v, want only truthful internal edge", focus.Dependencies)
	}
	if len(focus.Gates) != 1 || focus.Gates[0].ID != "g1" {
		t.Fatalf("focused Gates = %#v, want only explicitly bound G#1", focus.Gates)
	}
	if got := []string{focus.Phases[0].ID, focus.Phases[1].ID}; !reflect.DeepEqual(got, []string{"intake", "proof"}) {
		t.Fatalf("focused Phases = %v, want explicit and Gate boundary Phases", got)
	}
	if len(focus.Lanes) != 1 || focus.Lanes[0].ID != "data" {
		t.Fatalf("focused Lanes = %#v, want only lanes used by explicit targets", focus.Lanes)
	}
}
