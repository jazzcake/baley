package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/jazzcake/baley/server/internal/application"
	"github.com/jazzcake/baley/server/internal/authn"
	"github.com/jazzcake/baley/server/internal/authz"
	"github.com/jazzcake/baley/server/internal/domain"
	"github.com/jazzcake/baley/server/internal/persistence/postgres"
)

type approvalCommandCase struct {
	name      string
	arguments func(string) map[string]any
	prepare   func(context.Context, *postgres.Repository, string) error
	onePhase  bool
}

func TestHumanOnlyCommandsRequireAndConsumeBrowserSessionGrant(t *testing.T) {
	url := os.Getenv("BALEY_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("BALEY_TEST_DATABASE_URL is not set")
	}
	requireDisposableDatabase(t, url)
	t.Setenv("BALEY_LEASE_TOKEN_SECRET", "human-approval-grant-integration-secret")
	ctx := context.Background()
	repo, err := postgres.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Pool.Close()
	resetApprovalGrantFixture(t, ctx, repo)
	human, authService := approvalHumanSession(t, ctx, repo)
	service := application.NewService(repo)

	cases := []approvalCommandCase{
		{name: "workspace.close", onePhase: true, arguments: func(w string) map[string]any { return map[string]any{"workspaceId": w} }},
		{name: "lane.close_out", arguments: func(w string) map[string]any {
			return map[string]any{"workspaceId": w, "laneId": "lane", "reason": "work completed"}
		}},
		{name: "lane.discard", arguments: func(w string) map[string]any {
			return map[string]any{"workspaceId": w, "laneId": "lane", "reason": "work cancelled"}
		}},
		{name: "task.confirm", arguments: func(w string) map[string]any { return map[string]any{"workspaceId": w, "taskId": 1} }},
		{name: "task.discard", arguments: func(w string) map[string]any {
			return map[string]any{"workspaceId": w, "taskId": 2, "reason": "not required"}
		}},
		{name: "task.acceptance_policy.change", arguments: func(w string) map[string]any {
			return map[string]any{"workspaceId": w, "policyVersion": "human-only-v2", "defaultMode": "human_required", "evidenceProfileId": "technical-v1"}
		}},
		{name: "gate.attach_task", arguments: func(w string) map[string]any { return map[string]any{"workspaceId": w, "gateId": "gate", "taskId": 4} }},
		{name: "gate.pass_task", arguments: func(w string) map[string]any {
			return map[string]any{"workspaceId": w, "gateTaskId": "gt-pass", "reason": "human exception"}
		}},
		{name: "gate.revoke_task_pass", prepare: func(ctx context.Context, repo *postgres.Repository, workspaceID string) error {
			_, err := repo.Pool.Exec(ctx, "UPDATE gate_tasks SET passed_at=now(),passed_by_actor_id=$1,pass_reason='temporary' WHERE workspace_id=$2 AND id='gt-pass'", postgres.DemoHumanActorID, workspaceID)
			return err
		}, arguments: func(w string) map[string]any {
			return map[string]any{"workspaceId": w, "gateTaskId": "gt-pass", "reason": "exception withdrawn"}
		}},
		{name: "gate.pass", arguments: func(w string) map[string]any { return map[string]any{"workspaceId": w, "gateId": "gate"} }},
	}

	for index, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			workspaceID := fmt.Sprintf("approval-matrix-%02d", index)
			createApprovalWorkspace(t, ctx, repo, workspaceID, item.onePhase)
			if item.prepare != nil {
				if err := item.prepare(ctx, repo, workspaceID); err != nil {
					t.Fatal(err)
				}
			}
			agent := approvalAgentPrincipal(t, ctx, repo, authService, workspaceID, fmt.Sprintf("matrix-%02d", index))
			request := approvalRequest(t, item.name, item.arguments(workspaceID), agent, 1, "missing")
			preview, err := service.Preview(ctx, request)
			if err != nil {
				t.Fatal(err)
			}
			if !approvalHasDiagnostic(preview.Errors, domain.CodeHumanApprovalRequired) {
				t.Fatalf("%s preview did not declare human approval: %+v", item.name, preview.Errors)
			}
			warnings, reason := approvalWarnings(preview)
			request.Envelope.AcknowledgedWarningCodes = warnings
			request.Envelope.ProceedReason = reason
			if _, err = service.Execute(ctx, request); err == nil {
				t.Fatalf("%s accepted an Agent bearer without a grant", item.name)
			}

			forged := request
			forged.Envelope.IdempotencyKey = "forged"
			forged.Envelope.ApprovalGrantID = "ffffffff-ffff-4fff-8fff-ffffffffffff"
			if _, err = service.Execute(ctx, forged); commandErrorCode(err) != domain.CodeApprovalGrantInvalid {
				t.Fatalf("%s forged grant result=%v", item.name, err)
			}

			legacy := request
			legacy.Envelope.IdempotencyKey = "legacy"
			legacy.Envelope.HumanApprovalAttestation = &application.HumanApprovalAttestation{
				ApprovedByActorID: postgres.DemoHumanActorID, ApprovedCommandHash: preview.CommandHash,
			}
			if _, err = service.Execute(ctx, legacy); commandErrorCode(err) != domain.CodeHumanApprovalMismatch {
				t.Fatalf("%s accepted legacy body approval authority: %v", item.name, err)
			}

			grant, err := repo.CreateApprovalGrant(ctx, human, workspaceID, item.name, preview, warnings, reason)
			if err != nil {
				t.Fatal(err)
			}
			browser := approvalRequest(t, item.name, item.arguments(workspaceID), application.CommandPrincipal{
				AccountID: human.AccountID, CredentialID: human.SessionID, SessionID: human.SessionID, Subject: human.Subject,
			}, 1, "browser-positive")
			browser.Envelope.ApprovalGrantID = grant.ID
			browser.Envelope.AcknowledgedWarningCodes = warnings
			browser.Envelope.ProceedReason = reason
			result, err := service.Execute(ctx, browser)
			if err != nil || result.ApprovalProtocol != "browser_session_approval_grant" {
				t.Fatalf("%s browser grant execution failed: result=%+v err=%v", item.name, result, err)
			}
			var status, commandID string
			if err = repo.Pool.QueryRow(ctx, "SELECT status,consumed_by_command_id FROM approval_grants WHERE id=$1", grant.ID).Scan(&status, &commandID); err != nil || status != "consumed" || commandID != result.CommandID {
				t.Fatalf("%s grant was not singly consumed: status=%q command=%q err=%v", item.name, status, commandID, err)
			}
		})
	}
}

