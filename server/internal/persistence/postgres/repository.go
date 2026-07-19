package postgres

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jazzcake/baley/server/internal/application"
	"github.com/jazzcake/baley/server/internal/domain"
	"github.com/pressly/goose/v3"
)

type Repository struct {
	Pool             *pgxpool.Pool
	leaseTokenSecret []byte
}

func Open(ctx context.Context, url string) (*Repository, error) {
	secretSource := os.Getenv("BALEY_LEASE_TOKEN_SECRET")
	if secretSource == "" {
		return nil, fmt.Errorf("BALEY_LEASE_TOKEN_SECRET is required")
	}
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	digest := sha256.Sum256([]byte(secretSource))
	secret := digest[:]
	return &Repository{Pool: pool, leaseTokenSecret: secret}, nil
}

func (r *Repository) RunLeaseToken(runID string) (string, error) {
	if len(r.leaseTokenSecret) == 0 || runID == "" {
		return "", fmt.Errorf("Run lease token secret and Run ID are required")
	}
	mac := hmac.New(sha256.New, r.leaseTokenSecret)
	_, _ = mac.Write([]byte("baley-run-lease-v1:" + runID))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}
func Migrate(url, dir, direction string) error {
	db, err := sql.Open("pgx", url)
	if err != nil {
		return err
	}
	defer db.Close()
	goose.SetDialect("postgres")
	if direction == "down" {
		return goose.Down(db, dir)
	}
	return goose.Up(db, dir)
}

type querier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

func (r *Repository) LoadSnapshot(ctx context.Context, wid string) (application.Snapshot, error) {
	return loadSnapshot(ctx, r.Pool, wid, false)
}

