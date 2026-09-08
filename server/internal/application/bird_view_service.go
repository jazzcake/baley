package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jazzcake/baley/server/internal/authz"
	"github.com/jazzcake/baley/server/internal/domain"
)

type BirdViewProjection struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Status      string     `json:"status"`
	Revision    int64      `json:"revision"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	ArchivedAt  *time.Time `json:"archivedAt,omitempty"`
}

type BirdViewNodeProjection struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Summary   string    `json:"summary"`
	Content   string    `json:"content"`
	Status    string    `json:"status"`
	PositionX float64   `json:"positionX"`
	PositionY float64   `json:"positionY"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type BirdViewEdgeProjection struct {
	ID         string    `json:"id"`
	FromNodeID string    `json:"fromNodeId"`
	ToNodeID   string    `json:"toNodeId"`
	Label      string    `json:"label"`
	CreatedAt  time.Time `json:"createdAt"`
}

type BirdViewBindingProjection struct {
	NodeID         string `json:"nodeId"`
	WorkspaceID    string `json:"workspaceId"`
	TargetID       string `json:"targetId"`
	TargetType     string `json:"targetType"`
	State          string `json:"state"`
	TargetTitle    string `json:"targetTitle"`
	TargetStatus   string `json:"targetStatus"`
	TargetPublicID *int   `json:"targetPublicId,omitempty"`
}

type BirdViewGraphProjection struct {
	BirdView BirdViewProjection       `json:"birdView"`
	Nodes    []BirdViewNodeProjection `json:"nodes"`
	Edges    []BirdViewEdgeProjection `json:"edges"`
}

type BirdViewWorkspaceFocus struct {
	Workspace    WorkspaceProjection         `json:"workspace"`
	Phases       []PhaseProjection           `json:"phases"`
	Lanes        []LaneProjection            `json:"lanes"`
	Tasks        []TaskProjection            `json:"tasks"`
	Dependencies []DependencyProjection      `json:"dependencies"`
	Gates        []GateProjection            `json:"gates"`
	BacklogItems []BacklogItemProjection     `json:"backlogItems"`
	Bindings     []BirdViewBindingProjection `json:"bindings"`
}

type BirdViewNodeFocusProjection struct {
	BirdView   BirdViewProjection       `json:"birdView"`
	Node       BirdViewNodeProjection   `json:"node"`
	Workspaces []BirdViewWorkspaceFocus `json:"workspaces"`
}

type BirdViewNodeContextProjection struct {
	BirdView BirdViewProjection          `json:"birdView"`
	Node     BirdViewNodeProjection      `json:"node"`
	Bindings []BirdViewBindingProjection `json:"bindings"`
	Rules    map[string]any              `json:"rules"`
}

type BirdViewSnapshot struct {
	View     domain.BirdView
	Nodes    []domain.BirdViewNode
	Edges    []domain.BirdViewEdge
	Bindings []domain.BirdViewBinding
}

type BirdViewMutation struct {
	CommandName string
	View        *domain.BirdView
	Node        *domain.BirdViewNode
	Edge        *domain.BirdViewEdge
	Binding     *domain.BirdViewBinding
	Overlay     []domain.BirdViewBinding
	Events      []EventWrite
	EntityType  string
	EntityID    string
	Projection  any
}

type BirdViewPreviewResult struct {
	CommandHash              string       `json:"commandHash"`
	ExpectedBirdViewRevision int64        `json:"expectedBirdViewRevision"`
	RequiredCapability       string       `json:"requiredCapability"`
	ProjectedDiff            any          `json:"projectedDiff"`
	Errors                   []Diagnostic `json:"errors"`
	Warnings                 []Diagnostic `json:"warnings"`
	Advisories               []Diagnostic `json:"advisories"`
	EntityType               string       `json:"entityType,omitempty"`
	EntityID                 string       `json:"entityId,omitempty"`
}

type BirdViewExecutionResult struct {
	CommandID        string   `json:"commandId"`
	BirdViewRevision int64    `json:"birdViewRevision"`
	EventIDs         []string `json:"eventIds"`
	Projection       any      `json:"projection"`
	Idempotent       bool     `json:"idempotent"`
	CommandHash      string   `json:"-"`
}

