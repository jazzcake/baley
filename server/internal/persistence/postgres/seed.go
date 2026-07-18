package postgres

import (
	"context"
	"github.com/jackc/pgx/v5"
)

const DemoWorkspaceID = "00000000-0000-4000-8000-000000000001"
const DemoHumanActorID = "00000000-0000-4000-8000-000000000002"
const DemoAgentActorID = "00000000-0000-4000-8000-000000000003"

func (r *Repository) SeedDemo(ctx context.Context) error {
	tx, err := r.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	statements := []string{
		`INSERT INTO actors(id,display_name,actor_type) VALUES ('` + DemoHumanActorID + `','Demo Owner','human'),('` + DemoAgentActorID + `','Codex Operator','agent') ON CONFLICT DO NOTHING`,
		`INSERT INTO workspaces(id,name,revision) VALUES ('` + DemoWorkspaceID + `','Baley Pilot',1) ON CONFLICT DO NOTHING`,
		`INSERT INTO phases(workspace_id,id,name,position,state) VALUES ('` + DemoWorkspaceID + `','build','Build',0,'active'),('` + DemoWorkspaceID + `','validate','Validate',1,'planned') ON CONFLICT DO NOTHING`,
		`INSERT INTO lanes(workspace_id,id,name,state) VALUES ('` + DemoWorkspaceID + `','server','Server','active'),('` + DemoWorkspaceID + `','client','Client','active'),('` + DemoWorkspaceID + `','art','Art','active') ON CONFLICT DO NOTHING`,
		`INSERT INTO tasks(workspace_id,id,public_id,lane_id,phase_id,title,description,status) VALUES ('` + DemoWorkspaceID + `','api',101,'server','build','API 구현','Pilot API 구현','implemented'),('` + DemoWorkspaceID + `','ui',104,'client','build','Pilot UI','Pilot UI 구현','implemented'),('` + DemoWorkspaceID + `','assets',106,'art','build','Asset 제작','Pilot asset 제작','implemented'),('` + DemoWorkspaceID + `','user-test',110,'client','validate','사용자 테스트','Gate 통과 후 사용자 테스트','pending') ON CONFLICT DO NOTHING`,
		`INSERT INTO task_dependencies(workspace_id,from_task_id,to_task_id) VALUES ('` + DemoWorkspaceID + `','ui','user-test') ON CONFLICT DO NOTHING`,
		`INSERT INTO gates(workspace_id,id,name,from_phase_id,to_phase_id) VALUES ('` + DemoWorkspaceID + `','pilot-ready','Pilot Ready','build','validate') ON CONFLICT DO NOTHING`,
		`INSERT INTO gate_tasks(workspace_id,id,gate_id,task_id) VALUES ('` + DemoWorkspaceID + `','gt-api','pilot-ready','api'),('` + DemoWorkspaceID + `','gt-ui','pilot-ready','ui'),('` + DemoWorkspaceID + `','gt-assets','pilot-ready','assets') ON CONFLICT DO NOTHING`,
		`INSERT INTO workspace_counters(workspace_id,next_task_public_id) VALUES ('` + DemoWorkspaceID + `',111) ON CONFLICT DO NOTHING`,
	}
	for _, statement := range statements {
		if _, err = tx.Exec(ctx, statement); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
