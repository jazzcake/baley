package domain

import "testing"

func TestBirdViewMutationHandlersAreExecutableAndAudited(t *testing.T) {
	view := BirdView{ID: "view", AccountID: "account", Title: "Release understanding", Status: BirdViewActive, Revision: 3}
	nodes := []BirdViewNode{
		{ID: "a", BirdViewID: view.ID, AccountID: view.AccountID, Title: "A", Status: BirdViewNodeActive},
		{ID: "b", BirdViewID: view.ID, AccountID: view.AccountID, Title: "B", Status: BirdViewNodeActive},
	}
	binding := BirdViewBinding{BirdViewID: view.ID, AccountID: view.AccountID, NodeID: "a", WorkspaceID: "workspace", TargetID: "task", Type: BirdViewBindingTask, State: BirdViewBindingSuggested}
	title, summary, label := "Updated", "Context", "Leads to"
	fixtures := map[string]BirdViewMutationContext{
		"bird_view.create":          {View: BirdView{ID: "created", AccountID: "account", Title: "Created", Status: BirdViewActive, Revision: 1}},
		"bird_view.update":          {View: view, Title: &title},
		"bird_view.archive":         {View: view},
		"bird_view.node.create":     {View: view, Nodes: nodes, Node: BirdViewNode{ID: "c", BirdViewID: view.ID, AccountID: view.AccountID, Title: "C", Status: BirdViewNodeActive}},
		"bird_view.node.update":     {View: view, Nodes: nodes, Node: nodes[0], Summary: &summary},
		"bird_view.node.delete":     {View: view, Nodes: nodes, Node: nodes[0]},
		"bird_view.node.achieve":    {View: view, Nodes: nodes, Node: nodes[0]},
		"bird_view.node.park":       {View: view, Nodes: nodes, Node: nodes[0]},
		"bird_view.edge.connect":    {View: view, Nodes: nodes, Edge: BirdViewEdge{ID: "edge", BirdViewID: view.ID, AccountID: view.AccountID, FromNodeID: "a", ToNodeID: "b"}},
		"bird_view.edge.update":     {View: view, Nodes: nodes, Edge: BirdViewEdge{ID: "edge", BirdViewID: view.ID, AccountID: view.AccountID, FromNodeID: "a", ToNodeID: "b"}, Label: &label},
		"bird_view.edge.disconnect": {View: view, Nodes: nodes, Edge: BirdViewEdge{ID: "edge", BirdViewID: view.ID, AccountID: view.AccountID, FromNodeID: "a", ToNodeID: "b"}},
		"bird_view.binding.pin":     {View: view, Nodes: nodes, Binding: binding, TargetVisible: true},
		"bird_view.binding.exclude": {View: view, Nodes: nodes, Binding: binding, TargetVisible: true},
		"bird_view.overlay.replace": {View: view, Nodes: nodes, Node: nodes[0], Overlay: []BirdViewBinding{binding}, Bindings: []BirdViewBinding{{BirdViewID: view.ID, AccountID: view.AccountID, NodeID: "a", WorkspaceID: "workspace", TargetID: "gate", Type: BirdViewBindingGate, State: BirdViewBindingPinned}}, TargetVisible: true},
	}
	if len(fixtures) != len(BirdViewMutationHandlers) {
		t.Fatalf("fixture count %d != handler count %d", len(fixtures), len(BirdViewMutationHandlers))
	}
	for command, fixture := range fixtures {
		t.Run(command, func(t *testing.T) {
			plan := PlanBirdViewMutation(command, fixture)
			if plan.Command != command || plan.RequiredCapability != "bird_view:operate" || plan.Evaluation.HasErrors() || len(plan.Events) != 1 {
				t.Fatalf("invalid Bird View plan: %+v", plan)
			}
			if evaluation := ValidateEventEvidence(plan.Events[0]); evaluation.HasErrors() {
				t.Fatalf("invalid Bird View event evidence: %+v", evaluation.Errors)
			}
		})
	}
}

func TestBirdViewEdgesAllowCyclesButRejectSelfAndDuplicates(t *testing.T) {
	view := BirdView{ID: "view", AccountID: "account", Title: "View", Status: BirdViewActive, Revision: 1}
	nodes := []BirdViewNode{{ID: "a", BirdViewID: view.ID, AccountID: view.AccountID}, {ID: "b", BirdViewID: view.ID, AccountID: view.AccountID}}
	existing := BirdViewEdge{ID: "a-b", BirdViewID: view.ID, AccountID: view.AccountID, FromNodeID: "a", ToNodeID: "b"}
	cycle := BirdViewEdge{ID: "b-a", BirdViewID: view.ID, AccountID: view.AccountID, FromNodeID: "b", ToNodeID: "a"}
	if plan := PlanBirdViewMutation("bird_view.edge.connect", BirdViewMutationContext{View: view, Nodes: nodes, Edges: []BirdViewEdge{existing}, Edge: cycle}); plan.Evaluation.HasErrors() {
		t.Fatalf("free-form Bird View cycle rejected: %+v", plan.Evaluation.Errors)
	}
	duplicate := existing
	duplicate.ID = "duplicate"
	if plan := PlanBirdViewMutation("bird_view.edge.connect", BirdViewMutationContext{View: view, Nodes: nodes, Edges: []BirdViewEdge{existing}, Edge: duplicate}); !plan.Evaluation.HasErrors() {
		t.Fatal("duplicate endpoints accepted")
	}
	self := BirdViewEdge{ID: "self", BirdViewID: view.ID, AccountID: view.AccountID, FromNodeID: "a", ToNodeID: "a"}
	if plan := PlanBirdViewMutation("bird_view.edge.connect", BirdViewMutationContext{View: view, Nodes: nodes, Edge: self}); !plan.Evaluation.HasErrors() {
		t.Fatal("self edge accepted")
	}
}

func TestBirdViewOverlayReplacementPreservesExplicitBindings(t *testing.T) {
	view := BirdView{ID: "view", AccountID: "account", Title: "View", Status: BirdViewActive, Revision: 1}
	node := BirdViewNode{ID: "node", BirdViewID: view.ID, AccountID: view.AccountID}
	existing := []BirdViewBinding{
		{BirdViewID: view.ID, AccountID: view.AccountID, NodeID: node.ID, WorkspaceID: "w", TargetID: "pinned", Type: BirdViewBindingTask, State: BirdViewBindingPinned},
		{BirdViewID: view.ID, AccountID: view.AccountID, NodeID: node.ID, WorkspaceID: "w", TargetID: "excluded", Type: BirdViewBindingTask, State: BirdViewBindingExcluded},
	}
	suggested := BirdViewBinding{BirdViewID: view.ID, AccountID: view.AccountID, NodeID: node.ID, WorkspaceID: "w", TargetID: "suggested", Type: BirdViewBindingTask, State: BirdViewBindingSuggested}
	plan := PlanBirdViewMutation("bird_view.overlay.replace", BirdViewMutationContext{View: view, Node: node, Nodes: []BirdViewNode{node}, Bindings: existing, Overlay: []BirdViewBinding{suggested}, TargetVisible: true})
	if plan.Evaluation.HasErrors() || plan.ProjectedDiff.(map[string]any)["preservedExplicitCount"] != 2 {
		t.Fatalf("explicit binding preservation missing: %+v", plan)
	}
}