type BirdViewRepository interface {
	ListBirdViews(context.Context, string, bool) ([]BirdViewProjection, error)
	GetBirdView(context.Context, string, string) (BirdViewProjection, error)
	BirdViewGraph(context.Context, string, string) (BirdViewGraphProjection, error)
	BirdViewNodeFocus(context.Context, string, string, string) (BirdViewNodeFocusProjection, error)
	BirdViewNodeContext(context.Context, string, string, string) (BirdViewNodeContextProjection, error)
	LoadBirdViewSnapshot(context.Context, string, string) (BirdViewSnapshot, error)
	BirdViewTargetsVisible(context.Context, string, []domain.BirdViewBinding) (bool, error)
	ExecuteBirdView(context.Context, string, CommandRequest, string, []domain.BirdViewBinding, func(BirdViewSnapshot) (BirdViewPreviewResult, BirdViewMutation, error)) (BirdViewExecutionResult, error)
}

type BirdViewService struct{ repo BirdViewRepository }

func NewBirdViewService(repo BirdViewRepository) *BirdViewService {
	return &BirdViewService{repo: repo}
}

func (s *BirdViewService) List(ctx context.Context, principal *CommandPrincipal, includeArchived bool) ([]BirdViewProjection, error) {
	accountID, err := authorizeBirdViewPrincipal(principal, authz.BirdViewRead)
	if err != nil {
		return nil, err
	}
	return s.repo.ListBirdViews(ctx, accountID, includeArchived)
}

func (s *BirdViewService) Get(ctx context.Context, principal *CommandPrincipal, id string) (BirdViewProjection, error) {
	accountID, err := authorizeBirdViewPrincipal(principal, authz.BirdViewRead)
	if err != nil {
		return BirdViewProjection{}, err
	}
	return s.repo.GetBirdView(ctx, accountID, id)
}

func (s *BirdViewService) Graph(ctx context.Context, principal *CommandPrincipal, id string) (BirdViewGraphProjection, error) {
	accountID, err := authorizeBirdViewPrincipal(principal, authz.BirdViewRead)
	if err != nil {
		return BirdViewGraphProjection{}, err
	}
	return s.repo.BirdViewGraph(ctx, accountID, id)
}

func (s *BirdViewService) Focus(ctx context.Context, principal *CommandPrincipal, id, nodeID string) (BirdViewNodeFocusProjection, error) {
	accountID, err := authorizeBirdViewPrincipal(principal, authz.BirdViewRead)
	if err != nil {
		return BirdViewNodeFocusProjection{}, err
	}
	return s.repo.BirdViewNodeFocus(ctx, accountID, id, nodeID)
}

func (s *BirdViewService) Context(ctx context.Context, principal *CommandPrincipal, id, nodeID string) (BirdViewNodeContextProjection, error) {
	accountID, err := authorizeBirdViewPrincipal(principal, authz.BirdViewRead)
	if err != nil {
		return BirdViewNodeContextProjection{}, err
	}
	return s.repo.BirdViewNodeContext(ctx, accountID, id, nodeID)
}

func authorizeBirdViewPrincipal(principal *CommandPrincipal, capability authz.Capability) (string, error) {
	if principal == nil || strings.TrimSpace(principal.AccountID) == "" || strings.TrimSpace(principal.Subject.ActorID) == "" {
		return "", &CommandError{Code: "unauthenticated", Message: "account authentication required"}
	}
	for _, scope := range principal.Subject.Scopes {
		if scope == capability {
			return principal.AccountID, nil
		}
	}
	return "", &CommandError{Code: "forbidden", Message: "Bird View capability denied"}
}

func IsBirdViewCommand(name string) bool { return strings.HasPrefix(name, "bird_view.") }

func (s *BirdViewService) Preview(ctx context.Context, request CommandRequest) (BirdViewPreviewResult, error) {
	accountID, err := authorizeBirdViewPrincipal(request.Principal, authz.BirdViewOperate)
	if err != nil {
		return BirdViewPreviewResult{}, err
	}
	typed, viewID, targets, err := decodeBirdViewArguments(request.Name, request.Arguments, accountID)
	if err != nil {
		return BirdViewPreviewResult{}, err
	}
	snapshot := BirdViewSnapshot{}
	if request.Name != "bird_view.create" {
		snapshot, err = s.repo.LoadBirdViewSnapshot(ctx, accountID, viewID)
		if err != nil {
			return BirdViewPreviewResult{}, err
		}
	}
	visible := true
	if len(targets) > 0 {
		visible, err = s.repo.BirdViewTargetsVisible(ctx, accountID, targets)
	}
	if err != nil {
		return BirdViewPreviewResult{}, err
	}
	preview, _, err := evaluateBirdView(request, accountID, typed, snapshot, visible)
	return preview, err
}

