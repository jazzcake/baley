package integration_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/jazzcake/baley/server/internal/application"
	"github.com/jazzcake/baley/server/internal/domain"
	"github.com/jazzcake/baley/server/internal/persistence/postgres"
)

func TestGateTransitionAgainstPostgres(t *testing.T) {
	url := os.Getenv("BALEY_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("BALEY_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	repo, err := postgres.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Pool.Close()
	if _, err = repo.Pool.Exec(ctx, "TRUNCATE events,human_approval_attestations,commands,workspace_counters,gate_tasks,gates,task_dependencies,tasks,lanes,phases,workspaces,actors CASCADE"); err != nil {
		t.Fatal(err)
	}
	if err = repo.SeedDemo(ctx); err != nil {
		t.Fatal(err)
	}
	service := application.NewService(repo)
	for _, statement := range []string{
		"INSERT INTO phases(workspace_id,id,name,position,state) VALUES($1,'release','Release',2,'planned')",
		"INSERT INTO gates(workspace_id,id,name,from_phase_id,to_phase_id) VALUES($1,'validate-ready','Validate Ready','validate','release')",
		"INSERT INTO gate_tasks(workspace_id,id,gate_id,task_id) VALUES($1,'gt-user-test','validate-ready','user-test')",
	} {
		if _, err = repo.Pool.Exec(ctx, statement, postgres.DemoWorkspaceID); err != nil {
			t.Fatal(err)
		}
	}
	future := request("gate.pass_task", map[string]any{"workspaceId": postgres.DemoWorkspaceID, "gateTaskId": "gt-user-test", "reason": "waive for test"}, "future-gate", 1)
	futurePreview, err := service.Preview(ctx, future)
	if err != nil {
		t.Fatal(err)
	}
	future.Envelope.HumanApprovalAttestation = &application.HumanApprovalAttestation{ApprovedByActorID: postgres.DemoHumanActorID, ApprovedCommandHash: futurePreview.CommandHash, DecisionSnapshotHash: futurePreview.DecisionSnapshotHash}
	_, err = service.Execute(ctx, future)
	assertCode(t, err, domain.CodeGateNotCurrent)

	oldPreview, oldRequest := previewTask(t, ctx, service, 101, "old-101", 1)
	withoutApproval := oldRequest
	_, err = service.Execute(ctx, withoutApproval)
	assertCode(t, err, domain.CodeHumanApprovalMismatch)
	graph, _ := repo.LoadSnapshot(ctx, postgres.DemoWorkspaceID)
	if graph.Workspace.Revision != 1 {
		t.Fatalf("approval failure wrote revision %d", graph.Workspace.Revision)
	}

	preview104, request104 := previewTask(t, ctx, service, 104, "task-104", 1)
	request104.Envelope.HumanApprovalAttestation = &application.HumanApprovalAttestation{ApprovedByActorID: postgres.DemoHumanActorID, ApprovedCommandHash: preview104.CommandHash}
	type outcome struct {
		result application.ExecutionResult
		err    error
	}
	concurrent := make(chan outcome, 2)
	for range 2 {
		go func() {
			result, executeErr := service.Execute(ctx, request104)
			concurrent <- outcome{result, executeErr}
		}()
	}
	one, two := <-concurrent, <-concurrent
	if one.err != nil {
		t.Fatal(one.err)
	}
	if two.err != nil {
		t.Fatal(two.err)
	}
	if one.result.CommandID != two.result.CommandID || one.result.Idempotent == two.result.Idempotent {
		t.Fatalf("concurrent idempotency failed: %#v %#v", one.result, two.result)
	}
	oldRequest.Envelope.HumanApprovalAttestation = &application.HumanApprovalAttestation{ApprovedByActorID: postgres.DemoHumanActorID, ApprovedCommandHash: oldPreview.CommandHash}
	_, err = service.Execute(ctx, oldRequest)
	assertCode(t, err, domain.CodeStaleRevision)
	confirmTask(t, ctx, service, 101, "task-101", 2)
	result106, request106 := confirmTask(t, ctx, service, 106, "task-106", 3)
	retry, err := service.Execute(ctx, request106)
	if err != nil || !retry.Idempotent || retry.CommandID != result106.CommandID {
		t.Fatalf("idempotent retry failed: %#v %v", retry, err)
	}
	conflict := request106
	conflict.Arguments, _ = json.Marshal(map[string]any{"workspaceId": postgres.DemoWorkspaceID, "taskId": 101})
	_, err = service.Execute(ctx, conflict)
	assertCode(t, err, domain.CodeIdempotencyConflict)

	graph, _ = repo.LoadSnapshot(ctx, postgres.DemoWorkspaceID)
	if graph.Gates[0].DecisionSnapshotHash == "" || graph.Gates[0].DecisionRequired != "gate.pass" {
		t.Fatalf("ready gate decision binding missing: %#v", graph.Gates[0])
	}
	gateRequest := request("gate.pass", map[string]any{"workspaceId": postgres.DemoWorkspaceID, "gateId": "pilot-ready"}, "gate-pass", graph.Workspace.Revision)
	gatePreview, err := service.Preview(ctx, gateRequest)
	if err != nil {
		t.Fatal(err)
	}
	gateRequest.Envelope.HumanApprovalAttestation = &application.HumanApprovalAttestation{ApprovedByActorID: postgres.DemoHumanActorID, ApprovedCommandHash: gatePreview.CommandHash, DecisionSnapshotHash: gatePreview.DecisionSnapshotHash}
	gateResult, err := service.Execute(ctx, gateRequest)
	if err != nil {
		t.Fatal(err)
	}
	if len(gateResult.EventIDs) != 4 {
		t.Fatalf("gate event count=%d", len(gateResult.EventIDs))
	}
	graph, err = repo.LoadSnapshot(ctx, postgres.DemoWorkspaceID)
	if err != nil {
		t.Fatal(err)
	}
	if graph.Gates[0].Status != "passed" || graph.Phases[0].State != "completed" || graph.Phases[1].State != "active" {
		t.Fatalf("unexpected final projection: %#v", graph)
	}
	events, err := repo.Events(ctx, postgres.DemoWorkspaceID)
	if err != nil {
		t.Fatal(err)
	}
	foundEvidence := false
	for _, event := range events {
		if event.EventType == "gate.passed" {
			var payload struct {
				Conditions []struct {
					TaskStatus string `json:"taskStatus"`
				} `json:"conditions"`
			}
			if json.Unmarshal(event.Payload, &payload) != nil || len(payload.Conditions) != 3 || payload.Conditions[0].TaskStatus == "" {
				t.Fatalf("gate evidence incomplete: %s", event.Payload)
			}
			foundEvidence = true
		}
	}
	if !foundEvidence {
		t.Fatal("gate.passed event missing")
	}
}

func previewTask(t *testing.T, ctx context.Context, s *application.Service, id int, key string, revision int64) (application.PreviewResult, application.CommandRequest) {
	t.Helper()
	req := request("task.confirm", map[string]any{"workspaceId": postgres.DemoWorkspaceID, "taskId": id}, key, revision)
	preview, err := s.Preview(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	return preview, req
}
func confirmTask(t *testing.T, ctx context.Context, s *application.Service, id int, key string, revision int64) (application.ExecutionResult, application.CommandRequest) {
	t.Helper()
	preview, req := previewTask(t, ctx, s, id, key, revision)
	req.Envelope.HumanApprovalAttestation = &application.HumanApprovalAttestation{ApprovedByActorID: postgres.DemoHumanActorID, ApprovedCommandHash: preview.CommandHash}
	result, err := s.Execute(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	return result, req
}
func request(name string, args any, key string, revision int64) application.CommandRequest {
	raw, _ := json.Marshal(args)
	return application.CommandRequest{Name: name, Arguments: raw, Envelope: application.CommandEnvelope{IdempotencyKey: key, ExpectedWorkspaceRevision: revision, ExecutedByActorID: postgres.DemoAgentActorID}}
}
func assertCode(t *testing.T, err error, code string) {
	t.Helper()
	var target *application.CommandError
	if !errors.As(err, &target) || target.Code != code {
		t.Fatalf("error=%v, want %s", err, code)
	}
}
