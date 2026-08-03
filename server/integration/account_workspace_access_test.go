package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/jazzcake/baley/server/internal/application"
	"github.com/jazzcake/baley/server/internal/authn"
	"github.com/jazzcake/baley/server/internal/authz"
	"github.com/jazzcake/baley/server/internal/domain"
	"github.com/jazzcake/baley/server/internal/persistence/postgres"
	"github.com/jazzcake/baley/server/internal/transport/httpapi"
)

func TestMigration14DownUp(t *testing.T) {
	url := os.Getenv("BALEY_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("BALEY_TEST_DATABASE_URL is not set")
	}
	requireDisposableDatabase(t, url)
	t.Setenv("BALEY_LEASE_TOKEN_SECRET", "migration-14-integration-secret")
	deletePilotMeasurementRecords(t, url)
	migrations := filepath.Join("..", "migrations")
	// Exercise migration 14 from the latest schema by stepping back to 13.
	for range 4 {
		if err := postgres.Migrate(url, migrations, "down"); err != nil {
			t.Fatal(err)
		}
	}
	if err := postgres.Migrate(url, migrations, "up"); err != nil {
		t.Fatal(err)
	}
	repo, err := postgres.Open(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Pool.Close()
	for _, table := range []string{"accounts", "account_sessions", "workspace_memberships", "agent_tokens", "security_events"} {
		var exists *string
		if err = repo.Pool.QueryRow(context.Background(), "SELECT to_regclass($1)::text", table).Scan(&exists); err != nil || exists == nil {
			t.Fatalf("table %s missing after down/up: %v", table, err)
		}
	}
}

func TestAccountWorkspaceAccessAndAuthenticatedApprovalAgainstPostgres(t *testing.T) {
	url := os.Getenv("BALEY_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("BALEY_TEST_DATABASE_URL is not set")
	}
	requireDisposableDatabase(t, url)
	t.Setenv("BALEY_LEASE_TOKEN_SECRET", "account-access-integration-secret")
	ctx := context.Background()
	repo, err := postgres.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Pool.Close()
	if _, err = repo.Pool.Exec(ctx, "SET session_replication_role='replica'; TRUNCATE security_events,agent_tokens,workspace_memberships,account_sessions,account_credentials,accounts,events,human_approval_attestations,commands,workspace_counters,runs,gate_entry_tasks,gate_tasks,gates,task_dependencies,tasks,lanes,phases,workspaces,actors CASCADE; SET session_replication_role='origin'"); err != nil {
		t.Fatal(err)
	}
	if err = repo.SeedDemo(ctx); err != nil {
		t.Fatal(err)
	}
	password := "correct horse battery staple"
	phc, err := (authn.PasswordHasher{}).Hash(password)
	if err != nil {
		t.Fatal(err)
	}
	accountID := "10000000-0000-4000-8000-000000000135"
	if err = repo.BootstrapOwner(ctx, postgres.DemoWorkspaceID, accountID, postgres.DemoHumanActorID, "owner", "owner", "Pilot Owner", phc); err != nil {
		t.Fatal(err)
	}
	if err = repo.ValidateEnforcedOwners(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.CreateOwnedWorkspace(ctx, postgres.DemoWorkspaceID, "Baley Pilot", postgres.DemoHumanActorID); err == nil {
		t.Fatal("seeded Workspace was misclassified as an idempotent create retry")
	}
	authService, err := authn.NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	handler := (&httpapi.API{
		Service: application.NewService(repo), Repo: repo, Auth: authService, AuthMode: "enforced",
		AllowedOrigins: []string{"http://localhost:5173"},
	}).Handler()
	var failureBodies []string
	for _, body := range []string{
		`{"loginId":"missing","password":"correct horse battery staple"}`,
		`{"loginId":"owner","password":"incorrect horse battery staple"}`,
	} {
		failedLogin := httptest.NewRequest(http.MethodPost, "/v1/auth/login", bytes.NewBufferString(body))
		failedLogin.Header.Set("Origin", "http://localhost:5173")
		failedResponse := httptest.NewRecorder()
		handler.ServeHTTP(failedResponse, failedLogin)
		if failedResponse.Code != http.StatusUnauthorized {
			t.Fatalf("generic login failure status=%d", failedResponse.Code)
		}
		failureBodies = append(failureBodies, failedResponse.Body.String())
	}
	if failureBodies[0] != failureBodies[1] {
		t.Fatalf("login enumeration response differs: %q / %q", failureBodies[0], failureBodies[1])
	}
	invalidOriginLogin := httptest.NewRequest(http.MethodPost, "/v1/auth/login", bytes.NewBufferString(`{"loginId":"owner","password":"correct horse battery staple"}`))
	invalidOriginLogin.Header.Set("Origin", "https://example.invalid")
	invalidOriginResponse := httptest.NewRecorder()
	handler.ServeHTTP(invalidOriginResponse, invalidOriginLogin)
	if invalidOriginResponse.Code != http.StatusForbidden {
		t.Fatalf("invalid login Origin status=%d", invalidOriginResponse.Code)
	}
	unauthenticated := httptest.NewRequest(http.MethodGet, "/v1/workspaces/"+postgres.DemoWorkspaceID+"/graph", nil)
	unauthenticatedResponse := httptest.NewRecorder()
	handler.ServeHTTP(unauthenticatedResponse, unauthenticated)
	if unauthenticatedResponse.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated graph status=%d", unauthenticatedResponse.Code)
	}
	for attempt := 0; attempt < 6; attempt++ {
		body, _ := json.Marshal(map[string]string{
			"loginId":  "unknown-behind-proxy-" + strconv.Itoa(attempt),
			"password": "correct horse battery staple",
		})
		proxyFailure := httptest.NewRequest(http.MethodPost, "/v1/auth/login", bytes.NewReader(body))
		proxyFailure.Header.Set("Origin", "http://localhost:5173")
		proxyFailure.RemoteAddr = "127.0.0.1:49000"
		proxyFailureResponse := httptest.NewRecorder()
		handler.ServeHTTP(proxyFailureResponse, proxyFailure)
		if proxyFailureResponse.Code != http.StatusUnauthorized {
			t.Fatalf("independent account failure behind proxy status=%d body=%s", proxyFailureResponse.Code, proxyFailureResponse.Body.String())
		}
	}
	loginBody := bytes.NewBufferString(`{"loginId":"owner","password":"correct horse battery staple"}`)
	loginRequest := httptest.NewRequest(http.MethodPost, "/v1/auth/login", loginBody)
	loginRequest.Header.Set("Origin", "http://localhost:5173")
	loginRequest.RemoteAddr = "127.0.0.1:49000"
	loginResponse := httptest.NewRecorder()
	handler.ServeHTTP(loginResponse, loginRequest)
	if loginResponse.Code != http.StatusOK {
		t.Fatalf("HTTP login status=%d body=%s", loginResponse.Code, loginResponse.Body.String())
	}
	var sessionCookie, csrfCookie *http.Cookie
	for _, cookie := range loginResponse.Result().Cookies() {
		switch cookie.Name {
		case "baley_session":
			sessionCookie = cookie
		case "baley_csrf":
			csrfCookie = cookie
		}
	}
	if sessionCookie == nil || csrfCookie == nil {
		t.Fatal("login cookies missing")
	}
	ownerMutation := func(method, path string, body []byte) *httptest.ResponseRecorder {
		request := httptest.NewRequest(method, path, bytes.NewReader(body))
		request.Header.Set("Origin", "http://localhost:5173")
		request.Header.Set("X-Baley-CSRF", csrfCookie.Value)
		request.AddCookie(sessionCookie)
		request.AddCookie(csrfCookie)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}
	createdWorkspaceID := "30000000-0000-4000-8000-000000000123"
	createWorkspaceResponse := ownerMutation(http.MethodPost, "/v1/workspaces", []byte(`{
		"workspaceId":"30000000-0000-4000-8000-000000000123",
		"name":"Adoption Pilot"
	}`))
	if createWorkspaceResponse.Code != http.StatusCreated {
		t.Fatalf("Workspace create status=%d body=%s", createWorkspaceResponse.Code, createWorkspaceResponse.Body.String())
	}
	createdSnapshot, err := repo.LoadSnapshot(ctx, createdWorkspaceID)
	if err != nil {
		t.Fatal(err)
	}
	if createdSnapshot.Workspace.State != "active" || createdSnapshot.Workspace.ActivePhaseID == nil || *createdSnapshot.Workspace.ActivePhaseID != "intake" ||
		len(createdSnapshot.Lanes) != 1 || createdSnapshot.Lanes[0].ID != "adoption" ||
		createdSnapshot.AcceptancePolicy.DefaultMode != domain.AcceptanceHumanRequired {
		t.Fatalf("created Workspace bootstrap drift: %+v", createdSnapshot)
	}
	createdMembership, err := repo.Membership(ctx, createdWorkspaceID, postgres.DemoHumanActorID)
	if err != nil || createdMembership == nil || createdMembership.Role != authz.RoleOwner || !createdMembership.Active {
		t.Fatalf("creator Owner binding missing: %+v %v", createdMembership, err)
	}
	retryWorkspaceResponse := ownerMutation(http.MethodPost, "/v1/workspaces", []byte(`{
		"workspaceId":"30000000-0000-4000-8000-000000000123",
		"name":"Adoption Pilot"
	}`))
	var retriedWorkspace struct {
		ID         string `json:"id"`
		Idempotent bool   `json:"idempotent"`
	}
	if retryWorkspaceResponse.Code != http.StatusOK || json.Unmarshal(retryWorkspaceResponse.Body.Bytes(), &retriedWorkspace) != nil ||
		retriedWorkspace.ID != createdWorkspaceID || !retriedWorkspace.Idempotent {
		t.Fatalf("Workspace create retry status=%d body=%s", retryWorkspaceResponse.Code, retryWorkspaceResponse.Body.String())
	}
	var workspaceCreateEvents int
	if err = repo.Pool.QueryRow(ctx, `SELECT count(*) FROM security_events
		WHERE workspace_id=$1 AND event_type='workspace.created'`, createdWorkspaceID).Scan(&workspaceCreateEvents); err != nil || workspaceCreateEvents != 1 {
		t.Fatalf("Workspace create retry emitted duplicate events: count=%d err=%v", workspaceCreateEvents, err)
	}
	duplicateWorkspaceResponse := ownerMutation(http.MethodPost, "/v1/workspaces", []byte(`{
		"workspaceId":"30000000-0000-4000-8000-000000000123",
		"name":"Different"
	}`))
	if duplicateWorkspaceResponse.Code != http.StatusConflict {
		t.Fatalf("duplicate Workspace create status=%d", duplicateWorkspaceResponse.Code)
	}
	missingCSRF := httptest.NewRequest(http.MethodPost, "/v1/auth/logout", nil)
	missingCSRF.Header.Set("Origin", "http://localhost:5173")
	missingCSRF.AddCookie(sessionCookie)
	missingCSRFResponse := httptest.NewRecorder()
	handler.ServeHTTP(missingCSRFResponse, missingCSRF)
	if missingCSRFResponse.Code != http.StatusForbidden {
		t.Fatalf("missing CSRF status=%d", missingCSRFResponse.Code)
	}
	if _, err = repo.Pool.Exec(ctx, "INSERT INTO workspaces(id,name,state,revision) VALUES('20000000-0000-4000-8000-000000000134','Other Workspace','draft',1)"); err != nil {
		t.Fatal(err)
	}
	crossWorkspace := httptest.NewRequest(http.MethodGet, "/v1/workspaces/20000000-0000-4000-8000-000000000134/graph", nil)
	crossWorkspace.AddCookie(sessionCookie)
	crossResponse := httptest.NewRecorder()
	handler.ServeHTTP(crossResponse, crossWorkspace)
	if crossResponse.Code != http.StatusNotFound {
		t.Fatalf("cross-Workspace status=%d", crossResponse.Code)
	}
	crossWorkspaceCommand, _ := json.Marshal(map[string]any{
		"name":      "phase.create",
		"arguments": map[string]any{"workspaceId": "20000000-0000-4000-8000-000000000134", "phaseId": "foreign", "name": "Foreign"},
		"envelope":  map[string]any{"idempotencyKey": "foreign-command", "expectedWorkspaceRevision": 1, "executedByActorId": postgres.DemoHumanActorID},
	})
	crossCommandResponse := ownerMutation(http.MethodPost, "/v1/commands/execute", crossWorkspaceCommand)
	if crossCommandResponse.Code != http.StatusNotFound {
		t.Fatalf("cross-Workspace command status=%d body=%s", crossCommandResponse.Code, crossCommandResponse.Body.String())
	}
	malformedCrossCommand := []byte(`{"name":"phase.create","arguments":{"workspaceId":"20000000-0000-4000-8000-000000000134"},"envelope":{"idempotencyKey":"malformed-foreign","expectedWorkspaceRevision":1,"executedByActorId":"forged"},"unexpected":true}`)
	malformedCrossResponse := ownerMutation(http.MethodPost, "/v1/commands/execute", malformedCrossCommand)
	if malformedCrossResponse.Code != http.StatusBadRequest {
		t.Fatalf("malformed cross-Workspace command status=%d body=%s", malformedCrossResponse.Code, malformedCrossResponse.Body.String())
	}
	var foreignAttemptCount, centralDenialCount int
	if err = repo.Pool.QueryRow(ctx, "SELECT count(*) FROM mutation_attempts WHERE workspace_id='20000000-0000-4000-8000-000000000134'").Scan(&foreignAttemptCount); err != nil || foreignAttemptCount != 0 {
		t.Fatalf("foreign Workspace mutation audit polluted: count=%d err=%v", foreignAttemptCount, err)
	}
	if err = repo.Pool.QueryRow(ctx, "SELECT count(*) FROM security_events WHERE workspace_id IS NULL AND event_type='authorization.workspace_denied'").Scan(&centralDenialCount); err != nil || centralDenialCount != 2 {
		t.Fatalf("central authorization denial events=%d err=%v", centralDenialCount, err)
	}
	login, err := authService.Login(ctx, "OWNER", password, "127.0.0.1:12345")
	if err != nil {
		t.Fatal(err)
	}
	if login.SessionToken == "" || login.CSRFToken == "" {
		t.Fatal("raw session material was not returned once")
	}
	var leaked int
	if err = repo.Pool.QueryRow(ctx, `SELECT count(*) FROM account_credentials
		WHERE password_phc LIKE '%' || $1 || '%'`, password).Scan(&leaked); err != nil || leaked != 0 {
		t.Fatalf("raw password persisted: count=%d err=%v", leaked, err)
	}
	token, err := repo.IssueAgentToken(ctx, postgres.DemoWorkspaceID, postgres.DemoAgentActorID, "pilot-agent", postgres.DemoHumanActorID, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.IssueAgentToken(ctx, postgres.DemoWorkspaceID, postgres.DemoAgentActorID, "escalated-agent", postgres.DemoHumanActorID, []authz.Capability{authz.GateApprove}, nil); err == nil {
		t.Fatal("Agent approval scope escalation was accepted")
	}
	agent, err := authService.AuthenticateBearer(ctx, token.Token)
	if err != nil {
		t.Fatal(err)
	}
	if agent.Subject.Kind != authz.ActorAgent || agent.WorkspaceID != postgres.DemoWorkspaceID || agent.ApprovalActorID != postgres.DemoHumanActorID {
		t.Fatalf("unexpected Agent principal: %+v", agent)
	}
	agentCrossRequest := httptest.NewRequest(http.MethodPost, "/v1/commands/execute", bytes.NewReader(crossWorkspaceCommand))
	agentCrossRequest.Header.Set("Authorization", "Bearer "+token.Token)
	agentCrossResponse := httptest.NewRecorder()
	handler.ServeHTTP(agentCrossResponse, agentCrossRequest)
	if agentCrossResponse.Code != http.StatusNotFound {
		t.Fatalf("Agent cross-Workspace command status=%d body=%s", agentCrossResponse.Code, agentCrossResponse.Body.String())
	}
	if err = repo.Pool.QueryRow(ctx, "SELECT count(*) FROM mutation_attempts WHERE workspace_id='20000000-0000-4000-8000-000000000134'").Scan(&foreignAttemptCount); err != nil || foreignAttemptCount != 0 {
		t.Fatalf("Agent polluted foreign Workspace mutation audit: count=%d err=%v", foreignAttemptCount, err)
	}
	if err = repo.Pool.QueryRow(ctx, "SELECT count(*) FROM security_events WHERE workspace_id IS NULL AND event_type='authorization.workspace_denied'").Scan(&centralDenialCount); err != nil || centralDenialCount != 3 {
		t.Fatalf("central authorization denial events after Agent request=%d err=%v", centralDenialCount, err)
	}

	service := application.NewService(repo)
	arguments, _ := json.Marshal(map[string]any{"workspaceId": postgres.DemoWorkspaceID, "taskId": 101})
	previewRequest := application.CommandRequest{Name: "task.confirm", Arguments: arguments, Envelope: application.CommandEnvelope{
		ExpectedWorkspaceRevision: 1, IdempotencyKey: "auth-preview", ExecutedByActorID: postgres.DemoAgentActorID,
	}}
	preview, err := service.Preview(ctx, previewRequest)
	if err != nil {
		t.Fatal(err)
	}
	warnings := []string{}
	for _, diagnostic := range preview.Warnings {
		warnings = append(warnings, diagnostic.Code)
	}
	proceedReason := ""
	if len(warnings) > 0 {
		proceedReason = "Owner reviewed the exact warning set"
	}
	execute := previewRequest
	execute.Envelope.IdempotencyKey = "chat-approved-confirm"
	execute.Envelope.AcknowledgedWarningCodes = warnings
	execute.Envelope.ProceedReason = proceedReason
	execute.Envelope.HumanApprovalAttestation = &application.HumanApprovalAttestation{
		ApprovedCommandHash: preview.CommandHash, DecisionSnapshotHash: preview.DecisionSnapshotHash,
		StatementHash: "sha256:chat-approval", ConversationRef: "account-access-test",
	}
	execute.Principal = &application.CommandPrincipal{CredentialID: agent.CredentialID, WorkspaceID: agent.WorkspaceID, ApprovalActorID: agent.ApprovalActorID, Subject: agent.Subject}

	missingApproval := execute
	missingApproval.Envelope.IdempotencyKey = "missing-chat-approval"
	missingApproval.Envelope.HumanApprovalAttestation = nil
	if _, err = service.Execute(ctx, missingApproval); err == nil {
		t.Fatal("human-only command without chat approval was accepted")
	}
	mismatch := execute
	mismatch.Envelope.IdempotencyKey = "connected-human-mismatch"
	mismatch.Envelope.HumanApprovalAttestation = &application.HumanApprovalAttestation{
		ApprovedByActorID: "different-human", ApprovedCommandHash: preview.CommandHash,
	}
	if _, err = service.Execute(ctx, mismatch); err == nil {
		t.Fatal("approval attributed to a different human was accepted")
	}
	executeRaw, _ := json.Marshal(execute)
	executeRequest := httptest.NewRequest(http.MethodPost, "/v1/commands/execute", bytes.NewReader(executeRaw))
	executeRequest.Header.Set("Authorization", "Bearer "+token.Token)
	executeResponse := httptest.NewRecorder()
	handler.ServeHTTP(executeResponse, executeRequest)
	if executeResponse.Code != http.StatusOK {
		t.Fatalf("HTTP Agent chat-approved execute status=%d body=%s", executeResponse.Code, executeResponse.Body.String())
	}
	var result application.ExecutionResult
	if err = json.Unmarshal(executeResponse.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.ApprovalProtocol != "connected_human_chat_attestation" {
		t.Fatalf("approval protocol=%q", result.ApprovalProtocol)
	}
	retryRequest := httptest.NewRequest(http.MethodPost, "/v1/commands/execute", bytes.NewReader(executeRaw))
	retryRequest.Header.Set("Authorization", "Bearer "+token.Token)
	retryResponse := httptest.NewRecorder()
	handler.ServeHTTP(retryResponse, retryRequest)
	var retry application.ExecutionResult
	if retryResponse.Code != http.StatusOK || json.Unmarshal(retryResponse.Body.Bytes(), &retry) != nil || !retry.Idempotent || retry.CommandID != result.CommandID {
		t.Fatalf("safe HTTP idempotent retry failed: status=%d result=%+v body=%s", retryResponse.Code, retry, retryResponse.Body.String())
	}
	tamperedRetry := execute
	tamperedRetry.Envelope.HumanApprovalAttestation = nil
	if _, err = service.Execute(ctx, tamperedRetry); commandErrorCode(err) != domain.CodeIdempotencyConflict {
		t.Fatalf("changed approval envelope reused successful idempotency key: %v", err)
	}
	var recordedApprover string
	if err = repo.Pool.QueryRow(ctx, "SELECT approved_by_actor_id FROM human_approval_attestations WHERE executed_command_id=$1", result.CommandID).Scan(&recordedApprover); err != nil || recordedApprover != postgres.DemoHumanActorID {
		t.Fatalf("connected human approval attribution=%q err=%v", recordedApprover, err)
	}
	forgedCommand := map[string]any{
		"name":      "phase.create",
		"arguments": map[string]any{"workspaceId": postgres.DemoWorkspaceID, "phaseId": "auth-test", "name": "Auth Test"},
		"envelope":  map[string]any{"idempotencyKey": "http-forged-actor", "expectedWorkspaceRevision": result.WorkspaceRevision, "executedByActorId": postgres.DemoHumanActorID, "initiatedByActorId": postgres.DemoHumanActorID},
	}
	forgedRaw, _ := json.Marshal(forgedCommand)
	forgedRequest := httptest.NewRequest(http.MethodPost, "/v1/commands/execute", bytes.NewReader(forgedRaw))
	forgedRequest.Header.Set("Authorization", "Bearer "+token.Token)
	forgedResponse := httptest.NewRecorder()
	handler.ServeHTTP(forgedResponse, forgedRequest)
	if forgedResponse.Code != http.StatusOK {
		t.Fatalf("authenticated Actor override status=%d body=%s", forgedResponse.Code, forgedResponse.Body.String())
	}
	var executedBy string
	if err = repo.Pool.QueryRow(ctx, "SELECT executed_by_actor_id FROM commands WHERE workspace_id=$1 AND idempotency_key='http-forged-actor'", postgres.DemoWorkspaceID).Scan(&executedBy); err != nil || executedBy != postgres.DemoAgentActorID {
		t.Fatalf("forged Actor reached audit: actor=%q err=%v", executedBy, err)
	}

	multiWorkspacePassword := "multi workspace account password"
	multiWorkspacePHC, err := (authn.PasswordHasher{}).Hash(multiWorkspacePassword)
	if err != nil {
		t.Fatal(err)
	}
	multiWorkspaceMember, err := repo.CreateMember(ctx, "20000000-0000-4000-8000-000000000134", postgres.DemoHumanActorID, "shared-account", "shared-account", "Shared Account", multiWorkspacePHC, authz.RoleViewer)
	if err != nil {
		t.Fatal(err)
	}
	attachBody := []byte(`{"loginId":"SHARED-ACCOUNT","role":"operator"}`)
	attachResponse := ownerMutation(http.MethodPost, "/v1/workspaces/"+postgres.DemoWorkspaceID+"/memberships", attachBody)
	if attachResponse.Code != http.StatusCreated {
		t.Fatalf("existing account attach status=%d body=%s", attachResponse.Code, attachResponse.Body.String())
	}
	var sharedAccountRows, sharedMembershipRows int
	if err = repo.Pool.QueryRow(ctx, "SELECT count(*) FROM accounts WHERE actor_id=$1", multiWorkspaceMember.ActorID).Scan(&sharedAccountRows); err != nil || sharedAccountRows != 1 {
		t.Fatalf("existing account was duplicated: count=%d err=%v", sharedAccountRows, err)
	}
	if err = repo.Pool.QueryRow(ctx, "SELECT count(*) FROM workspace_memberships WHERE actor_id=$1 AND active", multiWorkspaceMember.ActorID).Scan(&sharedMembershipRows); err != nil || sharedMembershipRows != 2 {
		t.Fatalf("existing account memberships=%d err=%v", sharedMembershipRows, err)
	}
	multiResetResponse := ownerMutation(http.MethodPost, "/v1/workspaces/"+postgres.DemoWorkspaceID+"/members/"+multiWorkspaceMember.ActorID+"/reset-password", []byte(`{"newPassword":"must not cross Workspace boundary"}`))
	if multiResetResponse.Code != http.StatusConflict || !strings.Contains(multiResetResponse.Body.String(), "system administration") {
		t.Fatalf("multi-Workspace password reset status=%d body=%s", multiResetResponse.Code, multiResetResponse.Body.String())
	}
	multiDisableResponse := ownerMutation(http.MethodPost, "/v1/workspaces/"+postgres.DemoWorkspaceID+"/members/"+multiWorkspaceMember.ActorID+"/disable-account", []byte(`{}`))
	if multiDisableResponse.Code != http.StatusConflict || !strings.Contains(multiDisableResponse.Body.String(), "system administration") {
		t.Fatalf("multi-Workspace account disable status=%d body=%s", multiDisableResponse.Code, multiDisableResponse.Body.String())
	}

	initialMemberPassword := "single member initial password"
	createMemberBody, _ := json.Marshal(map[string]any{
		"loginId": "single-member", "displayName": "Single Member",
		"initialPassword": initialMemberPassword, "role": "viewer",
	})
	createMemberResponse := ownerMutation(http.MethodPost, "/v1/workspaces/"+postgres.DemoWorkspaceID+"/members", createMemberBody)
	if createMemberResponse.Code != http.StatusCreated {
		t.Fatalf("single Workspace member create status=%d body=%s", createMemberResponse.Code, createMemberResponse.Body.String())
	}
	var singleMember struct {
		ActorID string `json:"actorId"`
	}
	if err = json.Unmarshal(createMemberResponse.Body.Bytes(), &singleMember); err != nil || singleMember.ActorID == "" {
		t.Fatalf("invalid created member: %+v err=%v", singleMember, err)
	}
	singleLogin, err := authService.Login(ctx, "single-member", initialMemberPassword, "127.0.0.1:23456")
	if err != nil {
		t.Fatal(err)
	}
	rotatedMemberPassword := "single member rotated password"
	resetResponse := ownerMutation(http.MethodPost, "/v1/workspaces/"+postgres.DemoWorkspaceID+"/members/"+singleMember.ActorID+"/reset-password", []byte(`{"newPassword":"`+rotatedMemberPassword+`"}`))
	if resetResponse.Code != http.StatusNoContent {
		t.Fatalf("password reset status=%d body=%s", resetResponse.Code, resetResponse.Body.String())
	}
	if _, _, err = authService.AuthenticateSession(ctx, singleLogin.SessionToken); err == nil {
		t.Fatal("Owner password reset did not revoke existing sessions")
	}
	if _, err = authService.Login(ctx, "single-member", initialMemberPassword, "127.0.0.1:23456"); err == nil {
		t.Fatal("old password remained valid after Owner reset")
	}
	rotatedLogin, err := authService.Login(ctx, "single-member", rotatedMemberPassword, "127.0.0.1:23456")
	if err != nil {
		t.Fatal(err)
	}
	disableResponse := ownerMutation(http.MethodPost, "/v1/workspaces/"+postgres.DemoWorkspaceID+"/members/"+singleMember.ActorID+"/disable-account", []byte(`{}`))
	if disableResponse.Code != http.StatusNoContent {
		t.Fatalf("account disable status=%d body=%s", disableResponse.Code, disableResponse.Body.String())
	}
	if _, _, err = authService.AuthenticateSession(ctx, rotatedLogin.SessionToken); err == nil {
		t.Fatal("account disable did not revoke existing sessions")
	}
	if _, err = authService.Login(ctx, "single-member", rotatedMemberPassword, "127.0.0.1:23456"); err == nil {
		t.Fatal("disabled account authenticated")
	}
	resetDisabledResponse := ownerMutation(http.MethodPost, "/v1/workspaces/"+postgres.DemoWorkspaceID+"/members/"+singleMember.ActorID+"/reset-password", []byte(`{"newPassword":"disabled account replacement"}`))
	if resetDisabledResponse.Code != http.StatusConflict {
		t.Fatalf("disabled account password reset status=%d body=%s", resetDisabledResponse.Code, resetDisabledResponse.Body.String())
	}
	if _, err = repo.AddExistingMember(ctx, "20000000-0000-4000-8000-000000000134", postgres.DemoHumanActorID, "single-member", authz.RoleViewer); err == nil {
		t.Fatal("disabled account was added to another Workspace")
	}
	var rawMemberPasswordLeaks int
	if err = repo.Pool.QueryRow(ctx, `SELECT count(*) FROM account_credentials
		WHERE password_phc LIKE '%' || $1 || '%' OR password_phc LIKE '%' || $2 || '%'`,
		initialMemberPassword, rotatedMemberPassword).Scan(&rawMemberPasswordLeaks); err != nil || rawMemberPasswordLeaks != 0 {
		t.Fatalf("raw member password persisted: count=%d err=%v", rawMemberPasswordLeaks, err)
	}

	secondPHC, err := (authn.PasswordHasher{}).Hash("another correct battery staple")
	if err != nil {
		t.Fatal(err)
	}
	secondOwner, err := repo.CreateMember(ctx, postgres.DemoWorkspaceID, postgres.DemoHumanActorID, "second-owner", "second-owner", "Second Owner", secondPHC, authz.RoleOwner)
	if err != nil {
		t.Fatal(err)
	}
	type ownerOutcome struct{ err error }
	outcomes := make(chan ownerOutcome, 2)
	inactive := false
	for _, actorID := range []string{postgres.DemoHumanActorID, secondOwner.ActorID} {
		go func(candidate string) {
			outcomes <- ownerOutcome{err: repo.UpdateMember(ctx, postgres.DemoWorkspaceID, candidate, nil, &inactive)}
		}(actorID)
	}
	firstOwnerResult, secondOwnerResult := <-outcomes, <-outcomes
	if (firstOwnerResult.err == nil) == (secondOwnerResult.err == nil) {
		t.Fatalf("concurrent last-Owner protection outcomes: %v / %v", firstOwnerResult.err, secondOwnerResult.err)
	}
	active := true
	for _, actorID := range []string{postgres.DemoHumanActorID, secondOwner.ActorID} {
		if err = repo.UpdateMember(ctx, postgres.DemoWorkspaceID, actorID, nil, &active); err != nil {
			t.Fatal(err)
		}
	}
	firstDirectTx, err := repo.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer firstDirectTx.Rollback(ctx)
	secondDirectTx, err := repo.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer secondDirectTx.Rollback(ctx)
	if _, err = firstDirectTx.Exec(ctx, "UPDATE workspace_memberships SET active=false,deactivated_at=now() WHERE workspace_id=$1 AND actor_id=$2", postgres.DemoWorkspaceID, postgres.DemoHumanActorID); err != nil {
		t.Fatal(err)
	}
	if _, err = secondDirectTx.Exec(ctx, "UPDATE workspace_memberships SET active=false,deactivated_at=now() WHERE workspace_id=$1 AND actor_id=$2", postgres.DemoWorkspaceID, secondOwner.ActorID); err != nil {
		t.Fatal(err)
	}
	directOutcomes := make(chan ownerOutcome, 2)
	go func() { directOutcomes <- ownerOutcome{err: firstDirectTx.Commit(ctx)} }()
	go func() { directOutcomes <- ownerOutcome{err: secondDirectTx.Commit(ctx)} }()
	firstDirectResult, secondDirectResult := <-directOutcomes, <-directOutcomes
	if (firstDirectResult.err == nil) == (secondDirectResult.err == nil) {
		t.Fatalf("concurrent direct-SQL last-Owner protection outcomes: %v / %v", firstDirectResult.err, secondDirectResult.err)
	}
	var activeOwnerID string
	if err = repo.Pool.QueryRow(ctx, "SELECT actor_id FROM workspace_memberships WHERE workspace_id=$1 AND active AND role='owner'", postgres.DemoWorkspaceID).Scan(&activeOwnerID); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Pool.Exec(ctx, "UPDATE workspace_memberships SET active=false,deactivated_at=now() WHERE workspace_id=$1 AND actor_id=$2", postgres.DemoWorkspaceID, activeOwnerID); err == nil {
		t.Fatal("direct SQL removed the last active Owner")
	}
	if _, err = repo.Pool.Exec(ctx, "UPDATE security_events SET payload='{}' WHERE workspace_id=$1", postgres.DemoWorkspaceID); err == nil {
		t.Fatal("append-only security event was updated")
	}
	var secretLeaks int
	if err = repo.Pool.QueryRow(ctx, `SELECT count(*) FROM security_events
		WHERE payload::text LIKE '%' || $1 || '%'
		   OR payload::text LIKE '%' || $2 || '%'
		   OR payload::text LIKE '%' || $3 || '%'
		   OR payload::text LIKE '%' || $4 || '%'
		   OR payload::text LIKE '%' || $5 || '%'`,
		token.Token, login.SessionToken, login.CSRFToken,
		initialMemberPassword, rotatedMemberPassword).Scan(&secretLeaks); err != nil || secretLeaks != 0 {
		t.Fatalf("raw auth secret leaked to security Events: count=%d err=%v", secretLeaks, err)
	}
}
