package integration_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jazzcake/baley/server/internal/application"
	"github.com/jazzcake/baley/server/internal/authn"
	"github.com/jazzcake/baley/server/internal/authz"
	"github.com/jazzcake/baley/server/internal/persistence/postgres"
	"github.com/jazzcake/baley/server/internal/transport/httpapi"
)

const (
	birdTestAccountID = "10000000-0000-4000-8000-000000000026"
	birdTestViewID    = "20000000-0000-4000-8000-000000000026"
	birdTestNodeID    = "30000000-0000-4000-8000-000000000026"
	birdTestNodeTwoID = "30000000-0000-4000-8000-000000000027"
	birdTestEdgeID    = "60000000-0000-4000-8000-000000000027"
	birdBackupActorID = "40000000-0000-4000-8000-000000000026"
	birdBackupAcctID  = "50000000-0000-4000-8000-000000000026"
)

func TestBirdViewMigrationAndAccountPrivateFlow(t *testing.T) {
	url := os.Getenv("BALEY_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("BALEY_TEST_DATABASE_URL is not set")
	}
	requireDisposableDatabase(t, url)
	ctx := context.Background()
	if err := postgres.Migrate(url, filepath.Join("..", "migrations"), "up"); err != nil {
		t.Fatal(err)
	}
	repo, err := postgres.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Pool.Close()
	if _, err = repo.Pool.Exec(ctx, `SET session_replication_role='replica'; TRUNCATE bird_view_events,bird_view_commands,bird_view_phase_bindings,bird_view_gate_bindings,bird_view_task_bindings,bird_view_backlog_bindings,bird_view_edges,bird_view_nodes,bird_views,security_events,workspace_memberships,account_credentials,accounts,events,human_approval_attestations,commands,workspace_counters,runs,gate_entry_tasks,gate_tasks,gates,task_dependencies,tasks,lanes,phases,workspaces,actors CASCADE; SET session_replication_role='origin'`); err != nil {
		t.Fatal(err)
	}
	if err = repo.SeedDemo(ctx); err != nil {
		t.Fatal(err)
	}
	if err = repo.BootstrapOwner(ctx, postgres.DemoWorkspaceID, birdTestAccountID, postgres.DemoHumanActorID, "bird-owner", "bird-owner", "Bird Owner", "test-only-phc"); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Pool.Exec(ctx, "INSERT INTO actors(id,display_name,actor_type) VALUES($1,'Backup Owner','human')", birdBackupActorID); err != nil {
		t.Fatal(err)
	}
	if err = repo.BootstrapOwner(ctx, postgres.DemoWorkspaceID, birdBackupAcctID, birdBackupActorID, "bird-backup", "bird-backup", "Backup Owner", "test-only-phc"); err != nil {
		t.Fatal(err)
	}

	service := application.NewBirdViewService(repo)
	principal := &application.CommandPrincipal{AccountID: birdTestAccountID, Subject: authz.Subject{ActorID: postgres.DemoHumanActorID, Kind: authz.ActorHuman, Credential: authz.HumanSession, Scopes: []authz.Capability{authz.BirdViewRead, authz.BirdViewOperate}}}
	execute := func(name, key string, revision int64, arguments map[string]any) application.BirdViewExecutionResult {
		t.Helper()
		raw, marshalErr := json.Marshal(arguments)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		result, executeErr := service.Execute(ctx, application.CommandRequest{Name: name, Arguments: raw, Principal: principal, Envelope: application.CommandEnvelope{IdempotencyKey: key, ExpectedBirdViewRevision: revision, ExecutedByActorID: postgres.DemoHumanActorID}})
		if executeErr != nil {
			t.Fatalf("%s: %v", name, executeErr)
		}
		return result
	}

	created := execute("bird_view.create", "bird-create", 0, map[string]any{"birdViewId": birdTestViewID, "title": "Launch map", "description": "Account-level plan"})
	if created.BirdViewRevision != 1 {
		t.Fatalf("create revision = %d", created.BirdViewRevision)
	}
	retry := execute("bird_view.create", "bird-create", 0, map[string]any{"birdViewId": birdTestViewID, "title": "Launch map", "description": "Account-level plan"})
	if !retry.Idempotent || retry.CommandID != created.CommandID {
		t.Fatalf("idempotent create was not replayed: %#v", retry)
	}
	backupPrincipal := &application.CommandPrincipal{AccountID: birdBackupAcctID, Subject: authz.Subject{ActorID: birdBackupActorID, Kind: authz.ActorHuman, Credential: authz.HumanSession, Scopes: []authz.Capability{authz.BirdViewRead}}}
	if _, err = service.Get(ctx, backupPrincipal, birdTestViewID); err == nil {
		t.Fatal("another Account read an account-private Bird View")
	}
	execute("bird_view.node.create", "node-create", 1, map[string]any{"birdViewId": birdTestViewID, "nodeId": birdTestNodeID, "title": "Pilot outcome", "summary": "Reach a usable pilot", "positionX": 120.5, "positionY": 240.25})
	execute("bird_view.node.update", "node-position", 2, map[string]any{"birdViewId": birdTestViewID, "nodeId": birdTestNodeID, "positionX": 444.0, "positionY": 555.0})
	execute("bird_view.node.create", "node-two-create", 3, map[string]any{"birdViewId": birdTestViewID, "nodeId": birdTestNodeTwoID, "title": "Second outcome", "positionX": 760.0, "positionY": 240.0})
	execute("bird_view.edge.connect", "edge-connect", 4, map[string]any{"birdViewId": birdTestViewID, "edgeId": birdTestEdgeID, "fromNodeId": birdTestNodeID, "toNodeId": birdTestNodeTwoID, "label": "then"})
	execute("bird_view.edge.update", "edge-update", 5, map[string]any{"birdViewId": birdTestViewID, "edgeId": birdTestEdgeID, "label": "unlocks"})
	execute("bird_view.edge.disconnect", "edge-disconnect", 6, map[string]any{"birdViewId": birdTestViewID, "edgeId": birdTestEdgeID})
	execute("bird_view.node.delete", "node-two-delete", 7, map[string]any{"birdViewId": birdTestViewID, "nodeId": birdTestNodeTwoID})
	graph, graphErr := service.Graph(ctx, principal, birdTestViewID)
	if graphErr != nil || len(graph.Nodes) != 1 || graph.Nodes[0].PositionX != 444 || graph.Nodes[0].PositionY != 555 || len(graph.Edges) != 0 {
		t.Fatalf("durable free-form graph=%#v err=%v", graph, graphErr)
	}
	primaryToken, err := repo.IssueAgentToken(ctx, postgres.DemoWorkspaceID, postgres.DemoAgentActorID, "bird-primary", postgres.DemoHumanActorID, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	backupToken, err := repo.IssueAgentToken(ctx, postgres.DemoWorkspaceID, postgres.DemoAgentActorID, "bird-backup", birdBackupActorID, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	authService, err := authn.NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	handler := (&httpapi.API{BirdViews: service, Repo: repo, Auth: authService, AuthMode: "enforced"}).Handler()
	readPath := func(token, path string) *httptest.ResponseRecorder {
		t.Helper()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}
	primaryList := readPath(primaryToken.Token, "/v1/bird-views")
	backupList := readPath(backupToken.Token, "/v1/bird-views")
	if primaryList.Code != http.StatusOK || !strings.Contains(primaryList.Body.String(), birdTestViewID) || backupList.Code != http.StatusOK || strings.Contains(backupList.Body.String(), birdTestViewID) {
		t.Fatalf("account-private HTTP lists leaked: primary=%d %s backup=%d %s", primaryList.Code, primaryList.Body.String(), backupList.Code, backupList.Body.String())
	}
	paths := []string{
		"/v1/bird-views/" + birdTestViewID,
		"/v1/bird-views/" + birdTestViewID + "/graph",
		"/v1/bird-views/" + birdTestViewID + "/nodes/" + birdTestNodeID + "/focus",
		"/v1/bird-views/" + birdTestViewID + "/nodes/" + birdTestNodeID + "/context",
	}
	for _, path := range paths {
		if response := readPath(primaryToken.Token, path); response.Code != http.StatusOK {
			t.Fatalf("owner token Bird View read %s status=%d body=%s", path, response.Code, response.Body.String())
		}
		privateResponse := readPath(backupToken.Token, path)
		missingPath := strings.Replace(path, birdTestViewID, "20000000-0000-4000-8000-000000000099", 1)
		missingResponse := readPath(primaryToken.Token, missingPath)
		if privateResponse.Code != http.StatusNotFound || privateResponse.Body.String() != missingResponse.Body.String() {
			t.Fatalf("account privacy leaked through HTTP %s: private=%d %s missing=%d %s", path, privateResponse.Code, privateResponse.Body.String(), missingResponse.Code, missingResponse.Body.String())
		}
	}
	if _, err = repo.Pool.Exec(ctx, "UPDATE workspace_memberships SET active=false,deactivated_at=now() WHERE workspace_id=$1 AND actor_id=$2", postgres.DemoWorkspaceID, postgres.DemoAgentActorID); err != nil {
		t.Fatal(err)
	}
	if response := readPath(primaryToken.Token, "/v1/bird-views/"+birdTestViewID); response.Code != http.StatusUnauthorized {
		t.Fatalf("removed token membership still derived an account principal: status=%d body=%s", response.Code, response.Body.String())
	}
	execute("bird_view.overlay.replace", "overlay", 8, map[string]any{"birdViewId": birdTestViewID, "nodeId": birdTestNodeID, "bindings": []map[string]any{
		{"targetType": "task", "workspaceId": postgres.DemoWorkspaceID, "targetId": "ui"},
		{"targetType": "task", "workspaceId": postgres.DemoWorkspaceID, "targetId": "assets"},
		{"targetType": "task", "workspaceId": postgres.DemoWorkspaceID, "targetId": "api"},
	}})
	execute("bird_view.binding.pin", "pin", 9, map[string]any{"birdViewId": birdTestViewID, "nodeId": birdTestNodeID, "targetType": "task", "targetWorkspaceId": postgres.DemoWorkspaceID, "targetId": "ui"})
	execute("bird_view.binding.exclude", "exclude", 10, map[string]any{"birdViewId": birdTestViewID, "nodeId": birdTestNodeID, "targetType": "task", "targetWorkspaceId": postgres.DemoWorkspaceID, "targetId": "assets"})
	execute("bird_view.overlay.replace", "overlay-replace", 11, map[string]any{"birdViewId": birdTestViewID, "nodeId": birdTestNodeID, "bindings": []map[string]any{{"targetType": "task", "workspaceId": postgres.DemoWorkspaceID, "targetId": "user-test"}}})
	staleArguments, _ := json.Marshal(map[string]any{"birdViewId": birdTestViewID, "nodeId": birdTestNodeID, "title": "Stale"})
	_, staleErr := service.Execute(ctx, application.CommandRequest{Name: "bird_view.node.update", Arguments: staleArguments, Principal: principal, Envelope: application.CommandEnvelope{IdempotencyKey: "stale", ExpectedBirdViewRevision: 11, ExecutedByActorID: postgres.DemoHumanActorID}})
	var commandErr *application.CommandError
	if !errors.As(staleErr, &commandErr) || commandErr.Code != "stale_bird_view_revision" {
		t.Fatalf("stale Bird View revision was not rejected: %v", staleErr)
	}

	focus, err := service.Focus(ctx, principal, birdTestViewID, birdTestNodeID)
	if err != nil {
		t.Fatal(err)
	}
	if len(focus.Workspaces) != 1 || len(focus.Workspaces[0].Lanes) == 0 {
		t.Fatalf("focused Workspace lane projection missing: %#v", focus.Workspaces)
	}
	type bindingState struct{ targetID, state string }
	rows, err := repo.Pool.Query(ctx, `SELECT task_id,binding_state FROM bird_view_task_bindings
		WHERE bird_view_id=$1 AND node_id=$2 ORDER BY task_id`, birdTestViewID, birdTestNodeID)
	if err != nil {
		t.Fatal(err)
	}
	states := []bindingState{}
	for rows.Next() {
		var state bindingState
		if err = rows.Scan(&state.targetID, &state.state); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		states = append(states, state)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	wantStates := []bindingState{{"assets", "excluded"}, {"ui", "pinned"}, {"user-test", "suggested"}}
	if len(states) != len(wantStates) {
		t.Fatalf("overlay replacement states=%v, want %v", states, wantStates)
	}
	for index := range wantStates {
		if states[index] != wantStates[index] {
			t.Fatalf("overlay replacement states=%v, want %v", states, wantStates)
		}
	}
	execute("bird_view.archive", "archive", 12, map[string]any{"birdViewId": birdTestViewID})
	if listed, listErr := service.List(ctx, principal, false); listErr != nil || len(listed) != 0 {
		t.Fatalf("default archived list=%#v err=%v", listed, listErr)
	}
	if listed, listErr := service.List(ctx, principal, true); listErr != nil || len(listed) != 1 || listed[0].Status != "archived" {
		t.Fatalf("inclusive archived list=%#v err=%v", listed, listErr)
	}
	if archived, getErr := service.Get(ctx, principal, birdTestViewID); getErr != nil || archived.Status != "archived" {
		t.Fatalf("direct archived owner read=%#v err=%v", archived, getErr)
	}
	updateArguments, _ := json.Marshal(map[string]any{"birdViewId": birdTestViewID, "title": "Archived mutation"})
	_, archivedErr := service.Execute(ctx, application.CommandRequest{Name: "bird_view.update", Arguments: updateArguments, Principal: principal, Envelope: application.CommandEnvelope{IdempotencyKey: "archived-update", ExpectedBirdViewRevision: 13, ExecutedByActorID: postgres.DemoHumanActorID}})
	if !errors.As(archivedErr, &commandErr) || commandErr.Code != "bird_view_archived" {
		t.Fatalf("archived Bird View mutation was not rejected: %v", archivedErr)
	}
	if _, err = repo.Pool.Exec(ctx, "UPDATE workspace_memberships SET active=false,deactivated_at=now() WHERE workspace_id=$1 AND actor_id=$2", postgres.DemoWorkspaceID, postgres.DemoHumanActorID); err != nil {
		t.Fatal(err)
	}
	focus, err = service.Focus(ctx, principal, birdTestViewID, birdTestNodeID)
	if err != nil {
		t.Fatal(err)
	}
	if len(focus.Workspaces) != 0 {
		t.Fatalf("revoked Workspace leaked through focus: %#v", focus.Workspaces)
	}
	contextValue, err := service.Context(ctx, principal, birdTestViewID, birdTestNodeID)
	if err != nil {
		t.Fatal(err)
	}
	if len(contextValue.Bindings) != 0 {
		t.Fatalf("revoked target leaked through LLM context: %#v", contextValue.Bindings)
	}
	var commandCount, eventCount int
	if err = repo.Pool.QueryRow(ctx, "SELECT (SELECT count(*) FROM bird_view_commands WHERE bird_view_id=$1),(SELECT count(*) FROM bird_view_events WHERE bird_view_id=$1)", birdTestViewID).Scan(&commandCount, &eventCount); err != nil {
		t.Fatal(err)
	}
	if commandCount != 13 || eventCount != 13 {
		t.Fatalf("own ledger counts commands=%d events=%d", commandCount, eventCount)
	}
}
