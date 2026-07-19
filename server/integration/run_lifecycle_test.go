package integration_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jazzcake/baley/server/internal/application"
	"github.com/jazzcake/baley/server/internal/domain"
	"github.com/jazzcake/baley/server/internal/persistence/postgres"
)

func TestRunLifecycleAgainstPostgres(t *testing.T) {
	url := os.Getenv("BALEY_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("BALEY_TEST_DATABASE_URL is not set")
	}
	t.Setenv("BALEY_LEASE_TOKEN_SECRET", "run-lifecycle-integration-secret")
	ctx := context.Background()
	repo, err := postgres.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Pool.Close()
	if _, err = repo.Pool.Exec(ctx, "TRUNCATE events,human_approval_attestations,commands,workspace_counters,runs,gate_tasks,gates,task_dependencies,tasks,lanes,phases,workspaces,actors CASCADE"); err != nil {
		t.Fatal(err)
	}
	if err = repo.SeedDemo(ctx); err != nil {
		t.Fatal(err)
	}
	service := application.NewService(repo)

	start := request("run.start", map[string]any{"workspaceId": postgres.DemoWorkspaceID, "taskId": 110, "clientRunId": "00000000-0000-4000-8000-000000000021", "kind": "detailed_planning"}, "lifecycle-start", 1)
	started, err := service.Execute(ctx, start)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := repo.LoadSnapshot(ctx, postgres.DemoWorkspaceID)
	if err != nil || len(snapshot.Runs) != 1 {
		t.Fatalf("load started Run: runs=%d err=%v", len(snapshot.Runs), err)
	}
	runID := snapshot.Runs[0].ID

	badHeartbeat := request("run.heartbeat", map[string]any{"workspaceId": postgres.DemoWorkspaceID, "runId": runID, "leaseToken": "wrong", "expectedRunVersion": 1}, "heartbeat-wrong-token", 0)
	_, err = service.Execute(ctx, badHeartbeat)
	assertCode(t, err, domain.CodeRunLeaseMismatch)

	heartbeat := request("run.heartbeat", map[string]any{"workspaceId": postgres.DemoWorkspaceID, "runId": runID, "leaseToken": started.LeaseToken, "expectedRunVersion": 1, "extensionSeconds": 60}, "heartbeat-good", 0)
	heartbeatResult, err := service.Execute(ctx, heartbeat)
	if err != nil {
		t.Fatal(err)
	}
	if heartbeatResult.WorkspaceRevision != 2 || len(heartbeatResult.EventIDs) != 0 {
		t.Fatalf("heartbeat changed domain revision or emitted Event: %#v", heartbeatResult)
	}
	snapshot, _ = repo.LoadSnapshot(ctx, postgres.DemoWorkspaceID)
	if snapshot.Workspace.Revision != 2 || snapshot.Runs[0].Version != 2 {
		t.Fatalf("heartbeat persistence mismatch: %#v", snapshot.Runs[0])
	}

	type outcome struct {
		result application.ExecutionResult
		err    error
	}
	results := make(chan outcome, 2)
	for _, terminal := range []struct {
		name, key, summary string
	}{{"run.succeed", "terminal-succeed", "planning completed"}, {"run.interrupt", "terminal-timeout", "lease timeout"}} {
		terminal := terminal
		go func() {
			req := request(terminal.name, map[string]any{"workspaceId": postgres.DemoWorkspaceID, "runId": runID, "expectedRunVersion": 2, "summary": terminal.summary}, terminal.key, 2)
			value, executeErr := service.Execute(ctx, req)
			results <- outcome{value, executeErr}
		}()
	}
	first, second := <-results, <-results
	successes := 0
	var successful outcome
	for _, candidate := range []outcome{first, second} {
		if candidate.err == nil {
			successes++
			successful = candidate
			continue
		}
		var commandErr *application.CommandError
		if !errors.As(candidate.err, &commandErr) || commandErr.Code != domain.CodeStaleRevision {
			t.Fatalf("terminal loser error=%v", candidate.err)
		}
	}
	if successes != 1 || len(successful.result.EventIDs) != 1 || successful.result.WorkspaceRevision != 3 {
		t.Fatalf("terminal race did not produce exactly one transition: %#v %#v", first, second)
	}

	snapshot, _ = repo.LoadSnapshot(ctx, postgres.DemoWorkspaceID)
	terminalRun := snapshot.Runs[0]
	if terminalRun.Version != 3 || terminalRun.Status == "running" || terminalRun.EndedAt == nil {
		t.Fatalf("terminal Run mismatch: %#v", terminalRun)
	}
	correct := request("run.correct", map[string]any{"workspaceId": postgres.DemoWorkspaceID, "runId": runID, "expectedRunVersion": 3, "status": "cancelled", "summary": "operator corrected result", "reason": "terminal classification was wrong"}, "terminal-correct", 3)
	corrected, err := service.Execute(ctx, correct)
	if err != nil || corrected.WorkspaceRevision != 4 || len(corrected.EventIDs) != 1 {
		t.Fatalf("Run correction failed: %#v %v", corrected, err)
	}
	correctedRetry, err := service.Execute(ctx, correct)
	if err != nil || !correctedRetry.Idempotent || correctedRetry.CommandID != corrected.CommandID || correctedRetry.LeaseToken != "" {
		t.Fatalf("Run correction retry changed result or exposed lease token: %#v %v", correctedRetry, err)
	}

	_, err = service.Execute(ctx, request("run.heartbeat", map[string]any{"workspaceId": postgres.DemoWorkspaceID, "runId": runID, "leaseToken": started.LeaseToken, "expectedRunVersion": 4}, "heartbeat-terminal", 0))
	assertCode(t, err, domain.CodeInvalidStateTransition)

	events, err := repo.Events(ctx, postgres.DemoWorkspaceID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 4 || events[0].EventType != "task.started" || events[1].EventType != "run.started" || events[3].EventType != "run.corrected" {
		t.Fatalf("unexpected Run lifecycle Events: %#v", events)
	}
	var correctionPayload map[string]any
	if err = json.Unmarshal(events[3].Payload, &correctionPayload); err != nil || !hasAllKeys(correctionPayload, "previousStatus", "previousResultSummary", "previousErrorSummary", "previousEndedAt", "newStatus", "newResultSummary", "newErrorSummary", "newEndedAt", "reason") {
		t.Fatalf("Run correction Event lacks before/after evidence: %s", events[3].Payload)
	}
	var heartbeatCommands, leakedTokens int
	if err = repo.Pool.QueryRow(ctx, "SELECT count(*) FROM commands WHERE workspace_id=$1 AND command_name='run.heartbeat'", postgres.DemoWorkspaceID).Scan(&heartbeatCommands); err != nil {
		t.Fatal(err)
	}
	if err = repo.Pool.QueryRow(ctx, "SELECT count(*) FROM commands WHERE workspace_id=$1 AND result ? 'leaseToken'", postgres.DemoWorkspaceID).Scan(&leakedTokens); err != nil {
		t.Fatal(err)
	}
	if heartbeatCommands != 1 || leakedTokens != 0 {
		t.Fatalf("heartbeat audit/token persistence mismatch: commands=%d leaked=%d", heartbeatCommands, leakedTokens)
	}

	secondStart := request("run.start", map[string]any{"workspaceId": postgres.DemoWorkspaceID, "taskId": 110, "clientRunId": "00000000-0000-4000-8000-000000000022", "kind": "detailed_planning"}, "restart-sweep-start", 4)
	if _, err = service.Execute(ctx, secondStart); err != nil {
		t.Fatal(err)
	}
	snapshot, _ = repo.LoadSnapshot(ctx, postgres.DemoWorkspaceID)
	var expiringRun application.RunProjection
	for _, candidate := range snapshot.Runs {
		if candidate.Status == "running" {
			expiringRun = candidate
		}
	}
	if expiringRun.ID == "" {
		t.Fatal("second running Run not found")
	}
	if _, err = repo.Pool.Exec(ctx, "UPDATE runs SET heartbeat_at=$1,lease_expires_at=$2 WHERE workspace_id=$3 AND id=$4", time.Now().UTC().Add(-2*time.Minute), time.Now().UTC().Add(-time.Minute), postgres.DemoWorkspaceID, expiringRun.ID); err != nil {
		t.Fatal(err)
	}
	restartedRepo, err := postgres.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer restartedRepo.Pool.Close()
	sweepResults, err := application.NewService(restartedRepo).InterruptExpiredRuns(ctx)
	if err != nil || len(sweepResults) != 1 || sweepResults[0].WorkspaceRevision != 6 {
		t.Fatalf("restart lease sweep failed: %#v %v", sweepResults, err)
	}
	snapshot, _ = restartedRepo.LoadSnapshot(ctx, postgres.DemoWorkspaceID)
	for _, candidate := range snapshot.Runs {
		if candidate.ID == expiringRun.ID && candidate.Status != "interrupted" {
			t.Fatalf("expired Run was not interrupted: %#v", candidate)
		}
	}
}

func hasAllKeys(values map[string]any, keys ...string) bool {
	for _, key := range keys {
		if _, ok := values[key]; !ok {
			return false
		}
	}
	return true
}
