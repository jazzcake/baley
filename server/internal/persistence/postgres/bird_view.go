package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jazzcake/baley/server/internal/application"
	"github.com/jazzcake/baley/server/internal/domain"
)

func (r *Repository) ListBirdViews(ctx context.Context, accountID string, includeArchived bool) ([]application.BirdViewProjection, error) {
	rows, err := r.Pool.Query(ctx, `SELECT view.id::text,view.title,view.description,view.status,view.revision,view.created_at,view.updated_at,view.archived_at
		FROM bird_views view JOIN accounts account ON account.id=view.account_id AND account.status='active'
		WHERE view.account_id=$1 AND ($2 OR view.status='active') ORDER BY view.updated_at DESC,view.id`, accountID, includeArchived)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []application.BirdViewProjection{}
	for rows.Next() {
		var item application.BirdViewProjection
		if err = rows.Scan(&item.ID, &item.Title, &item.Description, &item.Status, &item.Revision, &item.CreatedAt, &item.UpdatedAt, &item.ArchivedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) GetBirdView(ctx context.Context, accountID, id string) (application.BirdViewProjection, error) {
	var item application.BirdViewProjection
	err := r.Pool.QueryRow(ctx, `SELECT view.id::text,view.title,view.description,view.status,view.revision,view.created_at,view.updated_at,view.archived_at
		FROM bird_views view JOIN accounts account ON account.id=view.account_id AND account.status='active'
		WHERE view.account_id=$1 AND view.id=$2`, accountID, id).
		Scan(&item.ID, &item.Title, &item.Description, &item.Status, &item.Revision, &item.CreatedAt, &item.UpdatedAt, &item.ArchivedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return item, birdViewNotFound()
	}
	return item, err
}

func (r *Repository) BirdViewGraph(ctx context.Context, accountID, id string) (application.BirdViewGraphProjection, error) {
	view, err := r.GetBirdView(ctx, accountID, id)
	if err != nil {
		return application.BirdViewGraphProjection{}, err
	}
	result := application.BirdViewGraphProjection{BirdView: view, Nodes: []application.BirdViewNodeProjection{}, Edges: []application.BirdViewEdgeProjection{}}
	rows, err := r.Pool.Query(ctx, `SELECT node.id::text,node.title,node.summary,node.content,node.status,node.position_x,node.position_y,node.created_at,node.updated_at
		FROM bird_view_nodes node WHERE node.account_id=$1 AND node.bird_view_id=$2 ORDER BY node.created_at,node.id`, accountID, id)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var node application.BirdViewNodeProjection
		if err = rows.Scan(&node.ID, &node.Title, &node.Summary, &node.Content, &node.Status, &node.PositionX, &node.PositionY, &node.CreatedAt, &node.UpdatedAt); err != nil {
			rows.Close()
			return result, err
		}
		result.Nodes = append(result.Nodes, node)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return result, err
	}
	rows.Close()
	rows, err = r.Pool.Query(ctx, `SELECT edge.id::text,edge.from_node_id::text,edge.to_node_id::text,edge.label,edge.created_at
		FROM bird_view_edges edge WHERE edge.account_id=$1 AND edge.bird_view_id=$2 ORDER BY edge.created_at,edge.id`, accountID, id)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var edge application.BirdViewEdgeProjection
		if err = rows.Scan(&edge.ID, &edge.FromNodeID, &edge.ToNodeID, &edge.Label, &edge.CreatedAt); err != nil {
			return result, err
		}
		result.Edges = append(result.Edges, edge)
	}
	return result, rows.Err()
}

