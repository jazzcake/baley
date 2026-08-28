package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestBaleyToolAnnotationsKeepOperatorWorkSilent(t *testing.T) {
	ctx := context.Background()
	server := newMCPServer(&client{})
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	clientSession, err := mcp.NewClient(&mcp.Implementation{Name: "annotation-test", Version: "test"}, nil).Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientSession.Close()

	listed, err := clientSession.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	readOnly := map[string]bool{
		"baley_workspace_get": true, "baley_workspace_context": true, "baley_workspace_graph": true,
		"baley_phase_tasks":     true,
		"baley_mcp_diagnostics": true,
		"baley_task_get":        true, "baley_task_acceptance_get": true,
		"baley_lane_brief": true, "baley_backlog_list": true,
		"baley_backlog_get": true, "baley_gate_status": true,
		"baley_decision_list": true, "baley_event_list": true,
		"baley_mutation_attempt_list": true, "baley_run_list": true,
		"baley_record_list": true,
	}
	humanApproval := map[string]bool{
		"baley_task_acceptance_policy_change_execute": true,
		"baley_task_acceptance_mode_escalate_execute": true,
		"baley_gate_attach_task_execute":              true,
		"baley_task_confirm_execute":                  true,
		"baley_task_discard_execute":                  true,
		"baley_gate_pass_task_execute":                true,
		"baley_gate_revoke_task_pass_execute":         true,
		"baley_gate_pass_execute":                     true,
	}
	for _, tool := range listed.Tools {
		annotations := tool.Annotations
		if annotations == nil || annotations.OpenWorldHint == nil || *annotations.OpenWorldHint {
			t.Errorf("%s must be annotated as closed-world: %#v", tool.Name, annotations)
			continue
		}
		wantReadOnly := readOnly[tool.Name] || strings.HasSuffix(tool.Name, "_preview")
		if annotations.ReadOnlyHint != wantReadOnly {
			t.Errorf("%s readOnlyHint=%v, want %v", tool.Name, annotations.ReadOnlyHint, wantReadOnly)
		}
		if annotations.DestructiveHint == nil {
			t.Errorf("%s is missing destructiveHint", tool.Name)
			continue
		}
		if got, want := *annotations.DestructiveHint, humanApproval[tool.Name]; got != want {
			t.Errorf("%s destructiveHint=%v, want %v", tool.Name, got, want)
		}
	}
}

