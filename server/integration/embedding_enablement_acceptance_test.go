package integration_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jazzcake/baley/server/internal/application"
	"github.com/jazzcake/baley/server/internal/persistence/postgres"
)

func TestEmbeddingEnablementCrossFeatureAcceptanceAgainstPostgres(t *testing.T) {
	url := os.Getenv("BALEY_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("BALEY_TEST_DATABASE_URL is not set")
	}
	requireDisposableDatabase(t, url)
	t.Setenv("BALEY_LEASE_TOKEN_SECRET", "embedding-enablement-cross-feature-secret")
	ctx := context.Background()
	repo, err := postgres.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Pool.Close()
	if _, err = repo.Pool.Exec(ctx, `SET session_replication_role='replica';
		TRUNCATE security_events,agent_tokens,workspace_memberships,
		account_sessions,account_credentials,accounts,mutation_attempts,
		events,human_approval_attestations,commands,task_acceptance_evidence,
		task_acceptance_assignments,workspace_acceptance_policies,evidence_profiles,
		backlog_items,workspace_counters,run_git_observations,commit_references,
		task_record_indexes,repositories,runs,gate_entry_tasks,gate_tasks,gates,
		task_dependencies,tasks,lanes,phases,workspaces,actors CASCADE;
		SET session_replication_role='origin'`); err != nil {
		t.Fatal(err)
	}
	if err = repo.SeedDemo(ctx); err != nil {
		t.Fatal(err)
	}

	service := application.NewService(repo)
	policyRequest := request("task.acceptance_policy.change", map[string]any{
		"workspaceId": postgres.DemoWorkspaceID, "policyVersion": "acceptance-e2e-v1",
		"defaultMode": "human_required", "evidenceProfileId": "technical-v1",
	}, "acceptance-e2e-policy", 1)
	policyPreview, err := service.Preview(ctx, policyRequest)
	if err != nil {
		t.Fatal(err)
	}
	policyRequest.Envelope.HumanApprovalAttestation = &application.HumanApprovalAttestation{
		ApprovedByActorID:   postgres.DemoHumanActorID,
		ApprovedCommandHash: policyPreview.CommandHash,
	}
	if _, err = service.Execute(ctx, policyRequest); err != nil {
		t.Fatal(err)
	}

	if _, err = service.Execute(ctx, request("task.create", map[string]any{
		"workspaceId": postgres.DemoWorkspaceID,
		"taskUuid":    "6279cb62-d52f-4642-942c-15e7bd72c920",
		"laneId":      "server", "phaseId": "build",
		"title":                   "Cross-feature human-required acceptance",
		"requestedAcceptanceMode": "human_required",
		"evidenceProfileId":       "technical-v1",
	}, "acceptance-e2e-task", 2)); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Execute(ctx, request("run.start", map[string]any{
		"workspaceId": postgres.DemoWorkspaceID, "taskId": 111,
		"clientRunId": "6279cb62-d52f-4642-942c-15e7bd72c921",
		"kind":        "implementation",
	}, "acceptance-e2e-run-start", 3)); err != nil {
		t.Fatal(err)
	}
	snapshot, err := repo.LoadSnapshot(ctx, postgres.DemoWorkspaceID)
	if err != nil {
		t.Fatal(err)
	}
	var runID string
	for _, candidate := range snapshot.Runs {
		if candidate.TaskID == "6279cb62-d52f-4642-942c-15e7bd72c920" {
			runID = candidate.ID
		}
	}
	if runID == "" {
		t.Fatal("implementation Run was not projected")
	}
	if _, err = service.Execute(ctx, request("run.succeed", map[string]any{
		"workspaceId": postgres.DemoWorkspaceID, "runId": runID,
		"expectedRunVersion": 1, "summary": "cross-feature implementation passed",
	}, "acceptance-e2e-run-succeed", 4)); err != nil {
		t.Fatal(err)
	}

	reportArguments := map[string]any{
		"workspaceId": postgres.DemoWorkspaceID, "taskId": 111,
		"assessment": "Implementation, isolated verification, and review evidence are ready.",
	}
	reportRequest := request("task.report_implemented", reportArguments, "acceptance-e2e-implemented", 5)
	reportPreview, err := service.Preview(ctx, reportRequest)
	if err != nil {
		t.Fatal(err)
	}
	if len(reportPreview.Warnings) > 0 {
		codes := make([]string, 0, len(reportPreview.Warnings))
		for _, warning := range reportPreview.Warnings {
			codes = append(codes, warning.Code)
		}
		reportRequest = request("task.report_implemented", reportArguments, "acceptance-e2e-implemented", 5)
		reportRequest.Envelope.AcknowledgedWarningCodes = codes
		reportRequest.Envelope.ProceedReason = "The fixture deliberately isolates human-required Task acceptance."
	}
	if _, err = service.Execute(ctx, reportRequest); err != nil {
		t.Fatal(err)
	}

	insertAcceptanceRecords(t, ctx, repo, "6279cb62-d52f-4642-942c-15e7bd72c920",
		"6279cb62-d52f-4642-942c-15e7bd72c922", "6279cb62-d52f-4642-942c-15e7bd72c923")
	if _, err = service.Execute(ctx, request("record.register", map[string]any{
		"workspaceId": postgres.DemoWorkspaceID,
		"recordId":    "6279cb62-d52f-4642-942c-15e7bd72c924",
		"taskId":      111, "runId": runID, "recordType": "pilot-measurement",
		"repositoryId": postgres.DemoRepositoryID,
		"relativePath": "task-records/embedding-enablement-acceptance/pilot-measurement-e2e.md",
		"shortSummary": "Cross-feature Enablement acceptance measurement",
	}, "acceptance-e2e-measurement", 6)); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Execute(ctx, request("task.evidence.report", map[string]any{
		"workspaceId": postgres.DemoWorkspaceID, "taskId": 111,
		"evidenceId":                "6279cb62-d52f-4642-942c-15e7bd72c925",
		"completionReportRecordId":  "6279cb62-d52f-4642-942c-15e7bd72c922",
		"verificationVerdict":       "passed",
		"verificationReference":     "6279cb62-d52f-4642-942c-15e7bd72c924",
		"verificationReferenceKind": "task_record",
		"independentReviewRecordId": "6279cb62-d52f-4642-942c-15e7bd72c923",
		"reviewVerdict":             "pass", "unresolvedBlockingCount": 0,
	}, "acceptance-e2e-evidence", 7)); err != nil {
		t.Fatal(err)
	}

	snapshot, err = repo.LoadSnapshot(ctx, postgres.DemoWorkspaceID)
	if err != nil {
		t.Fatal(err)
	}
	var delegatedStatus string
	for _, task := range snapshot.Tasks {
		if task.PublicID == 111 {
			delegatedStatus = task.Status
		}
	}
	if delegatedStatus != "implemented" {
		t.Fatalf("Agent evidence confirmed a human-required Task: %s", delegatedStatus)
	}
	if snapshot.Workspace.ActivePhaseID == nil || *snapshot.Workspace.ActivePhaseID != "build" || snapshot.Gates[0].Status != "open" {
		t.Fatalf("Task acceptance widened authority: phase=%s gate=%s",
			valueOrEmpty(snapshot.Workspace.ActivePhaseID), snapshot.Gates[0].Status)
	}
	if len(snapshot.Records) != 3 {
		t.Fatalf("completion, review, and measurement evidence not restored: %#v", snapshot.Records)
	}

	if _, err = repo.Pool.Exec(ctx, "UPDATE tasks SET status='implemented' WHERE workspace_id=$1 AND public_id=110", postgres.DemoWorkspaceID); err != nil {
		t.Fatal(err)
	}
	insertAcceptanceRecords(t, ctx, repo, "user-test",
		"6279cb62-d52f-4642-942c-15e7bd72c926", "6279cb62-d52f-4642-942c-15e7bd72c927")
	if _, err = service.Execute(ctx, request("task.evidence.report", map[string]any{
		"workspaceId": postgres.DemoWorkspaceID, "taskId": 110,
		"evidenceId":                "6279cb62-d52f-4642-942c-15e7bd72c928",
		"completionReportRecordId":  "6279cb62-d52f-4642-942c-15e7bd72c926",
		"verificationVerdict":       "passed",
		"verificationReference":     "6279cb62-d52f-4642-942c-15e7bd72c926",
		"verificationReferenceKind": "task_record",
		"independentReviewRecordId": "6279cb62-d52f-4642-942c-15e7bd72c927",
		"reviewVerdict":             "pass", "unresolvedBlockingCount": 0,
	}, "acceptance-e2e-human-boundary", 8)); err != nil {
		t.Fatal(err)
	}
	if err = repo.Pool.QueryRow(ctx,
		"SELECT status FROM tasks WHERE workspace_id=$1 AND public_id=110",
		postgres.DemoWorkspaceID).Scan(&delegatedStatus); err != nil {
		t.Fatal(err)
	}
	if delegatedStatus != "implemented" {
		t.Fatalf("human-required Task crossed its authority boundary: %s", delegatedStatus)
	}

	attempts, err := repo.MutationAttempts(ctx, postgres.DemoWorkspaceID, "succeeded", "task.evidence.report", time.Time{}, "", 20)
	if err != nil || len(attempts) != 2 {
		t.Fatalf("acceptance mutation audit missing: %#v %v", attempts, err)
	}
	for _, attempt := range attempts {
		if attempt.IdempotencyKeyHash == "" || attempt.ArgumentDigest == "" {
			t.Fatalf("acceptance mutation audit is incomplete: %#v", attempt)
		}
	}
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