func loadSnapshot(ctx context.Context, q querier, wid string, locked bool) (application.Snapshot, error) {
	s := application.Snapshot{Phases: []application.PhaseProjection{}, Lanes: []application.LaneProjection{}, Tasks: []application.TaskProjection{}, Dependencies: []application.DependencyProjection{}, Gates: []application.GateProjection{}, Runs: []application.RunProjection{}, Repositories: []application.RepositoryProjection{}, Records: []application.TaskRecordProjection{}, Commits: []application.CommitReferenceProjection{}, GitObservations: []application.GitObservationProjection{}, HumanActorIDs: map[string]bool{}, ActorIDs: map[string]bool{}}
	lock := ""
	if locked {
		lock = " FOR UPDATE"
	}
	err := q.QueryRow(ctx, "SELECT w.id,w.name,w.state,w.revision,(SELECT id FROM phases WHERE workspace_id=w.id AND state='active') FROM workspaces w WHERE id=$1"+lock, wid).Scan(&s.Workspace.ID, &s.Workspace.Name, &s.Workspace.State, &s.Workspace.Revision, &s.Workspace.ActivePhaseID)
	if errors.Is(err, pgx.ErrNoRows) {
		return s, &application.CommandError{Code: domain.CodeNotFound, Message: "workspace not found"}
	}
	if err != nil {
		return s, err
	}
	if err = q.QueryRow(ctx, "SELECT next_task_public_id FROM workspace_counters WHERE workspace_id=$1", wid).Scan(&s.NextTaskPublicID); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return s, err
	}
	rows, err := q.Query(ctx, "SELECT id,name,position,state FROM phases WHERE workspace_id=$1 ORDER BY position", wid)
	if err != nil {
		return s, err
	}
	for rows.Next() {
		var v application.PhaseProjection
		if err = rows.Scan(&v.ID, &v.Name, &v.Position, &v.State); err != nil {
			rows.Close()
			return s, err
		}
		s.Phases = append(s.Phases, v)
	}
	rows.Close()
	rows, err = q.Query(ctx, "SELECT id,name,goal,summary,state FROM lanes WHERE workspace_id=$1 ORDER BY id", wid)
	if err != nil {
		return s, err
	}
	for rows.Next() {
		var v application.LaneProjection
		if err = rows.Scan(&v.ID, &v.Name, &v.Goal, &v.Summary, &v.State); err != nil {
			rows.Close()
			return s, err
		}
		s.Lanes = append(s.Lanes, v)
	}
	rows.Close()
	rows, err = q.Query(ctx, "SELECT id,public_id,lane_id,phase_id,COALESCE(parent_task_id,''),title,description,current_summary,next_action,status,blocker_reason,COALESCE(terminal_reason,''),COALESCE(implemented_assessment,'') FROM tasks WHERE workspace_id=$1 ORDER BY public_id", wid)
	if err != nil {
		return s, err
	}
	for rows.Next() {
		var v application.TaskProjection
		if err = rows.Scan(&v.ID, &v.PublicID, &v.LaneID, &v.PhaseID, &v.ParentTaskID, &v.Title, &v.Description, &v.CurrentSummary, &v.NextAction, &v.Status, &v.BlockerReason, &v.TerminalReason, &v.ImplementedAssessment); err != nil {
			rows.Close()
			return s, err
		}
		s.Tasks = append(s.Tasks, v)
	}
	rows.Close()
	for i := range s.Tasks {
		if s.Tasks[i].Status == "implemented" {
			s.Tasks[i].DecisionRequired = "task.confirm"
			s.Tasks[i].ExpectedWorkspaceRevision = s.Workspace.Revision
		}
	}
	rows, err = q.Query(ctx, "SELECT from_task_id,to_task_id FROM task_dependencies WHERE workspace_id=$1 ORDER BY from_task_id,to_task_id", wid)
	if err != nil {
		return s, err
	}
	for rows.Next() {
		var v application.DependencyProjection
		if err = rows.Scan(&v.FromTaskID, &v.ToTaskID); err != nil {
			rows.Close()
			return s, err
		}
		s.Dependencies = append(s.Dependencies, v)
	}
	rows.Close()
	rows, err = q.Query(ctx, "SELECT id,task_id,client_run_id,kind,status,operator_actor_id,COALESCE(session_ref,''),COALESCE(parent_run_id,''),COALESCE(target_run_id,''),lease_token_hash,heartbeat_at,lease_expires_at,version,started_at,ended_at,COALESCE(result_summary,''),COALESCE(error_summary,'') FROM runs WHERE workspace_id=$1 ORDER BY started_at,id", wid)
	if err != nil {
		return s, err
	}
	for rows.Next() {
		var v application.RunProjection
		if err = rows.Scan(&v.ID, &v.TaskID, &v.ClientRunID, &v.Kind, &v.Status, &v.OperatorActorID, &v.SessionRef, &v.ParentRunID, &v.TargetRunID, &v.LeaseTokenHash, &v.HeartbeatAt, &v.LeaseExpiresAt, &v.Version, &v.StartedAt, &v.EndedAt, &v.ResultSummary, &v.ErrorSummary); err != nil {
			rows.Close()
			return s, err
		}
		s.Runs = append(s.Runs, v)
	}
	rows.Close()
	rows, err = q.Query(ctx, "SELECT id,name,remote_url,COALESCE(default_branch,''),is_record_repository,COALESCE(task_records_root,'') FROM repositories WHERE workspace_id=$1 ORDER BY id", wid)
	if err != nil {
		return s, err
	}
	for rows.Next() {
		var v application.RepositoryProjection
		if err = rows.Scan(&v.ID, &v.Name, &v.RemoteURL, &v.DefaultBranch, &v.IsRecordRepository, &v.TaskRecordsRoot); err != nil {
			rows.Close()
			return s, err
		}
		s.Repositories = append(s.Repositories, v)
	}
	rows.Close()
	rows, err = q.Query(ctx, "SELECT id,task_id,COALESCE(run_id,''),record_type,repository_id,relative_path,COALESCE(working_tree_hash,''),COALESCE(commit_sha,''),COALESCE(blob_sha,''),state,short_summary,COALESCE(supersedes_record_id::text,'') FROM task_record_indexes WHERE workspace_id=$1 ORDER BY created_at,id", wid)
	if err != nil {
		return s, err
	}
	for rows.Next() {
		var v application.TaskRecordProjection
		if err = rows.Scan(&v.ID, &v.TaskID, &v.RunID, &v.Type, &v.RepositoryID, &v.RelativePath, &v.WorkingTreeHash, &v.CommitSHA, &v.BlobSHA, &v.State, &v.ShortSummary, &v.SupersedesRecordID); err != nil {
			rows.Close()
			return s, err
		}
		s.Records = append(s.Records, v)
	}
	rows.Close()
	rows, err = q.Query(ctx, "SELECT id,task_id,COALESCE(run_id,''),repository_id,commit_sha,relation,verification_state FROM commit_references WHERE workspace_id=$1 ORDER BY created_at,id", wid)
	if err != nil {
		return s, err
	}
	for rows.Next() {
		var v application.CommitReferenceProjection
		if err = rows.Scan(&v.ID, &v.TaskID, &v.RunID, &v.RepositoryID, &v.CommitSHA, &v.Relation, &v.VerificationState); err != nil {
			rows.Close()
			return s, err
		}
		s.Commits = append(s.Commits, v)
	}
	rows.Close()
	rows, err = q.Query(ctx, "SELECT id,run_id,repository_id,observed_at,COALESCE(head_commit_sha,''),COALESCE(branch_hint,''),COALESCE(worktree_label,''),dirty FROM run_git_observations WHERE workspace_id=$1 ORDER BY observed_at,id", wid)
	if err != nil {
		return s, err
	}
	for rows.Next() {
		var v application.GitObservationProjection
		if err = rows.Scan(&v.ID, &v.RunID, &v.RepositoryID, &v.ObservedAt, &v.HeadCommitSHA, &v.BranchHint, &v.WorktreeLabel, &v.Dirty); err != nil {
			rows.Close()
			return s, err
		}
		s.GitObservations = append(s.GitObservations, v)
	}
	rows.Close()
	rows, err = q.Query(ctx, "SELECT id,name,from_phase_id,to_phase_id,criteria_revision,passed_at FROM gates WHERE workspace_id=$1 ORDER BY id", wid)
	if err != nil {
		return s, err
	}
	for rows.Next() {
		var v application.GateProjection
		if err = rows.Scan(&v.ID, &v.Name, &v.FromPhaseID, &v.ToPhaseID, &v.CriteriaRevision, &v.PassedAt); err != nil {
			rows.Close()
			return s, err
		}
		v.Conditions = []application.GateTaskProjection{}
		s.Gates = append(s.Gates, v)
	}
	rows.Close()
	for i := range s.Gates {
		rows, err = q.Query(ctx, "SELECT id,gate_id,task_id,passed_at,pass_reason FROM gate_tasks WHERE workspace_id=$1 AND gate_id=$2 ORDER BY id", wid, s.Gates[i].ID)
		if err != nil {
			return s, err
		}
		for rows.Next() {
			var v application.GateTaskProjection
			if err = rows.Scan(&v.ID, &v.GateID, &v.TaskID, &v.PassedAt, &v.PassReason); err != nil {
				rows.Close()
				return s, err
			}
			s.Gates[i].Conditions = append(s.Gates[i].Conditions, v)
		}
		rows.Close()
		conditions := make([]domain.GateTaskCondition, 0, len(s.Gates[i].Conditions))
		for _, c := range s.Gates[i].Conditions {
			status := domain.TaskPending
			for _, t := range s.Tasks {
				if t.ID == c.TaskID {
					status = domain.TaskStatus(t.Status)
				}
			}
			for conditionIndex := range s.Gates[i].Conditions {
				if s.Gates[i].Conditions[conditionIndex].ID == c.ID {
					s.Gates[i].Conditions[conditionIndex].Satisfied = status == domain.TaskConfirmed || c.PassedAt != nil
					if status == domain.TaskConfirmed {
						s.Gates[i].Conditions[conditionIndex].SatisfactionReason = "task_confirmed"
					} else if c.PassedAt != nil {
						s.Gates[i].Conditions[conditionIndex].SatisfactionReason = "explicit_gate_task_pass"
					} else {
						s.Gates[i].Conditions[conditionIndex].SatisfactionReason = "unsatisfied"
					}
				}
			}
			conditions = append(conditions, domain.GateTaskCondition{TaskID: c.TaskID, TaskStatus: status, Passed: c.PassedAt != nil})
		}
		g := domain.Gate{PassedAt: s.Gates[i].PassedAt}
		s.Gates[i].Status = string(domain.GateStatusFor(g, conditions))
	}
	rows, err = q.Query(ctx, "SELECT id,actor_type FROM actors")
	if err != nil {
		return s, err
	}
	for rows.Next() {
		var id, actorType string
		if err = rows.Scan(&id, &actorType); err != nil {
			rows.Close()
			return s, err
		}
		s.ActorIDs[id] = true
		if actorType == "human" {
			s.HumanActorIDs[id] = true
		}
	}
	rows.Close()
	for i := range s.Gates {
		if s.Gates[i].Status == "ready" {
			s.Gates[i].DecisionRequired = "gate.pass"
			s.Gates[i].ExpectedWorkspaceRevision = s.Workspace.Revision
			s.Gates[i].DecisionSnapshotHash = application.DecisionSnapshotHash(s, s.Gates[i])
		}
	}
	return s, nil
}
func (r *Repository) Task(ctx context.Context, wid string, publicID int) (application.TaskProjection, error) {
	var v application.TaskProjection
	err := r.Pool.QueryRow(ctx, "SELECT t.id,t.public_id,t.lane_id,t.phase_id,COALESCE(t.parent_task_id,''),t.title,t.description,t.current_summary,t.next_action,t.status,t.blocker_reason,COALESCE(t.terminal_reason,''),COALESCE(t.implemented_assessment,''),w.revision FROM tasks t JOIN workspaces w ON w.id=t.workspace_id WHERE t.workspace_id=$1 AND t.public_id=$2", wid, publicID).Scan(&v.ID, &v.PublicID, &v.LaneID, &v.PhaseID, &v.ParentTaskID, &v.Title, &v.Description, &v.CurrentSummary, &v.NextAction, &v.Status, &v.BlockerReason, &v.TerminalReason, &v.ImplementedAssessment, &v.ExpectedWorkspaceRevision)
	if errors.Is(err, pgx.ErrNoRows) {
		err = &application.CommandError{Code: domain.CodeNotFound, Message: "task not found"}
	}
	if v.Status == "implemented" {
		v.DecisionRequired = "task.confirm"
	}
	return v, err
}
func (r *Repository) WorkspaceIDs(ctx context.Context) ([]string, error) {
	rows, err := r.Pool.Query(ctx, "SELECT id FROM workspaces ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
func (r *Repository) Events(ctx context.Context, wid string) ([]application.EventProjection, error) {
	rows, err := r.Pool.Query(ctx, "SELECT id,command_id,event_type,COALESCE(entity_type,''),COALESCE(entity_id,''),COALESCE(initiated_by_actor_id,''),COALESCE(executed_by_actor_id,''),COALESCE(approved_by_actor_id,''),workspace_revision,payload,created_at FROM events WHERE workspace_id=$1 ORDER BY workspace_revision,command_event_index,id", wid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []application.EventProjection{}
	for rows.Next() {
		var v application.EventProjection
		if err = rows.Scan(&v.ID, &v.CommandID, &v.EventType, &v.EntityType, &v.EntityID, &v.InitiatedByActorID, &v.ExecutedByActorID, &v.ApprovedByActorID, &v.WorkspaceRevision, &v.Payload, &v.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *Repository) Execute(ctx context.Context, wid string, req application.CommandRequest, requestFingerprint string, evaluate func(application.Snapshot) (application.PreviewResult, application.MutationPlan, error)) (result application.ExecutionResult, err error) {
	tx, err := r.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return result, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()
	if _, err = tx.Exec(ctx, "SELECT 1 FROM workspaces WHERE id=$1 FOR UPDATE", wid); err != nil {
		return result, err
	}
	var existingHash, existingFingerprint string
	var existingJSON []byte
	err = tx.QueryRow(ctx, "SELECT command_hash,request_fingerprint,result FROM commands WHERE workspace_id=$1 AND idempotency_key=$2", wid, req.Envelope.IdempotencyKey).Scan(&existingHash, &existingFingerprint, &existingJSON)
	if err == nil {
		if existingFingerprint != requestFingerprint {
			return result, &application.CommandError{Code: domain.CodeIdempotencyConflict, Message: "idempotency key reused for a different command"}
		}
		if err = json.Unmarshal(existingJSON, &result); err != nil {
			return result, err
		}
		result.Idempotent = true
		if req.Name == "run.start" {
			if err = r.restoreRunLeaseToken(&result); err != nil {
				return result, err
			}
		}
		_ = tx.Rollback(ctx)
		return result, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return result, err
	}
	snapshot, err := loadSnapshot(ctx, tx, wid, true)
	if err != nil {
		return result, err
	}
	preview, plan, err := evaluate(snapshot)
	if err != nil {
		return result, err
	}
	if plan.ExistingRunClientID != "" {
		err = tx.QueryRow(ctx, "SELECT result FROM commands WHERE workspace_id=$1 AND command_name='run.start' AND result->'projection'->'run'->>'clientRunId'=$2 ORDER BY created_at LIMIT 1", wid, plan.ExistingRunClientID).Scan(&existingJSON)
		if err != nil {
			return result, err
		}
		if err = json.Unmarshal(existingJSON, &result); err != nil {
			return result, err
		}
		result.Idempotent = true
		if err = r.restoreRunLeaseToken(&result); err != nil {
			return result, err
		}
		_ = tx.Rollback(ctx)
		return result, nil
	}
	if preview.CommandHash == "" {
		return result, fmt.Errorf("empty command hash")
	}
	newRevision := snapshot.Workspace.Revision + 1
	if plan.NoWorkspaceRevision {
		newRevision = snapshot.Workspace.Revision
	}
	now := time.Now().UTC()
	commandID := newID()
	approvalAction := strings.ReplaceAll(plan.Action, ".", "_")
	eventWrites := append([]application.EventWrite{}, plan.Events...)
	attestationID := ""
	if req.Envelope.HumanApprovalAttestation != nil {
		attestationID = newID()
		att := req.Envelope.HumanApprovalAttestation
		for index := range eventWrites {
			if eventWrites[index].Type == "gate.passed" {
				payload, _ := eventWrites[index].Payload.(map[string]any)
				payload["humanApprovalAttestationId"] = attestationID
				payload["workspaceRevision"] = newRevision
				payload["decisionSnapshotHash"] = preview.DecisionSnapshotHash
			}
		}
		eventWrites = append(eventWrites, application.EventWrite{Type: "human_approval_attestation.recorded", EntityType: "attestation", EntityID: attestationID, Payload: map[string]any{"action": approvalAction, "entityType": plan.EntityType, "entityId": plan.EntityID, "workspaceRevision": snapshot.Workspace.Revision, "approvedByActorId": att.ApprovedByActorID, "approvedCommandHash": att.ApprovedCommandHash, "decisionSnapshotHash": att.DecisionSnapshotHash}})
	}
	for index := range eventWrites {
		eventWrites[index] = normalizeEventWrite(eventWrites[index], plan)
		payload, ok := eventWrites[index].Payload.(map[string]any)
		if !ok {
			return result, fmt.Errorf("invalid Event payload for %s", eventWrites[index].Type)
		}
		evaluation := domain.ValidateEventEvidence(domain.PlannedEvent{Type: eventWrites[index].Type, EntityType: eventWrites[index].EntityType, EntityID: eventWrites[index].EntityID, Payload: payload})
		if evaluation.HasErrors() {
			return result, fmt.Errorf("invalid Event evidence for %s: %+v", eventWrites[index].Type, evaluation.Errors)
		}
	}
	if !plan.IdempotentNoMutation {
		planned := make([]domain.PlannedEvent, 0, len(eventWrites))
		for _, event := range eventWrites {
			planned = append(planned, domain.PlannedEvent{Type: event.Type, EntityType: event.EntityType, EntityID: event.EntityID, Payload: event.Payload.(map[string]any)})
		}
		approvedBy := ""
		if req.Envelope.HumanApprovalAttestation != nil {
			approvedBy = req.Envelope.HumanApprovalAttestation.ApprovedByActorID
		}
		audit := domain.ValidateCommandAudit(domain.AuditExpectation{Command: req.Name, EntityType: plan.EntityType, EntityID: plan.EntityID, WorkspaceRevision: snapshot.Workspace.Revision, CommandHash: preview.CommandHash, DecisionSnapshotHash: preview.DecisionSnapshotHash, ActiveGate: plan.ForceHumanApproval}, planned, domain.ActorProvenance{InitiatedBy: req.Envelope.InitiatedByActorID, ExecutedBy: req.Envelope.ExecutedByActorID, ApprovedBy: approvedBy})
		if audit.HasErrors() {
			return result, fmt.Errorf("invalid command audit for %s: %+v", req.Name, audit.Errors)
		}
	}
	switch plan.CommandName {
	case "phase.create":
		phase := plan.PhaseCreate
		if phase == nil {
			return result, fmt.Errorf("phase.create plan is missing Phase")
		}
		_, err = tx.Exec(ctx, "INSERT INTO phases(workspace_id,id,name,position,state) VALUES($1,$2,$3,$4,$5)", wid, phase.ID, plan.PhaseName, phase.Position, phase.State)
	case "task.create":
		task := plan.TaskCreate
		if task == nil {
			return result, fmt.Errorf("task.create plan is missing Task")
		}
		_, err = tx.Exec(ctx, `INSERT INTO tasks(workspace_id,id,public_id,lane_id,phase_id,parent_task_id,title,description,current_summary,next_action,status,terminal_reason,implemented_assessment,updated_at)
			VALUES($1,$2,$3,$4,$5,NULLIF($6,''),$7,$8,$9,$10,$11,NULLIF($12,''),NULLIF($13,''),$14)`, wid, task.ID, task.PublicID, task.LaneID, task.PhaseID, task.ParentTaskID, task.Title, task.Description, task.CurrentSummary, task.NextAction, task.Status, task.TerminalReason, task.ImplementedAssessment, now)
		for _, edge := range plan.DependencyAdd {
			if err != nil {
				break
			}
			_, err = tx.Exec(ctx, "INSERT INTO task_dependencies(workspace_id,from_task_id,to_task_id) VALUES($1,$2,$3)", wid, edge.FromTaskID, edge.ToTaskID)
		}
		if err == nil {
			var tag pgconn.CommandTag
			tag, err = tx.Exec(ctx, "UPDATE workspace_counters SET next_task_public_id=$1 WHERE workspace_id=$2 AND next_task_public_id=$3", task.PublicID+1, wid, plan.ExpectedTaskPublicID)
			if err == nil && tag.RowsAffected() != 1 {
				err = &application.CommandError{Code: domain.CodeStaleRevision, Message: "Task public ID counter changed"}
			}
		}
	case "gate.create":
		gate := plan.GateCreate
		if gate == nil {
			return result, fmt.Errorf("gate.create plan is missing Gate")
		}
		_, err = tx.Exec(ctx, "INSERT INTO gates(workspace_id,id,name,from_phase_id,to_phase_id,criteria_revision) VALUES($1,$2,$3,$4,$5,1)", wid, gate.ID, plan.GateName, gate.FromPhaseID, gate.ToPhaseID)
	case "gate.attach_task":
		condition := plan.GateTaskCreate
		if condition == nil {
			return result, fmt.Errorf("gate.attach_task plan is missing Gate Task")
		}
		if plan.TaskUpdate != nil {
			_, err = tx.Exec(ctx, "UPDATE tasks SET terminal_reason=NULL,updated_at=$1 WHERE workspace_id=$2 AND id=$3", now, wid, plan.TaskUpdate.ID)
		}
		if err == nil {
			_, err = tx.Exec(ctx, "INSERT INTO gate_tasks(workspace_id,id,gate_id,task_id) VALUES($1,$2,$3,$4)", wid, condition.LinkID, condition.GateID, condition.TaskID)
		}
		if err == nil {
			_, err = tx.Exec(ctx, "UPDATE gates SET criteria_revision=$1 WHERE workspace_id=$2 AND id=$3", plan.GateCriteriaRevision, wid, plan.GateID)
		}
	case "gate.detach_task":
		if plan.GateTaskDeleteID == "" {
			return result, fmt.Errorf("gate.detach_task plan is missing Gate Task ID")
		}
		_, err = tx.Exec(ctx, "DELETE FROM gate_tasks WHERE workspace_id=$1 AND id=$2", wid, plan.GateTaskDeleteID)
		if err == nil {
			_, err = tx.Exec(ctx, "UPDATE gates SET criteria_revision=$1 WHERE workspace_id=$2 AND id=$3", plan.GateCriteriaRevision, wid, plan.GateID)
		}
	case "lane.create":
		lane := plan.LaneUpdate
		if lane == nil {
			return result, fmt.Errorf("lane.create plan is missing Lane")
		}
		_, err = tx.Exec(ctx, "INSERT INTO lanes(workspace_id,id,name,goal,summary,state) VALUES($1,$2,$3,$4,$5,$6)", wid, lane.ID, lane.Name, lane.Goal, lane.Summary, lane.State)
	case "lane.update", "lane.close_out", "lane.discard":
		lane := plan.LaneUpdate
		if lane == nil {
			return result, fmt.Errorf("%s plan is missing Lane", plan.CommandName)
		}
		_, err = tx.Exec(ctx, "UPDATE lanes SET name=$1,goal=$2,summary=$3,state=$4 WHERE workspace_id=$5 AND id=$6", lane.Name, lane.Goal, lane.Summary, lane.State, wid, lane.ID)
	case "task.update", "task.set_terminal", "task.clear_terminal", "task.block", "task.unblock", "task.discard", "task.rework":
		task := plan.TaskUpdate
		if task == nil {
			return result, fmt.Errorf("%s plan is missing Task update", plan.CommandName)
		}
		_, err = tx.Exec(ctx, `UPDATE tasks SET title=$1,description=$2,current_summary=$3,next_action=$4,status=$5,blocked_at=$6,blocker_reason=NULLIF($7,''),terminal_reason=NULLIF($8,''),implemented_assessment=NULLIF($9,''),updated_at=$10 WHERE workspace_id=$11 AND id=$12`, task.Title, task.Description, task.CurrentSummary, task.NextAction, task.Status, task.BlockedAt, task.BlockerReason, task.TerminalReason, task.ImplementedAssessment, now, wid, task.ID)
	case "dependency.connect", "dependency.disconnect", "dependency.patch":
		for _, edge := range plan.DependencyRemove {
			if _, err = tx.Exec(ctx, "DELETE FROM task_dependencies WHERE workspace_id=$1 AND from_task_id=$2 AND to_task_id=$3", wid, edge.FromTaskID, edge.ToTaskID); err != nil {
				break
			}
		}
		for _, edge := range plan.DependencyAdd {
			if err != nil {
				break
			}
			if _, err = tx.Exec(ctx, "INSERT INTO task_dependencies(workspace_id,from_task_id,to_task_id) VALUES($1,$2,$3)", wid, edge.FromTaskID, edge.ToTaskID); err != nil {
				break
			}
		}
		for _, update := range plan.TerminalUpdates {
			if err != nil {
				break
			}
			var reason any
			if update.TerminalReason != nil {
				reason = *update.TerminalReason
			}
			_, err = tx.Exec(ctx, "UPDATE tasks SET terminal_reason=$1,updated_at=$2 WHERE workspace_id=$3 AND id=$4", reason, now, wid, update.TaskID)
		}
	case "task.report_implemented":
		task := plan.TaskUpdate
		if task == nil {
			return result, fmt.Errorf("task.report_implemented plan is missing Task update")
		}
		_, err = tx.Exec(ctx, "UPDATE tasks SET status=$1,implemented_assessment=NULLIF($2,''),updated_at=$3 WHERE workspace_id=$4 AND id=$5", task.Status, task.ImplementedAssessment, now, wid, task.ID)
	case "task.confirm":
		_, err = tx.Exec(ctx, "UPDATE tasks SET status=$1 WHERE workspace_id=$2 AND id=$3", plan.TaskStatus, wid, plan.TaskID)
	case "gate.pass_task":
		_, err = tx.Exec(ctx, "UPDATE gate_tasks SET passed_at=$1,passed_by_actor_id=$2,pass_reason=$3 WHERE workspace_id=$4 AND id=$5", now, req.Envelope.HumanApprovalAttestation.ApprovedByActorID, plan.GateTaskReason, wid, plan.GateTaskID)
	case "gate.revoke_task_pass":
		_, err = tx.Exec(ctx, "UPDATE gate_tasks SET passed_at=NULL,passed_by_actor_id=NULL,pass_reason=NULL WHERE workspace_id=$1 AND id=$2", wid, plan.GateTaskID)
	case "gate.pass":
		_, err = tx.Exec(ctx, "UPDATE phases SET state='completed' WHERE workspace_id=$1 AND id=$2", wid, plan.FromPhaseID)
		if err == nil {
			_, err = tx.Exec(ctx, "UPDATE phases SET state='active' WHERE workspace_id=$1 AND id=$2", wid, plan.ToPhaseID)
		}
		if err == nil {
			_, err = tx.Exec(ctx, "UPDATE gates SET passed_at=$1,passed_by_actor_id=$2 WHERE workspace_id=$3 AND id=$4", now, req.Envelope.HumanApprovalAttestation.ApprovedByActorID, wid, plan.GateID)
		}
	case "run.start":
		run := plan.Run
		if run == nil {
			return result, fmt.Errorf("run.start plan is missing Run")
		}
		if plan.RunTaskStatus != "" {
			_, err = tx.Exec(ctx, "UPDATE tasks SET status=$1 WHERE workspace_id=$2 AND id=$3", plan.RunTaskStatus, wid, run.TaskID)
		}
		if err == nil {
			_, err = tx.Exec(ctx, `INSERT INTO runs(workspace_id,id,task_id,client_run_id,kind,status,operator_actor_id,session_ref,parent_run_id,target_run_id,lease_token_hash,heartbeat_at,lease_expires_at,version,started_at)
				VALUES($1,$2,$3,$4,$5,$6,$7,NULLIF($8,''),NULLIF($9,''),NULLIF($10,''),$11,$12,$13,$14,$15)`, wid, run.ID, run.TaskID, run.ClientRunID, run.Kind, run.Status, run.OperatorActorID, run.SessionRef, run.ParentRunID, run.TargetRunID, run.LeaseTokenHash, run.HeartbeatAt, run.LeaseExpiresAt, run.Version, run.StartedAt)
		}
	case "run.heartbeat":
		run := plan.RunUpdate
		if run == nil {
			return result, fmt.Errorf("run.heartbeat plan is missing Run update")
		}
		var tag pgconn.CommandTag
		tag, err = tx.Exec(ctx, "UPDATE runs SET heartbeat_at=$1,lease_expires_at=$2,version=$3 WHERE workspace_id=$4 AND id=$5 AND status='running' AND version=$6", run.HeartbeatAt, run.LeaseExpiresAt, run.Version, wid, run.ID, plan.RunExpectedVersion)
		if err == nil && tag.RowsAffected() != 1 {
			err = &application.CommandError{Code: domain.CodeStaleRunVersion, Message: "Run version changed during heartbeat"}
		}
	case "run.succeed", "run.fail", "run.cancel", "run.interrupt", "run.correct":
		if plan.IdempotentNoMutation {
			break
		}
		run := plan.RunUpdate
		if run == nil {
			return result, fmt.Errorf("%s plan is missing Run update", plan.CommandName)
		}
		var tag pgconn.CommandTag
		tag, err = tx.Exec(ctx, "UPDATE runs SET status=$1,version=$2,ended_at=$3,result_summary=NULLIF($4,''),error_summary=NULLIF($5,'') WHERE workspace_id=$6 AND id=$7 AND version=$8", run.Status, run.Version, run.EndedAt, run.ResultSummary, run.ErrorSummary, wid, run.ID, plan.RunExpectedVersion)
		if err == nil && tag.RowsAffected() != 1 {
			err = &application.CommandError{Code: domain.CodeStaleRunVersion, Message: "Run version changed during terminal transition"}
		}
	case "repository.register":
		if plan.IdempotentNoMutation {
			break
		}
		repository := plan.Repository
		if repository == nil {
			return result, fmt.Errorf("repository.register plan is missing Repository")
		}
		_, err = tx.Exec(ctx, "INSERT INTO repositories(workspace_id,id,name,remote_url,default_branch,is_record_repository,task_records_root) VALUES($1,$2,$3,$4,NULLIF($5,''),$6,NULLIF($7,''))", wid, repository.ID, repository.Name, repository.RemoteURL, repository.DefaultBranch, repository.IsRecordRepository, repository.TaskRecordsRoot)
	case "record.register":
		if plan.IdempotentNoMutation {
			break
		}
		record := plan.Record
		if record == nil {
			return result, fmt.Errorf("record.register plan is missing Task Record")
		}
		_, err = tx.Exec(ctx, `INSERT INTO task_record_indexes(workspace_id,id,task_id,run_id,record_type,repository_id,relative_path,working_tree_hash,state,short_summary,supersedes_record_id)
			VALUES($1,$2,$3,NULLIF($4,''),$5,$6,$7,NULLIF($8,''),$9,$10,NULLIF($11,'')::uuid)`, wid, record.ID, record.TaskID, record.RunID, record.Type, record.RepositoryID, record.RelativePath, record.WorkingTreeHash, record.State, record.ShortSummary, record.SupersedesRecordID)
	case "record.attach_commit":
		if plan.IdempotentNoMutation {
			break
		}
		record := plan.Record
		if record == nil {
			return result, fmt.Errorf("record.attach_commit plan is missing Task Record")
		}
		_, err = tx.Exec(ctx, "UPDATE task_record_indexes SET commit_sha=$1,blob_sha=$2,state=$3 WHERE workspace_id=$4 AND id=$5", record.CommitSHA, record.BlobSHA, record.State, wid, record.ID)
	case "commit.attach":
		if plan.IdempotentNoMutation {
			break
		}
		commit := plan.CommitReference
		if commit == nil {
			return result, fmt.Errorf("commit.attach plan is missing CommitReference")
		}
		_, err = tx.Exec(ctx, `INSERT INTO commit_references(workspace_id,id,task_id,run_id,repository_id,commit_sha,relation,verification_state)
			VALUES($1,$2,$3,NULLIF($4,''),$5,$6,$7,$8)`, wid, commit.ID, commit.TaskID, commit.RunID, commit.RepositoryID, commit.CommitSHA, commit.Relation, commit.VerificationState)
	case "git.observe":
		if plan.IdempotentNoMutation {
			break
		}
		observation := plan.GitObservation
		if observation == nil {
			return result, fmt.Errorf("git.observe plan is missing RunGitObservation")
		}
		_, err = tx.Exec(ctx, `INSERT INTO run_git_observations(workspace_id,id,run_id,repository_id,observed_at,head_commit_sha,branch_hint,worktree_label,dirty)
			VALUES($1,$2,$3,$4,$5,NULLIF($6,''),NULLIF($7,''),NULLIF($8,''),$9)`, wid, observation.ID, observation.RunID, observation.RepositoryID, observation.ObservedAt, observation.HeadCommitSHA, observation.BranchHint, observation.WorktreeLabel, observation.Dirty)
	}
	if err != nil {
		return result, err
	}
	if !plan.NoWorkspaceRevision {
		if _, err = tx.Exec(ctx, "UPDATE workspaces SET revision=$1 WHERE id=$2", newRevision, wid); err != nil {
			return result, err
		}
	}
	result = application.ExecutionResult{CommandID: commandID, WorkspaceRevision: newRevision, EventIDs: make([]string, 0, len(eventWrites)), Projection: preview.ProjectedDiff, LeaseToken: plan.RunLeaseToken, Idempotent: plan.IdempotentNoMutation}
	if req.Envelope.HumanApprovalAttestation != nil {
		result.ApprovalProtocol = "audit_metadata_not_authenticated_identity"
	}
	for range eventWrites {
		result.EventIDs = append(result.EventIDs, newID())
	}
	storedResult := result
	storedResult.LeaseToken = ""
	resultJSON, _ := json.Marshal(storedResult)
	if _, err = tx.Exec(ctx, "INSERT INTO commands(id,workspace_id,idempotency_key,command_name,command_hash,request_fingerprint,workspace_revision,result,initiated_by_actor_id,executed_by_actor_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,NULLIF($9,''),$10)", commandID, wid, req.Envelope.IdempotencyKey, req.Name, preview.CommandHash, requestFingerprint, newRevision, resultJSON, req.Envelope.InitiatedByActorID, req.Envelope.ExecutedByActorID); err != nil {
		return result, err
	}
	if att := req.Envelope.HumanApprovalAttestation; att != nil {
		if _, err = tx.Exec(ctx, "INSERT INTO human_approval_attestations(id,workspace_id,approved_by_actor_id,approved_command_hash,decision_snapshot_hash,action,entity_type,entity_id,workspace_revision,executed_command_id,statement_hash,conversation_ref,approved_at) VALUES($1,$2,$3,$4,NULLIF($5,''),$6,$7,$8,$9,$10,NULLIF($11,''),NULLIF($12,''),$13)", attestationID, wid, att.ApprovedByActorID, att.ApprovedCommandHash, att.DecisionSnapshotHash, approvalAction, plan.EntityType, plan.EntityID, snapshot.Workspace.Revision, commandID, att.StatementHash, att.ConversationRef, att.ApprovedAt); err != nil {
			return result, err
		}
	}
	for i, event := range eventWrites {
		payload, _ := json.Marshal(event.Payload)
		approvedBy := ""
		if req.Envelope.HumanApprovalAttestation != nil {
			approvedBy = req.Envelope.HumanApprovalAttestation.ApprovedByActorID
		}
		if _, err = tx.Exec(ctx, "INSERT INTO events(id,workspace_id,command_id,workspace_revision,command_event_index,event_type,entity_type,entity_id,initiated_by_actor_id,executed_by_actor_id,approved_by_actor_id,payload) VALUES($1,$2,$3,$4,$5,$6,$7,$8,NULLIF($9,''),$10,NULLIF($11,''),$12)", result.EventIDs[i], wid, commandID, newRevision, i, event.Type, event.EntityType, event.EntityID, req.Envelope.InitiatedByActorID, req.Envelope.ExecutedByActorID, approvedBy, payload); err != nil {
			return result, err
		}
	}
	err = tx.Commit(ctx)
	return result, err
}

func normalizeEventWrite(event application.EventWrite, plan application.MutationPlan) application.EventWrite {
	payload, ok := event.Payload.(map[string]any)
	if !ok {
		return event
	}
	if event.EntityType != "" && event.EntityID != "" {
		return event
	}
	key := ""
	switch event.Type {
	case "project.bootstrapped":
		event.EntityType, key = "project", "projectId"
	case "repository.registered":
		event.EntityType, key = "repository", "repositoryId"
	case "workspace.created", "workspace.activated", "workspace.closed":
		event.EntityType, key = "workspace", "workspaceId"
	case "phase.created", "phase.activated", "phase.completed":
		event.EntityType, key = "phase", "phaseId"
	case "lane.created", "lane.updated", "lane.closed_out", "lane.discarded":
		event.EntityType, key = "lane", "laneId"
	case "task.created":
		event.EntityType = "task"
		if task, taskOK := payload["task"].(domain.Task); taskOK {
			event.EntityID = task.ID
		}
	case "task.updated", "task.terminal_set", "task.terminal_cleared", "task.started", "task.implemented_reported", "task.confirmed", "task.discarded", "task.rework_started", "task.blocked", "task.unblocked":
		event.EntityType, key = "task", "taskId"
	case "dependency.connected", "dependency.disconnected", "dependency.patched":
		event.EntityType, event.EntityID = "dependency_graph", plan.EntityID
	case "gate.created", "gate.task_attached", "gate.task_detached", "gate.passed":
		event.EntityType, key = "gate", "gateId"
	case "gate.task_passed", "gate.task_pass_revoked":
		event.EntityType, key = "gate_task", "gateTaskId"
	case "run.started", "run.succeeded", "run.failed", "run.cancelled", "run.interrupted", "run.corrected":
		event.EntityType, key = "run", "runId"
	case "record.registered", "record.commit_attached":
		event.EntityType, key = "task_record", "recordId"
	case "commit.attached":
		event.EntityType, key = "commit_reference", "commitId"
	case "git.observed":
		event.EntityType, key = "run_git_observation", "observationId"
	}
	if event.EntityID == "" && key != "" {
		event.EntityID = fmt.Sprint(payload[key])
	}
	if event.EntityID == "" {
		event.EntityID = plan.EntityID
	}
	return event
}

func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	s := hex.EncodeToString(b[:])
	return s[:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:]
}

func (r *Repository) restoreRunLeaseToken(result *application.ExecutionResult) error {
	rawProjection, err := json.Marshal(result.Projection)
	if err != nil {
		return err
	}
	var projection map[string]any
	if err = json.Unmarshal(rawProjection, &projection); err != nil {
		return err
	}
	run, ok := projection["run"].(map[string]any)
	if !ok {
		return nil
	}
	runID, _ := run["id"].(string)
	if runID == "" {
		return nil
	}
	result.LeaseToken, err = r.RunLeaseToken(runID)
	return err
}
