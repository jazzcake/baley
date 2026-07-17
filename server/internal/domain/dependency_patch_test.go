package domain

import (
	"reflect"
	"testing"
)

func graphForPatch(t *testing.T, tasks []Task, edges []Dependency, gates []string) *WorkspaceGraph {
	t.Helper()
	graph, evaluation := NewWorkspaceGraph(tasks, edges, gates)
	if evaluation.HasErrors() {
		t.Fatalf("invalid fixture graph: %+v", evaluation)
	}
	return graph
}

func TestDependencyPatchReversesMultipleEdgesAtomically(t *testing.T) {
	tasks := []Task{{ID: "a", WorkspaceID: "w"}, {ID: "b", WorkspaceID: "w"}, {ID: "c", WorkspaceID: "w"}, {ID: "isolated", WorkspaceID: "w"}}
	graph := graphForPatch(t, tasks, []Dependency{{FromTaskID: "a", ToTaskID: "b"}, {FromTaskID: "b", ToTaskID: "c"}}, nil)
	preview := graph.ApplyPatch(DependencyPatch{
		Remove: []Dependency{{FromTaskID: "a", ToTaskID: "b"}, {FromTaskID: "b", ToTaskID: "c"}},
		Add:    []Dependency{{FromTaskID: "c", ToTaskID: "b"}, {FromTaskID: "b", ToTaskID: "a"}},
	})
	if preview.Evaluation.HasErrors() {
		t.Fatalf("valid reversal rejected: %+v", preview.Evaluation)
	}
	want := []Dependency{{FromTaskID: "b", ToTaskID: "a"}, {FromTaskID: "c", ToTaskID: "b"}}
	if got := graph.DependencyList(); !reflect.DeepEqual(got, want) {
		t.Fatalf("dependencies=%v want=%v", got, want)
	}
}

func TestDependencyPatchCycleRollsBackEntirePatch(t *testing.T) {
	graph := graphForPatch(t, []Task{{ID: "a", WorkspaceID: "w"}, {ID: "b", WorkspaceID: "w"}, {ID: "c", WorkspaceID: "w"}}, []Dependency{{FromTaskID: "a", ToTaskID: "b"}}, nil)
	before := graph.DependencyList()
	preview := graph.ApplyPatch(DependencyPatch{Add: []Dependency{{FromTaskID: "b", ToTaskID: "c"}, {FromTaskID: "c", ToTaskID: "a"}}})
	if !hasDiagnostic(preview.Evaluation.Errors, CodeDependencyCycle) {
		t.Fatalf("expected cycle: %+v", preview.Evaluation)
	}
	if got := graph.DependencyList(); !reflect.DeepEqual(got, before) {
		t.Fatalf("failed patch mutated graph: before=%v after=%v", before, got)
	}
}

func TestDependencyPatchFailureDoesNotPartiallyRemove(t *testing.T) {
	graph := graphForPatch(t, []Task{{ID: "a", WorkspaceID: "w"}, {ID: "b", WorkspaceID: "w"}, {ID: "c", WorkspaceID: "w"}}, []Dependency{{FromTaskID: "a", ToTaskID: "b"}}, nil)
	preview := graph.ApplyPatch(DependencyPatch{Remove: []Dependency{{FromTaskID: "a", ToTaskID: "b"}, {FromTaskID: "b", ToTaskID: "c"}}})
	if !preview.Evaluation.HasErrors() {
		t.Fatal("missing edge should reject patch")
	}
	if got := graph.DependencyList(); len(got) != 1 || got[0].FromTaskID != "a" {
		t.Fatalf("valid removal leaked from failed patch: %v", got)
	}
}

func TestDependencyPatchAllowsBoundariesAndWarnsForPhaseInversion(t *testing.T) {
	graph := graphForPatch(t, []Task{
		{ID: "early-server", WorkspaceID: "w", LaneID: "server", PhasePosition: 1},
		{ID: "late-client", WorkspaceID: "w", LaneID: "client", PhasePosition: 2},
	}, nil, nil)
	preview := graph.ApplyPatch(DependencyPatch{Add: []Dependency{{FromTaskID: "late-client", ToTaskID: "early-server"}}})
	if preview.Evaluation.HasErrors() || !hasDiagnostic(preview.Evaluation.Warnings, CodePhaseOrderInversion) {
		t.Fatalf("phase inversion result: %+v", preview.Evaluation)
	}
}

func TestDependencyPatchRejectsCrossWorkspace(t *testing.T) {
	graph := graphForPatch(t, []Task{{ID: "a", WorkspaceID: "w1"}, {ID: "b", WorkspaceID: "w2"}}, nil, nil)
	preview := graph.ApplyPatch(DependencyPatch{Add: []Dependency{{FromTaskID: "a", ToTaskID: "b"}}})
	if !hasDiagnostic(preview.Evaluation.Errors, CodeCrossWorkspaceDependency) {
		t.Fatalf("expected cross-workspace error: %+v", preview.Evaluation)
	}
}

