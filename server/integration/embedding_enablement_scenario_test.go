package integration_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jazzcake/baley/server/internal/application"
	"github.com/jazzcake/baley/server/internal/authn"
	"github.com/jazzcake/baley/server/internal/authz"
	"github.com/jazzcake/baley/server/internal/domain"
	"github.com/jazzcake/baley/server/internal/persistence/postgres"
)

func TestEmbeddingEnablementSingleRepositoryScenarioAgainstPostgres(t *testing.T) {
	url := os.Getenv("BALEY_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("BALEY_TEST_DATABASE_URL is not set")
	}
	requireDisposableDatabase(t, url)
	t.Setenv("BALEY_LEASE_TOKEN_SECRET", "embedding-enablement-scenario-secret")
	ctx := context.Background()
	repo, err := postgres.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Pool.Close()
	if _, err = repo.Pool.Exec(ctx, `SET session_replication_role='replica';
		TRUNCATE security_events,approval_grants,agent_tokens,workspace_memberships,
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

	passwordPHC, err := (authn.PasswordHasher{}).Hash("scenario-owner-password")
	if err != nil {
		t.Fatal(err)
	}
	if err = repo.BootstrapOwner(ctx, postgres.DemoWorkspaceID,
		"6279cb62-d52f-4642-942c-15e7bd72c930", postgres.DemoHumanActorID,
		"scenario-owner", "scenario-owner", "Scenario Owner", passwordPHC); err != nil {
		t.Fatal(err)
	}
	workspaceID := "6279cb62-d52f-4642-942c-15e7bd72c931"
	if _, err = repo.CreateOwnedWorkspace(ctx, workspaceID, "Enablement Scenario", postgres.DemoHumanActorID); err != nil {
		t.Fatal(err)
	}
	operatorPHC, err := (authn.PasswordHasher{}).Hash("scenario-operator-password")
	if err != nil {
		t.Fatal(err)
	}
	operator, err := repo.CreateMember(ctx, workspaceID, postgres.DemoHumanActorID,
		"scenario-operator", "scenario-operator", "Scenario Operator", operatorPHC, authz.RoleOperator)
	if err != nil {
		t.Fatal(err)
	}

	gitRoot := t.TempDir()
	gitRun(t, gitRoot, "init")
	gitRun(t, gitRoot, "config", "user.email", "scenario@example.test")
	gitRun(t, gitRoot, "config", "user.name", "Baley Scenario")
	remoteURL := "https://example.test/embedding-enablement-scenario.git"
	gitRun(t, gitRoot, "remote", "add", "origin", remoteURL)
	recordRoot := filepath.Join(gitRoot, "task-records", "e2e")
	if err = os.MkdirAll(recordRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	recordPaths := map[string]string{
		"plan":        filepath.Join(recordRoot, "detailed-plan.md"),
		"completion":  filepath.Join(recordRoot, "completion-report.md"),
		"review":      filepath.Join(recordRoot, "independent-review.md"),
		"measurement": filepath.Join(recordRoot, "pilot-measurement.md"),
	}
	for kind, path := range recordPaths {
		if kind == "measurement" {
			continue
		}
		if err = os.WriteFile(path, []byte("# "+kind+"\n\nscenario evidence\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gitRun(t, gitRoot, "add", "task-records")
	gitRun(t, gitRoot, "commit", "-m", "seed scenario evidence")
	headSHA := strings.TrimSpace(gitRun(t, gitRoot, "rev-parse", "HEAD"))
	planHash := fileSHA256(t, recordPaths["plan"])
	if err = os.WriteFile(recordPaths["plan"], []byte("# plan\n\nscenario evidence\nworking tree drift\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	previousDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	measurementValidator, err := filepath.Abs(filepath.Join(
		"..", "..", ".agents", "skills", "baley-adopt-project", "scripts", "validate_pilot_measurement.py",
	))
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Chdir(gitRoot); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if restoreErr := os.Chdir(previousDirectory); restoreErr != nil {
			t.Errorf("restore working directory: %v", restoreErr)
		}
	}()

	service := application.NewService(repo)
	revision := int64(1)
	execute := func(name string, arguments map[string]any, key string) application.ExecutionResult {
		t.Helper()
		command := request(name, arguments, key, revision)
		command.Envelope.ExecutedByActorID = operator.ActorID
		result, executeErr := service.Execute(ctx, command)
		if executeErr != nil {
			t.Fatalf("%s failed at revision %d: %v", name, revision, executeErr)
		}
		revision = result.WorkspaceRevision
		return result
	}
	humanExecute := func(name string, arguments map[string]any, key string) application.ExecutionResult {
		t.Helper()
		command := request(name, arguments, key, revision)
		command.Envelope.ExecutedByActorID = operator.ActorID
		preview, previewErr := service.Preview(ctx, command)
		if previewErr != nil {
			t.Fatalf("%s preview failed: %v", name, previewErr)
		}
		command.Envelope.HumanApprovalAttestation = &application.HumanApprovalAttestation{
			ApprovedByActorID:    postgres.DemoHumanActorID,
			ApprovedCommandHash:  preview.CommandHash,
			DecisionSnapshotHash: preview.DecisionSnapshotHash,
		}
		result, executeErr := service.Execute(ctx, command)
		if executeErr != nil {
			t.Fatalf("%s human execution failed: %v", name, executeErr)
		}
		revision = result.WorkspaceRevision
		return result
	}

	execute("phase.create", map[string]any{
		"workspaceId": workspaceID, "phaseId": "enablement", "name": "Enablement",
	}, "scenario-phase")
	execute("gate.create", map[string]any{
		"workspaceId": workspaceID, "gateId": "enablement-entry-internal",
		"alias": "enablement-entry", "name": "Enablement Entry",
		"fromPhaseId": "intake", "toPhaseId": "enablement",
	}, "scenario-gate")
	humanExecute("task.acceptance_policy.change", map[string]any{
		"workspaceId": workspaceID, "policyVersion": "scenario-v1",
		"defaultMode": "delegated", "evidenceProfileId": "technical-v1",
	}, "scenario-policy")
	execute("backlog.create", map[string]any{
		"workspaceId": workspaceID, "backlogUuid": "6279cb62-d52f-4642-942c-15e7bd72c932",
		"laneId": "adoption", "title": "Promote delegated acceptance",
	}, "scenario-backlog-one")
	execute("backlog.create", map[string]any{
		"workspaceId": workspaceID, "backlogUuid": "6279cb62-d52f-4642-942c-15e7bd72c933",
		"laneId": "adoption", "title": "Discarded ordering fixture",
	}, "scenario-backlog-two")
	execute("backlog.reorder", map[string]any{
		"workspaceId": workspaceID, "laneId": "adoption",
		"orderedBacklogPublicIds": []int{2, 1},
	}, "scenario-backlog-order")
	execute("backlog.promote", map[string]any{
		"workspaceId": workspaceID, "backlogPublicId": 1,
		"taskUuid": "6279cb62-d52f-4642-942c-15e7bd72c934",
		"phaseId":  "intake", "requestedAcceptanceMode": "delegated",
		"evidenceProfileId": "technical-v1",
	}, "scenario-backlog-promote")
	execute("backlog.discard", map[string]any{
		"workspaceId": workspaceID, "backlogPublicId": 2, "reason": "ordering fixture complete",
	}, "scenario-backlog-discard")
	execute("task.create", map[string]any{
		"workspaceId": workspaceID, "taskUuid": "6279cb62-d52f-4642-942c-15e7bd72c935",
		"laneId": "adoption", "phaseId": "enablement", "title": "Human entry verification",
		"requestedAcceptanceMode": "human_required",
	}, "scenario-entry-task")
	execute("task.create", map[string]any{
		"workspaceId": workspaceID, "taskUuid": "6279cb62-d52f-4642-942c-15e7bd72c947",
		"laneId": "adoption", "phaseId": "intake", "title": "Human acceptance boundary",
		"requestedAcceptanceMode": "human_required",
	}, "scenario-human-task")
	humanExecute("gate.attach_task", map[string]any{
		"workspaceId": workspaceID, "gateId": "G#1", "taskId": 1,
	}, "scenario-gate-condition")
	execute("gate.attach_entry_task", map[string]any{
		"workspaceId": workspaceID, "gateId": "enablement-entry", "taskId": 2,
	}, "scenario-gate-entry")
	execute("repository.register", map[string]any{
		"workspaceId": workspaceID, "repositoryId": "6279cb62-d52f-4642-942c-15e7bd72c936",
		"name": "Scenario Repository", "remoteUrl": remoteURL, "defaultBranch": "master",
		"isRecordRepository": true, "taskRecordsRoot": "task-records",
	}, "scenario-repository")

	snapshot, err := repo.LoadSnapshot(ctx, workspaceID)
	if err != nil {
		t.Fatal(err)
	}
	gateByInternal := application.FindGateByReference(snapshot.Gates, "enablement-entry-internal")
	gateByPublic := application.FindGateByReference(snapshot.Gates, "G#1")
	gateByAlias := application.FindGateByReference(snapshot.Gates, "enablement-entry")
	if gateByInternal == nil || gateByPublic == nil || gateByAlias == nil ||
		gateByInternal.ID != gateByPublic.ID || gateByInternal.ID != gateByAlias.ID ||
		len(gateByInternal.Conditions) != 1 || len(gateByInternal.EntryTasks) != 1 {
		t.Fatalf("Gate references or bindings diverged: %#v", snapshot.Gates)
	}

	execute("run.start", map[string]any{
		"workspaceId": workspaceID, "taskId": 1,
		"clientRunId": "6279cb62-d52f-4642-942c-15e7bd72c937", "kind": "implementation",
	}, "scenario-expiring-run")
	snapshot, err = repo.LoadSnapshot(ctx, workspaceID)
	if err != nil {
		t.Fatal(err)
	}
	expiredRun := snapshot.Runs[len(snapshot.Runs)-1]
	if _, err = repo.Pool.Exec(ctx, `UPDATE runs SET started_at=$1,heartbeat_at=$1,lease_expires_at=$2
		WHERE workspace_id=$3 AND id=$4`, time.Now().UTC().Add(-3*time.Minute),
		time.Now().UTC().Add(-time.Minute), workspaceID, expiredRun.ID); err != nil {
		t.Fatal(err)
	}
	freshRepo, err := postgres.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer freshRepo.Pool.Close()
	freshService := application.NewService(freshRepo)
	swept, err := freshService.InterruptExpiredRuns(ctx)
	if err != nil || len(swept) != 1 {
		t.Fatalf("fresh service did not interrupt expired Run: %#v %v", swept, err)
	}
	revision = swept[0].WorkspaceRevision
	service, repo = freshService, freshRepo
	assertLaneBriefReadOnly(t, ctx, service, repo, workspaceID, "adoption", "")

	execute("run.start", map[string]any{
		"workspaceId": workspaceID, "taskId": 1,
		"clientRunId": "6279cb62-d52f-4642-942c-15e7bd72c938", "kind": "implementation",
		"parentRunId": expiredRun.ID,
	}, "scenario-recovery-run")
	snapshot, err = repo.LoadSnapshot(ctx, workspaceID)
	if err != nil {
		t.Fatal(err)
	}
	recoveryRun := snapshot.Runs[len(snapshot.Runs)-1]
	measurementRecordID := "6279cb62-d52f-4642-942c-15e7bd72c942"
	measurementTime := time.Now().UTC().Truncate(time.Second)
	measurement := fmt.Sprintf(`---
baley_record: 1
record_id: "%s"
task_id: 1
record_type: pilot-measurement
run_id: "%s"
created_at: "%s"
created_by: "scenario-operator"
supersedes: null
---

# Embedding Enablement coherent scenario measurement

`+"```json"+`
{
  "measurement_id": "%s",
  "workspace_id": "%s",
  "lane_id": "adoption",
  "session_id": "embedding-enablement-coherent-scenario",
  "sample_id": "treatment-coherent-01",
  "started_at": "%s",
  "ended_at": "%s",
  "workspace_revision": %d,
  "actor_id": "%s",
  "candidate_ids": ["delegated-acceptance", "human-authority-boundary"],
  "accepted_candidate_ids": ["delegated-acceptance", "human-authority-boundary"],
  "rejection_reasons": [],
  "evidence_reference_ids": ["6279cb62-d52f-4642-942c-15e7bd72c940", "6279cb62-d52f-4642-942c-15e7bd72c941"],
  "mismatch_keys": ["detailed-plan:working-tree-drift"],
  "correction_event_ids": [],
  "gate_id": "G#1",
  "conversation_ref": "integration:embedding-enablement-coherent-scenario",
  "human_decision_turn_count": 0,
  "baseline_or_treatment": "treatment"
}
`+"```"+`
`, measurementRecordID, recoveryRun.ID, measurementTime.Format(time.RFC3339),
		measurementRecordID, workspaceID, measurementTime.Add(-time.Minute).Format(time.RFC3339),
		measurementTime.Format(time.RFC3339), revision, operator.ActorID)
	if err = os.WriteFile(recordPaths["measurement"], []byte(measurement), 0o644); err != nil {
		t.Fatal(err)
	}
	validator := exec.Command("python", measurementValidator, recordPaths["measurement"])
	if output, validateErr := validator.CombinedOutput(); validateErr != nil {
		t.Fatalf("coherent scenario pilot measurement is invalid: %v\n%s", validateErr, output)
	}
	repositoryID := "6279cb62-d52f-4642-942c-15e7bd72c936"
	recordDefinitions := []struct {
		id, kind, path, summary, hash string
	}{
		{"6279cb62-d52f-4642-942c-15e7bd72c939", "detailed-plan", "task-records/e2e/detailed-plan.md", "stale plan evidence", planHash},
		{"6279cb62-d52f-4642-942c-15e7bd72c940", "completion-report", "task-records/e2e/completion-report.md", "completion evidence", fileSHA256(t, recordPaths["completion"])},
		{"6279cb62-d52f-4642-942c-15e7bd72c941", "independent-agent-review", "task-records/e2e/independent-review.md", "review evidence", fileSHA256(t, recordPaths["review"])},
		{measurementRecordID, "pilot-measurement", "task-records/e2e/pilot-measurement.md", "measurement evidence", fileSHA256(t, recordPaths["measurement"])},
	}
	for index, record := range recordDefinitions {
		execute("record.register", map[string]any{
			"workspaceId": workspaceID, "recordId": record.id, "taskId": 1,
			"runId": recoveryRun.ID, "recordType": record.kind,
			"repositoryId": repositoryID, "relativePath": record.path,
			"shortSummary": record.summary, "workingTreeHash": record.hash,
		}, "scenario-record-"+string(rune('a'+index)))
	}
	execute("commit.attach", map[string]any{
		"workspaceId": workspaceID, "commitId": "6279cb62-d52f-4642-942c-15e7bd72c943",
		"taskId": 1, "runId": recoveryRun.ID, "repositoryId": repositoryID,
		"commitSha": headSHA, "relation": "produced",
	}, "scenario-commit")
	dirty := true
	execute("git.observe", map[string]any{
		"workspaceId": workspaceID, "observationId": "6279cb62-d52f-4642-942c-15e7bd72c944",
		"runId": recoveryRun.ID, "repositoryId": repositoryID,
		"observedAt": time.Now().UTC(), "headCommitSha": headSHA,
		"branchHint": "master", "worktreeLabel": "scenario", "dirty": dirty,
	}, "scenario-git-observation")
	assertLaneBriefReadOnly(t, ctx, service, repo, workspaceID, "adoption", "stale")

	execute("run.succeed", map[string]any{
		"workspaceId": workspaceID, "runId": recoveryRun.ID,
		"expectedRunVersion": 1, "summary": "recovered implementation complete",
	}, "scenario-recovery-succeed")
	reportImplementedWithWarnings(t, ctx, service, operator.ActorID, workspaceID, 1, &revision, "scenario-task-one-implemented")
	execute("task.evidence.report", map[string]any{
		"workspaceId": workspaceID, "taskId": 1,
		"evidenceId":                "6279cb62-d52f-4642-942c-15e7bd72c945",
		"completionReportRecordId":  "6279cb62-d52f-4642-942c-15e7bd72c940",
		"verificationVerdict":       "passed",
		"verificationReference":     "6279cb62-d52f-4642-942c-15e7bd72c942",
		"verificationReferenceKind": "task_record",
		"independentReviewRecordId": "6279cb62-d52f-4642-942c-15e7bd72c941",
		"reviewVerdict":             "pass", "unresolvedBlockingCount": 0,
	}, "scenario-delegated-evidence")

	execute("run.start", map[string]any{
		"workspaceId": workspaceID, "taskId": 3,
		"clientRunId": "6279cb62-d52f-4642-942c-15e7bd72c946", "kind": "implementation",
	}, "scenario-human-run")
	snapshot, _ = repo.LoadSnapshot(ctx, workspaceID)
	humanRun := snapshot.Runs[len(snapshot.Runs)-1]
	execute("run.succeed", map[string]any{
		"workspaceId": workspaceID, "runId": humanRun.ID,
		"expectedRunVersion": 1, "summary": "human-required implementation complete",
	}, "scenario-human-succeed")
	reportImplementedWithWarnings(t, ctx, service, operator.ActorID, workspaceID, 3, &revision, "scenario-task-three-implemented")

	snapshot, err = repo.LoadSnapshot(ctx, workspaceID)
	if err != nil {
		t.Fatal(err)
	}
	if taskByPublicID(snapshot.Tasks, 1).Status != "confirmed" ||
		taskByPublicID(snapshot.Tasks, 2).Status != "pending" ||
		taskByPublicID(snapshot.Tasks, 3).Status != "implemented" ||
		snapshot.Workspace.ActivePhaseID == nil || *snapshot.Workspace.ActivePhaseID != "intake" ||
		snapshot.Workspace.State != "active" || snapshot.Lanes[0].State != "active" ||
		snapshot.Gates[0].Status != "ready" || snapshot.Gates[0].PassedAt != nil ||
		snapshot.Gates[0].DecisionRequired != "gate.pass" {
		t.Fatalf("acceptance crossed authority boundary: workspace=%#v lanes=%#v gates=%#v tasks=%#v",
			snapshot.Workspace, snapshot.Lanes, snapshot.Gates, snapshot.Tasks)
	}

	assertHumanOnlyNoWrite(t, ctx, service, repo, operator.ActorID, workspaceID, revision,
		"task.confirm", map[string]any{"workspaceId": workspaceID, "taskId": 3}, "scenario-no-human-task")
	assertHumanOnlyNoWrite(t, ctx, service, repo, operator.ActorID, workspaceID, revision,
		"gate.pass", map[string]any{"workspaceId": workspaceID, "gateId": "G#1"}, "scenario-no-human-gate")
	assertHumanOnlyNoWrite(t, ctx, service, repo, operator.ActorID, workspaceID, revision,
		"lane.close_out", map[string]any{"workspaceId": workspaceID, "laneId": "adoption", "reason": "must remain human"}, "scenario-no-human-lane")
	workspaceClose := request("workspace.close", map[string]any{"workspaceId": workspaceID}, "scenario-no-human-workspace", revision)
	workspaceClose.Envelope.ExecutedByActorID = operator.ActorID
	if _, err = service.Execute(ctx, workspaceClose); commandErrorCode(err) != "invalid_request" {
		t.Fatalf("unexposed workspace.close unexpectedly executable: %v", err)
	}
	afterWorkspaceClose, _ := repo.LoadSnapshot(ctx, workspaceID)
	if afterWorkspaceClose.Workspace.Revision != revision || afterWorkspaceClose.Workspace.State != "active" {
		t.Fatal("unsupported workspace.close changed state")
	}

	rawCanaries := []string{"approval-text-canary-124", "agent-token-canary-124", "password-canary-124"}
	rejected := request("task.update", map[string]any{
		"workspaceId": workspaceID, "taskId": 3,
		"description": strings.Join(rawCanaries, " "),
	}, "scenario-redaction-rejected", revision-1)
	rejected.Envelope.ExecutedByActorID = operator.ActorID
	if _, err = service.Execute(ctx, rejected); commandErrorCode(err) != domain.CodeStaleRevision {
		t.Fatalf("redaction fixture was not rejected as stale: %v", err)
	}
	repeated := request("task.update", map[string]any{
		"workspaceId": workspaceID, "taskId": 3,
		"description": strings.Join(rawCanaries, " "),
	}, "scenario-redaction-rejected-repeat", revision-1)
	repeated.Envelope.ExecutedByActorID = operator.ActorID
	if _, err = service.Execute(ctx, repeated); commandErrorCode(err) != domain.CodeStaleRevision {
		t.Fatalf("repeated redaction fixture was not rejected as stale: %v", err)
	}
	var auditRows string
	if err = repo.Pool.QueryRow(ctx, `SELECT coalesce(string_agg(to_jsonb(attempt)::text,' '),'')
		FROM mutation_attempts attempt
		WHERE workspace_id=$1 AND command_name='task.update'`, workspaceID).Scan(&auditRows); err != nil {
		t.Fatal(err)
	}
	for _, canary := range rawCanaries {
		if strings.Contains(auditRows, canary) {
			t.Fatalf("raw canary leaked into mutation audit: %s", canary)
		}
	}
	if !strings.Contains(auditRows, "argumentDigest") && !strings.Contains(auditRows, "argument_digest") {
		t.Fatalf("stable argument digest missing from audit: %s", auditRows)
	}
	var attemptCount, distinctDigestCount int
	var stableDigest string
	if err = repo.Pool.QueryRow(ctx, `SELECT count(*),count(DISTINCT argument_digest),coalesce(min(argument_digest),'')
		FROM mutation_attempts WHERE workspace_id=$1 AND command_name='task.update'`,
		workspaceID).Scan(&attemptCount, &distinctDigestCount, &stableDigest); err != nil {
		t.Fatal(err)
	}
	if attemptCount != 2 || distinctDigestCount != 1 || stableDigest == "" {
		t.Fatalf("identical rejected arguments did not produce one stable digest: count=%d distinct=%d digest=%q",
			attemptCount, distinctDigestCount, stableDigest)
	}
}

func reportImplementedWithWarnings(t *testing.T, ctx context.Context, service *application.Service, actorID, workspaceID string, taskID int, revision *int64, key string) {
	t.Helper()
	command := request("task.report_implemented", map[string]any{
		"workspaceId": workspaceID, "taskId": taskID,
		"assessment": "Scenario implementation and verification complete.",
	}, key, *revision)
	command.Envelope.ExecutedByActorID = actorID
	preview, err := service.Preview(ctx, command)
	if err != nil {
		t.Fatal(err)
	}
	for _, warning := range preview.Warnings {
		command.Envelope.AcknowledgedWarningCodes = append(command.Envelope.AcknowledgedWarningCodes, warning.Code)
	}
	if len(preview.Warnings) > 0 {
		command.Envelope.ProceedReason = "The scenario explicitly verifies this Task boundary."
	}
	result, err := service.Execute(ctx, command)
	if err != nil {
		t.Fatal(err)
	}
	*revision = result.WorkspaceRevision
}

func assertLaneBriefReadOnly(t *testing.T, ctx context.Context, service *application.Service, repo *postgres.Repository, workspaceID, laneID, expectedAlignment string) {
	t.Helper()
	before, err := repo.LoadSnapshot(ctx, workspaceID)
	if err != nil {
		t.Fatal(err)
	}
	var commandsBefore, eventsBefore int
	if err = repo.Pool.QueryRow(ctx, "SELECT count(*) FROM commands WHERE workspace_id=$1", workspaceID).Scan(&commandsBefore); err != nil {
		t.Fatal(err)
	}
	if err = repo.Pool.QueryRow(ctx, "SELECT count(*) FROM events WHERE workspace_id=$1", workspaceID).Scan(&eventsBefore); err != nil {
		t.Fatal(err)
	}
	brief, err := service.LaneBrief(ctx, workspaceID, laneID)
	if err != nil {
		snapshot, _ := repo.LoadSnapshot(ctx, workspaceID)
		t.Fatalf("lane brief failed: %v workspace=%#v lanes=%#v phases=%#v tasks=%#v gates=%#v runs=%#v",
			err, snapshot.Workspace, snapshot.Lanes, snapshot.Phases, snapshot.Tasks, snapshot.Gates, snapshot.Runs)
	}
	if expectedAlignment != "" {
		found := false
		for _, evidence := range brief.RecentEvidence {
			if evidence.Alignment == expectedAlignment {
				found = true
			}
		}
		if !found {
			t.Fatalf("lane brief lacks %s Git/Record mismatch: %#v", expectedAlignment, brief.RecentEvidence)
		}
	}
	after, err := repo.LoadSnapshot(ctx, workspaceID)
	if err != nil {
		t.Fatal(err)
	}
	var commandsAfter, eventsAfter int
	_ = repo.Pool.QueryRow(ctx, "SELECT count(*) FROM commands WHERE workspace_id=$1", workspaceID).Scan(&commandsAfter)
	_ = repo.Pool.QueryRow(ctx, "SELECT count(*) FROM events WHERE workspace_id=$1", workspaceID).Scan(&eventsAfter)
	if after.Workspace.Revision != before.Workspace.Revision || commandsAfter != commandsBefore || eventsAfter != eventsBefore {
		t.Fatalf("lane brief was not read-only: revision %d->%d commands %d->%d events %d->%d",
			before.Workspace.Revision, after.Workspace.Revision, commandsBefore, commandsAfter, eventsBefore, eventsAfter)
	}
}

func assertHumanOnlyNoWrite(t *testing.T, ctx context.Context, service *application.Service, repo *postgres.Repository, actorID, workspaceID string, revision int64, name string, arguments map[string]any, key string) {
	t.Helper()
	command := request(name, arguments, key, revision)
	command.Envelope.ExecutedByActorID = actorID
	preview, err := service.Preview(ctx, command)
	if err != nil || !diagnosticCodePresent(preview.Errors, domain.CodeHumanApprovalRequired) {
		t.Fatalf("%s preview lacks human approval boundary: %#v %v", name, preview.Errors, err)
	}
	if _, err = service.Execute(ctx, command); commandErrorCode(err) != domain.CodeHumanApprovalRequired &&
		commandErrorCode(err) != domain.CodeHumanApprovalMismatch {
		t.Fatalf("%s executed without human approval: %v", name, err)
	}
	snapshot, loadErr := repo.LoadSnapshot(ctx, workspaceID)
	if loadErr != nil || snapshot.Workspace.Revision != revision {
		t.Fatalf("%s changed revision without approval: %d %v", name, snapshot.Workspace.Revision, loadErr)
	}
}

func gitRun(t *testing.T, directory string, arguments ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", directory}, arguments...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(arguments, " "), err, output)
	}
	return string(output)
}

func fileSHA256(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(content)
	return "sha256:" + hex.EncodeToString(digest[:])
}
