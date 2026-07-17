package domain

import (
	"errors"
	"testing"
)

func mustNewDependencyGraph(t *testing.T, tasks []Task, dependencies []Dependency) *DependencyGraph {
	t.Helper()
	graph, evaluation := NewDependencyGraph(tasks, dependencies)
	if evaluation.HasErrors() {
		t.Fatalf("invalid graph fixture: %+v", evaluation)
	}
	return graph
}

func TestDependencyGraphAllowsLaneAndPhaseBoundaries(t *testing.T) {
	tasks := []Task{
		{ID: "server-build", WorkspaceID: "w1", LaneID: "server", PhasePosition: 1},
		{ID: "client-build", WorkspaceID: "w1", LaneID: "client", PhasePosition: 1},
		{ID: "client-validate", WorkspaceID: "w1", LaneID: "client", PhasePosition: 2},
	}
	graph := mustNewDependencyGraph(t, tasks, nil)

	if diagnostics, err := graph.Connect("server-build", "client-build"); err != nil || len(diagnostics) != 0 {
		t.Fatalf("cross-lane dependency should be valid: diagnostics=%v err=%v", diagnostics, err)
	}
	if diagnostics, err := graph.Connect("client-build", "client-validate"); err != nil || len(diagnostics) != 0 {
		t.Fatalf("forward cross-phase dependency should be valid: diagnostics=%v err=%v", diagnostics, err)
	}
}

func TestDependencyGraphWarnsForPhaseOrderInversion(t *testing.T) {
	tasks := []Task{
		{ID: "build", WorkspaceID: "w1", PhasePosition: 1},
		{ID: "validate", WorkspaceID: "w1", PhasePosition: 2},
	}
	graph := mustNewDependencyGraph(t, tasks, nil)

	diagnostics, err := graph.Connect("validate", "build")
	if err != nil {
		t.Fatalf("phase-order inversion is allowed: %v", err)
	}
	if len(diagnostics) != 1 || diagnostics[0].Code != CodePhaseOrderInversion {
		t.Fatalf("expected phase_order_inversion warning, got %v", diagnostics)
	}
}

func TestDependencyGraphRejectsCycleWithoutMutating(t *testing.T) {
	tasks := []Task{
		{ID: "a", WorkspaceID: "w1"},
		{ID: "b", WorkspaceID: "w1"},
	}
	graph := mustNewDependencyGraph(t, tasks, []Dependency{{FromTaskID: "a", ToTaskID: "b"}})

	_, err := graph.Connect("b", "a")
	var violation *Violation
	if !errors.As(err, &violation) || violation.Code != CodeDependencyCycle {
		t.Fatalf("expected dependency_cycle, got %v", err)
	}
	if got := graph.Dependencies(); len(got) != 1 || got[0].FromTaskID != "a" || got[0].ToTaskID != "b" {
		t.Fatalf("rejected change mutated graph: %v", got)
	}
}

func TestDependencyGraphRejectsCrossWorkspace(t *testing.T) {
	graph := mustNewDependencyGraph(t, []Task{
		{ID: "a", WorkspaceID: "w1"},
		{ID: "b", WorkspaceID: "w2"},
	}, nil)

	_, err := graph.Connect("a", "b")
	var violation *Violation
	if !errors.As(err, &violation) || violation.Code != CodeCrossWorkspaceDependency {
		t.Fatalf("expected cross_workspace_dependency, got %v", err)
	}
}

func TestDependencyGraphRejectsSelfAndDuplicateLinks(t *testing.T) {
	tasks := []Task{
		{ID: "a", WorkspaceID: "w1"},
		{ID: "b", WorkspaceID: "w1"},
	}

	selfGraph := mustNewDependencyGraph(t, tasks, nil)
	_, err := selfGraph.Connect("a", "a")
	var violation *Violation
	if !errors.As(err, &violation) || violation.Code != CodeSelfDependency {
		t.Fatalf("expected self_dependency, got %v", err)
	}

	duplicateGraph := mustNewDependencyGraph(t, tasks, []Dependency{{FromTaskID: "a", ToTaskID: "b"}})
	_, err = duplicateGraph.Connect("a", "b")
	if !errors.As(err, &violation) || violation.Code != CodeDuplicateDependency {
		t.Fatalf("expected duplicate_dependency, got %v", err)
	}
}

func TestDependencyGraphConstructorExposesInvalidInitialGraph(t *testing.T) {
	tests := []struct {
		name         string
		tasks        []Task
		dependencies []Dependency
		code         string
	}{
		{"missing-reference", []Task{{ID: "a", WorkspaceID: "w"}}, []Dependency{{FromTaskID: "a", ToTaskID: "missing"}}, CodeNotFound},
		{"duplicate", []Task{{ID: "a", WorkspaceID: "w"}, {ID: "b", WorkspaceID: "w"}}, []Dependency{{FromTaskID: "a", ToTaskID: "b"}, {FromTaskID: "a", ToTaskID: "b"}}, CodeDuplicateDependency},
		{"cycle", []Task{{ID: "a", WorkspaceID: "w"}, {ID: "b", WorkspaceID: "w"}}, []Dependency{{FromTaskID: "a", ToTaskID: "b"}, {FromTaskID: "b", ToTaskID: "a"}}, CodeDependencyCycle},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			graph, evaluation := NewDependencyGraph(test.tasks, test.dependencies)
			if graph != nil || !hasDiagnostic(evaluation.Errors, test.code) {
				t.Fatalf("graph=%v evaluation=%+v", graph, evaluation)
			}
		})
	}
}
