package integration_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	neturl "net/url"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/jazzcake/baley/server/internal/application"
	"github.com/jazzcake/baley/server/internal/domain"
	"github.com/jazzcake/baley/server/internal/persistence/postgres"
	"github.com/jazzcake/baley/server/internal/transport/httpapi"
)

func TestTaskJournalLifecycleIsAtomicIdempotentAndQueryable(t *testing.T) {
	url := os.Getenv("BALEY_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("BALEY_TEST_DATABASE_URL is not set")
	}
	requireDisposableDatabase(t, url)
	t.Setenv("BALEY_LEASE_TOKEN_SECRET", "task-journal-integration-secret")
	ctx := context.Background()
	repo, err := postgres.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Pool.Close()
	if _, err = repo.Pool.Exec(ctx, "SET session_replication_role='replica'; TRUNCATE task_journal_entries,events,human_approval_attestations,commands,workspace_counters,runs,gate_entry_tasks,gate_tasks,gates,task_dependencies,tasks,lanes,phases,workspaces,actors CASCADE; SET session_replication_role='origin'"); err != nil {
		t.Fatal(err)
	}
	if err = repo.SeedDemo(ctx); err != nil {
		t.Fatal(err)
	}
	service := application.NewService(repo)
	taskUUID := "18300000-0000-4000-8000-000000000001"
	clientRunID := "18300000-0000-4000-8000-000000000002"

	create := journalRequest("task.create", map[string]any{
		"workspaceId": postgres.DemoWorkspaceID, "taskUuid": taskUUID, "laneId": "server", "phaseId": "build",
		"title": "Journal lifecycle", "terminalReason": "intentional independent test leaf",
		"contextNote": map[string]any{"narrative": "The operator stated the problem.", "context": map[string]any{"problem": "context was fragmented", "goal": "deliver"}},
	}, "journal-create", 1)
	create.Envelope.InitiatedByActorID = postgres.DemoHumanActorID
	created := executeJournalCommand(t, ctx, service, create)
	if created.TaskJournalEntryID == "" {
		t.Fatal("task.create did not return a journal projection ID")
	}

	update := journalRequest("task.update", map[string]any{
		"workspaceId": postgres.DemoWorkspaceID, "taskId": 111, "title": "Journal lifecycle clarified",
		"contextNote": map[string]any{"narrative": "New evidence changed the scope.", "context": map[string]any{"situationChange": "new evidence", "alternatives": []any{"separate decision entity", "task journal"}}},
	}, "journal-update", 2)
	updated := executeJournalCommand(t, ctx, service, update)
	retried, err := service.Execute(ctx, update)
	if err != nil || !retried.Idempotent || retried.CommandID != updated.CommandID || retried.TaskJournalEntryID != updated.TaskJournalEntryID {
		t.Fatalf("journal retry=%+v original=%+v err=%v", retried, updated, err)
	}
	conflict := update
	conflict.Arguments = mustJSON(t, map[string]any{
		"workspaceId": postgres.DemoWorkspaceID, "taskId": 111, "title": "Journal lifecycle clarified",
		"contextNote": map[string]any{"narrative": "Different retry context."},
	})
	if _, err = service.Execute(ctx, conflict); commandErrorCode(err) != domain.CodeIdempotencyConflict {
		t.Fatalf("context-changing retry error=%v", err)
	}

	executeJournalCommand(t, ctx, service, journalRequest("task.block", map[string]any{
		"workspaceId": postgres.DemoWorkspaceID, "taskId": 111, "reason": "waiting for evidence",
		"contextNote": map[string]any{"context": map[string]any{"situationChange": "evidence unavailable"}},
	}, "journal-block", 3))
	executeJournalCommand(t, ctx, service, journalRequest("task.unblock", map[string]any{
		"workspaceId": postgres.DemoWorkspaceID, "taskId": 111, "reason": "evidence arrived",
		"contextNote": map[string]any{"narrative": "Evidence arrived."},
	}, "journal-unblock", 4))
	executeJournalCommand(t, ctx, service, journalRequest("run.start", map[string]any{
		"workspaceId": postgres.DemoWorkspaceID, "taskId": 111, "clientRunId": clientRunID, "kind": "implementation",
		"contextNote": map[string]any{"context": map[string]any{"completionContract": "tests pass"}},
	}, "journal-run-start", 5))
	snapshot, err := repo.LoadSnapshot(ctx, postgres.DemoWorkspaceID)
	if err != nil || len(snapshot.Runs) != 1 {
		t.Fatalf("run snapshot=%+v err=%v", snapshot.Runs, err)
	}
	run := snapshot.Runs[0]
	withoutContext := journalRequest("run.succeed", map[string]any{
		"workspaceId": postgres.DemoWorkspaceID, "runId": run.ID, "expectedRunVersion": 1, "summary": "verified",
	}, "journal-run-succeed", 6)
	executeJournalCommand(t, ctx, service, withoutContext)
	executeJournalCommand(t, ctx, service, journalRequest("task.report_implemented", map[string]any{
		"workspaceId": postgres.DemoWorkspaceID, "taskId": 111, "assessment": "implemented and tested",
		"contextNote": map[string]any{"narrative": "Delivered with automated evidence.", "context": map[string]any{"outcome": "journal delivered", "residualRisks": []any{"operational migration"}}},
	}, "journal-implemented", 7))
	executeJournalCommand(t, ctx, service, journalRequest("task.rework", map[string]any{
		"workspaceId": postgres.DemoWorkspaceID, "taskId": 111, "reason": "review found a gap",
		"contextNote": map[string]any{"context": map[string]any{"judgmentUpdate": "close the review gap"}},
	}, "journal-rework", 8))

	entries, err := repo.TaskJournal(ctx, postgres.DemoWorkspaceID, 111, time.Time{}, "", 100)
	if err != nil || len(entries) != 7 {
		t.Fatalf("journal entries=%+v err=%v", entries, err)
	}
	for index, entry := range entries {
		if entry.WorkspaceID != postgres.DemoWorkspaceID || entry.TaskPublicID != 111 || entry.EventID == "" || entry.EventType == "" || entry.CommandID == "" || entry.CommandName == "" || entry.ExecutedByActorID != postgres.DemoAgentActorID || entry.RecordedAt.Before(entry.OccurredAt) {
			t.Fatalf("entry %d lost stable envelope/provenance: %+v", index, entry)
		}
		if index > 0 && (entries[index-1].RecordedAt.Before(entry.RecordedAt) || entries[index-1].RecordedAt.Equal(entry.RecordedAt) && entries[index-1].ID < entry.ID) {
			t.Fatalf("journal order is not recordedAt/id descending: %+v", entries)
		}
	}
	actualByEvent := make(map[string]application.TaskJournalEntryProjection, len(entries))
	for _, entry := range entries {
		actualByEvent[entry.EventID] = entry
	}
	events, err := repo.Events(ctx, postgres.DemoWorkspaceID)
	if err != nil {
		t.Fatal(err)
	}
	rebuiltCount := 0
	for _, event := range events {
		rebuilt, rebuildErr := application.TaskJournalFromEvent(event)
		if rebuildErr != nil {
			t.Fatalf("rebuild Event %s: %v", event.ID, rebuildErr)
		}
		if rebuilt == nil {
			continue
		}
		rebuiltCount++
		actual, exists := actualByEvent[event.ID]
		if !exists {
			t.Fatalf("persisted Event %s rebuilt a missing Journal row", event.ID)
		}
		var actualContext, rebuiltContext any
		if json.Unmarshal(actual.Context, &actualContext) != nil || json.Unmarshal(rebuilt.Context, &rebuiltContext) != nil {
			t.Fatalf("invalid rebuilt context for Event %s", event.ID)
		}
		if actual.ID != event.ID || actual.TaskID != rebuilt.TaskID || actual.EventType != rebuilt.SourceEventType ||
			actual.LifecycleStage != rebuilt.LifecycleStage || actual.Narrative != rebuilt.Narrative ||
			actual.SchemaVersion != rebuilt.SchemaVersion || !reflect.DeepEqual(actualContext, rebuiltContext) ||
			actual.CommandID != event.CommandID || actual.InitiatedByActorID != event.InitiatedByActorID ||
			actual.ExecutedByActorID != event.ExecutedByActorID || actual.ApprovedByActorID != event.ApprovedByActorID ||
			!actual.OccurredAt.Equal(event.CreatedAt) || !actual.RecordedAt.Equal(event.CreatedAt) {
			t.Fatalf("Journal row is not an equivalent persisted-Event rebuild: actual=%+v rebuilt=%+v event=%+v", actual, rebuilt, event)
		}
	}
	if rebuiltCount != len(entries) {
		t.Fatalf("rebuilt %d Journal rows from Events, want %d", rebuiltCount, len(entries))
	}
	createdEntry := entries[len(entries)-1]
	if createdEntry.InitiatedByActorID != postgres.DemoHumanActorID || createdEntry.CommandName != "task.create" || createdEntry.EventType != "task.created" {
		t.Fatalf("created actor/source provenance=%+v", createdEntry)
	}
	var eventCreated time.Time
	if err = repo.Pool.QueryRow(ctx, "SELECT created_at FROM events WHERE id=$1", createdEntry.EventID).Scan(&eventCreated); err != nil || !eventCreated.Equal(createdEntry.OccurredAt) {
		t.Fatalf("Event occurrence mismatch: event=%s journal=%s err=%v", eventCreated, createdEntry.OccurredAt, err)
	}
	var jsonMatchCount int
	if err = repo.Pool.QueryRow(ctx, `SELECT count(*) FROM task_journal_entries WHERE workspace_id=$1 AND context @> '{"goal":"deliver"}'::jsonb`, postgres.DemoWorkspaceID).Scan(&jsonMatchCount); err != nil || jsonMatchCount != 1 {
		t.Fatalf("JSONB query count=%d err=%v", jsonMatchCount, err)
	}
	page, err := repo.TaskJournal(ctx, postgres.DemoWorkspaceID, 111, time.Time{}, "", 2)
	if err != nil || len(page) != 2 {
		t.Fatalf("first page=%+v err=%v", page, err)
	}
	next, err := repo.TaskJournal(ctx, postgres.DemoWorkspaceID, 111, page[1].RecordedAt, page[1].ID, 2)
	if err != nil || len(next) != 2 || next[0].ID == page[0].ID || next[0].ID == page[1].ID {
		t.Fatalf("second page=%+v err=%v", next, err)
	}
	foreign, err := repo.TaskJournal(ctx, "not-this-workspace", 0, time.Time{}, "", 100)
	if err != nil || len(foreign) != 0 {
		t.Fatalf("cross-Workspace journal=%+v err=%v", foreign, err)
	}
	handler := (&httpapi.API{Repo: repo}).Handler()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet,
		fmt.Sprintf("/v1/workspaces/%s/tasks/111/journal?limit=2", postgres.DemoWorkspaceID), nil))
	var httpPage struct {
		Items        []application.TaskJournalEntryProjection `json:"items"`
		NextCursor   string                                   `json:"nextCursor"`
		NextCursorID string                                   `json:"nextCursorId"`
	}
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &httpPage) != nil || len(httpPage.Items) != 2 || httpPage.NextCursor == "" || httpPage.NextCursorID == "" {
		t.Fatalf("Task journal HTTP page status=%d body=%s", response.Code, response.Body.String())
	}
	response = httptest.NewRecorder()
	query := neturl.Values{"taskId": {"111"}, "after": {httpPage.NextCursor}, "afterId": {httpPage.NextCursorID}, "limit": {"2"}}
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet,
		fmt.Sprintf("/v1/workspaces/%s/task-journal?%s", postgres.DemoWorkspaceID, query.Encode()), nil))
	if response.Code != http.StatusOK {
		t.Fatalf("Workspace journal HTTP cursor status=%d body=%s", response.Code, response.Body.String())
	}

	before := len(entries)
	stale := journalRequest("task.update", map[string]any{
		"workspaceId": postgres.DemoWorkspaceID, "taskId": 111, "title": "must roll back",
		"contextNote": map[string]any{"narrative": "must not persist"},
	}, "journal-stale", 1)
	if _, err = service.Execute(ctx, stale); commandErrorCode(err) != domain.CodeStaleRevision {
		t.Fatalf("stale journal command error=%v", err)
	}
	entries, err = repo.TaskJournal(ctx, postgres.DemoWorkspaceID, 111, time.Time{}, "", 100)
	if err != nil || len(entries) != before {
		t.Fatalf("failed command leaked journal entries=%d want=%d err=%v", len(entries), before, err)
	}
	for _, statement := range []string{
		"UPDATE task_journal_entries SET narrative='tampered' WHERE workspace_id='" + postgres.DemoWorkspaceID + "'",
		"DELETE FROM task_journal_entries WHERE workspace_id='" + postgres.DemoWorkspaceID + "'",
		"TRUNCATE task_journal_entries",
	} {
		if _, err = repo.Pool.Exec(ctx, statement); err == nil {
			t.Fatalf("append-only journal accepted %q", statement)
		}
	}
}