func (r *Repository) BirdViewNodeContext(ctx context.Context, accountID, id, nodeID string) (application.BirdViewNodeContextProjection, error) {
	graph, err := r.BirdViewGraph(ctx, accountID, id)
	if err != nil {
		return application.BirdViewNodeContextProjection{}, err
	}
	node, ok := birdNodeProjection(graph.Nodes, nodeID)
	if !ok {
		return application.BirdViewNodeContextProjection{}, birdViewNotFound()
	}
	bindings, err := r.visibleBirdViewBindings(ctx, accountID, id, nodeID, true)
	if err != nil {
		return application.BirdViewNodeContextProjection{}, err
	}
	return application.BirdViewNodeContextProjection{BirdView: graph.BirdView, Node: node, Bindings: bindings, Rules: map[string]any{
		"bindingStates": []string{"suggested", "pinned", "excluded"}, "overlayReplacePreserves": []string{"pinned", "excluded"},
		"automaticTaskCreation": false, "taskDependenciesRemainWorkspaceLocal": true,
	}}, nil
}

func (r *Repository) BirdViewNodeFocus(ctx context.Context, accountID, id, nodeID string) (application.BirdViewNodeFocusProjection, error) {
	graph, err := r.BirdViewGraph(ctx, accountID, id)
	if err != nil {
		return application.BirdViewNodeFocusProjection{}, err
	}
	node, ok := birdNodeProjection(graph.Nodes, nodeID)
	if !ok {
		return application.BirdViewNodeFocusProjection{}, birdViewNotFound()
	}
	bindings, err := r.visibleBirdViewBindings(ctx, accountID, id, nodeID, false)
	if err != nil {
		return application.BirdViewNodeFocusProjection{}, err
	}
	grouped := map[string][]application.BirdViewBindingProjection{}
	for _, binding := range bindings {
		grouped[binding.WorkspaceID] = append(grouped[binding.WorkspaceID], binding)
	}
	workspaceIDs := make([]string, 0, len(grouped))
	for workspaceID := range grouped {
		workspaceIDs = append(workspaceIDs, workspaceID)
	}
	sort.Strings(workspaceIDs)
	result := application.BirdViewNodeFocusProjection{BirdView: graph.BirdView, Node: node, Workspaces: []application.BirdViewWorkspaceFocus{}}
	for _, workspaceID := range workspaceIDs {
		snapshot, visible, snapshotErr := r.loadVisibleBirdWorkspaceSnapshot(ctx, accountID, workspaceID)
		if snapshotErr != nil {
			return result, snapshotErr
		}
		if !visible {
			continue
		}
		result.Workspaces = append(result.Workspaces, focusWorkspace(snapshot, grouped[workspaceID]))
	}
	return result, nil
}

func focusWorkspace(snapshot application.Snapshot, bindings []application.BirdViewBindingProjection) application.BirdViewWorkspaceFocus {
	taskIDs, gateIDs, backlogIDs, phaseIDs, laneIDs := map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, binding := range bindings {
		switch binding.TargetType {
		case "phase":
			phaseIDs[binding.TargetID] = true
		case "gate":
			gateIDs[binding.TargetID] = true
		case "task":
			taskIDs[binding.TargetID] = true
		case "backlog":
			backlogIDs[binding.TargetID] = true
		}
	}
	// Bindings are the semantic curation boundary. A Phase provides context but
	// does not mean every Task in that Phase contributes to this outcome, and a
	// dependency adjacency is not itself a reason to pull another Task into focus.
	for _, task := range snapshot.Tasks {
		if taskIDs[task.ID] {
			phaseIDs[task.PhaseID] = true
			laneIDs[task.LaneID] = true
		}
	}
	for _, gate := range snapshot.Gates {
		if gateIDs[gate.ID] {
			phaseIDs[gate.FromPhaseID] = true
			phaseIDs[gate.ToPhaseID] = true
		}
	}
	for _, item := range snapshot.BacklogItems {
		if backlogIDs[item.ID] {
			laneIDs[item.LaneID] = true
		}
	}
	result := application.BirdViewWorkspaceFocus{Workspace: snapshot.Workspace, Phases: []application.PhaseProjection{}, Lanes: []application.LaneProjection{}, Tasks: []application.TaskProjection{}, Dependencies: []application.DependencyProjection{}, Gates: []application.GateProjection{}, BacklogItems: []application.BacklogItemProjection{}, Bindings: bindings}
	for _, phase := range snapshot.Phases {
		if phaseIDs[phase.ID] {
			result.Phases = append(result.Phases, phase)
		}
	}
	for _, lane := range snapshot.Lanes {
		if laneIDs[lane.ID] {
			result.Lanes = append(result.Lanes, lane)
		}
	}
	for _, task := range snapshot.Tasks {
		if taskIDs[task.ID] {
			result.Tasks = append(result.Tasks, task)
		}
	}
	for _, edge := range snapshot.Dependencies {
		if taskIDs[edge.FromTaskID] && taskIDs[edge.ToTaskID] {
			result.Dependencies = append(result.Dependencies, edge)
		}
	}
	for _, gate := range snapshot.Gates {
		if gateIDs[gate.ID] {
			result.Gates = append(result.Gates, gate)
		}
	}
	for _, item := range snapshot.BacklogItems {
		if backlogIDs[item.ID] {
			result.BacklogItems = append(result.BacklogItems, item)
		}
	}
	return result
}