func (s *BirdViewService) Execute(ctx context.Context, request CommandRequest) (BirdViewExecutionResult, error) {
	accountID, err := authorizeBirdViewPrincipal(request.Principal, authz.BirdViewOperate)
	if err != nil {
		return BirdViewExecutionResult{}, err
	}
	if strings.TrimSpace(request.Envelope.IdempotencyKey) == "" || strings.TrimSpace(request.Envelope.ExecutedByActorID) == "" {
		return BirdViewExecutionResult{}, &CommandError{Code: "invalid_request", Message: "execute requires idempotencyKey and executedByActorId"}
	}
	if request.Envelope.ExecutedByActorID != request.Principal.Subject.ActorID {
		return BirdViewExecutionResult{}, &CommandError{Code: "forbidden", Message: "command Actor does not match authenticated principal"}
	}
	typed, _, targets, err := decodeBirdViewArguments(request.Name, request.Arguments, accountID)
	if err != nil {
		return BirdViewExecutionResult{}, err
	}
	fingerprint := birdViewFingerprint(request, typed)
	return s.repo.ExecuteBirdView(ctx, accountID, request, fingerprint, targets, func(snapshot BirdViewSnapshot) (BirdViewPreviewResult, BirdViewMutation, error) {
		return evaluateBirdView(request, accountID, typed, snapshot, true)
	})
}

type birdViewArgs struct {
	CredentialWorkspaceID string                     `json:"workspaceId,omitempty"`
	BirdViewID            string                     `json:"birdViewId"`
	NodeID                string                     `json:"nodeId,omitempty"`
	EdgeID                string                     `json:"edgeId,omitempty"`
	FromNodeID            string                     `json:"fromNodeId,omitempty"`
	ToNodeID              string                     `json:"toNodeId,omitempty"`
	Title                 *string                    `json:"title,omitempty"`
	Description           *string                    `json:"description,omitempty"`
	Summary               *string                    `json:"summary,omitempty"`
	Content               *string                    `json:"content,omitempty"`
	Label                 *string                    `json:"label,omitempty"`
	PositionX             *float64                   `json:"positionX,omitempty"`
	PositionY             *float64                   `json:"positionY,omitempty"`
	TargetType            domain.BirdViewBindingType `json:"targetType,omitempty"`
	TargetWorkspaceID     string                     `json:"targetWorkspaceId,omitempty"`
	TargetID              string                     `json:"targetId,omitempty"`
	Bindings              []birdViewBindingArgs      `json:"bindings,omitempty"`
}

type birdViewBindingArgs struct {
	TargetType  domain.BirdViewBindingType `json:"targetType"`
	WorkspaceID string                     `json:"workspaceId"`
	TargetID    string                     `json:"targetId"`
}

func decodeBirdViewArguments(name string, raw json.RawMessage, accountID string) (birdViewArgs, string, []domain.BirdViewBinding, error) {
	if BirdViewMutationHandlersMissing(name) {
		return birdViewArgs{}, "", nil, &CommandError{Code: "invalid_request", Message: "unsupported command: " + name}
	}
	var args birdViewArgs
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&args); err != nil {
		return args, "", nil, &CommandError{Code: "invalid_request", Message: err.Error()}
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return args, "", nil, &CommandError{Code: "invalid_request", Message: "arguments must contain one JSON object"}
	}
	args.BirdViewID, args.NodeID, args.EdgeID = strings.TrimSpace(args.BirdViewID), strings.TrimSpace(args.NodeID), strings.TrimSpace(args.EdgeID)
	bindings := []domain.BirdViewBinding{}
	if name == "bird_view.binding.pin" || name == "bird_view.binding.exclude" {
		bindings = append(bindings, domain.BirdViewBinding{AccountID: accountID, BirdViewID: args.BirdViewID, NodeID: args.NodeID, Type: args.TargetType, WorkspaceID: strings.TrimSpace(args.TargetWorkspaceID), TargetID: strings.TrimSpace(args.TargetID)})
	}
	if name == "bird_view.overlay.replace" {
		for _, input := range args.Bindings {
			bindings = append(bindings, domain.BirdViewBinding{AccountID: accountID, BirdViewID: args.BirdViewID, NodeID: args.NodeID, Type: input.TargetType, WorkspaceID: strings.TrimSpace(input.WorkspaceID), TargetID: strings.TrimSpace(input.TargetID), State: domain.BirdViewBindingSuggested})
		}
	}
	return args, args.BirdViewID, bindings, nil
}