func TestTaskJournalBacklogPromotionAdapterUsesTaskCreatedEvent(t *testing.T) {
	url := os.Getenv("BALEY_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("BALEY_TEST_DATABASE_URL is not set")
	}
	requireDisposableDatabase(t, url)
	t.Setenv("BALEY_LEASE_TOKEN_SECRET", "task-journal-promotion-secret")
	ctx := context.Background()
	repo, err := postgres.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Pool.Close()
	if _, err = repo.Pool.Exec(ctx, "SET session_replication_role='replica'; TRUNCATE task_journal_entries,events,human_approval_attestations,commands,workspace_counters,runs,gate_entry_tasks,gate_tasks,gates,task_dependencies,backlog_items,tasks,lanes,phases,workspaces,actors CASCADE; SET session_replication_role='origin'"); err != nil {
		t.Fatal(err)
	}
	if err = repo.SeedDemo(ctx); err != nil {
		t.Fatal(err)
	}
	service := application.NewService(repo)
	executeJournalCommand(t, ctx, service, journalRequest("backlog.create", map[string]any{
		"workspaceId": postgres.DemoWorkspaceID, "backlogUuid": "18300000-0000-4000-8000-000000000010",
		"laneId": "server", "title": "Promote journal context", "description": "promotion adapter",
	}, "journal-backlog-create", 1))
	result := executeJournalCommand(t, ctx, service, journalRequest("backlog.promote", map[string]any{
		"workspaceId": postgres.DemoWorkspaceID, "backlogPublicId": 1,
		"taskUuid": "18300000-0000-4000-8000-000000000011", "phaseId": "build",
		"terminalReason": "intentional test leaf",
		"contextNote":    map[string]any{"narrative": "The intake became committed work.", "context": map[string]any{"whyNow": "priority changed"}},
	}, "journal-backlog-promote", 2))
	if result.TaskJournalEntryID == "" {
		t.Fatal("backlog.promote did not return a journal projection ID")
	}
	entries, err := repo.TaskJournal(ctx, postgres.DemoWorkspaceID, 111, time.Time{}, "", 10)
	if err != nil || len(entries) != 1 || entries[0].CommandName != "backlog.promote" || entries[0].EventType != "task.created" || entries[0].LifecycleStage != "created" {
		t.Fatalf("promotion journal=%+v err=%v", entries, err)
	}
}

