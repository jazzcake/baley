package integration_test

import (
	"context"
	"os"
	"testing"

	"github.com/jazzcake/baley/server/internal/application"
	"github.com/jazzcake/baley/server/internal/persistence/postgres"
)

func TestDelegatedAcceptanceAutoConfirmsOnlyEligibleTasks(t *testing.T) {
	url := os.Getenv("BALEY_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("BALEY_TEST_DATABASE_URL is not set")
	}
	requireDisposableDatabase(t, url)
	ctx := context.Background()
	t.Setenv("BALEY_LEASE_TOKEN_SECRET", "task-acceptance-test-secret")
	repo, err := postgres.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Pool.Close()
	if _, err = repo.Pool.Exec(ctx, "SET session_replication_role='replica'; TRUNCATE events,human_approval_attestations,commands,task_acceptance_evidence,task_acceptance_assignments,backlog_items,workspace_counters,run_git_observations,commit_references,task_record_indexes,repositories,runs,gate_entry_tasks,gate_tasks,gates,task_dependencies,tasks,lanes,phases,workspace_acceptance_policies,evidence_profiles,workspaces,actors CASCADE; SET session_replication_role='origin'"); err != nil {
		t.Fatal(err)
	}
	if err = repo.SeedDemo(ctx); err != nil {
		t.Fatal(err)
	}

	var migratedMode string
	if err = repo.Pool.QueryRow(ctx, "SELECT effective_acceptance_mode FROM tasks WHERE workspace_id=$1 AND public_id=110", postgres.DemoWorkspaceID).Scan(&migratedMode); err != nil {
		t.Fatal(err)
	}
	if migratedMode != "human_required" {
		t.Fatalf("existing task migration mode=%s", migratedMode)
	}

	service := application.NewService(repo)
	policyRequest := request("task.acceptance_policy.change", map[string]any{
		"workspaceId": postgres.DemoWorkspaceID, "policyVersion": "pilot-v2",
		"defaultMode": "delegated", "evidenceProfileId": "technical-v1",
	}, "acceptance-policy-v2", 1)
	policyPreview, err := service.Preview(ctx, policyRequest)
	if err != nil {
		t.Fatal(err)
	}
	policyRequest.Envelope.HumanApprovalAttestation = &application.HumanApprovalAttestation{
		ApprovedByActorID: postgres.DemoHumanActorID, ApprovedCommandHash: policyPreview.CommandHash,
	}
	policyResult, err := service.Execute(ctx, policyRequest)
	if err != nil || policyResult.WorkspaceRevision != 2 {
		t.Fatalf("policy change failed: %#v %v", policyResult, err)
	}
	if _, err = repo.Pool.Exec(ctx, `INSERT INTO evidence_profiles(
		workspace_id,id,version,required_completion_reports,required_verifications,
		required_independent_reviews,allowed_reference_kinds,
		verification_reference_required,review_requires_zero_blockers
	) VALUES($1,'weaker-v1','1',1,1,1,ARRAY['task_record'],true,false)`, postgres.DemoWorkspaceID); err != nil {
		t.Fatal(err)
	}
	_, err = service.Execute(ctx, request("task.create", map[string]any{
		"workspaceId": postgres.DemoWorkspaceID,
		"taskUuid":    "00000000-0000-4000-8000-000000000090",
		"laneId":      "server", "phaseId": "build", "title": "Weakened profile fixture",
		"requestedAcceptanceMode": "delegated", "evidenceProfileId": "weaker-v1",
	}, "weakened-profile-create", 2))
	if commandErrorCode(err) != "human_approval_required" {
		t.Fatalf("delegated profile weakening accepted: %v", err)
	}

	createResult, err := service.Execute(ctx, request("task.create", map[string]any{
		"workspaceId": postgres.DemoWorkspaceID,
		"taskUuid":    "00000000-0000-4000-8000-000000000091",
		"laneId":      "server", "phaseId": "build", "title": "Delegated acceptance fixture",
		"requestedAcceptanceMode": "delegated", "evidenceProfileId": "technical-v1",
	}, "delegated-task-create", 2))
	if err != nil || createResult.WorkspaceRevision != 3 {
		t.Fatalf("delegated task create failed: %#v %v", createResult, err)
	}
	if _, err = repo.Pool.Exec(ctx, "UPDATE tasks SET status='implemented' WHERE workspace_id=$1 AND public_id IN (110,111)", postgres.DemoWorkspaceID); err != nil {
		t.Fatal(err)
	}

	insertAcceptanceRecords(t, ctx, repo, "00000000-0000-4000-8000-000000000091",
		"00000000-0000-4000-8000-000000000092", "00000000-0000-4000-8000-000000000093")
	delegatedEvidence := map[string]any{
		"workspaceId": postgres.DemoWorkspaceID, "taskId": 111,
		"evidenceId":               "00000000-0000-4000-8000-000000000094",
		"completionReportRecordId": "00000000-0000-4000-8000-000000000092",
		"verificationVerdict":      "passed", "verificationReference": "00000000-0000-4000-8000-000000000092",
		"verificationReferenceKind": "task_record",
		"independentReviewRecordId": "00000000-0000-4000-8000-000000000093",
		"reviewVerdict":             "pass", "unresolvedBlockingCount": 0,
	}
	evidenceResult, err := service.Execute(ctx, request("task.evidence.report", delegatedEvidence, "delegated-evidence", 3))
	if err != nil || evidenceResult.WorkspaceRevision != 4 || len(evidenceResult.EventIDs) != 2 {
		t.Fatalf("delegated evidence failed: %#v %v", evidenceResult, err)
	}
	var delegatedStatus string
	if err = repo.Pool.QueryRow(ctx, "SELECT status FROM tasks WHERE workspace_id=$1 AND public_id=111", postgres.DemoWorkspaceID).Scan(&delegatedStatus); err != nil {
		t.Fatal(err)
	}
	if delegatedStatus != "confirmed" {
		t.Fatalf("eligible delegated task status=%s", delegatedStatus)
	}

	insertAcceptanceRecords(t, ctx, repo, "user-test",
		"00000000-0000-4000-8000-000000000095", "00000000-0000-4000-8000-000000000096")
	humanEvidence := map[string]any{
		"workspaceId": postgres.DemoWorkspaceID, "taskId": 110,
		"evidenceId":               "00000000-0000-4000-8000-000000000097",
		"completionReportRecordId": "00000000-0000-4000-8000-000000000095",
		"verificationVerdict":      "passed", "verificationReference": "00000000-0000-4000-8000-000000000095",
		"verificationReferenceKind": "task_record",
		"independentReviewRecordId": "00000000-0000-4000-8000-000000000096",
		"reviewVerdict":             "pass", "unresolvedBlockingCount": 0,
	}
	humanResult, err := service.Execute(ctx, request("task.evidence.report", humanEvidence, "human-required-evidence", 4))
	if err != nil || humanResult.WorkspaceRevision != 5 || len(humanResult.EventIDs) != 1 {
		t.Fatalf("human-required evidence failed: %#v %v", humanResult, err)
	}
	var humanStatus string
	if err = repo.Pool.QueryRow(ctx, "SELECT status FROM tasks WHERE workspace_id=$1 AND public_id=110", postgres.DemoWorkspaceID).Scan(&humanStatus); err != nil {
		t.Fatal(err)
	}
	if humanStatus != "implemented" {
		t.Fatalf("human-required task auto-confirmed: %s", humanStatus)
	}
	invalidReference := map[string]any{
		"workspaceId": postgres.DemoWorkspaceID, "taskId": 110,
		"evidenceId":               "00000000-0000-4000-8000-000000000099",
		"completionReportRecordId": "00000000-0000-4000-8000-000000000095",
		"verificationVerdict":      "passed", "verificationReference": "00000000-0000-4000-8000-000000000088",
		"verificationReferenceKind": "task_record",
		"independentReviewRecordId": "00000000-0000-4000-8000-000000000096",
		"reviewVerdict":             "pass", "unresolvedBlockingCount": 0,
	}
	if _, err = service.Execute(ctx, request("task.evidence.report", invalidReference, "invalid-reference", 5)); commandErrorCode(err) != "invalid_state_transition" {
		t.Fatalf("unbound verification reference accepted: %v", err)
	}
	if _, err = repo.Pool.Exec(ctx, "UPDATE task_acceptance_assignments SET reason='rewrite' WHERE workspace_id=$1", postgres.DemoWorkspaceID); err == nil {
		t.Fatal("append-only acceptance assignments accepted UPDATE")
	}
	if _, err = repo.Pool.Exec(ctx, "DELETE FROM task_acceptance_evidence WHERE workspace_id=$1", postgres.DemoWorkspaceID); err == nil {
		t.Fatal("append-only acceptance evidence accepted DELETE")
	}
	if _, err = repo.Pool.Exec(ctx, "TRUNCATE task_acceptance_evidence"); err == nil {
		t.Fatal("append-only acceptance evidence accepted TRUNCATE")
	}
}

func insertAcceptanceRecords(t *testing.T, ctx context.Context, repo *postgres.Repository, taskID, completionID, reviewID string) {
	t.Helper()
	_, err := repo.Pool.Exec(ctx, `INSERT INTO task_record_indexes(
		workspace_id,id,task_id,record_type,repository_id,relative_path,state,short_summary
	) VALUES
		($1,$2,$3,'completion-report',$4,$5,'reported_uncommitted','completion'),
		($1,$6,$3,'independent-agent-review',$4,$7,'reported_uncommitted','review')`,
		postgres.DemoWorkspaceID, completionID, taskID, postgres.DemoRepositoryID,
		"task-records/acceptance/"+completionID+".md", reviewID,
		"task-records/acceptance/"+reviewID+".md")
	if err != nil {
		t.Fatal(err)
	}
}