func BirdViewMutationHandlersMissing(name string) bool {
	return domain.BirdViewMutationHandlers[name] == nil
}

func evaluateBirdView(request CommandRequest, accountID string, args birdViewArgs, snapshot BirdViewSnapshot, targetsVisible bool) (BirdViewPreviewResult, BirdViewMutation, error) {
	preview := BirdViewPreviewResult{ExpectedBirdViewRevision: request.Envelope.ExpectedBirdViewRevision, RequiredCapability: string(authz.BirdViewOperate), Errors: []Diagnostic{}, Warnings: []Diagnostic{}, Advisories: []Diagnostic{}}
	mutation := BirdViewMutation{CommandName: request.Name}
	invalidNodeID := (strings.Contains(request.Name, ".node.") || strings.Contains(request.Name, ".binding.") || request.Name == "bird_view.overlay.replace") && !isUUID(args.NodeID)
	invalidEdge := strings.Contains(request.Name, ".edge.") && (!isUUID(args.EdgeID) || request.Name == "bird_view.edge.connect" && (!isUUID(strings.TrimSpace(args.FromNodeID)) || !isUUID(strings.TrimSpace(args.ToNodeID))))
	if !isUUID(args.BirdViewID) || invalidNodeID || invalidEdge {
		preview.Errors = append(preview.Errors, Diagnostic{Code: domain.CodeInvalidStateTransition, EntityID: args.BirdViewID})
		return preview, mutation, nil
	}
	if request.Name != "bird_view.create" && (request.Envelope.ExpectedBirdViewRevision <= 0 || request.Envelope.ExpectedBirdViewRevision != snapshot.View.Revision) {
		preview.Errors = append(preview.Errors, Diagnostic{Code: domain.CodeStaleBirdViewRevision, EntityID: snapshot.View.ID})
		return preview, mutation, nil
	}
	context := domain.BirdViewMutationContext{View: snapshot.View, Nodes: snapshot.Nodes, Edges: snapshot.Edges, Bindings: snapshot.Bindings, Title: args.Title, Description: args.Description, Summary: args.Summary, Content: args.Content, Label: args.Label, PositionX: args.PositionX, PositionY: args.PositionY, TargetVisible: targetsVisible}
	if args.NodeID != "" {
		if node := findBirdNode(snapshot.Nodes, args.NodeID); node != nil {
			context.Node = *node
		}
	}
	if args.EdgeID != "" {
		if edge := findBirdEdge(snapshot.Edges, args.EdgeID); edge != nil {
			context.Edge = *edge
		}
	}
	switch request.Name {
	case "bird_view.create":
		title, description := birdStringValue(args.Title), birdStringValue(args.Description)
		context.View = domain.BirdView{ID: args.BirdViewID, AccountID: accountID, Title: title, Description: description, Status: domain.BirdViewActive, Revision: 1}
	case "bird_view.node.create":
		context.Node = domain.BirdViewNode{ID: args.NodeID, BirdViewID: snapshot.View.ID, AccountID: accountID, Title: birdStringValue(args.Title), Summary: birdStringValue(args.Summary), Content: birdStringValue(args.Content), Status: domain.BirdViewNodeActive, PositionX: birdFloatValue(args.PositionX), PositionY: birdFloatValue(args.PositionY)}
	case "bird_view.edge.connect":
		context.Edge = domain.BirdViewEdge{ID: args.EdgeID, BirdViewID: snapshot.View.ID, AccountID: accountID, FromNodeID: strings.TrimSpace(args.FromNodeID), ToNodeID: strings.TrimSpace(args.ToNodeID), Label: birdStringValue(args.Label)}
	case "bird_view.binding.pin", "bird_view.binding.exclude":
		if len(targetsVisibleBindings(args, accountID)) == 1 {
			context.Binding = targetsVisibleBindings(args, accountID)[0]
		}
	case "bird_view.overlay.replace":
		context.Overlay = make([]domain.BirdViewBinding, 0, len(args.Bindings))
		for _, input := range args.Bindings {
			context.Overlay = append(context.Overlay, domain.BirdViewBinding{AccountID: accountID, BirdViewID: snapshot.View.ID, NodeID: args.NodeID, WorkspaceID: strings.TrimSpace(input.WorkspaceID), TargetID: strings.TrimSpace(input.TargetID), Type: input.TargetType, State: domain.BirdViewBindingSuggested})
		}
	}
	plan := domain.PlanBirdViewMutation(request.Name, context)
	preview.ProjectedDiff, preview.Errors, preview.Warnings, preview.Advisories = plan.ProjectedDiff, plan.Evaluation.Errors, plan.Evaluation.Warnings, plan.Evaluation.Advisories
	if len(preview.Errors) > 0 {
		return preview, mutation, nil
	}
	preview.EntityType, preview.EntityID = plan.Events[0].EntityType, plan.Events[0].EntityID
	preview.ExpectedBirdViewRevision = snapshot.View.Revision
	if request.Name == "bird_view.create" {
		preview.ExpectedBirdViewRevision = 0
	}
	preview.CommandHash = birdViewCommandHash(request.Name, accountID, preview.ExpectedBirdViewRevision, args)
	mutation.EntityType, mutation.EntityID, mutation.Projection = preview.EntityType, preview.EntityID, plan.ProjectedDiff
	for _, event := range plan.Events {
		mutation.Events = append(mutation.Events, EventWrite{Type: event.Type, EntityType: event.EntityType, EntityID: event.EntityID, Payload: event.Payload})
	}
	switch request.Name {
	case "bird_view.create":
		view := context.View
		mutation.View = &view
	case "bird_view.update":
		next := plan.ProjectedDiff.(map[string]any)["after"].(domain.BirdView)
		mutation.View = &next
	case "bird_view.archive":
		next := snapshot.View
		next.Status = domain.BirdViewArchived
		mutation.View = &next
	case "bird_view.node.create":
		node := context.Node
		mutation.Node = &node
	case "bird_view.node.update":
		next := plan.ProjectedDiff.(map[string]any)["after"].(domain.BirdViewNode)
		mutation.Node = &next
	case "bird_view.node.delete":
		node := context.Node
		mutation.Node = &node
	case "bird_view.node.achieve":
		next := context.Node
		next.Status = domain.BirdViewNodeAchieved
		mutation.Node = &next
	case "bird_view.node.park":
		next := context.Node
		next.Status = domain.BirdViewNodeParked
		mutation.Node = &next
	case "bird_view.edge.connect", "bird_view.edge.disconnect":
		edge := context.Edge
		mutation.Edge = &edge
	case "bird_view.edge.update":
		next := plan.ProjectedDiff.(map[string]any)["after"].(domain.BirdViewEdge)
		mutation.Edge = &next
	case "bird_view.binding.pin":
		binding := context.Binding
		binding.State = domain.BirdViewBindingPinned
		mutation.Binding = &binding
	case "bird_view.binding.exclude":
		binding := context.Binding
		binding.State = domain.BirdViewBindingExcluded
		mutation.Binding = &binding
	case "bird_view.overlay.replace":
		mutation.Overlay = context.Overlay
	}
	return preview, mutation, nil
}

