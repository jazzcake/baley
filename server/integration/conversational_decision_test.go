package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jazzcake/baley/server/internal/application"
	"github.com/jazzcake/baley/server/internal/authn"
	"github.com/jazzcake/baley/server/internal/authz"
	"github.com/jazzcake/baley/server/internal/domain"
	"github.com/jazzcake/baley/server/internal/persistence/postgres"
)

func TestLinkedAccountConversationalTaskConfirmationTrustBoundary(t *testing.T) {
	url := os.Getenv("BALEY_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("BALEY_TEST_DATABASE_URL is not set")
	}
	requireDisposableDatabase(t, url)
	ctx := context.Background()
	t.Setenv("BALEY_LEASE_TOKEN_SECRET", "conversational-decision-test-secret")
	repo, err := postgres.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Pool.Close()
	if _, err = repo.Pool.Exec(ctx, `SET session_replication_role='replica';
		TRUNCATE security_events,approval_grants,agent_tokens,mcp_gateway_registrations,workspace_memberships,
		account_sessions,account_credentials,accounts,mutation_attempts,events,human_approval_attestations,
		commands,task_journal_entries,workspace_counters,runs,gate_entry_tasks,gate_tasks,gates,
		task_dependencies,tasks,lanes,phases,workspaces,actors CASCADE;
		SET session_replication_role='origin'`); err != nil {
		t.Fatal(err)
	}
	if err = repo.SeedDemo(ctx); err != nil {
		t.Fatal(err)
	}
	if err = repo.BootstrapOwner(ctx, postgres.DemoWorkspaceID,
		"11111111-1111-4111-8111-111111111119", postgres.DemoHumanActorID,
		"conversation-owner", "conversation-owner", "Conversation Owner", "test-password-phc"); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Pool.Exec(ctx, "UPDATE tasks SET status='implemented' WHERE workspace_id=$1 AND public_id IN (101,110)", postgres.DemoWorkspaceID); err != nil {
		t.Fatal(err)
	}
	principal := linkedConversationalPrincipal(t, ctx, repo, postgres.DemoWorkspaceID,
		postgres.DemoHumanActorID, postgres.DemoAgentActorID, "conversation-gateway")
	service := application.NewService(repo)
	newRequest := func(taskID int, revision int64, key string) application.CommandRequest {
		raw, _ := json.Marshal(map[string]any{
			"workspaceId": postgres.DemoWorkspaceID, "taskId": taskID,
			"contextNote": map[string]any{"narrative": "PM explicitly confirmed this delivered outcome.", "context": map[string]any{"outcome": "confirmed"}},
		})
		return application.CommandRequest{Name: "task.confirm", Arguments: raw, Principal: &principal, Envelope: application.CommandEnvelope{
			ExpectedWorkspaceRevision: revision, IdempotencyKey: key, ExecutedByActorID: postgres.DemoAgentActorID,
		}}
	}

	request := newRequest(110, 1, "conversation-confirm-110")
	preview, err := service.Preview(ctx, request)
	if err != nil || len(preview.Warnings) != 0 {
		t.Fatalf("ordinary leaf preview=%+v err=%v", preview, err)
	}
	if _, err = service.Execute(ctx, request); commandErrorCode(err) != domain.CodeDecisionEvidenceRequired {
		t.Fatalf("missing evidence error=%v", err)
	}
	request.Envelope.DecisionEvidence = &application.ConversationalDecisionEvidence{
		DecisionID: "11111111-1111-4111-8111-111111111111", Source: "conversation", ConversationRef: "pm-session:task-186:turn-approval",
		Statement: "confirm #110", Scope: "ambiguous", Action: "task.confirm", TaskID: 110,
		WorkspaceRevision: 1, CommandHash: preview.CommandHash,
	}
	if _, err = service.Execute(ctx, request); commandErrorCode(err) != domain.CodeDecisionEvidenceMismatch {
		t.Fatalf("ambiguous evidence error=%v", err)
	}
	request.Envelope.DecisionEvidence.Scope = "task"
	request.Envelope.DecisionEvidence.Statement = "yes"
	if _, err = service.Execute(ctx, request); commandErrorCode(err) != domain.CodeDecisionEvidenceMismatch {
		t.Fatalf("vague evidence error=%v", err)
	}
	request.Envelope.DecisionEvidence.Statement = "confirm #110"
	result, err := service.Execute(ctx, request)
	if err != nil || result.ApprovalProtocol != "linked_account_conversation" || result.TaskJournalEntryID == "" {
		t.Fatalf("conversational confirmation result=%+v err=%v", result, err)
	}
	var initiatedBy, executedBy, approvedBy, evidenceID string
	if err = repo.Pool.QueryRow(ctx, `SELECT command.initiated_by_actor_id,command.executed_by_actor_id,
		attestation.approved_by_actor_id,attestation.decision_evidence_id::text
		FROM commands command JOIN human_approval_attestations attestation ON attestation.executed_command_id=command.id
		WHERE command.id=$1`, result.CommandID).Scan(&initiatedBy, &executedBy, &approvedBy, &evidenceID); err != nil {
		t.Fatal(err)
	}
	if initiatedBy != postgres.DemoHumanActorID || approvedBy != postgres.DemoHumanActorID || executedBy != postgres.DemoAgentActorID || evidenceID != request.Envelope.DecisionEvidence.DecisionID {
		t.Fatalf("provenance initiated=%s executed=%s approved=%s evidence=%s", initiatedBy, executedBy, approvedBy, evidenceID)
	}
	journal, err := repo.TaskJournal(ctx, postgres.DemoWorkspaceID, 110, time.Time{}, "", 10)
	if err != nil || len(journal) != 1 || journal[0].InitiatedByActorID != postgres.DemoHumanActorID || journal[0].ExecutedByActorID != postgres.DemoAgentActorID || journal[0].ApprovedByActorID != postgres.DemoHumanActorID {
		t.Fatalf("Task Journal provenance=%+v err=%v", journal, err)
	}

	second := newRequest(101, result.WorkspaceRevision, "conversation-confirm-101")
	secondPreview, err := service.Preview(ctx, second)
	if err != nil {
		t.Fatal(err)
	}
	second.Envelope.DecisionEvidence = &application.ConversationalDecisionEvidence{
		DecisionID: request.Envelope.DecisionEvidence.DecisionID, Source: "conversation", ConversationRef: "pm-session:task-186:turn-approval",
		Statement: "complete all awaiting confirmation", Scope: "all_awaiting_confirmation", Action: "task.confirm", TaskID: 101,
		WorkspaceRevision: result.WorkspaceRevision, CommandHash: secondPreview.CommandHash,
	}
	if _, err = service.Execute(ctx, second); commandErrorCode(err) != domain.CodeDecisionEvidenceReplayed {
		t.Fatalf("cross-target replay error=%v", err)
	}
	second.Envelope.DecisionEvidence.DecisionID = "22222222-2222-4222-8222-222222222222"
	second.Envelope.DecisionEvidence.WorkspaceRevision--
	if _, err = service.Execute(ctx, second); commandErrorCode(err) != domain.CodeDecisionEvidenceMismatch {
		t.Fatalf("stale evidence binding error=%v", err)
	}
	second.Envelope.DecisionEvidence.WorkspaceRevision = result.WorkspaceRevision
	if _, err = repo.CreateMember(ctx, postgres.DemoWorkspaceID, postgres.DemoHumanActorID,
		"conversation-backup-owner", "conversation-backup-owner", "Conversation Backup Owner",
		"test-password-phc", authz.RoleOwner); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Pool.Exec(ctx, "UPDATE workspace_memberships SET role='viewer' WHERE workspace_id=$1 AND actor_id=$2", postgres.DemoWorkspaceID, postgres.DemoHumanActorID); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Execute(ctx, second); commandErrorCode(err) != domain.CodeDecisionEvidenceInvalid {
		t.Fatalf("unauthorized linked member error=%v", err)
	}
}