func TestStreamableHTTPMCPPersistsEncryptedWorkspaceCredentialsAcrossSessions(t *testing.T) {
	const gatewayToken = "local-gateway-token"
	const workspaceToken = "workspace-agent-token"
	var connectionCreated, connectionPolled, workspaceReads int
	var upstreamAuthorizations []string
	var upstreamMu sync.Mutex
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamMu.Lock()
		defer upstreamMu.Unlock()
		upstreamAuthorizations = append(upstreamAuthorizations, r.Header.Get("Authorization"))
		switch r.URL.Path {
		case "/v1/mcp/connections":
			connectionCreated++
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"connection","workspaceId":"workspace","status":"pending","connectionSecret":"secret","approvalUrl":"http://viewer/approve"}`))
		case "/v1/mcp/connections/connection":
			connectionPolled++
			if r.Header.Get("X-Baley-Connection-Secret") != "secret" {
				http.Error(w, "missing connection secret", http.StatusForbidden)
				return
			}
			_, _ = w.Write([]byte(`{"id":"connection","workspaceId":"workspace","status":"consumed","agentToken":"` + workspaceToken + `"}`))
		case "/v1/workspaces/workspace":
			workspaceReads++
			if r.Header.Get("Authorization") != "Bearer "+workspaceToken {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(`{"id":"workspace"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	credentials := filepath.Join(t.TempDir(), "credentials.json")
	c := &client{base: upstream.URL, http: upstream.Client(), gatewayToken: gatewayToken, credentialStorePath: credentials, agentActorID: "agent"}
	handler := c.requireGatewayBearer(mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return newMCPServer(c) }, &mcp.StreamableHTTPOptions{JSONResponse: true, SessionTimeout: time.Minute}))
	mcpHTTP := httptest.NewServer(handler)
	defer mcpHTTP.Close()

	unauthenticated, err := http.Post(mcpHTTP.URL, "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if unauthenticated.StatusCode != http.StatusUnauthorized {
		t.Fatalf("missing bearer status=%d", unauthenticated.StatusCode)
	}
	_ = unauthenticated.Body.Close()

	newSession := func() (*mcp.ClientSession, error) {
		httpClient := &http.Client{Transport: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
			clone := request.Clone(request.Context())
			clone.Header.Set("Authorization", "Bearer "+gatewayToken)
			return http.DefaultTransport.RoundTrip(clone)
		})}
		return mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil).Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: mcpHTTP.URL, HTTPClient: httpClient, DisableStandaloneSSE: true}, nil)
	}
	firstSession, err := newSession()
	if err != nil {
		t.Fatal(err)
	}
	first, err := firstSession.CallTool(context.Background(), &mcp.CallToolParams{Name: "baley_workspace_get", Arguments: map[string]any{"workspaceId": "workspace"}})
	_ = firstSession.Close()
	if err != nil || !first.IsError {
		t.Fatalf("initial connection result=%#v err=%v", first, err)
	}

	secondSession, err := newSession()
	if err != nil {
		t.Fatal(err)
	}
	second, err := secondSession.CallTool(context.Background(), &mcp.CallToolParams{Name: "baley_workspace_get", Arguments: map[string]any{"workspaceId": "workspace"}})
	_ = secondSession.Close()
	if err != nil || second.IsError {
		t.Fatalf("approved connection did not persist into next MCP session: result=%#v err=%v", second, err)
	}

	thirdSession, err := newSession()
	if err != nil {
		t.Fatal(err)
	}
	third, err := thirdSession.CallTool(context.Background(), &mcp.CallToolParams{Name: "baley_workspace_get", Arguments: map[string]any{"workspaceId": "workspace"}})
	_ = thirdSession.Close()
	if err != nil || third.IsError {
		t.Fatalf("stored credential did not survive another MCP session: result=%#v err=%v", third, err)
	}

	upstreamMu.Lock()
	defer upstreamMu.Unlock()
	if connectionCreated != 1 || connectionPolled != 1 || workspaceReads != 2 {
		t.Fatalf("connections=%d polls=%d workspaceReads=%d, want 1/1/2", connectionCreated, connectionPolled, workspaceReads)
	}
	for _, authorization := range upstreamAuthorizations {
		if authorization == "Bearer "+gatewayToken {
			t.Fatalf("gateway token leaked to the Baley API: %v", upstreamAuthorizations)
		}
	}
	raw, err := os.ReadFile(credentials)
	if err != nil {
		t.Fatal(err)
	}
	if containsJSONSecret(raw, workspaceToken) {
		t.Fatal("Workspace token was written to the HTTP credential store in plaintext")
	}
	entries, err := os.ReadDir(filepath.Dir(credentials))
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected one persistent credential store, entries=%v err=%v", entries, err)
	}
}
func TestClientSendsConfiguredAgentTokenWithoutPuttingItInThePayload(t *testing.T) {
	const token = "agent-secret-that-must-stay-in-the-header"
	var authorization string
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"workspaceRevision":2}`))
	}))
	defer server.Close()

	c := &client{base: server.URL, http: server.Client(), agentToken: token}
	result, _, err := c.runStart(context.Background(), nil, runStartInput{
		WorkspaceID: "workspace",
		TaskID:      135,
		ClientRunID: "client-run",
		Kind:        "implementation",
		automaticEnvelope: automaticEnvelope{
			ExpectedWorkspaceRevision: 1,
			IdempotencyKey:            "idempotency-key",
			ExecutedByActorID:         "agent",
		},
	})
	if err != nil || result.IsError {
		t.Fatalf("run start failed: %#v %v", result, err)
	}
	if authorization != "Bearer "+token {
		t.Fatalf("missing agent bearer token: %q", authorization)
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) == "" || containsJSONSecret(raw, token) {
		t.Fatal("agent token leaked into the command payload")
	}
}

func TestClientConnectsWorkspaceOnceAndPersistsScopedCredential(t *testing.T) {
	const workspaceID = "410f335e-ddb2-443f-be3c-7d1d18ccd534"
	const token = "workspace-scoped-agent-token"
	var connectionCreated, workspaceRead bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/mcp/connections":
			connectionCreated = true
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"c1","workspaceId":"` + workspaceID + `","status":"pending","connectionSecret":"secret","approvalUrl":"http://viewer/workspaces/` + workspaceID + `/mcp-connect/c1"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/mcp/connections/c1":
			if r.Header.Get("X-Baley-Connection-Secret") != "secret" {
				http.Error(w, "missing connection secret", http.StatusForbidden)
				return
			}
			_, _ = w.Write([]byte(`{"id":"c1","workspaceId":"` + workspaceID + `","status":"consumed","agentToken":"` + token + `"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/workspaces/"+workspaceID:
			workspaceRead = true
			if r.Header.Get("Authorization") != "Bearer "+token {
				http.Error(w, "missing workspace token", http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(`{"id":"` + workspaceID + `","revision":1}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	storePath := filepath.Join(t.TempDir(), "credentials.json")
	c := &client{
		base: server.URL, http: server.Client(), credentialStorePath: storePath,
		agentActorID: "agent",
	}
	result, _, err := c.workspaceGet(context.Background(), nil, workspaceInput{WorkspaceID: workspaceID})
	if err != nil || result == nil || !result.IsError || !connectionCreated {
		t.Fatalf("expected one-time connection request: result=%#v err=%v", result, err)
	}
	structured, ok := result.StructuredContent.(map[string]any)
	if !ok || structured["code"] != "workspace_login_required" || structured["approvalUrl"] == "" {
		t.Fatalf("missing actionable login result: %#v", result.StructuredContent)
	}

	// A new MCP process must resume the approval request from the local
	// credential store rather than relying on the original process memory.
	restarted := &client{
		base: server.URL, http: server.Client(), credentialStorePath: storePath,
		agentActorID: "agent",
	}
	result, _, err = restarted.workspaceGet(context.Background(), nil, workspaceInput{WorkspaceID: workspaceID})
	if err != nil || result.IsError || !workspaceRead {
		t.Fatalf("approved request did not continue automatically: result=%#v err=%v", result, err)
	}
	raw, err := os.ReadFile(storePath)
	if err != nil {
		t.Fatal(err)
	}
	if containsJSONSecret(raw, token) {
		t.Fatal("Workspace token was persisted")
	}
	store, err := restarted.readCredentialStore(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := store.PendingConnections[workspaceID]; exists {
		t.Fatal("approved pending connection was not removed")
	}
}

func TestClientPreservesStoredCredentialForMissingWorkspaceResource(t *testing.T) {
	const workspaceID = "410f335e-ddb2-443f-be3c-7d1d18ccd534"
	const token = "workspace-agent-token"
	var connectionCreated bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/workspaces/" + workspaceID + "/tasks/999999":
			if r.Header.Get("Authorization") != "Bearer "+token {
				t.Fatalf("unexpected credential: %q", r.Header.Get("Authorization"))
			}
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"code":"not_found","message":"task not found"}}`))
		case "/v1/workspaces/" + workspaceID:
			if r.Header.Get("Authorization") != "Bearer "+token {
				t.Fatalf("unexpected validation credential: %q", r.Header.Get("Authorization"))
			}
			_, _ = w.Write([]byte(`{"id":"` + workspaceID + `"}`))
		case "/v1/mcp/connections":
			connectionCreated = true
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"unexpected","workspaceId":"` + workspaceID + `"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	storePath := filepath.Join(t.TempDir(), "credentials.json")
	c := &client{base: server.URL, http: server.Client(), credentialStorePath: storePath, agentActorID: "agent"}
	if err := c.writeCredentialStore(context.Background(), &credentialStore{
		Version: 1, ServerURL: server.URL,
		Workspaces: map[string]workspaceCredential{workspaceID: {AgentToken: token}},
	}); err != nil {
		t.Fatal(err)
	}
	c.rememberSessionToken(workspaceID, token)

	result, _, err := c.taskGet(context.Background(), nil, taskInput{WorkspaceID: workspaceID, TaskID: 999999})
	if err != nil || result == nil || !result.IsError || connectionCreated {
		t.Fatalf("missing resource should stay a 404: result=%#v err=%v connectionCreated=%v", result, err, connectionCreated)
	}
	store, err := c.readCredentialStore(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := store.Workspaces[workspaceID]; ok {
		t.Fatalf("token-only Workspace credential survived persistence: %#v", store.Workspaces)
	}
}
func TestClientReplacesStoredCredentialWhenWorkspaceReadIsConcealedAsNotFound(t *testing.T) {
	const workspaceID = "410f335e-ddb2-443f-be3c-7d1d18ccd534"
	var connectionCreated bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet && r.URL.Path == "/v1/workspaces/"+workspaceID {
			if r.Header.Get("Authorization") != "Bearer stale-or-cross-workspace-token" {
				t.Fatalf("unexpected credential: %q", r.Header.Get("Authorization"))
			}
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"code":"not_found","message":"workspace not found"}}`))
			return
		}
		if r.Method == http.MethodPost && r.URL.Path == "/v1/mcp/connections" {
			connectionCreated = true
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"replacement","workspaceId":"` + workspaceID + `","status":"pending","connectionSecret":"secret","approvalUrl":"http://viewer/workspaces/` + workspaceID + `/mcp-connect/replacement"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	storePath := filepath.Join(t.TempDir(), "credentials.json")
	c := &client{
		base: server.URL, http: server.Client(), credentialStorePath: storePath,
		agentActorID: "agent",
	}
	if err := c.writeCredentialStore(context.Background(), &credentialStore{
		Version: 1, ServerURL: server.URL,
		Workspaces: map[string]workspaceCredential{workspaceID: {AgentToken: "stale-or-cross-workspace-token"}},
	}); err != nil {
		t.Fatal(err)
	}
	c.rememberSessionToken(workspaceID, "stale-or-cross-workspace-token")

	result, _, err := c.workspaceGet(context.Background(), nil, workspaceInput{WorkspaceID: workspaceID})
	if err != nil || result == nil || !result.IsError || !connectionCreated {
		t.Fatalf("expected replacement Owner connection: result=%#v err=%v", result, err)
	}
	structured, ok := result.StructuredContent.(map[string]any)
	if !ok || structured["code"] != "workspace_login_required" {
		t.Fatalf("404 did not become a login connection request: %#v", result.StructuredContent)
	}
	store, err := c.readCredentialStore(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := store.Workspaces[workspaceID]; exists {
		t.Fatal("stale Workspace credential was not removed")
	}
}

func containsJSONSecret(raw []byte, secret string) bool {
	for index := 0; index+len(secret) <= len(raw); index++ {
		if string(raw[index:index+len(secret)]) == secret {
			return true
		}
	}
	return false
}

func TestTaskConfirmExecuteForwardsWarningAcknowledgementEnvelope(t *testing.T) {
	var body map[string]any
	var decodeErr error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		decodeErr = json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"workspaceRevision":2}`))
	}))
	defer server.Close()

	c := &client{base: server.URL, http: server.Client()}
	in := taskConfirmExecuteInput{WorkspaceID: "workspace", TaskID: 110, executeEnvelope: executeEnvelope{
		ExpectedWorkspaceRevision: 1,
		IdempotencyKey:            "retry-key",
		ExecutedByActorID:         "agent",
		AcknowledgedWarningCodes:  []string{"dangling_path"},
		ProceedReason:             "Intentional terminal validation task.",
		ApprovedByActorID:         "human",
		ApprovedCommandHash:       "sha256:command",
	}}
	result, _, err := c.taskConfirmExecute(context.Background(), nil, in)
	if err != nil || result.IsError {
		t.Fatalf("task confirm execute failed: %#v %v", result, err)
	}
	if decodeErr != nil {
		t.Fatal(decodeErr)
	}
	envelope, ok := body["envelope"].(map[string]any)
	if !ok {
		t.Fatalf("missing envelope: %#v", body)
	}
	codes, ok := envelope["acknowledgedWarningCodes"].([]any)
	if !ok || len(codes) != 1 || codes[0] != "dangling_path" {
		t.Fatalf("warning acknowledgement not forwarded: %#v", envelope)
	}
	if envelope["proceedReason"] != "Intentional terminal validation task." {
		t.Fatalf("proceed reason not forwarded: %#v", envelope)
	}
}

