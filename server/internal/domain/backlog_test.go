package domain

import "testing"

func TestBacklogLifecycleAndOrdering(t *testing.T) {
	lane := Lane{ID: "adoption", WorkspaceID: "w", State: LaneActive}
	a, err := NewBacklogItem(BacklogItem{ID: "a", WorkspaceID: "w", LaneID: lane.ID, PublicID: 1, Title: " A ", Status: BacklogActive}, lane, 1)
	if err != nil || a.Title != "A" || a.Position == nil || *a.Position != 1 {
		t.Fatalf("create: %#v %v", a, err)
	}
	b, _ := NewBacklogItem(BacklogItem{ID: "b", WorkspaceID: "w", LaneID: lane.ID, PublicID: 2, Title: "B", Status: BacklogActive}, lane, 2)
	reordered, err := ReorderBacklog([]BacklogItem{a, b}, lane.ID, []int{2, 1})
	if err != nil || *reordered[0].Position != 2 || *reordered[1].Position != 1 {
		t.Fatalf("reorder: %#v %v", reordered, err)
	}
	discarded, err := a.Discard(" no longer needed ")
	if err != nil || discarded.Status != BacklogDiscarded || discarded.Position != nil || discarded.DiscardReason != "no longer needed" {
		t.Fatalf("discard: %#v %v", discarded, err)
	}
	if _, err = discarded.Update(nil, ptr("x")); err == nil {
		t.Fatal("terminal item was mutable")
	}
	promoted, err := b.Promote("task")
	if err != nil || promoted.Status != BacklogPromoted || promoted.Position != nil || promoted.PromotedTaskID != "task" {
		t.Fatalf("promote: %#v %v", promoted, err)
	}
}

func TestBacklogReorderExactSetAndNoop(t *testing.T) {
	if _, err := ReorderBacklog(nil, "lane", []int{}); violationCodeForTest(err) != CodeBacklogOrderUnchanged {
		t.Fatalf("empty noop: %v", err)
	}
	p := 1
	items := []BacklogItem{{PublicID: 1, LaneID: "lane", Status: BacklogActive, Position: &p}}
	if _, err := ReorderBacklog(items, "lane", []int{2}); violationCodeForTest(err) != CodeBacklogOrderMismatch {
		t.Fatalf("mismatch: %v", err)
	}
}

func TestActiveBacklogBlocksLaneTerminationAndWarnsWorkspaceClose(t *testing.T) {
	position := 1
	workspace := Workspace{ID: "w", State: WorkspaceActive, ActivePhaseID: "phase", Revision: 2}
	lane := Lane{ID: "lane", WorkspaceID: "w", State: LaneActive}
	item := BacklogItem{ID: "b", WorkspaceID: "w", PublicID: 1, LaneID: "lane", Title: "B", Status: BacklogActive, Position: &position}
	lanePlan := PlanMutation("lane.close_out", MutationContext{Mode: MutationPreview, Workspace: workspace, Lane: lane, BacklogItems: []BacklogItem{item}, Reason: "done"})
	if !lanePlan.Evaluation.HasErrors() || lanePlan.Evaluation.Errors[0].Code != CodeLaneHasActiveBacklog {
		t.Fatalf("lane plan: %+v", lanePlan)
	}
	phase := Phase{ID: "phase", WorkspaceID: "w", Position: 1, State: PhaseActive}
	closePlan := PlanMutation("workspace.close", MutationContext{Mode: MutationPreview, Workspace: workspace, Phase: phase, Phases: []Phase{phase}, BacklogItems: []BacklogItem{item}})
	found := false
	for _, warning := range closePlan.Evaluation.Warnings {
		found = found || warning.Code == CodeWorkspaceCloseResidualBacklog
	}
	if closePlan.Evaluation.HasErrors() || !found {
		t.Fatalf("workspace close plan: %+v", closePlan)
	}
}

func TestBacklogMutationRegistryRejectsClosedWorkspace(t *testing.T) {
	position := 1
	item := BacklogItem{ID: "b", WorkspaceID: "w", PublicID: 1, LaneID: "lane", Title: "B", Status: BacklogActive, Position: &position}
	plan := PlanMutation("backlog.update", MutationContext{Mode: MutationPreview, Workspace: Workspace{ID: "w", State: WorkspaceClosed}, Backlog: item, Title: "changed"})
	if !plan.Evaluation.HasErrors() || plan.Evaluation.Errors[0].Code != CodeInvalidStateTransition {
		t.Fatalf("closed workspace plan: %+v", plan)
	}
}

func ptr(value string) *string { return &value }
func violationCodeForTest(err error) string {
	if v, ok := err.(*Violation); ok {
		return v.Code
	}
	return ""
}