func (r *Repository) visibleBirdViewBindings(ctx context.Context, accountID, id, nodeID string, includeExcluded bool) ([]application.BirdViewBindingProjection, error) {
	query := `WITH member_workspaces AS (
		SELECT membership.workspace_id FROM accounts account
		JOIN workspace_memberships membership ON membership.actor_id=account.actor_id AND membership.active
		JOIN workspaces workspace ON workspace.id=membership.workspace_id AND workspace.state='active'
		WHERE account.id=$1 AND account.status='active'
	), bindings AS (
		SELECT binding.node_id::text,binding.workspace_id,binding.phase_id target_id,'phase' target_type,binding.binding_state,phase.name target_title,phase.state target_status,NULL::integer public_id
		FROM bird_view_phase_bindings binding JOIN phases phase ON phase.workspace_id=binding.workspace_id AND phase.id=binding.phase_id
		WHERE binding.account_id=$1 AND binding.bird_view_id=$2 AND binding.node_id=$3
		UNION ALL
		SELECT binding.node_id::text,binding.workspace_id,binding.gate_id,'gate',binding.binding_state,gate.name,CASE WHEN gate.passed_at IS NULL THEN 'open' ELSE 'passed' END,gate.public_id
		FROM bird_view_gate_bindings binding JOIN gates gate ON gate.workspace_id=binding.workspace_id AND gate.id=binding.gate_id
		WHERE binding.account_id=$1 AND binding.bird_view_id=$2 AND binding.node_id=$3
		UNION ALL
		SELECT binding.node_id::text,binding.workspace_id,binding.task_id,'task',binding.binding_state,task.title,task.status,task.public_id
		FROM bird_view_task_bindings binding JOIN tasks task ON task.workspace_id=binding.workspace_id AND task.id=binding.task_id
		WHERE binding.account_id=$1 AND binding.bird_view_id=$2 AND binding.node_id=$3
		UNION ALL
		SELECT binding.node_id::text,binding.workspace_id,binding.backlog_id,'backlog',binding.binding_state,item.title,item.status,item.public_id
		FROM bird_view_backlog_bindings binding JOIN backlog_items item ON item.workspace_id=binding.workspace_id AND item.id=binding.backlog_id
		WHERE binding.account_id=$1 AND binding.bird_view_id=$2 AND binding.node_id=$3
	)
	SELECT binding.node_id,binding.workspace_id,binding.target_id,binding.target_type,binding.binding_state,binding.target_title,binding.target_status,binding.public_id
	FROM bindings binding JOIN member_workspaces visible ON visible.workspace_id=binding.workspace_id
	WHERE ($4 OR binding.binding_state<>'excluded')
	ORDER BY binding.workspace_id,binding.target_type,binding.target_id`
	rows, err := r.Pool.Query(ctx, query, accountID, id, nodeID, includeExcluded)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []application.BirdViewBindingProjection{}
	for rows.Next() {
		var item application.BirdViewBindingProjection
		if err = rows.Scan(&item.NodeID, &item.WorkspaceID, &item.TargetID, &item.TargetType, &item.State, &item.TargetTitle, &item.TargetStatus, &item.TargetPublicID); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) LoadBirdViewSnapshot(ctx context.Context, accountID, id string) (application.BirdViewSnapshot, error) {
	return loadBirdViewSnapshot(ctx, r.Pool, accountID, id, false)
}

func loadBirdViewSnapshot(ctx context.Context, q querier, accountID, id string, locked bool) (application.BirdViewSnapshot, error) {
	result := application.BirdViewSnapshot{Nodes: []domain.BirdViewNode{}, Edges: []domain.BirdViewEdge{}, Bindings: []domain.BirdViewBinding{}}
	lock := ""
	if locked {
		lock = " FOR UPDATE OF view"
	}
	err := q.QueryRow(ctx, `SELECT view.id::text,view.account_id::text,view.title,view.description,view.status,view.revision
		FROM bird_views view JOIN accounts account ON account.id=view.account_id AND account.status='active'
		WHERE view.account_id=$1 AND view.id=$2`+lock, accountID, id).
		Scan(&result.View.ID, &result.View.AccountID, &result.View.Title, &result.View.Description, &result.View.Status, &result.View.Revision)
	if errors.Is(err, pgx.ErrNoRows) {
		return result, birdViewNotFound()
	}
	if err != nil {
		return result, err
	}
	rows, err := q.Query(ctx, `SELECT id::text,bird_view_id::text,account_id::text,title,summary,content,status,position_x,position_y FROM bird_view_nodes WHERE account_id=$1 AND bird_view_id=$2 ORDER BY created_at,id`, accountID, id)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var node domain.BirdViewNode
		if err = rows.Scan(&node.ID, &node.BirdViewID, &node.AccountID, &node.Title, &node.Summary, &node.Content, &node.Status, &node.PositionX, &node.PositionY); err != nil {
			rows.Close()
			return result, err
		}
		result.Nodes = append(result.Nodes, node)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return result, err
	}
	rows.Close()
	rows, err = q.Query(ctx, `SELECT id::text,bird_view_id::text,account_id::text,from_node_id::text,to_node_id::text,label FROM bird_view_edges WHERE account_id=$1 AND bird_view_id=$2 ORDER BY created_at,id`, accountID, id)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var edge domain.BirdViewEdge
		if err = rows.Scan(&edge.ID, &edge.BirdViewID, &edge.AccountID, &edge.FromNodeID, &edge.ToNodeID, &edge.Label); err != nil {
			rows.Close()
			return result, err
		}
		result.Edges = append(result.Edges, edge)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return result, err
	}
	rows.Close()
	for _, spec := range birdBindingSpecs() {
		rows, err = q.Query(ctx, fmt.Sprintf(`SELECT bird_view_id::text,account_id::text,node_id::text,workspace_id,%s,binding_state FROM %s WHERE account_id=$1 AND bird_view_id=$2 ORDER BY node_id,workspace_id,%s`, spec.column, spec.table, spec.column), accountID, id)
		if err != nil {
			return result, err
		}
		for rows.Next() {
			var binding domain.BirdViewBinding
			binding.Type = spec.kind
			if err = rows.Scan(&binding.BirdViewID, &binding.AccountID, &binding.NodeID, &binding.WorkspaceID, &binding.TargetID, &binding.State); err != nil {
				rows.Close()
				return result, err
			}
			result.Bindings = append(result.Bindings, binding)
		}
		if err = rows.Err(); err != nil {
			rows.Close()
			return result, err
		}
		rows.Close()
	}
	return result, nil
}