func TestTaskDiscardPreviewAndExecuteForwardReason(t *testing.T) {
	var requests []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		requests = append(requests, body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"workspaceRevision":2}`))
	}))
	defer server.Close()

	c := &client{base: server.URL, http: server.Client()}
	preview := taskDiscardPreviewInput{WorkspaceID: "workspace", TaskID: 1, Reason: "No longer needed", previewEnvelope: previewEnvelope{ExpectedWorkspaceRevision: 1, IdempotencyKey: "preview-key", ExecutedByActorID: "agent"}}
	if result, _, err := c.taskDiscardPreview(context.Background(), nil, preview); err != nil || result.IsError {
		t.Fatalf("task discard preview failed: %#v %v", result, err)
	}
	execute := taskDiscardExecuteInput{WorkspaceID: "workspace", TaskID: 1, Reason: "No longer needed", executeEnvelope: executeEnvelope{ExpectedWorkspaceRevision: 1, IdempotencyKey: "execute-key", ExecutedByActorID: "agent", ApprovedByActorID: "human", ApprovedCommandHash: "sha256:command"}}
	if result, _, err := c.taskDiscardExecute(context.Background(), nil, execute); err != nil || result.IsError {
		t.Fatalf("task discard execute failed: %#v %v", result, err)
	}
	if len(requests) != 2 {
		t.Fatalf("got %d requests, want 2", len(requests))
	}
	for _, body := range requests {
		arguments, _ := body["arguments"].(map[string]any)
		if body["name"] != "task.discard" || arguments["taskId"] != float64(1) || arguments["reason"] != "No longer needed" {
			t.Fatalf("task discard payload mismatch: %#v", body)
		}
	}
}

func TestTaskCreatePreviewAndExecuteForwardTypedPayloads(t *testing.T) {
	type capturedRequest struct {
		path string
		body map[string]any
	}
	requests := make(chan capturedRequest, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		requests <- capturedRequest{path: r.URL.Path, body: body}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"commandHash":"sha256:test","workspaceRevision":12}`))
	}))
	defer server.Close()

	c := &client{base: server.URL, http: server.Client()}
	fields := taskCreateFields{
		WorkspaceID: "workspace", TaskUUID: "00000000-0000-4000-8000-000000000111",
		LaneID: "client", PhaseID: "validate", ParentTaskID: 110, Title: "Restart API",
		Description: "Align runtime with source", PredecessorTaskIDs: []int{110},
		SuccessorTaskIDs: []int{101},
		TerminalReason:   "Operational checkpoint",
	}
	preview := taskCreatePreviewInput{taskCreateFields: fields, previewEnvelope: previewEnvelope{
		ExpectedWorkspaceRevision: 11, IdempotencyKey: "preview-key", ExecutedByActorID: "agent",
	}}
	result, _, err := c.taskCreatePreview(context.Background(), nil, preview)
	if err != nil || result.IsError {
		t.Fatalf("task create preview failed: %#v %v", result, err)
	}
	previewRequest := <-requests
	previewBody := previewRequest.body
	if previewRequest.path != "/v1/commands/preview" {
		t.Fatalf("wrong preview path: %s", previewRequest.path)
	}
	if previewBody["name"] != "task.create" {
		t.Fatalf("wrong preview command: %#v", previewBody)
	}
	arguments, ok := previewBody["arguments"].(map[string]any)
	if !ok || arguments["taskUuid"] != fields.TaskUUID || arguments["title"] != fields.Title || len(arguments["successorTaskIds"].([]any)) != 1 {
		t.Fatalf("task create arguments not forwarded: %#v", previewBody)
	}
	previewEnvelope, ok := previewBody["envelope"].(map[string]any)
	if !ok || previewEnvelope["acknowledgedWarningCodes"] != nil || previewEnvelope["proceedReason"] != nil {
		t.Fatalf("warning evidence leaked into preview envelope: %#v", previewBody)
	}

	execute := taskCreateExecuteInput{taskCreateFields: fields,
		AcknowledgedWarningCodes: []string{"phase_order_inversion"}, ProceedReason: "Reviewed cross-phase relationship.",
		automaticEnvelope: automaticEnvelope{ExpectedWorkspaceRevision: 11, IdempotencyKey: "execute-key", ExecutedByActorID: "agent"},
	}
	result, _, err = c.taskCreateExecute(context.Background(), nil, execute)
	if err != nil || result.IsError {
		t.Fatalf("task create execute failed: %#v %v", result, err)
	}
	executeRequest := <-requests
	executeBody := executeRequest.body
	if executeRequest.path != "/v1/commands/execute" {
		t.Fatalf("wrong execute path: %s", executeRequest.path)
	}
	envelope, ok := executeBody["envelope"].(map[string]any)
	if !ok || envelope["proceedReason"] != execute.ProceedReason {
		t.Fatalf("task create execute envelope not forwarded: %#v", executeBody)
	}
	codes, ok := envelope["acknowledgedWarningCodes"].([]any)
	if !ok || len(codes) != 1 || codes[0] != "phase_order_inversion" {
		t.Fatalf("task create warning acknowledgement not forwarded: %#v", executeBody)
	}
	executeArguments, ok := executeBody["arguments"].(map[string]any)
	if !ok || executeArguments["acknowledgedWarningCodes"] != nil || executeArguments["proceedReason"] != nil {
		t.Fatalf("warning evidence leaked into task.create arguments: %#v", executeBody)
	}
}