func TestApprovalGrantRejectsStaleReplayCrossWorkspaceAndRevokedSession(t *testing.T) {
	url := os.Getenv("BALEY_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("BALEY_TEST_DATABASE_URL is not set")
	}
	requireDisposableDatabase(t, url)
	t.Setenv("BALEY_LEASE_TOKEN_SECRET", "human-approval-binding-integration-secret")
	ctx := context.Background()
	repo, err := postgres.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Pool.Close()
	resetApprovalGrantFixture(t, ctx, repo)
	human, authService := approvalHumanSession(t, ctx, repo)
	service := application.NewService(repo)

	createGrant := func(t *testing.T, workspaceID string) (application.CommandRequest, postgres.ApprovalGrantResult) {
		createApprovalWorkspace(t, ctx, repo, workspaceID, false)
		agent := approvalAgentPrincipal(t, ctx, repo, authService, workspaceID, workspaceID)
		request := approvalRequest(t, "task.confirm", map[string]any{"workspaceId": workspaceID, "taskId": 1}, agent, 1, workspaceID)
		preview, previewErr := service.Preview(ctx, request)
		if previewErr != nil {
			t.Fatal(previewErr)
		}
		warnings, reason := approvalWarnings(preview)
		request.Envelope.AcknowledgedWarningCodes = warnings
		request.Envelope.ProceedReason = reason
		grant, grantErr := repo.CreateApprovalGrant(ctx, human, workspaceID, request.Name, preview, warnings, reason)
		if grantErr != nil {
			t.Fatal(grantErr)
		}
		request.Envelope.ApprovalGrantID = grant.ID
		request.Envelope.AcknowledgedWarningCodes = warnings
		request.Envelope.ProceedReason = reason
		return request, grant
	}

	t.Run("stale", func(t *testing.T) {
		request, _ := createGrant(t, "approval-stale")
		update := approvalRequest(t, "lane.update", map[string]any{"workspaceId": "approval-stale", "laneId": "lane", "name": "Lane", "summary": "revision changed"}, *request.Principal, 1, "advance")
		if _, err := service.Execute(ctx, update); err != nil {
			t.Fatal(err)
		}
		if _, err := service.Execute(ctx, request); commandErrorCode(err) != domain.CodeStaleRevision {
			t.Fatalf("stale grant result=%v", err)
		}
	})

	t.Run("cross_workspace", func(t *testing.T) {
		requestA, grantA := createGrant(t, "approval-cross-a")
		requestB, _ := createGrant(t, "approval-cross-b")
		requestB.Envelope.IdempotencyKey = "cross-use"
		requestB.Envelope.ApprovalGrantID = grantA.ID
		if _, err := service.Execute(ctx, requestB); commandErrorCode(err) != domain.CodeApprovalGrantInvalid {
			t.Fatalf("cross-Workspace grant result=%v requestA=%s", err, requestA.Envelope.ApprovalGrantID)
		}
	})

	t.Run("replay", func(t *testing.T) {
		request, grant := createGrant(t, "approval-replay")
		if _, err := service.Execute(ctx, request); err != nil {
			t.Fatal(err)
		}
		request.Envelope.IdempotencyKey = "replay-new-idempotency"
		request.Envelope.ExpectedWorkspaceRevision = 2
		if _, err := service.Execute(ctx, request); err == nil {
			t.Fatal("consumed grant replay was accepted")
		}
		var status string
		if err := repo.Pool.QueryRow(ctx, "SELECT status FROM approval_grants WHERE id=$1", grant.ID).Scan(&status); err != nil || status != "consumed" {
			t.Fatalf("replayed grant status=%q err=%v", status, err)
		}
	})

	t.Run("command_binding_mismatch", func(t *testing.T) {
		request, _ := createGrant(t, "approval-command-mismatch")
		request.Envelope.ProceedReason += " tampered"
		if _, err := service.Execute(ctx, request); commandErrorCode(err) != domain.CodeApprovalGrantMismatch {
			t.Fatalf("command-binding mismatch result=%v", err)
		}
	})

	t.Run("explicitly_revoked", func(t *testing.T) {
		request, grant := createGrant(t, "approval-explicit-revoke")
		if err := repo.RevokeApprovalGrant(ctx, "approval-explicit-revoke", grant.ID, human); err != nil {
			t.Fatal(err)
		}
		if _, err := service.Execute(ctx, request); commandErrorCode(err) != domain.CodeApprovalGrantInvalid {
			t.Fatalf("explicitly revoked grant result=%v", err)
		}
		var auditCount int
		if err := repo.Pool.QueryRow(ctx, "SELECT count(*) FROM security_events WHERE workspace_id=$1 AND entity_id=$2 AND event_type='approval_grant.revoked'", "approval-explicit-revoke", grant.ID).Scan(&auditCount); err != nil || auditCount != 1 {
			t.Fatalf("explicit revocation audit count=%d err=%v", auditCount, err)
		}
	})

	t.Run("expired", func(t *testing.T) {
		request, grant := createGrant(t, "approval-expired")
		if _, err := repo.Pool.Exec(ctx, "UPDATE approval_grants SET expires_at=now()-interval '1 second' WHERE id=$1", grant.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := service.Execute(ctx, request); commandErrorCode(err) != domain.CodeApprovalGrantInvalid {
			t.Fatalf("expired grant result=%v", err)
		}
		var status string
		var auditCount int
		if err := repo.Pool.QueryRow(ctx, "SELECT status FROM approval_grants WHERE id=$1", grant.ID).Scan(&status); err != nil || status != "expired" {
			t.Fatalf("expired grant status=%q err=%v", status, err)
		}
		if err := repo.Pool.QueryRow(ctx, "SELECT count(*) FROM security_events WHERE workspace_id=$1 AND entity_id=$2 AND event_type='approval_grant.expired'", "approval-expired", grant.ID).Scan(&auditCount); err != nil || auditCount != 1 {
			t.Fatalf("expiry audit count=%d err=%v", auditCount, err)
		}
	})

	t.Run("session_revoked", func(t *testing.T) {
		request, grant := createGrant(t, "approval-session-revoked")
		if err := authService.Logout(ctx, human.SessionID); err != nil {
			t.Fatal(err)
		}
		if _, err := service.Execute(ctx, request); commandErrorCode(err) != domain.CodeApprovalGrantInvalid {
			t.Fatalf("revoked-session grant result=%v", err)
		}
		var status string
		if err := repo.Pool.QueryRow(ctx, "SELECT status FROM approval_grants WHERE id=$1", grant.ID).Scan(&status); err != nil || status != "revoked" {
			t.Fatalf("session-revoked grant status=%q err=%v", status, err)
		}
	})

	t.Run("membership_revoked", func(t *testing.T) {
		resetApprovalGrantFixture(t, ctx, repo)
		_, authService2 := approvalHumanSession(t, ctx, repo)
		workspaceID := "approval-membership-revoked"
		createApprovalWorkspace(t, ctx, repo, workspaceID, false)
		approverPassword := "membership revocation approver password"
		approverPHC, err := (authn.PasswordHasher{}).Hash(approverPassword)
		if err != nil {
			t.Fatal(err)
		}
		approver, err := repo.CreateMember(ctx, workspaceID, postgres.DemoHumanActorID,
			"approval-revoked-approver", "approval-revoked-approver", "Revoked Approver", approverPHC, authz.RoleApprover)
		if err != nil {
			t.Fatal(err)
		}
		approverLogin, err := authService2.Login(ctx, "approval-revoked-approver", approverPassword, "127.0.0.1:2")
		if err != nil {
			t.Fatal(err)
		}
		agent := approvalAgentPrincipal(t, ctx, repo, authService2, workspaceID, "membership-revoked")
		request := approvalRequest(t, "task.confirm", map[string]any{"workspaceId": workspaceID, "taskId": 1}, agent, 1, "membership-revoked")
		preview, err := service.Preview(ctx, request)
		if err != nil {
			t.Fatal(err)
		}
		warnings, reason := approvalWarnings(preview)
		grant, err := repo.CreateApprovalGrant(ctx, approverLogin.Principal, workspaceID, request.Name, preview, warnings, reason)
		if err != nil {
			t.Fatal(err)
		}
		request.Envelope.ApprovalGrantID = grant.ID
		request.Envelope.AcknowledgedWarningCodes = warnings
		request.Envelope.ProceedReason = reason
		if _, err = repo.Pool.Exec(ctx, "UPDATE workspace_memberships SET active=false,deactivated_at=now() WHERE workspace_id=$1 AND actor_id=$2", workspaceID, approver.ActorID); err != nil {
			t.Fatal(err)
		}
		if _, err = service.Execute(ctx, request); commandErrorCode(err) != domain.CodeApprovalGrantInvalid {
			t.Fatalf("membership-revoked grant result=%v", err)
		}
		var status string
		var auditCount int
		if err = repo.Pool.QueryRow(ctx, "SELECT status FROM approval_grants WHERE id=$1", grant.ID).Scan(&status); err != nil || status != "revoked" {
			t.Fatalf("membership-revoked grant status=%q err=%v", status, err)
		}
		if err = repo.Pool.QueryRow(ctx, "SELECT count(*) FROM security_events WHERE workspace_id=$1 AND entity_id=$2 AND event_type='approval_grant.revoked'", workspaceID, grant.ID).Scan(&auditCount); err != nil || auditCount != 1 {
			t.Fatalf("membership revocation audit count=%d err=%v", auditCount, err)
		}
	})
}

func resetApprovalGrantFixture(t *testing.T, ctx context.Context, repo *postgres.Repository) {
	t.Helper()
	_, err := repo.Pool.Exec(ctx, "SET session_replication_role='replica'; TRUNCATE security_events,approval_grants,agent_tokens,workspace_memberships,account_sessions,account_credentials,accounts,events,human_approval_attestations,commands,workspace_counters,runs,gate_entry_tasks,gate_tasks,gates,task_dependencies,tasks,lanes,phases,workspaces,actors CASCADE; SET session_replication_role='origin'")
	if err != nil {
		t.Fatal(err)
	}
	if err = repo.SeedDemo(ctx); err != nil {
		t.Fatal(err)
	}
}

func approvalHumanSession(t *testing.T, ctx context.Context, repo *postgres.Repository) (authn.Principal, *authn.Service) {
	t.Helper()
	password := "approval grant browser password"
	phc, err := (authn.PasswordHasher{}).Hash(password)
	if err != nil {
		t.Fatal(err)
	}
	if err = repo.BootstrapOwner(ctx, postgres.DemoWorkspaceID, "90000000-0000-4000-8000-000000000001", postgres.DemoHumanActorID, "grant-owner", "grant-owner", "Grant Owner", phc); err != nil {
		t.Fatal(err)
	}
	authService, err := authn.NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	login, err := authService.Login(ctx, "grant-owner", password, "127.0.0.1:1")
	if err != nil {
		t.Fatal(err)
	}
	return login.Principal, authService
}

func createApprovalWorkspace(t *testing.T, ctx context.Context, repo *postgres.Repository, workspaceID string, onePhase bool) {
	t.Helper()
	tx, err := repo.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	statements := []string{
		"INSERT INTO workspaces(id,name,state,revision) VALUES($1,$2,'active',1)",
		"INSERT INTO workspace_memberships(workspace_id,actor_id,role,active,created_by_actor_id) VALUES($1,$2,'owner',true,$2),($1,$3,'operator',true,$2)",
		"INSERT INTO phases(workspace_id,id,name,position,state) VALUES($1,'p1','Active',1,'active')",
		"INSERT INTO lanes(workspace_id,id,name,state) VALUES($1,'lane','Lane','active')",
		"INSERT INTO tasks(workspace_id,id,public_id,lane_id,phase_id,title,status) VALUES($1,'t-confirm',1,'lane','p1','Confirm','implemented'),($1,'t-discard',2,'lane','p1','Discard','pending'),($1,'t-gate',3,'lane','p1','Gate condition','confirmed'),($1,'t-attach',4,'lane','p1','Attach','implemented')",
		"INSERT INTO workspace_counters(workspace_id,next_task_public_id,next_gate_public_id) VALUES($1,5,2)",
	}
	for index, statement := range statements {
		var execErr error
		switch index {
		case 0:
			_, execErr = tx.Exec(ctx, statement, workspaceID, workspaceID)
		case 1:
			_, execErr = tx.Exec(ctx, statement, workspaceID, postgres.DemoHumanActorID, postgres.DemoAgentActorID)
		default:
			_, execErr = tx.Exec(ctx, statement, workspaceID)
		}
		if execErr != nil {
			t.Fatal(execErr)
		}
	}
	if !onePhase {
		if _, err = tx.Exec(ctx, "INSERT INTO phases(workspace_id,id,name,position,state) VALUES($1,'p2','Next',2,'planned')", workspaceID); err == nil {
			_, err = tx.Exec(ctx, "INSERT INTO gates(workspace_id,id,public_id,alias,name,from_phase_id,to_phase_id) VALUES($1,'gate',1,'gate','Gate','p1','p2')", workspaceID)
		}
		if err == nil {
			_, err = tx.Exec(ctx, "INSERT INTO gate_tasks(workspace_id,id,gate_id,task_id) VALUES($1,'gt-pass','gate','t-gate')", workspaceID)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}

func approvalAgentPrincipal(t *testing.T, ctx context.Context, repo *postgres.Repository, authService *authn.Service, workspaceID, name string) application.CommandPrincipal {
	t.Helper()
	token, err := repo.IssueAgentToken(ctx, workspaceID, postgres.DemoAgentActorID, name, postgres.DemoHumanActorID, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	principal, err := authService.AuthenticateBearer(ctx, token.Token)
	if err != nil {
		t.Fatal(err)
	}
	return application.CommandPrincipal{CredentialID: principal.CredentialID, WorkspaceID: principal.WorkspaceID, Subject: principal.Subject}
}

func approvalRequest(t *testing.T, name string, arguments map[string]any, principal application.CommandPrincipal, revision int64, key string) application.CommandRequest {
	t.Helper()
	raw, err := json.Marshal(arguments)
	if err != nil {
		t.Fatal(err)
	}
	return application.CommandRequest{Name: name, Arguments: raw, Principal: &principal, Envelope: application.CommandEnvelope{
		ExpectedWorkspaceRevision: int64(revision), IdempotencyKey: key, ExecutedByActorID: principal.Subject.ActorID,
	}}
}

func approvalWarnings(preview application.PreviewResult) ([]string, string) {
	warnings := make([]string, 0, len(preview.Warnings))
	seen := map[string]bool{}
	for _, warning := range preview.Warnings {
		if !seen[warning.Code] {
			warnings = append(warnings, warning.Code)
			seen[warning.Code] = true
		}
	}
	if len(warnings) == 0 {
		return warnings, ""
	}
	return warnings, "Human reviewed the exact warning set"
}

func approvalHasDiagnostic(values []domain.Diagnostic, code string) bool {
	for _, value := range values {
		if value.Code == code {
			return true
		}
	}
	return false
}