func TestTaskJournalInsertFailureRollsBackLifecycleEventAndCommand(t *testing.T) {
	url := os.Getenv("BALEY_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("BALEY_TEST_DATABASE_URL is not set")
	}
	requireDisposableDatabase(t, url)
	t.Setenv("BALEY_LEASE_TOKEN_SECRET", "task-journal-rollback-secret")
	ctx := context.Background()
	repo, err := postgres.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Pool.Close()
	if _, err = repo.Pool.Exec(ctx, "SET session_replication_role='replica'; TRUNCATE task_journal_entries,events,human_approval_attestations,commands,workspace_counters,runs,gate_entry_tasks,gate_tasks,gates,task_dependencies,tasks,lanes,phases,workspaces,actors CASCADE; SET session_replication_role='origin'"); err != nil {
		t.Fatal(err)
	}
	if err = repo.SeedDemo(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Pool.Exec(ctx, `
		DROP TRIGGER IF EXISTS task_journal_insert_failure ON task_journal_entries;
		CREATE OR REPLACE FUNCTION fail_task_journal_insert() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN RAISE EXCEPTION 'injected journal failure'; END $$;
		CREATE TRIGGER task_journal_insert_failure BEFORE INSERT ON task_journal_entries
		FOR EACH ROW EXECUTE FUNCTION fail_task_journal_insert();
	`); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = repo.Pool.Exec(ctx, "DROP TRIGGER IF EXISTS task_journal_insert_failure ON task_journal_entries; DROP FUNCTION IF EXISTS fail_task_journal_insert()")
	}()
	service := application.NewService(repo)
	request := journalRequest("task.update", map[string]any{
		"workspaceId": postgres.DemoWorkspaceID, "taskId": 101, "title": "must roll back",
		"contextNote": map[string]any{"narrative": "must roll back with lifecycle"},
	}, "journal-injected-failure", 1)
	if _, err = service.Execute(ctx, request); err == nil {
		t.Fatal("injected journal failure did not fail command")
	}
	var title string
	var revision int64
	var commands, events, journal int
	if err = repo.Pool.QueryRow(ctx, "SELECT title FROM tasks WHERE workspace_id=$1 AND public_id=101", postgres.DemoWorkspaceID).Scan(&title); err != nil {
		t.Fatal(err)
	}
	if err = repo.Pool.QueryRow(ctx, "SELECT revision FROM workspaces WHERE id=$1", postgres.DemoWorkspaceID).Scan(&revision); err != nil {
		t.Fatal(err)
	}
	if err = repo.Pool.QueryRow(ctx, "SELECT count(*) FROM commands WHERE workspace_id=$1", postgres.DemoWorkspaceID).Scan(&commands); err != nil {
		t.Fatal(err)
	}
	if err = repo.Pool.QueryRow(ctx, "SELECT count(*) FROM events WHERE workspace_id=$1", postgres.DemoWorkspaceID).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if err = repo.Pool.QueryRow(ctx, "SELECT count(*) FROM task_journal_entries WHERE workspace_id=$1", postgres.DemoWorkspaceID).Scan(&journal); err != nil {
		t.Fatal(err)
	}
	if title == "must roll back" || revision != 1 || commands != 0 || events != 0 || journal != 0 {
		t.Fatalf("transaction leaked: title=%q revision=%d commands=%d events=%d journal=%d", title, revision, commands, events, journal)
	}
}