func TestTaskUpdatePreviewAndExecuteForwardOnlyContentFields(t *testing.T) {
	type capturedRequest struct {
		path string
		body map[string]any
	}
	requests := make(chan capturedRequest, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		requests <- capturedRequest{path: r.URL.Path, body: body}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"commandHash":"sha256:test","workspaceRevision":12}`))
	}))
	defer server.Close()

	c := &client{base: server.URL, http: server.Client()}
	description := "Updated description"
	summary := "People can understand the result quickly."
	fields := taskUpdateFields{WorkspaceID: "workspace", TaskID: 22, Description: &description, CurrentSummary: &summary}
	_, _, err := c.taskUpdatePreview(context.Background(), nil, taskUpdatePreviewInput{taskUpdateFields: fields, previewEnvelope: previewEnvelope{ExpectedWorkspaceRevision: 11, IdempotencyKey: "preview-key", ExecutedByActorID: "agent"}})
	if err != nil {
		t.Fatal(err)
	}
	preview := <-requests
	arguments := preview.body["arguments"].(map[string]any)
	if preview.path != "/v1/commands/preview" || preview.body["name"] != "task.update" || arguments["taskId"] != float64(22) || arguments["description"] != description || arguments["currentSummary"] != summary || arguments["title"] != nil {
		t.Fatalf("task.update preview was not limited to content fields: %#v", preview)
	}
	_, _, err = c.taskUpdateExecute(context.Background(), nil, taskUpdateExecuteInput{taskUpdateFields: fields, mutationExecuteEnvelope: mutationExecuteEnvelope{automaticEnvelope: automaticEnvelope{ExpectedWorkspaceRevision: 11, IdempotencyKey: "execute-key", ExecutedByActorID: "agent"}}})
	if err != nil {
		t.Fatal(err)
	}
	execute := <-requests
	if execute.path != "/v1/commands/execute" || execute.body["name"] != "task.update" {
		t.Fatalf("task.update execute was not forwarded: %#v", execute)
	}
}

func TestTaskClearTerminalPreviewAndExecuteForwardTaskOnly(t *testing.T) {
	type capturedRequest struct {
		path string
		body map[string]any
	}
	requests := make(chan capturedRequest, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		requests <- capturedRequest{r.URL.Path, body}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"commandHash":"sha256:test","workspaceRevision":12}`))
	}))
	defer server.Close()
	c := &client{base: server.URL, http: server.Client()}
	fields := taskClearTerminalFields{WorkspaceID: "workspace", TaskID: 23}
	_, _, err := c.taskClearTerminalPreview(context.Background(), nil, taskClearTerminalPreviewInput{taskClearTerminalFields: fields, previewEnvelope: previewEnvelope{ExpectedWorkspaceRevision: 11, IdempotencyKey: "preview-key", ExecutedByActorID: "agent"}})
	if err != nil {
		t.Fatal(err)
	}
	preview := <-requests
	if preview.path != "/v1/commands/preview" || preview.body["name"] != "task.clear_terminal" || preview.body["arguments"].(map[string]any)["taskId"] != float64(23) {
		t.Fatalf("clear terminal preview was not forwarded: %#v", preview)
	}
	_, _, err = c.taskClearTerminalExecute(context.Background(), nil, taskClearTerminalExecuteInput{taskClearTerminalFields: fields, mutationExecuteEnvelope: mutationExecuteEnvelope{automaticEnvelope: automaticEnvelope{ExpectedWorkspaceRevision: 11, IdempotencyKey: "execute-key", ExecutedByActorID: "agent"}}})
	if err != nil {
		t.Fatal(err)
	}
	execute := <-requests
	if execute.path != "/v1/commands/execute" || execute.body["name"] != "task.clear_terminal" {
		t.Fatalf("clear terminal execute was not forwarded: %#v", execute)
	}
}

