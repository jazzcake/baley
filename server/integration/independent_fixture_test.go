package integration_test

import (
	"context"
	"os"
	"testing"

	"github.com/jazzcake/baley/server/internal/application"
	"github.com/jazzcake/baley/server/internal/domain"
	"github.com/jazzcake/baley/server/internal/persistence/postgres"
)

func TestIndependentFixtureGraphAndGateAgainstPostgres(t *testing.T) {
	url := os.Getenv("BALEY_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("BALEY_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	t.Setenv("BALEY_LEASE_TOKEN_SECRET", "independent-fixture-secret")
	repo, err := postgres.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Pool.Close()
	if _, err = repo.Pool.Exec(ctx, "TRUNCATE events,human_approval_attestations,commands,workspace_counters,run_git_observations,commit_references,task_record_indexes,repositories,runs,gate_tasks,gates,task_dependencies,tasks,lanes,phases,workspaces,actors CASCADE"); err != nil {
		t.Fatal(err)
	}
	const workspaceID = "fixture-independent"
	statements := []string{
		`INSERT INTO actors(id,display_name,actor_type) VALUES ('` + postgres.DemoHumanActorID + `','Fixture Owner','human'),('` + postgres.DemoAgentActorID + `','Fixture Operator','agent')`,
		`INSERT INTO workspaces(id,name,state,revision) VALUES ('` + workspaceID + `','Editorial Launch','active',1)`,
		`INSERT INTO phases(workspace_id,id,name,position,state) VALUES ('` + workspaceID + `','prepare','Prepare',0,'active'),('` + workspaceID + `','publish','Publish',1,'planned')`,
		`INSERT INTO lanes(workspace_id,id,name,state) VALUES ('` + workspaceID + `','content','Content','active'),('` + workspaceID + `','legal','Legal','active')`,
		`INSERT INTO tasks(workspace_id,id,public_id,lane_id,phase_id,title,status) VALUES ('` + workspaceID + `','article',201,'content','prepare','Final article','confirmed'),('` + workspaceID + `','approval',202,'legal','prepare','Legal approval','confirmed')`,
		`INSERT INTO gates(workspace_id,id,name,from_phase_id,to_phase_id) VALUES ('` + workspaceID + `','publish-ready','Publish Ready','prepare','publish')`,
		`INSERT INTO gate_tasks(workspace_id,id,gate_id,task_id) VALUES ('` + workspaceID + `','gt-article','publish-ready','article'),('` + workspaceID + `','gt-approval','publish-ready','approval')`,
		`INSERT INTO workspace_counters(workspace_id,next_task_public_id) VALUES ('` + workspaceID + `',203)`,
	}
	for _, statement := range statements {
		if _, err = repo.Pool.Exec(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	service := application.NewService(repo)
	connect := request("dependency.connect", map[string]any{"workspaceId": workspaceID, "predecessorTaskId": 201, "successorTaskId": 202}, "fixture-connect", 1)
	if _, err = service.Execute(ctx, connect); err != nil {
		t.Fatal(err)
	}
	_, err = service.Execute(ctx, request("dependency.connect", map[string]any{"workspaceId": workspaceID, "predecessorTaskId": 202, "successorTaskId": 201}, "fixture-cycle", 2))
	assertCode(t, err, domain.CodeDependencyCycle)
	snapshot, _ := repo.LoadSnapshot(ctx, workspaceID)
	if snapshot.Workspace.Revision != 2 || len(snapshot.Dependencies) != 1 || snapshot.Gates[0].Status != "ready" {
		t.Fatalf("independent fixture projection mismatch: %#v", snapshot)
	}
	gateRequest := request("gate.pass", map[string]any{"workspaceId": workspaceID, "gateId": "publish-ready"}, "fixture-gate-pass", 2)
	preview, err := service.Preview(ctx, gateRequest)
	if err != nil {
		t.Fatal(err)
	}
	gateRequest.Envelope.HumanApprovalAttestation = &application.HumanApprovalAttestation{ApprovedByActorID: postgres.DemoHumanActorID, ApprovedCommandHash: preview.CommandHash, DecisionSnapshotHash: preview.DecisionSnapshotHash}
	if _, err = service.Execute(ctx, gateRequest); err != nil {
		t.Fatal(err)
	}
	snapshot, _ = repo.LoadSnapshot(ctx, workspaceID)
	if snapshot.Gates[0].Status != "passed" || snapshot.Phases[0].State != "completed" || snapshot.Phases[1].State != "active" {
		t.Fatalf("independent Gate transition mismatch: %#v", snapshot)
	}
}