func TestTaskJournalContextIsBoundToBrowserApprovalGrant(t *testing.T) {
	url := os.Getenv("BALEY_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("BALEY_TEST_DATABASE_URL is not set")
	}
	requireDisposableDatabase(t, url)
	t.Setenv("BALEY_LEASE_TOKEN_SECRET", "task-journal-approval-secret")
	ctx := context.Background()
	repo, err := postgres.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Pool.Close()
	resetApprovalGrantFixture(t, ctx, repo)
	human, authService := approvalHumanSession(t, ctx, repo)
	workspaceID := "journal-approval"
	createApprovalWorkspace(t, ctx, repo, workspaceID, true)
	agent := approvalAgentPrincipal(t, ctx, repo, authService, workspaceID, "journal-approval-agent")
	service := application.NewService(repo)

	confirmArgs := map[string]any{"workspaceId": workspaceID, "taskId": 1, "contextNote": map[string]any{
		"narrative": "Human accepted the stated outcome.", "context": map[string]any{"outcome": "accepted"},
	}}
	agentRequest := approvalRequest(t, "task.confirm", confirmArgs, agent, 1, "journal-confirm-preview")
	preview, err := service.Preview(ctx, agentRequest)
	if err != nil {
		t.Fatal(err)
	}
	warnings, reason := approvalWarnings(preview)
	grant, err := repo.CreateApprovalGrant(ctx, human, workspaceID, agentRequest.Name, preview, warnings, reason)
	if err != nil {
		t.Fatal(err)
	}
	browserPrincipal := application.CommandPrincipal{AccountID: human.AccountID, CredentialID: human.SessionID, SessionID: human.SessionID, Subject: human.Subject}
	tamperedArgs := map[string]any{"workspaceId": workspaceID, "taskId": 1, "contextNote": map[string]any{"narrative": "Changed after approval."}}
	tampered := approvalRequest(t, "task.confirm", tamperedArgs, browserPrincipal, 1, "journal-confirm-tampered")
	tampered.Envelope.ApprovalGrantID = grant.ID
	tampered.Envelope.AcknowledgedWarningCodes = warnings
	tampered.Envelope.ProceedReason = reason
	if _, err = service.Execute(ctx, tampered); commandErrorCode(err) != domain.CodeApprovalGrantMismatch {
		t.Fatalf("approval accepted changed context: %v", err)
	}
	var count int
	if err = repo.Pool.QueryRow(ctx, "SELECT count(*) FROM task_journal_entries WHERE workspace_id=$1", workspaceID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("mismatched approval wrote journal: count=%d err=%v", count, err)
	}
	exact := approvalRequest(t, "task.confirm", confirmArgs, browserPrincipal, 1, "journal-confirm-exact")
	exact.Envelope.ApprovalGrantID = grant.ID
	exact.Envelope.AcknowledgedWarningCodes = warnings
	exact.Envelope.ProceedReason = reason
	confirmed, err := service.Execute(ctx, exact)
	if err != nil || confirmed.TaskJournalEntryID == "" {
		t.Fatalf("exact approved context result=%+v err=%v", confirmed, err)
	}

	discardArgs := map[string]any{"workspaceId": workspaceID, "taskId": 2, "reason": "superseded", "contextNote": map[string]any{
		"context": map[string]any{"outcome": "discarded", "judgmentUpdate": "superseded"},
	}}
	discardPreviewRequest := approvalRequest(t, "task.discard", discardArgs, agent, 2, "journal-discard-preview")
	discardPreview, err := service.Preview(ctx, discardPreviewRequest)
	if err != nil {
		t.Fatal(err)
	}
	discardWarnings, discardReason := approvalWarnings(discardPreview)
	discardGrant, err := repo.CreateApprovalGrant(ctx, human, workspaceID, "task.discard", discardPreview, discardWarnings, discardReason)
	if err != nil {
		t.Fatal(err)
	}
	discard := approvalRequest(t, "task.discard", discardArgs, browserPrincipal, 2, "journal-discard-exact")
	discard.Envelope.ApprovalGrantID = discardGrant.ID
	discard.Envelope.AcknowledgedWarningCodes = discardWarnings
	discard.Envelope.ProceedReason = discardReason
	if _, err = service.Execute(ctx, discard); err != nil {
		t.Fatal(err)
	}
	entries, err := repo.TaskJournal(ctx, workspaceID, 0, time.Time{}, "", 100)
	if err != nil || len(entries) != 2 || entries[0].LifecycleStage != "discarded" || entries[1].LifecycleStage != "confirmed" {
		t.Fatalf("approved outcome journal=%+v err=%v", entries, err)
	}
	for _, entry := range entries {
		if entry.ApprovedByActorID != postgres.DemoHumanActorID || entry.ExecutedByActorID != postgres.DemoHumanActorID {
			t.Fatalf("approved actor provenance=%+v", entry)
		}
	}
}