func TestDependencyPatchPreviewAndExecuteForwardAtomicRewrite(t *testing.T) {
	type capturedRequest struct {
		path string
		body map[string]any
	}
	requests := make(chan capturedRequest, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		requests <- capturedRequest{path: r.URL.Path, body: body}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"commandHash":"sha256:test","workspaceRevision":13}`))
	}))
	defer server.Close()

	c := &client{base: server.URL, http: server.Client()}
	fields := dependencyPatchFields{WorkspaceID: "workspace", Remove: []dependencyRefInput{{PredecessorTaskID: 16, SuccessorTaskID: 17}, {PredecessorTaskID: 16, SuccessorTaskID: 13}}}
	preview := dependencyPatchPreviewInput{dependencyPatchFields: fields, previewEnvelope: previewEnvelope{ExpectedWorkspaceRevision: 12, IdempotencyKey: "preview-key", ExecutedByActorID: "agent"}}
	if result, _, err := c.dependencyPatchPreview(context.Background(), nil, preview); err != nil || result.IsError {
		t.Fatalf("dependency patch preview failed: %#v %v", result, err)
	}
	previewRequest := <-requests
	if previewRequest.path != "/v1/commands/preview" || previewRequest.body["name"] != "dependency.patch" {
		t.Fatalf("dependency preview routing mismatch: %#v", previewRequest)
	}
	arguments := previewRequest.body["arguments"].(map[string]any)
	if _, exists := arguments["add"]; exists || len(arguments["remove"].([]any)) != 2 {
		t.Fatalf("dependency rewrite arguments mismatch: %#v", arguments)
	}

	execute := dependencyPatchExecuteInput{dependencyPatchFields: fields, mutationExecuteEnvelope: mutationExecuteEnvelope{automaticEnvelope: automaticEnvelope{ExpectedWorkspaceRevision: 12, IdempotencyKey: "execute-key", ExecutedByActorID: "agent"}}}
	if result, _, err := c.dependencyPatchExecute(context.Background(), nil, execute); err != nil || result.IsError {
		t.Fatalf("dependency patch execute failed: %#v %v", result, err)
	}
	executeRequest := <-requests
	if executeRequest.path != "/v1/commands/execute" || executeRequest.body["name"] != "dependency.patch" {
		t.Fatalf("dependency execute routing mismatch: %#v", executeRequest)
	}
}

func TestTaskCreateExecuteOmitsEmptyOptionalWarningEvidence(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"workspaceRevision":12}`))
	}))
	defer server.Close()

	c := &client{base: server.URL, http: server.Client()}
	in := taskCreateExecuteInput{
		taskCreateFields: taskCreateFields{
			WorkspaceID: "workspace", TaskUUID: "00000000-0000-4000-8000-000000000111",
			LaneID: "server", PhaseID: "validate", Title: "Restart API",
		},
		automaticEnvelope: automaticEnvelope{
			ExpectedWorkspaceRevision: 11, IdempotencyKey: "execute-key", ExecutedByActorID: "agent",
		},
	}
	result, _, err := c.taskCreateExecute(context.Background(), nil, in)
	if err != nil || result.IsError {
		t.Fatalf("task create execute failed: %#v %v", result, err)
	}
	envelope, ok := body["envelope"].(map[string]any)
	if !ok {
		t.Fatalf("missing envelope: %#v", body)
	}
	if _, exists := envelope["acknowledgedWarningCodes"]; exists {
		t.Fatalf("empty warning acknowledgement must be omitted: %#v", envelope)
	}
	if _, exists := envelope["proceedReason"]; exists {
		t.Fatalf("empty proceed reason must be omitted: %#v", envelope)
	}
}