type birdBindingSpec struct {
	kind          domain.BirdViewBindingType
	table, column string
}

func birdBindingSpecs() []birdBindingSpec {
	return []birdBindingSpec{{domain.BirdViewBindingPhase, "bird_view_phase_bindings", "phase_id"}, {domain.BirdViewBindingGate, "bird_view_gate_bindings", "gate_id"}, {domain.BirdViewBindingTask, "bird_view_task_bindings", "task_id"}, {domain.BirdViewBindingBacklog, "bird_view_backlog_bindings", "backlog_id"}}
}

func (r *Repository) BirdViewTargetsVisible(ctx context.Context, accountID string, bindings []domain.BirdViewBinding) (bool, error) {
	return birdViewTargetsVisible(ctx, r.Pool, accountID, bindings, false)
}

func birdViewTargetsVisible(ctx context.Context, q querier, accountID string, bindings []domain.BirdViewBinding, lock bool) (bool, error) {
	for _, binding := range bindings {
		spec, ok := bindingSpec(binding.Type)
		if !ok {
			return false, nil
		}
		lockClause := ""
		if lock {
			lockClause = " FOR SHARE OF membership,workspace,target"
		}
		query := fmt.Sprintf(`SELECT 1 FROM accounts account
			JOIN workspace_memberships membership ON membership.actor_id=account.actor_id AND membership.workspace_id=$2 AND membership.active
			JOIN workspaces workspace ON workspace.id=membership.workspace_id AND workspace.state='active'
			JOIN %s target ON target.workspace_id=workspace.id AND target.id=$3
			WHERE account.id=$1 AND account.status='active'`+lockClause, specTargetTable(spec))
		var one int
		if err := q.QueryRow(ctx, query, accountID, binding.WorkspaceID, binding.TargetID).Scan(&one); errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		} else if err != nil {
			return false, err
		}
	}
	return true, nil
}

