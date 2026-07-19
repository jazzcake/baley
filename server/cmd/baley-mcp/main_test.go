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