func targetsVisibleBindings(args birdViewArgs, accountID string) []domain.BirdViewBinding {
	if args.TargetType == "" {
		return nil
	}
	return []domain.BirdViewBinding{{AccountID: accountID, BirdViewID: args.BirdViewID, NodeID: args.NodeID, Type: args.TargetType, WorkspaceID: strings.TrimSpace(args.TargetWorkspaceID), TargetID: strings.TrimSpace(args.TargetID)}}
}

func findBirdNode(values []domain.BirdViewNode, id string) *domain.BirdViewNode {
	for i := range values {
		if values[i].ID == id {
			return &values[i]
		}
	}
	return nil
}
func findBirdEdge(values []domain.BirdViewEdge, id string) *domain.BirdViewEdge {
	for i := range values {
		if values[i].ID == id {
			return &values[i]
		}
	}
	return nil
}
func birdStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func birdFloatValue(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

func birdViewCommandHash(name, accountID string, revision int64, typed birdViewArgs) string {
	typed.CredentialWorkspaceID = ""
	raw, _ := json.Marshal(struct {
		Name, AccountID string
		Revision        int64
		Arguments       birdViewArgs
	}{name, accountID, revision, typed})
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

func birdViewFingerprint(request CommandRequest, typed birdViewArgs) string {
	typed.CredentialWorkspaceID = ""
	credential := ""
	if request.Principal != nil {
		credential = request.Principal.CredentialID
	}
	raw, _ := json.Marshal(struct {
		Name, AccountID, ActorID, Credential string
		Revision                             int64
		Arguments                            birdViewArgs
	}{request.Name, request.Principal.AccountID, request.Envelope.ExecutedByActorID, credential, request.Envelope.ExpectedBirdViewRevision, typed})
	digest := sha256.Sum256(raw)
	return fmt.Sprintf("bird-view-v1:%x", digest)
}
