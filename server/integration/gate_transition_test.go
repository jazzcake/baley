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
	requireDisposableDatabase(t, url)
	ctx := context.Background()
	t.Setenv("BALEY_LEASE_TOKEN_SECRET", "gate-transition-integration-secret")
	repo, err := postgres.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Pool.Close()
	if _, err = repo.Pool.Exec(ctx, "SET session_replication_role='replica'; TRUNCATE events,human_approval_attestations,commands,workspace_counters,runs,gate_tasks,gates,task_dependencies,tasks,lanes,phases,workspaces,actors CASCADE; SET session_replication_role='origin'"); err != nil {
		t.Fatal(err)
	}
	if err = repo.SeedDemo(ctx); err != nil {
		t.Fatal(err)
	}
	if err = repo.BootstrapOwner(ctx, postgres.DemoWorkspaceID,
		"22222222-2222-4222-8222-222222222229", postgres.DemoHumanActorID,
		"gate-owner", "gate-owner", "Gate Owner", "test-password-phc"); err != nil {
		t.Fatal(err)
	}
	principal := linkedConversationalPrincipal(t, ctx, repo, postgres.DemoWorkspaceID,
		postgres.DemoHumanActorID, postgres.DemoAgentActorID, "gate-transition-gateway")
	service := application.NewService(repo)
	for _, statement := range []string{
		"INSERT INTO phases(workspace_id,id,name,position,state) VALUES($1,'release','Release',2,'planned')",
		"INSERT INTO gates(workspace_id,id,public_id,alias,name,from_phase_id,to_phase_id) VALUES($1,'validate-ready',2,'validate-ready','Validate Ready','validate','release')",
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

	oldPreview, oldRequest := previewTask(t, ctx, service, &principal, 101, "old-101", 1)
	withoutApproval := oldRequest
	_, err = service.Execute(ctx, withoutApproval)
	assertCode(t, err, domain.CodeDecisionEvidenceRequired)
	graph, _ := repo.LoadSnapshot(ctx, postgres.DemoWorkspaceID)
	if graph.Workspace.Revision != 1 {
		t.Fatalf("approval failure wrote revision %d", graph.Workspace.Revision)
	}

	preview104, request104 := previewTask(t, ctx, service, &principal, 104, "task-104", 1)
	attachTaskDecisionEvidence(t, &request104, preview104, 104, "33333333-3333-4333-8333-333333333334")
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
	attachTaskDecisionEvidence(t, &oldRequest, oldPreview, 101, "33333333-3333-4333-8333-333333333331")
	_, err = service.Execute(ctx, oldRequest)
	assertCode(t, err, domain.CodeStaleRevision)
	confirmTask(t, ctx, service, &principal, 101, "task-101", 2, "33333333-3333-4333-8333-333333333332")
	result106, request106 := confirmTask(t, ctx, service, &principal, 106, "task-106", 3, "33333333-3333-4333-8333-333333333336")
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
	foundCanonicalApproval := false
	for _, event := range events {
		if event.EventType == "gate.passed" {
			var payload struct {
				Conditions []struct {
					TaskStatus string `json:"taskStatus"`
				} `json:"conditions"`
				EntryTasks []struct {
					TaskID          string `json:"taskId"`
					SelectionSource string `json:"selectionSource"`
				} `json:"entryTasks"`
			}
			if json.Unmarshal(event.Payload, &payload) != nil || len(payload.Conditions) != 3 || payload.Conditions[0].TaskStatus == "" || len(payload.EntryTasks) == 0 || payload.EntryTasks[0].TaskID == "" || payload.EntryTasks[0].SelectionSource == "" {
				t.Fatalf("gate evidence incomplete: %s", event.Payload)
			}
			foundEvidence = true
		}
		if event.EventType == "human_approval_attestation.recorded" {
			var payload map[string]any
			if json.Unmarshal(event.Payload, &payload) == nil && payload["action"] == "gate_pass" {
				foundCanonicalApproval = true
			}
		}
	}
	if !foundEvidence {
		t.Fatal("gate.passed event missing")
	}
	if !foundCanonicalApproval {
		t.Fatal("canonical gate_pass approval evidence missing")
	}
}

func previewTask(t *testing.T, ctx context.Context, s *application.Service, principal *application.CommandPrincipal, id int, key string, revision int64) (application.PreviewResult, application.CommandRequest) {
	t.Helper()
	req := request("task.confirm", map[string]any{"workspaceId": postgres.DemoWorkspaceID, "taskId": id}, key, revision)
	req.Principal = principal
	preview, err := s.Preview(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	return preview, req
}
func confirmTask(t *testing.T, ctx context.Context, s *application.Service, principal *application.CommandPrincipal, id int, key string, revision int64, decisionID string) (application.ExecutionResult, application.CommandRequest) {
	t.Helper()
	preview, req := previewTask(t, ctx, s, principal, id, key, revision)
	attachTaskDecisionEvidence(t, &req, preview, id, decisionID)
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
