package postgres

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jazzcake/baley/server/internal/application"
	"github.com/jazzcake/baley/server/internal/domain"
	"github.com/pressly/goose/v3"
)

type Repository struct{ Pool *pgxpool.Pool }

func Open(ctx context.Context, url string) (*Repository, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &Repository{Pool: pool}, nil
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
	s := application.Snapshot{Phases: []application.PhaseProjection{}, Lanes: []application.LaneProjection{}, Tasks: []application.TaskProjection{}, Dependencies: []application.DependencyProjection{}, Gates: []application.GateProjection{}, HumanActorIDs: map[string]bool{}}
	lock := ""
	if locked {
		lock = " FOR UPDATE"
	}
	err := q.QueryRow(ctx, "SELECT w.id,w.name,w.revision,(SELECT id FROM phases WHERE workspace_id=w.id AND state='active') FROM workspaces w WHERE id=$1"+lock, wid).Scan(&s.Workspace.ID, &s.Workspace.Name, &s.Workspace.Revision, &s.Workspace.ActivePhaseID)
	if errors.Is(err, pgx.ErrNoRows) {
		return s, &application.CommandError{Code: domain.CodeNotFound, Message: "workspace not found"}
	}
	if err != nil {
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
	rows, err = q.Query(ctx, "SELECT id,name,state FROM lanes WHERE workspace_id=$1 ORDER BY id", wid)
	if err != nil {
		return s, err
	}
	for rows.Next() {
		var v application.LaneProjection
		if err = rows.Scan(&v.ID, &v.Name, &v.State); err != nil {
			rows.Close()
			return s, err
		}
		s.Lanes = append(s.Lanes, v)
	}
	rows.Close()
	rows, err = q.Query(ctx, "SELECT id,public_id,lane_id,phase_id,title,description,status,blocker_reason FROM tasks WHERE workspace_id=$1 ORDER BY public_id", wid)
	if err != nil {
		return s, err
	}
	for rows.Next() {
		var v application.TaskProjection
		if err = rows.Scan(&v.ID, &v.PublicID, &v.LaneID, &v.PhaseID, &v.Title, &v.Description, &v.Status, &v.BlockerReason); err != nil {
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
	rows, err = q.Query(ctx, "SELECT id FROM actors WHERE actor_type='human'")
	if err != nil {
		return s, err
	}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return s, err
		}
		s.HumanActorIDs[id] = true
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
	err := r.Pool.QueryRow(ctx, "SELECT t.id,t.public_id,t.lane_id,t.phase_id,t.title,t.description,t.status,t.blocker_reason,w.revision FROM tasks t JOIN workspaces w ON w.id=t.workspace_id WHERE t.workspace_id=$1 AND t.public_id=$2", wid, publicID).Scan(&v.ID, &v.PublicID, &v.LaneID, &v.PhaseID, &v.Title, &v.Description, &v.Status, &v.BlockerReason, &v.ExpectedWorkspaceRevision)
	if errors.Is(err, pgx.ErrNoRows) {
		err = &application.CommandError{Code: domain.CodeNotFound, Message: "task not found"}
	}
	if v.Status == "implemented" {
		v.DecisionRequired = "task.confirm"
	}
	return v, err
}
func (r *Repository) Events(ctx context.Context, wid string) ([]application.EventProjection, error) {
	rows, err := r.Pool.Query(ctx, "SELECT id,command_id,event_type,workspace_revision,payload,created_at FROM events WHERE workspace_id=$1 ORDER BY created_at,id", wid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []application.EventProjection{}
	for rows.Next() {
		var v application.EventProjection
		if err = rows.Scan(&v.ID, &v.CommandID, &v.EventType, &v.WorkspaceRevision, &v.Payload, &v.CreatedAt); err != nil {
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
	if preview.CommandHash == "" {
		return result, fmt.Errorf("empty command hash")
	}
	newRevision := snapshot.Workspace.Revision + 1
	now := time.Now().UTC()
	commandID := newID()
	eventWrites := append([]application.EventWrite{}, plan.Events...)
	eventWrites = append(eventWrites, application.EventWrite{Type: "human_approval_attestation.recorded", Payload: map[string]any{"approvedByActorId": req.Envelope.HumanApprovalAttestation.ApprovedByActorID}})
	switch plan.CommandName {
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
	}
	if err != nil {
		return result, err
	}
	if _, err = tx.Exec(ctx, "UPDATE workspaces SET revision=$1 WHERE id=$2", newRevision, wid); err != nil {
		return result, err
	}
	result = application.ExecutionResult{CommandID: commandID, WorkspaceRevision: newRevision, EventIDs: make([]string, 0, len(eventWrites)), Projection: preview.ProjectedDiff, ApprovalProtocol: "audit_metadata_not_authenticated_identity"}
	for range eventWrites {
		result.EventIDs = append(result.EventIDs, newID())
	}
	resultJSON, _ := json.Marshal(result)
	if _, err = tx.Exec(ctx, "INSERT INTO commands(id,workspace_id,idempotency_key,command_name,command_hash,request_fingerprint,workspace_revision,result) VALUES($1,$2,$3,$4,$5,$6,$7,$8)", commandID, wid, req.Envelope.IdempotencyKey, req.Name, preview.CommandHash, requestFingerprint, newRevision, resultJSON); err != nil {
		return result, err
	}
	att := req.Envelope.HumanApprovalAttestation
	if _, err = tx.Exec(ctx, "INSERT INTO human_approval_attestations(id,workspace_id,approved_by_actor_id,approved_command_hash,decision_snapshot_hash,action,entity_type,entity_id,workspace_revision,executed_command_id,statement_hash,conversation_ref,approved_at) VALUES($1,$2,$3,$4,NULLIF($5,''),$6,$7,$8,$9,$10,NULLIF($11,''),NULLIF($12,''),$13)", newID(), wid, att.ApprovedByActorID, att.ApprovedCommandHash, att.DecisionSnapshotHash, plan.Action, plan.EntityType, plan.EntityID, snapshot.Workspace.Revision, commandID, att.StatementHash, att.ConversationRef, att.ApprovedAt); err != nil {
		return result, err
	}
	for i, event := range eventWrites {
		payload, _ := json.Marshal(event.Payload)
		if _, err = tx.Exec(ctx, "INSERT INTO events(id,workspace_id,command_id,workspace_revision,event_type,payload) VALUES($1,$2,$3,$4,$5,$6)", result.EventIDs[i], wid, commandID, newRevision, event.Type, payload); err != nil {
			return result, err
		}
	}
	err = tx.Commit(ctx)
	return result, err
}

func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	s := hex.EncodeToString(b[:])
	return s[:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:]
}
