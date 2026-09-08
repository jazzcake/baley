package domain

import (
	"math"
	"sort"
	"strings"
)

type BirdViewStatus string
type BirdViewNodeStatus string
type BirdViewBindingState string
type BirdViewBindingType string

const (
	BirdViewActive   BirdViewStatus = "active"
	BirdViewArchived BirdViewStatus = "archived"

	BirdViewNodeActive   BirdViewNodeStatus = "active"
	BirdViewNodeAchieved BirdViewNodeStatus = "achieved"
	BirdViewNodeParked   BirdViewNodeStatus = "parked"

	BirdViewBindingSuggested BirdViewBindingState = "suggested"
	BirdViewBindingPinned    BirdViewBindingState = "pinned"
	BirdViewBindingExcluded  BirdViewBindingState = "excluded"

	BirdViewBindingPhase   BirdViewBindingType = "phase"
	BirdViewBindingGate    BirdViewBindingType = "gate"
	BirdViewBindingTask    BirdViewBindingType = "task"
	BirdViewBindingBacklog BirdViewBindingType = "backlog"
)

type BirdView struct {
	ID          string         `json:"id"`
	AccountID   string         `json:"accountId"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Status      BirdViewStatus `json:"status"`
	Revision    int64          `json:"revision"`
}

type BirdViewNode struct {
	ID         string             `json:"id"`
	BirdViewID string             `json:"birdViewId"`
	AccountID  string             `json:"accountId"`
	Title      string             `json:"title"`
	Summary    string             `json:"summary"`
	Content    string             `json:"content"`
	Status     BirdViewNodeStatus `json:"status"`
	PositionX  float64            `json:"positionX"`
	PositionY  float64            `json:"positionY"`
}

type BirdViewEdge struct {
	ID         string `json:"id"`
	BirdViewID string `json:"birdViewId"`
	AccountID  string `json:"accountId"`
	FromNodeID string `json:"fromNodeId"`
	ToNodeID   string `json:"toNodeId"`
	Label      string `json:"label"`
}

type BirdViewBinding struct {
	BirdViewID  string               `json:"birdViewId"`
	AccountID   string               `json:"accountId"`
	NodeID      string               `json:"nodeId"`
	WorkspaceID string               `json:"workspaceId"`
	TargetID    string               `json:"targetId"`
	Type        BirdViewBindingType  `json:"targetType"`
	State       BirdViewBindingState `json:"state"`
}

type BirdViewMutationContext struct {
	View          BirdView
	Node          BirdViewNode
	Nodes         []BirdViewNode
	Edge          BirdViewEdge
	Edges         []BirdViewEdge
	Binding       BirdViewBinding
	Bindings      []BirdViewBinding
	Overlay       []BirdViewBinding
	Title         *string
	Description   *string
	Summary       *string
	Content       *string
	Label         *string
	PositionX     *float64
	PositionY     *float64
	TargetVisible bool
}

type BirdViewMutationHandler func(BirdViewMutationContext) DomainMutationPlan

var BirdViewMutationHandlers = map[string]BirdViewMutationHandler{
	"bird_view.create":          planBirdViewCreate,
	"bird_view.update":          planBirdViewUpdate,
	"bird_view.archive":         planBirdViewArchive,
	"bird_view.node.create":     planBirdViewNodeCreate,
	"bird_view.node.update":     planBirdViewNodeUpdate,
	"bird_view.node.delete":     planBirdViewNodeDelete,
	"bird_view.node.achieve":    planBirdViewNodeStatus(BirdViewNodeAchieved, "bird_view.node.achieved"),
	"bird_view.node.park":       planBirdViewNodeStatus(BirdViewNodeParked, "bird_view.node.parked"),
	"bird_view.edge.connect":    planBirdViewEdgeConnect,
	"bird_view.edge.update":     planBirdViewEdgeUpdate,
	"bird_view.edge.disconnect": planBirdViewEdgeDisconnect,
	"bird_view.binding.pin":     planBirdViewBinding(BirdViewBindingPinned, "bird_view.binding.pinned"),
	"bird_view.binding.exclude": planBirdViewBinding(BirdViewBindingExcluded, "bird_view.binding.excluded"),
	"bird_view.overlay.replace": planBirdViewOverlayReplace,
}

func PlanBirdViewMutation(command string, context BirdViewMutationContext) DomainMutationPlan {
	handler := BirdViewMutationHandlers[command]
	if handler == nil {
		return invalidMutationPlan(command, command, CodeInvalidStateTransition)
	}
	return handler(context)
}

func validateActiveBirdView(view BirdView) *Diagnostic {
	if view.ID == "" || view.AccountID == "" || view.Status == BirdViewArchived {
		return &Diagnostic{Code: CodeBirdViewArchived, EntityID: view.ID}
	}
	if view.Status != BirdViewActive || view.Revision <= 0 {
		return &Diagnostic{Code: CodeInvalidStateTransition, EntityID: view.ID}
	}
	return nil
}

func planBirdViewCreate(context BirdViewMutationContext) DomainMutationPlan {
	plan := newDomainPlan("bird_view.create", false)
	view := context.View
	view.Title = strings.TrimSpace(view.Title)
	view.Description = strings.TrimSpace(view.Description)
	if view.ID == "" || view.AccountID == "" || view.Title == "" || view.Status != BirdViewActive || view.Revision != 1 {
		return invalidPlan(plan, view.ID, CodeInvalidStateTransition)
	}
	plan.ProjectedDiff = view
	plan.Events = []PlannedEvent{{Type: "bird_view.created", EntityType: "bird_view", EntityID: view.ID, Payload: map[string]any{"birdViewId": view.ID, "title": view.Title}}}
	return plan
}

func planBirdViewUpdate(context BirdViewMutationContext) DomainMutationPlan {
	plan := newDomainPlan("bird_view.update", false)
	if diagnostic := validateActiveBirdView(context.View); diagnostic != nil {
		return invalidPlan(plan, diagnostic.EntityID, diagnostic.Code)
	}
	if context.Title == nil && context.Description == nil {
		return invalidPlan(plan, context.View.ID, CodeInvalidStateTransition)
	}
	next := context.View
	if context.Title != nil {
		next.Title = strings.TrimSpace(*context.Title)
		if next.Title == "" {
			return invalidPlan(plan, next.ID, CodeInvalidStateTransition)
		}
	}
	if context.Description != nil {
		next.Description = strings.TrimSpace(*context.Description)
	}
	plan.ProjectedDiff = map[string]any{"before": context.View, "after": next}
	plan.Events = []PlannedEvent{{Type: "bird_view.updated", EntityType: "bird_view", EntityID: next.ID, Payload: map[string]any{"birdViewId": next.ID, "before": context.View, "after": next}}}
	return plan
}

func planBirdViewArchive(context BirdViewMutationContext) DomainMutationPlan {
	plan := newDomainPlan("bird_view.archive", false)
	if diagnostic := validateActiveBirdView(context.View); diagnostic != nil {
		return invalidPlan(plan, diagnostic.EntityID, diagnostic.Code)
	}
	next := context.View
	next.Status = BirdViewArchived
	plan.ProjectedDiff = next
	plan.Events = []PlannedEvent{{Type: "bird_view.archived", EntityType: "bird_view", EntityID: next.ID, Payload: map[string]any{"birdViewId": next.ID}}}
	return plan
}

func planBirdViewNodeCreate(context BirdViewMutationContext) DomainMutationPlan {
	plan := newDomainPlan("bird_view.node.create", false)
	if diagnostic := validateActiveBirdView(context.View); diagnostic != nil {
		return invalidPlan(plan, diagnostic.EntityID, diagnostic.Code)
	}
	node := context.Node
	node.Title, node.Summary, node.Content = strings.TrimSpace(node.Title), strings.TrimSpace(node.Summary), strings.TrimSpace(node.Content)
	if node.ID == "" || node.BirdViewID != context.View.ID || node.AccountID != context.View.AccountID || node.Title == "" || node.Status != BirdViewNodeActive || !validBirdViewPosition(node.PositionX, node.PositionY) || findBirdViewNode(context.Nodes, node.ID) != nil {
		return invalidPlan(plan, node.ID, CodeInvalidStateTransition)
	}
	plan.ProjectedDiff = node
	plan.Events = []PlannedEvent{{Type: "bird_view.node.created", EntityType: "bird_view_node", EntityID: node.ID, Payload: map[string]any{"nodeId": node.ID, "birdViewId": node.BirdViewID}}}
	return plan
}

func planBirdViewNodeUpdate(context BirdViewMutationContext) DomainMutationPlan {
	plan := newDomainPlan("bird_view.node.update", false)
	if diagnostic := validateActiveBirdView(context.View); diagnostic != nil {
		return invalidPlan(plan, diagnostic.EntityID, diagnostic.Code)
	}
	positionChanged := context.PositionX != nil || context.PositionY != nil
	if context.Node.BirdViewID != context.View.ID || context.Title == nil && context.Summary == nil && context.Content == nil && !positionChanged || positionChanged && (context.PositionX == nil || context.PositionY == nil) {
		return invalidPlan(plan, context.Node.ID, CodeInvalidStateTransition)
	}
	next := context.Node
	if context.Title != nil {
		next.Title = strings.TrimSpace(*context.Title)
		if next.Title == "" {
			return invalidPlan(plan, next.ID, CodeInvalidStateTransition)
		}
	}
	if context.Summary != nil {
		next.Summary = strings.TrimSpace(*context.Summary)
	}
	if context.Content != nil {
		next.Content = strings.TrimSpace(*context.Content)
	}
	if context.PositionX != nil && context.PositionY != nil {
		if !validBirdViewPosition(*context.PositionX, *context.PositionY) {
			return invalidPlan(plan, next.ID, CodeInvalidStateTransition)
		}
		next.PositionX, next.PositionY = *context.PositionX, *context.PositionY
	}
	plan.ProjectedDiff = map[string]any{"before": context.Node, "after": next}
	plan.Events = []PlannedEvent{{Type: "bird_view.node.updated", EntityType: "bird_view_node", EntityID: next.ID, Payload: map[string]any{"nodeId": next.ID, "before": context.Node, "after": next}}}
	return plan
}

func planBirdViewNodeDelete(context BirdViewMutationContext) DomainMutationPlan {
	plan := newDomainPlan("bird_view.node.delete", false)
	if diagnostic := validateActiveBirdView(context.View); diagnostic != nil {
		return invalidPlan(plan, diagnostic.EntityID, diagnostic.Code)
	}
	if context.Node.ID == "" || context.Node.BirdViewID != context.View.ID {
		return invalidPlan(plan, context.Node.ID, CodeNotFound)
	}
	plan.ProjectedDiff = context.Node
	plan.Events = []PlannedEvent{{Type: "bird_view.node.deleted", EntityType: "bird_view_node", EntityID: context.Node.ID, Payload: map[string]any{"nodeId": context.Node.ID, "birdViewId": context.View.ID}}}
	return plan
}

func planBirdViewNodeStatus(status BirdViewNodeStatus, eventType string) BirdViewMutationHandler {
	return func(context BirdViewMutationContext) DomainMutationPlan {
		command := "bird_view.node.achieve"
		if status == BirdViewNodeParked {
			command = "bird_view.node.park"
		}
		plan := newDomainPlan(command, false)
		if diagnostic := validateActiveBirdView(context.View); diagnostic != nil {
			return invalidPlan(plan, diagnostic.EntityID, diagnostic.Code)
		}
		if context.Node.BirdViewID != context.View.ID || context.Node.ID == "" {
			return invalidPlan(plan, context.Node.ID, CodeNotFound)
		}
		next := context.Node
		next.Status = status
		plan.ProjectedDiff = next
		plan.Events = []PlannedEvent{{Type: eventType, EntityType: "bird_view_node", EntityID: next.ID, Payload: map[string]any{"nodeId": next.ID, "status": status}}}
		return plan
	}
}

func planBirdViewEdgeConnect(context BirdViewMutationContext) DomainMutationPlan {
	plan := newDomainPlan("bird_view.edge.connect", false)
	if diagnostic := validateActiveBirdView(context.View); diagnostic != nil {
		return invalidPlan(plan, diagnostic.EntityID, diagnostic.Code)
	}
	edge := context.Edge
	edge.Label = strings.TrimSpace(edge.Label)
	if edge.ID == "" || edge.BirdViewID != context.View.ID || edge.AccountID != context.View.AccountID || edge.FromNodeID == edge.ToNodeID || findBirdViewNode(context.Nodes, edge.FromNodeID) == nil || findBirdViewNode(context.Nodes, edge.ToNodeID) == nil {
		return invalidPlan(plan, edge.ID, CodeInvalidBirdViewEdge)
	}
	for _, existing := range context.Edges {
		if existing.ID == edge.ID || existing.FromNodeID == edge.FromNodeID && existing.ToNodeID == edge.ToNodeID {
			return invalidPlan(plan, edge.ID, CodeInvalidBirdViewEdge)
		}
	}
	plan.ProjectedDiff = edge
	plan.Events = []PlannedEvent{{Type: "bird_view.edge.connected", EntityType: "bird_view_edge", EntityID: edge.ID, Payload: map[string]any{"edgeId": edge.ID, "fromNodeId": edge.FromNodeID, "toNodeId": edge.ToNodeID}}}
	return plan
}

func planBirdViewEdgeDisconnect(context BirdViewMutationContext) DomainMutationPlan {
	plan := newDomainPlan("bird_view.edge.disconnect", false)
	if diagnostic := validateActiveBirdView(context.View); diagnostic != nil {
		return invalidPlan(plan, diagnostic.EntityID, diagnostic.Code)
	}
	if context.Edge.ID == "" || context.Edge.BirdViewID != context.View.ID {
		return invalidPlan(plan, context.Edge.ID, CodeNotFound)
	}
	plan.ProjectedDiff = context.Edge
	plan.Events = []PlannedEvent{{Type: "bird_view.edge.disconnected", EntityType: "bird_view_edge", EntityID: context.Edge.ID, Payload: map[string]any{"edgeId": context.Edge.ID}}}
	return plan
}

func planBirdViewEdgeUpdate(context BirdViewMutationContext) DomainMutationPlan {
	plan := newDomainPlan("bird_view.edge.update", false)
	if diagnostic := validateActiveBirdView(context.View); diagnostic != nil {
		return invalidPlan(plan, diagnostic.EntityID, diagnostic.Code)
	}
	if context.Edge.ID == "" || context.Edge.BirdViewID != context.View.ID || context.Label == nil {
		return invalidPlan(plan, context.Edge.ID, CodeNotFound)
	}
	next := context.Edge
	next.Label = strings.TrimSpace(*context.Label)
	plan.ProjectedDiff = map[string]any{"before": context.Edge, "after": next}
	plan.Events = []PlannedEvent{{Type: "bird_view.edge.updated", EntityType: "bird_view_edge", EntityID: next.ID, Payload: map[string]any{"edgeId": next.ID, "before": context.Edge, "after": next}}}
	return plan
}

func validBirdViewPosition(x, y float64) bool {
	return !math.IsNaN(x) && !math.IsNaN(y) && !math.IsInf(x, 0) && !math.IsInf(y, 0) && math.Abs(x) <= 1_000_000 && math.Abs(y) <= 1_000_000
}

func planBirdViewBinding(state BirdViewBindingState, eventType string) BirdViewMutationHandler {
	return func(context BirdViewMutationContext) DomainMutationPlan {
		command := "bird_view.binding.pin"
		if state == BirdViewBindingExcluded {
			command = "bird_view.binding.exclude"
		}
		plan := newDomainPlan(command, false)
		binding := context.Binding
		if diagnostic := validateActiveBirdView(context.View); diagnostic != nil {
			return invalidPlan(plan, diagnostic.EntityID, diagnostic.Code)
		}
		if findBirdViewNode(context.Nodes, binding.NodeID) == nil || binding.BirdViewID != context.View.ID || binding.AccountID != context.View.AccountID || !validBirdViewBinding(binding) || !context.TargetVisible {
			return invalidPlan(plan, binding.NodeID, CodeInvalidBirdViewBinding)
		}
		binding.State = state
		plan.ProjectedDiff = binding
		plan.Events = []PlannedEvent{{Type: eventType, EntityType: "bird_view_binding", EntityID: birdViewBindingID(binding), Payload: map[string]any{"nodeId": binding.NodeID, "targetType": binding.Type, "workspaceId": binding.WorkspaceID, "targetId": binding.TargetID, "state": state}}}
		return plan
	}
}

func planBirdViewOverlayReplace(context BirdViewMutationContext) DomainMutationPlan {
	plan := newDomainPlan("bird_view.overlay.replace", false)
	if diagnostic := validateActiveBirdView(context.View); diagnostic != nil {
		return invalidPlan(plan, diagnostic.EntityID, diagnostic.Code)
	}
	if findBirdViewNode(context.Nodes, context.Node.ID) == nil || !context.TargetVisible {
		return invalidPlan(plan, context.Node.ID, CodeInvalidBirdViewBinding)
	}
	seen := map[string]bool{}
	for _, binding := range context.Overlay {
		if binding.NodeID != context.Node.ID || binding.BirdViewID != context.View.ID || binding.AccountID != context.View.AccountID || binding.State != BirdViewBindingSuggested || !validBirdViewBinding(binding) || seen[birdViewBindingID(binding)] {
			return invalidPlan(plan, context.Node.ID, CodeInvalidBirdViewBinding)
		}
		seen[birdViewBindingID(binding)] = true
	}
	preserved := 0
	for _, binding := range context.Bindings {
		if binding.State == BirdViewBindingPinned || binding.State == BirdViewBindingExcluded {
			preserved++
		}
	}
	sort.Slice(context.Overlay, func(i, j int) bool {
		return birdViewBindingID(context.Overlay[i]) < birdViewBindingID(context.Overlay[j])
	})
	plan.ProjectedDiff = map[string]any{"nodeId": context.Node.ID, "suggested": context.Overlay, "preservedExplicitCount": preserved}
	plan.Events = []PlannedEvent{{Type: "bird_view.overlay.replaced", EntityType: "bird_view_node", EntityID: context.Node.ID, Payload: map[string]any{"nodeId": context.Node.ID, "suggestedCount": len(context.Overlay), "preservedExplicitCount": preserved}}}
	return plan
}

func findBirdViewNode(nodes []BirdViewNode, id string) *BirdViewNode {
	for index := range nodes {
		if nodes[index].ID == id {
			return &nodes[index]
		}
	}
	return nil
}

func validBirdViewBinding(binding BirdViewBinding) bool {
	if binding.NodeID == "" || binding.WorkspaceID == "" || binding.TargetID == "" {
		return false
	}
	return binding.Type == BirdViewBindingPhase || binding.Type == BirdViewBindingGate || binding.Type == BirdViewBindingTask || binding.Type == BirdViewBindingBacklog
}

func birdViewBindingID(binding BirdViewBinding) string {
	return string(binding.Type) + ":" + binding.WorkspaceID + ":" + binding.TargetID
}