func bindingSpec(kind domain.BirdViewBindingType) (birdBindingSpec, bool) {
	for _, spec := range birdBindingSpecs() {
		if spec.kind == kind {
			return spec, true
		}
	}
	return birdBindingSpec{}, false
}
func specTargetTable(spec birdBindingSpec) string {
	return map[domain.BirdViewBindingType]string{domain.BirdViewBindingPhase: "phases", domain.BirdViewBindingGate: "gates", domain.BirdViewBindingTask: "tasks", domain.BirdViewBindingBacklog: "backlog_items"}[spec.kind]
}

func (r *Repository) ExecuteBirdView(ctx context.Context, accountID string, request application.CommandRequest, fingerprint string, targets []domain.BirdViewBinding, evaluate func(application.BirdViewSnapshot) (application.BirdViewPreviewResult, application.BirdViewMutation, error)) (result application.BirdViewExecutionResult, err error) {
	tx, err := r.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	var accountActor string
	if err = tx.QueryRow(ctx, "SELECT actor_id FROM accounts WHERE id=$1 AND status='active' FOR UPDATE", accountID).Scan(&accountActor); err != nil {
		return result, birdViewNotFound()
	}
	var existingFingerprint, existingHash, existingActor string
	var existingJSON []byte
	err = tx.QueryRow(ctx, `SELECT request_fingerprint,command_hash,executed_by_actor_id,result FROM bird_view_commands WHERE account_id=$1 AND idempotency_key=$2`, accountID, request.Envelope.IdempotencyKey).Scan(&existingFingerprint, &existingHash, &existingActor, &existingJSON)
	if err == nil {
		if existingFingerprint != fingerprint || existingActor != request.Envelope.ExecutedByActorID {
			return result, &application.CommandError{Code: domain.CodeIdempotencyConflict, Message: "idempotency key reused for a different Bird View command"}
		}
		if err = json.Unmarshal(existingJSON, &result); err != nil {
			return result, err
		}
		result.Idempotent = true
		result.CommandHash = existingHash
		return result, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return result, err
	}
	var snapshot application.BirdViewSnapshot
	if request.Name != "bird_view.create" {
		snapshot, err = loadBirdViewSnapshot(ctx, tx, accountID, extractBirdViewID(request.Arguments), true)
		if err != nil {
			return result, err
		}
	}
	if len(targets) > 0 {
		var visible bool
		visible, err = birdViewTargetsVisible(ctx, tx, accountID, targets, true)
		if err != nil {
			return result, err
		}
		if !visible {
			return result, &application.CommandError{Code: domain.CodeNotFound, Message: "Bird View binding target not found"}
		}
	}
	preview, mutation, err := evaluate(snapshot)
	if err != nil {
		return result, err
	}
	if len(preview.Errors) > 0 {
		return result, &application.CommandError{Code: preview.Errors[0].Code, Message: "Bird View command evaluation failed: " + preview.Errors[0].Code}
	}
	if preview.CommandHash == "" {
		return result, fmt.Errorf("empty Bird View command hash")
	}
	now := time.Now().UTC()
	viewID := extractBirdViewID(request.Arguments)
	newRevision := int64(1)
	if request.Name != "bird_view.create" {
		newRevision = snapshot.View.Revision + 1
	}
	switch request.Name {
	case "bird_view.create":
		view := mutation.View
		if view == nil {
			return result, fmt.Errorf("missing Bird View create projection")
		}
		_, err = tx.Exec(ctx, `INSERT INTO bird_views(id,account_id,title,description,status,revision,created_at,updated_at) VALUES($1,$2,$3,$4,'active',1,$5,$5)`, view.ID, accountID, view.Title, view.Description, now)
	case "bird_view.update":
		view := mutation.View
		if view == nil {
			return result, fmt.Errorf("missing Bird View update projection")
		}
		_, err = tx.Exec(ctx, "UPDATE bird_views SET title=$1,description=$2,updated_at=$3 WHERE account_id=$4 AND id=$5 AND status='active'", view.Title, view.Description, now, accountID, viewID)
	case "bird_view.archive":
		_, err = tx.Exec(ctx, "UPDATE bird_views SET status='archived',archived_at=$1,updated_at=$1 WHERE account_id=$2 AND id=$3 AND status='active'", now, accountID, viewID)
	case "bird_view.node.create":
		node := mutation.Node
		if node == nil {
			return result, fmt.Errorf("missing Bird View Node")
		}
		_, err = tx.Exec(ctx, `INSERT INTO bird_view_nodes(account_id,bird_view_id,id,title,summary,content,status,position_x,position_y,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$10)`, accountID, viewID, node.ID, node.Title, node.Summary, node.Content, node.Status, node.PositionX, node.PositionY, now)
	case "bird_view.node.update", "bird_view.node.achieve", "bird_view.node.park":
		node := mutation.Node
		if node == nil {
			return result, fmt.Errorf("missing Bird View Node")
		}
		_, err = tx.Exec(ctx, "UPDATE bird_view_nodes SET title=$1,summary=$2,content=$3,status=$4,position_x=$5,position_y=$6,updated_at=$7 WHERE account_id=$8 AND bird_view_id=$9 AND id=$10", node.Title, node.Summary, node.Content, node.Status, node.PositionX, node.PositionY, now, accountID, viewID, node.ID)
	case "bird_view.node.delete":
		node := mutation.Node
		if node == nil {
			return result, fmt.Errorf("missing Bird View Node")
		}
		_, err = tx.Exec(ctx, "DELETE FROM bird_view_nodes WHERE account_id=$1 AND bird_view_id=$2 AND id=$3", accountID, viewID, node.ID)
	case "bird_view.edge.connect":
		edge := mutation.Edge
		if edge == nil {
			return result, fmt.Errorf("missing Bird View edge")
		}
		_, err = tx.Exec(ctx, `INSERT INTO bird_view_edges(account_id,bird_view_id,id,from_node_id,to_node_id,label,created_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, accountID, viewID, edge.ID, edge.FromNodeID, edge.ToNodeID, edge.Label, now)
	case "bird_view.edge.disconnect":
		edge := mutation.Edge
		if edge == nil {
			return result, fmt.Errorf("missing Bird View edge")
		}
		_, err = tx.Exec(ctx, "DELETE FROM bird_view_edges WHERE account_id=$1 AND bird_view_id=$2 AND id=$3", accountID, viewID, edge.ID)
	case "bird_view.edge.update":
		edge := mutation.Edge
		if edge == nil {
			return result, fmt.Errorf("missing Bird View edge")
		}
		_, err = tx.Exec(ctx, "UPDATE bird_view_edges SET label=$1 WHERE account_id=$2 AND bird_view_id=$3 AND id=$4", edge.Label, accountID, viewID, edge.ID)
	case "bird_view.binding.pin", "bird_view.binding.exclude":
		if mutation.Binding == nil {
			return result, fmt.Errorf("missing Bird View binding")
		}
		err = upsertBirdBinding(ctx, tx, *mutation.Binding, now)
	case "bird_view.overlay.replace":
		err = replaceBirdOverlay(ctx, tx, accountID, viewID, mutation.EntityID, mutation.Overlay, now)
	default:
		return result, &application.CommandError{Code: "invalid_request", Message: "unsupported Bird View command"}
	}
	if err != nil {
		return result, err
	}
	if request.Name != "bird_view.create" {
		tag, updateErr := tx.Exec(ctx, "UPDATE bird_views SET revision=$1,updated_at=$2 WHERE account_id=$3 AND id=$4 AND revision=$5", newRevision, now, accountID, viewID, snapshot.View.Revision)
		if updateErr != nil {
			return result, updateErr
		}
		if tag.RowsAffected() != 1 {
			return result, &application.CommandError{Code: domain.CodeStaleBirdViewRevision, Message: "Bird View revision changed"}
		}
	}
	commandID := newID()
	eventIDs := make([]string, len(mutation.Events))
	for i := range eventIDs {
		eventIDs[i] = newID()
	}
	result = application.BirdViewExecutionResult{CommandID: commandID, BirdViewRevision: newRevision, EventIDs: eventIDs, Projection: mutation.Projection, CommandHash: preview.CommandHash}
	resultJSON, _ := json.Marshal(result)
	_, err = tx.Exec(ctx, `INSERT INTO bird_view_commands(id,account_id,bird_view_id,idempotency_key,command_name,command_hash,request_fingerprint,expected_bird_view_revision,bird_view_revision,executed_by_actor_id,initiated_by_actor_id,result,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,NULLIF($8,0),$9,$10,NULLIF($11,''),$12,$13)`, commandID, accountID, viewID, request.Envelope.IdempotencyKey, request.Name, preview.CommandHash, fingerprint, request.Envelope.ExpectedBirdViewRevision, newRevision, request.Envelope.ExecutedByActorID, request.Envelope.InitiatedByActorID, resultJSON, now)
	if err != nil {
		return result, err
	}
	for i, event := range mutation.Events {
		payload, marshalErr := json.Marshal(event.Payload)
		if marshalErr != nil {
			return result, marshalErr
		}
		_, err = tx.Exec(ctx, `INSERT INTO bird_view_events(id,account_id,bird_view_id,command_id,bird_view_revision,event_type,entity_type,entity_id,initiated_by_actor_id,executed_by_actor_id,payload,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,NULLIF($9,''),$10,$11,$12)`, eventIDs[i], accountID, viewID, commandID, newRevision, event.Type, event.EntityType, event.EntityID, request.Envelope.InitiatedByActorID, request.Envelope.ExecutedByActorID, payload, now)
		if err != nil {
			return result, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return result, err
	}
	return result, nil
}

func upsertBirdBinding(ctx context.Context, tx pgx.Tx, binding domain.BirdViewBinding, now time.Time) error {
	spec, ok := bindingSpec(binding.Type)
	if !ok {
		return &application.CommandError{Code: domain.CodeInvalidBirdViewBinding, Message: "unknown binding type"}
	}
	query := fmt.Sprintf(`INSERT INTO %s(account_id,bird_view_id,node_id,workspace_id,%s,binding_state,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,$7) ON CONFLICT(bird_view_id,node_id,workspace_id,%s) DO UPDATE SET binding_state=EXCLUDED.binding_state,updated_at=EXCLUDED.updated_at`, spec.table, spec.column, spec.column)
	_, err := tx.Exec(ctx, query, binding.AccountID, binding.BirdViewID, binding.NodeID, binding.WorkspaceID, binding.TargetID, binding.State, now)
	return err
}
func replaceBirdOverlay(ctx context.Context, tx pgx.Tx, accountID, viewID, nodeID string, bindings []domain.BirdViewBinding, now time.Time) error {
	for _, spec := range birdBindingSpecs() {
		if _, err := tx.Exec(ctx, fmt.Sprintf("DELETE FROM %s WHERE account_id=$1 AND bird_view_id=$2 AND node_id=$3 AND binding_state='suggested'", spec.table), accountID, viewID, nodeID); err != nil {
			return err
		}
	}
	for _, binding := range bindings {
		spec, _ := bindingSpec(binding.Type)
		query := fmt.Sprintf(`INSERT INTO %s(account_id,bird_view_id,node_id,workspace_id,%s,binding_state,created_at,updated_at) VALUES($1,$2,$3,$4,$5,'suggested',$6,$6) ON CONFLICT(bird_view_id,node_id,workspace_id,%s) DO NOTHING`, spec.table, spec.column, spec.column)
		if _, err := tx.Exec(ctx, query, accountID, viewID, nodeID, binding.WorkspaceID, binding.TargetID, now); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) loadVisibleBirdWorkspaceSnapshot(ctx context.Context, accountID, workspaceID string) (application.Snapshot, bool, error) {
	tx, err := r.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return application.Snapshot{}, false, err
	}
	defer tx.Rollback(ctx)
	var one int
	err = tx.QueryRow(ctx, `SELECT 1 FROM accounts account
		JOIN workspace_memberships membership ON membership.actor_id=account.actor_id AND membership.active
		JOIN workspaces workspace ON workspace.id=membership.workspace_id AND workspace.state='active'
		WHERE account.id=$1 AND account.status='active' AND workspace.id=$2
		FOR SHARE OF account,membership,workspace`, accountID, workspaceID).Scan(&one)
	if errors.Is(err, pgx.ErrNoRows) {
		return application.Snapshot{}, false, nil
	}
	if err != nil {
		return application.Snapshot{}, false, err
	}
	snapshot, err := loadSnapshot(ctx, tx, workspaceID, false)
	if err != nil {
		return application.Snapshot{}, false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return application.Snapshot{}, false, err
	}
	return snapshot, true, nil
}
func extractBirdViewID(raw json.RawMessage) string {
	var value struct {
		BirdViewID string `json:"birdViewId"`
	}
	_ = json.Unmarshal(raw, &value)
	return strings.TrimSpace(value.BirdViewID)
}
func birdNodeProjection(nodes []application.BirdViewNodeProjection, id string) (application.BirdViewNodeProjection, bool) {
	for _, node := range nodes {
		if node.ID == id {
			return node, true
		}
	}
	return application.BirdViewNodeProjection{}, false
}
func birdViewNotFound() error {
	return &application.CommandError{Code: domain.CodeNotFound, Message: "Bird View not found"}
}