func linkedConversationalPrincipal(t *testing.T, ctx context.Context, repo *postgres.Repository, workspaceID, humanActorID, agentActorID, gatewayID string) application.CommandPrincipal {
	t.Helper()
	secret := "linked-gateway-secret-" + gatewayID
	var registrationID string
	if err := repo.Pool.QueryRow(ctx, `INSERT INTO mcp_gateway_registrations(
		id,workspace_id,account_actor_id,agent_actor_id,gateway_id,gateway_secret_hash,status,generation,created_at)
		VALUES(gen_random_uuid()::text,$1,$2,$3,$4,$5,'active',1,$6) RETURNING id`,
		workspaceID, humanActorID, agentActorID, gatewayID, postgres.DigestSecret(secret), time.Now().UTC()).Scan(&registrationID); err != nil {
		t.Fatal(err)
	}
	token, err := repo.ResumeMCPGateway(ctx, workspaceID, gatewayID, secret, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	authService, err := authn.NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	authenticated, err := authService.AuthenticateBearer(ctx, token.Token)
	if err != nil {
		t.Fatal(err)
	}
	if authenticated.GatewayRegistrationID != registrationID {
		t.Fatalf("gateway registration mismatch: authenticated=%s created=%s", authenticated.GatewayRegistrationID, registrationID)
	}
	return application.CommandPrincipal{
		CredentialID: authenticated.CredentialID, WorkspaceID: authenticated.WorkspaceID, Subject: authenticated.Subject,
		LinkedAccountID: authenticated.LinkedAccountID, LinkedHumanActorID: authenticated.LinkedHumanActorID,
		GatewayRegistrationID: authenticated.GatewayRegistrationID,
	}
}

func attachTaskDecisionEvidence(t *testing.T, request *application.CommandRequest, preview application.PreviewResult, taskID int, decisionID string) {
	t.Helper()
	request.Envelope.DecisionEvidence = &application.ConversationalDecisionEvidence{
		DecisionID: decisionID, Source: "conversation", ConversationRef: "integration-test:" + decisionID,
		Statement: fmt.Sprintf("confirm #%d", taskID), Scope: "task", Action: "task.confirm", TaskID: taskID,
		WorkspaceRevision: preview.ExpectedWorkspaceRevision, CommandHash: preview.CommandHash,
	}
}