func journalRequest(name string, arguments map[string]any, key string, revision int64) application.CommandRequest {
	return application.CommandRequest{Name: name, Arguments: json.RawMessage(mustJSONRaw(arguments)), Envelope: application.CommandEnvelope{
		IdempotencyKey: key, ExpectedWorkspaceRevision: revision, ExecutedByActorID: postgres.DemoAgentActorID,
	}}
}

func mustJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	content, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return content
}

func mustJSONRaw(value any) []byte {
	content, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return content
}

func executeJournalCommand(t *testing.T, ctx context.Context, service *application.Service, request application.CommandRequest) application.ExecutionResult {
	t.Helper()
	preview, err := service.Preview(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Errors) != 0 {
		t.Fatalf("preview errors=%+v", preview.Errors)
	}
	for _, warning := range preview.Warnings {
		request.Envelope.AcknowledgedWarningCodes = append(request.Envelope.AcknowledgedWarningCodes, warning.Code)
	}
	if len(preview.Warnings) > 0 {
		request.Envelope.ProceedReason = "Integration test acknowledges the exact warning set."
	}
	result, err := service.Execute(ctx, request)
	if err != nil {
		var commandErr *application.CommandError
		if errors.As(err, &commandErr) {
			t.Fatalf("execute %s failed: %s: %s", request.Name, commandErr.Code, commandErr.Message)
		}
		t.Fatal(err)
	}
	return result
}