func TestDependencyPatchTerminalReasonIsAtomicWithEdgesAndGateConditions(t *testing.T) {
	reason := "intentional leaf"
	graph := graphForPatch(t, []Task{{ID: "a", WorkspaceID: "w", TerminalReason: reason}, {ID: "b", WorkspaceID: "w"}}, nil, nil)
	preview := graph.ApplyPatch(DependencyPatch{TerminalUpdates: []TerminalUpdate{{TaskID: "a"}}, Add: []Dependency{{FromTaskID: "a", ToTaskID: "b"}}})
	if preview.Evaluation.HasErrors() || graph.Tasks["a"].TerminalReason != "" {
		t.Fatalf("clear + connect failed: %+v", preview)
	}

	conflicting := graphForPatch(t, []Task{{ID: "a", WorkspaceID: "w", TerminalReason: reason}, {ID: "b", WorkspaceID: "w"}}, nil, nil)
	result := conflicting.ApplyPatch(DependencyPatch{Add: []Dependency{{FromTaskID: "a", ToTaskID: "b"}}})
	if !hasDiagnostic(result.Evaluation.Errors, CodeTerminalPathConflict) || conflicting.hasOutgoing("a") {
		t.Fatalf("terminal conflict result: %+v", result)
	}

	gateGraph := graphForPatch(t, []Task{{ID: "gate-task", WorkspaceID: "w"}}, nil, []string{"gate-task"})
	result = gateGraph.SetTerminal("gate-task", reason)
	if !hasDiagnostic(result.Evaluation.Errors, CodeTerminalPathConflict) || gateGraph.Tasks["gate-task"].TerminalReason != "" {
		t.Fatalf("gate conflict result: %+v", result)
	}
}

func TestDependencyPatchProjectsRootLeafAndDanglingChanges(t *testing.T) {
	graph := graphForPatch(t, []Task{{ID: "a", WorkspaceID: "w"}, {ID: "b", WorkspaceID: "w"}, {ID: "c", WorkspaceID: "w"}}, []Dependency{{FromTaskID: "a", ToTaskID: "b"}, {FromTaskID: "b", ToTaskID: "c"}}, nil)
	preview := graph.ApplyPatch(DependencyPatch{Remove: []Dependency{{FromTaskID: "a", ToTaskID: "b"}}, Add: []Dependency{{FromTaskID: "c", ToTaskID: "a"}}})
	if !reflect.DeepEqual(preview.Diff.NewRootTaskIDs, []string{"b"}) || !reflect.DeepEqual(preview.Diff.NewLeafTaskIDs, []string{"a"}) {
		t.Fatalf("path diff: %+v", preview.Diff)
	}
	if !reflect.DeepEqual(preview.Diff.BecameDanglingTaskIDs, []string{"a"}) || !reflect.DeepEqual(preview.Diff.ResolvedDanglingTaskIDs, []string{"c"}) {
		t.Fatalf("dangling diff: %+v", preview.Diff)
	}
	if !hasDiagnostic(preview.Evaluation.Warnings, CodeDanglingPath) {
		t.Fatalf("missing dangling warning: %+v", preview.Evaluation)
	}
}

func TestWorkspaceGraphRejectsInvalidInitialGraph(t *testing.T) {
	_, evaluation := NewWorkspaceGraph([]Task{{ID: "a", WorkspaceID: "w"}, {ID: "b", WorkspaceID: "w"}}, []Dependency{{FromTaskID: "a", ToTaskID: "b"}, {FromTaskID: "a", ToTaskID: "b"}}, nil)
	if !hasDiagnostic(evaluation.Errors, CodeDuplicateDependency) {
		t.Fatalf("duplicate initial edge accepted: %+v", evaluation)
	}
}

func TestWorkspaceGraphRejectsBlankInitialTerminalReason(t *testing.T) {
	_, evaluation := NewWorkspaceGraph([]Task{{ID: "a", WorkspaceID: "w", TerminalReason: "  "}}, nil, nil)
	if !hasDiagnostic(evaluation.Errors, CodeInvalidDependencyPatch) {
		t.Fatalf("blank terminal reason accepted: %+v", evaluation)
	}
}

func TestWorkspaceGraphWarnsForInitialPhaseInversion(t *testing.T) {
	_, evaluation := NewWorkspaceGraph(
		[]Task{{ID: "early", WorkspaceID: "w", PhasePosition: 1}, {ID: "late", WorkspaceID: "w", PhasePosition: 2}},
		[]Dependency{{FromTaskID: "late", ToTaskID: "early"}}, nil,
	)
	if evaluation.HasErrors() || !hasDiagnostic(evaluation.Warnings, CodePhaseOrderInversion) {
		t.Fatalf("initial inversion result: %+v", evaluation)
	}
}
