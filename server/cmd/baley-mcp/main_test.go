package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

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
	requests := make(chan capturedRequest, 8)
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

	wantNames := []string{"phase.create", "phase.create", "lane.create", "lane.create", "gate.create", "gate.create", "gate.attach_task", "gate.attach_task"}
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