func TestStructuralCreateHandlersForwardTypedCommandsAndConditionalApproval(t *testing.T) {
	type capturedRequest struct {
		path string
		body map[string]any
	}
	requests := make(chan capturedRequest, 12)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		requests <- capturedRequest{path: r.URL.Path, body: body}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"commandHash":"sha256:test","workspaceRevision":12}`))
	}))
	defer server.Close()

	c := &client{base: server.URL, http: server.Client()}
	previewEnvValue := previewEnvelope{ExpectedWorkspaceRevision: 11, IdempotencyKey: "preview-key", ExecutedByActorID: "agent", InitiatedByActorID: "human"}
	executeEnvValue := mutationExecuteEnvelope{automaticEnvelope: automaticEnvelope{ExpectedWorkspaceRevision: 11, IdempotencyKey: "execute-key", ExecutedByActorID: "agent", InitiatedByActorID: "human"}}

	_, _, _ = c.phaseCreatePreview(context.Background(), nil, phaseCreatePreviewInput{phaseCreateFields: phaseCreateFields{WorkspaceID: "workspace", PhaseID: "contract", Name: "Embedding Contract"}, previewEnvelope: previewEnvValue})
	_, _, _ = c.phaseCreateExecute(context.Background(), nil, phaseCreateExecuteInput{phaseCreateFields: phaseCreateFields{WorkspaceID: "workspace", PhaseID: "contract", Name: "Embedding Contract"}, mutationExecuteEnvelope: executeEnvValue})
	_, _, _ = c.laneCreatePreview(context.Background(), nil, laneCreatePreviewInput{laneCreateFields: laneCreateFields{WorkspaceID: "workspace", LaneID: "adoption", Name: "Adoption", Goal: "Adopt", Summary: "Enablement"}, previewEnvelope: previewEnvValue})
	_, _, _ = c.laneCreateExecute(context.Background(), nil, laneCreateExecuteInput{laneCreateFields: laneCreateFields{WorkspaceID: "workspace", LaneID: "adoption", Name: "Adoption", Goal: "Adopt", Summary: "Enablement"}, mutationExecuteEnvelope: executeEnvValue})
	_, _, _ = c.gateCreatePreview(context.Background(), nil, gateCreatePreviewInput{gateCreateFields: gateCreateFields{WorkspaceID: "workspace", GateID: "contract-ready", Name: "Contract Ready", FromPhaseID: "validate", ToPhaseID: "contract"}, previewEnvelope: previewEnvValue})
	_, _, _ = c.gateCreateExecute(context.Background(), nil, gateCreateExecuteInput{gateCreateFields: gateCreateFields{WorkspaceID: "workspace", GateID: "contract-ready", Name: "Contract Ready", FromPhaseID: "validate", ToPhaseID: "contract"}, mutationExecuteEnvelope: executeEnvValue})
	_, _, _ = c.gateAttachTaskPreview(context.Background(), nil, gateAttachTaskPreviewInput{gateAttachTaskFields: gateAttachTaskFields{WorkspaceID: "workspace", GateID: "contract-ready", TaskID: 116, ClearTerminal: true}, previewEnvelope: previewEnvValue})
	_, _, _ = c.gateAttachTaskExecute(context.Background(), nil, gateAttachTaskExecuteInput{gateAttachTaskFields: gateAttachTaskFields{WorkspaceID: "workspace", GateID: "contract-ready", TaskID: 116, ClearTerminal: true}, conditionalExecuteEnvelope: conditionalExecuteEnvelope{mutationExecuteEnvelope: executeEnvValue, ApprovedByActorID: "human", ApprovedCommandHash: "sha256:test", ConversationRef: "thread"}})
	_, _, _ = c.gateAttachEntryTaskPreview(context.Background(), nil, gateEntryTaskPreviewInput{gateEntryTaskFields: gateEntryTaskFields{WorkspaceID: "workspace", GateID: "contract-ready", TaskID: 129}, previewEnvelope: previewEnvValue})
	_, _, _ = c.gateAttachEntryTaskExecute(context.Background(), nil, gateEntryTaskExecuteInput{gateEntryTaskFields: gateEntryTaskFields{WorkspaceID: "workspace", GateID: "contract-ready", TaskID: 129}, automaticEnvelope: executeEnvValue.automaticEnvelope})
	_, _, _ = c.gateDetachEntryTaskPreview(context.Background(), nil, gateEntryTaskPreviewInput{gateEntryTaskFields: gateEntryTaskFields{WorkspaceID: "workspace", GateID: "contract-ready", TaskID: 129}, previewEnvelope: previewEnvValue})
	_, _, _ = c.gateDetachEntryTaskExecute(context.Background(), nil, gateEntryTaskExecuteInput{gateEntryTaskFields: gateEntryTaskFields{WorkspaceID: "workspace", GateID: "contract-ready", TaskID: 129}, automaticEnvelope: executeEnvValue.automaticEnvelope})

	wantNames := []string{"phase.create", "phase.create", "lane.create", "lane.create", "gate.create", "gate.create", "gate.attach_task", "gate.attach_task", "gate.attach_entry_task", "gate.attach_entry_task", "gate.detach_entry_task", "gate.detach_entry_task"}
	for index, wantName := range wantNames {
		request := <-requests
		wantPath := "/v1/commands/preview"
		if index%2 == 1 {
			wantPath = "/v1/commands/execute"
		}
		if request.path != wantPath || request.body["name"] != wantName {
			t.Fatalf("request %d mismatch: path=%s body=%#v", index, request.path, request.body)
		}
		envelope, ok := request.body["envelope"].(map[string]any)
		if !ok {
			t.Fatalf("request %d missing envelope: %#v", index, request.body)
		}
		if envelope["initiatedByActorId"] != "human" {
			t.Fatalf("request %d missing initiator attribution: %#v", index, envelope)
		}
		if _, exists := envelope["approvalGrantToken"]; exists {
			t.Fatalf("request %d forwarded removed approvalGrantToken: %#v", index, envelope)
		}
		if index == 7 {
			attestation, ok := envelope["humanApprovalAttestation"].(map[string]any)
			if !ok || attestation["approvedByActorId"] != "human" || attestation["approvedCommandHash"] != "sha256:test" || attestation["conversationRef"] != "thread" {
				t.Fatalf("conditional approval not forwarded: %#v", envelope)
			}
			for _, field := range []string{"decisionSnapshotHash", "statementHash", "approvedAt"} {
				if _, exists := attestation[field]; exists {
					t.Fatalf("empty optional approval field %s must be omitted: %#v", field, attestation)
				}
			}
		} else if envelope["humanApprovalAttestation"] != nil {
			t.Fatalf("approval leaked into non-conditional request %d: %#v", index, envelope)
		}
	}
}
