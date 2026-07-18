package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/jazzcake/baley/server/internal/application"
	"github.com/jazzcake/baley/server/internal/domain"
)

func TestParseGoldenQueriesAndMutations(t *testing.T) {
	tests := []struct {
		args      []string
		kind      InvocationKind
		name      string
		arguments string
		execute   bool
	}{
		{[]string{"lane", "brief", "lane-a", "--workspace", "workspace"}, QueryInvocation, "lane.brief", `{"laneId":"lane-a","workspaceId":"workspace"}`, false},
		{[]string{"task", "list", "--workspace", "workspace", "--status", "pending", "--lane", "lane-a"}, QueryInvocation, "task.list", `{"laneId":"lane-a","status":"pending","workspaceId":"workspace"}`, false},
		{[]string{"gate", "status", "gate-a", "--workspace", "workspace"}, QueryInvocation, "gate.status", `{"gateId":"gate-a","workspaceId":"workspace"}`, false},
		{[]string{"run", "list", "--workspace", "workspace", "--task", "104"}, QueryInvocation, "run.list", `{"taskId":104,"workspaceId":"workspace"}`, false},
		{[]string{"record", "list", "--workspace", "workspace", "--task", "104"}, QueryInvocation, "record.list", `{"taskId":104,"workspaceId":"workspace"}`, false},
		{[]string{"run", "start", "104", "--workspace", "workspace", "--kind", "implementation", "--actor", "agent", "--idempotency", "run-104", "--revision", "7", "--execute"}, MutationInvocation, "run.start", `{"kind":"implementation","taskId":104,"workspaceId":"workspace"}`, true},
		{[]string{"record", "attach-commit", "record-a", "--workspace", "workspace", "--commit-sha", `"abc"`, "--actor", "agent", "--idempotency", "record-a-commit"}, MutationInvocation, "record.attach_commit", `{"commitSha":"abc","recordId":"record-a","workspaceId":"workspace"}`, false},
		{[]string{"lane", "close-out", "lane-a", "--workspace", "workspace", "--actor", "agent", "--idempotency", "lane-close"}, MutationInvocation, "lane.close_out", `{"laneId":"lane-a","workspaceId":"workspace"}`, false},
		{[]string{"gate", "pass", "gate-a", "--workspace", "workspace", "--actor", "agent", "--idempotency", "gate-pass"}, MutationInvocation, "gate.pass", `{"gateId":"gate-a","workspaceId":"workspace"}`, false},
	}
	for _, test := range tests {
		invocation, err := Parse(test.args)
		if err != nil || invocation.Kind != test.kind || invocation.Name != test.name || string(invocation.Arguments) != test.arguments || invocation.Execute != test.execute {
			t.Errorf("Parse(%v) = %+v, %v", test.args, invocation, err)
		}
	}
}

func TestRunQueryUsesHTTPClientPortShape(t *testing.T) {
	client := &fakeClient{queryResult: json.RawMessage(`{"items":[]}`)}
	invocation, _ := Parse([]string{"task", "list", "--workspace", "workspace", "--status", "pending"})
	outcome, err := Run(context.Background(), client, invocation, nil)
	if err != nil || string(outcome.QueryResult) != `{"items":[]}` || client.query.Name != "task.list" || client.query.WorkspaceID != "workspace" || string(client.query.Arguments) != `{"status":"pending","workspaceId":"workspace"}` {
		t.Fatalf("query request drift: %+v %+v %v", client.query, outcome, err)
	}
}

func TestRunStopsHumanOnlyCommandAfterPreviewUntilApproval(t *testing.T) {
	client := &fakeClient{preview: application.PreviewResult{
		CommandHash: "sha256:command", DecisionSnapshotHash: "sha256:decision", ExpectedWorkspaceRevision: 8,
		Errors: []domain.Diagnostic{{Code: domain.CodeHumanApprovalRequired, EntityID: "104"}},
	}}
	invocation, _ := Parse([]string{"task", "confirm", "104", "--workspace", "workspace", "--actor", "agent", "--idempotency", "confirm-104", "--execute"})
	outcome, err := Run(context.Background(), client, invocation, nil)
	if err != nil || !outcome.ApprovalRequired || client.executeCalls != 0 {
		t.Fatalf("human-only command did not stop: %+v %v", outcome, err)
	}
	approval := &Approval{
		ApprovedByActorID: "human", ApprovedCommandHash: outcome.Preview.CommandHash,
		ExpectedWorkspaceRevision: outcome.Preview.ExpectedWorkspaceRevision, DecisionSnapshotHash: outcome.Preview.DecisionSnapshotHash,
		StatementHash: "sha256:statement", ConversationRef: "conversation",
	}
	outcome, err = Run(context.Background(), client, invocation, approval)
	attestation := client.executed.Envelope.HumanApprovalAttestation
	if err != nil || outcome.Execution == nil || client.executeCalls != 1 || attestation == nil || attestation.ApprovedCommandHash != "sha256:command" || attestation.DecisionSnapshotHash != "sha256:decision" || client.executed.Envelope.ExpectedWorkspaceRevision != 8 {
		t.Fatalf("approved execute shape wrong: %+v %+v %v", client.executed, outcome, err)
	}
}

func TestRunRejectsApprovalWhenFreshPreviewChanged(t *testing.T) {
	client := &fakeClient{preview: application.PreviewResult{
		CommandHash: "sha256:old", DecisionSnapshotHash: "sha256:old-decision", ExpectedWorkspaceRevision: 8,
		Errors: []domain.Diagnostic{{Code: domain.CodeHumanApprovalRequired}},
	}}
	invocation, _ := Parse([]string{"gate", "pass", "gate-a", "--workspace", "workspace", "--actor", "agent", "--idempotency", "gate-pass", "--execute"})
	first, err := Run(context.Background(), client, invocation, nil)
	if err != nil || !first.ApprovalRequired {
		t.Fatalf("initial preview failed: %+v %v", first, err)
	}
	approval := &Approval{
		ApprovedByActorID: "human", ApprovedCommandHash: first.Preview.CommandHash,
		ExpectedWorkspaceRevision: first.Preview.ExpectedWorkspaceRevision, DecisionSnapshotHash: first.Preview.DecisionSnapshotHash,
	}
	client.preview = application.PreviewResult{
		CommandHash: "sha256:new", DecisionSnapshotHash: "sha256:new-decision", ExpectedWorkspaceRevision: 9,
		Errors: []domain.Diagnostic{{Code: domain.CodeHumanApprovalRequired}},
	}
	_, err = Run(context.Background(), client, invocation, approval)
	if !IsCode(err, domain.CodeStaleRevision) || client.executeCalls != 0 {
		t.Fatalf("changed preview silently re-approved: %v calls=%d", err, client.executeCalls)
	}
	client.preview.ExpectedWorkspaceRevision = 8
	_, err = Run(context.Background(), client, invocation, approval)
	if !IsCode(err, domain.CodeHumanApprovalMismatch) || client.executeCalls != 0 {
		t.Fatalf("changed command hash silently re-approved: %v calls=%d", err, client.executeCalls)
	}
}

func TestRunAutomaticRunAndRecordLifecycleNeedsNoApproval(t *testing.T) {
	for _, args := range [][]string{
		{"run", "succeed", "run-a", "--workspace", "workspace", "--summary", "done", "--actor", "agent", "--idempotency", "run-done", "--execute"},
		{"record", "register", "record-a", "--workspace", "workspace", "--relative-path", "task-records/report.md", "--actor", "agent", "--idempotency", "record-a", "--execute"},
	} {
		client := &fakeClient{preview: application.PreviewResult{CommandHash: "sha256:auto", ExpectedWorkspaceRevision: 9}}
		invocation, parseErr := Parse(args)
		outcome, err := Run(context.Background(), client, invocation, nil)
		if parseErr != nil || err != nil || outcome.Execution == nil || client.executeCalls != 1 || client.executed.Envelope.HumanApprovalAttestation != nil {
			t.Fatalf("automatic lifecycle interrupted: %+v %v %v", outcome, parseErr, err)
		}
	}
}

func TestRunStopsForWarningsAndPreservesStructuredStaleError(t *testing.T) {
	client := &fakeClient{preview: application.PreviewResult{ExpectedWorkspaceRevision: 7, Warnings: []domain.Diagnostic{{Code: domain.CodeDanglingPath}}}}
	invocation, _ := Parse([]string{"task", "report-implemented", "104", "--workspace", "workspace", "--actor", "agent", "--idempotency", "implemented", "--execute"})
	outcome, err := Run(context.Background(), client, invocation, nil)
	if err != nil || !reflect.DeepEqual(outcome.WarningAcknowledgementRequired, []string{domain.CodeDanglingPath}) || client.executeCalls != 0 {
		t.Fatalf("warning did not stop execute: %+v %v", outcome, err)
	}
	invocation, _ = Parse([]string{"task", "report-implemented", "104", "--workspace", "workspace", "--actor", "agent", "--idempotency", "implemented", "--ack", domain.CodeDanglingPath, "--execute"})
	client.executeErr = &StructuredError{Code: domain.CodeStaleRevision, Message: "workspace changed"}
	_, err = Run(context.Background(), client, invocation, nil)
	if !IsCode(err, domain.CodeStaleRevision) {
		t.Fatalf("stale revision lost structured code: %v", err)
	}
}

func TestParseRejectsUnsupportedMissingAndDuplicateInputs(t *testing.T) {
	for _, args := range [][]string{
		{"lane", "list", "--workspace", "workspace"},
		{"task", "get", "--workspace", "workspace"},
		{"run", "start", "104", "--workspace", "workspace"},
		{"task", "list", "--workspace", "workspace", "--status", "pending", "--status", "done"},
		{"task", "list", "--workspace", "workspace", "--arg", "workspaceId=other"},
		{"task", "list", "--workspace", "workspace", "--execute"},
	} {
		if _, err := Parse(args); !IsCode(err, "invalid_request") {
			t.Fatalf("invalid input accepted: %v, %v", args, err)
		}
	}
}

func TestSupportedQueryNamesMatchLiteralContract(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "contracts", "v1", "commands.json"))
	if err != nil {
		t.Fatal(err)
	}
	var contract struct {
		Queries []string `json:"queries"`
	}
	if err := json.Unmarshal(data, &contract); err != nil {
		t.Fatal(err)
	}
	if len(contract.Queries) != len(queryNames) {
		t.Fatalf("query catalog size drift: %v vs %v", contract.Queries, queryNames)
	}
	for _, name := range contract.Queries {
		if !queryNames[name] {
			t.Errorf("query %s missing from CLI model", name)
		}
	}
}

type fakeClient struct {
	query        QueryRequest
	queryResult  json.RawMessage
	preview      application.PreviewResult
	executed     application.CommandRequest
	execution    application.ExecutionResult
	executeErr   error
	executeCalls int
}

func (f *fakeClient) Query(_ context.Context, request QueryRequest) (json.RawMessage, error) {
	f.query = request
	return f.queryResult, nil
}

func (f *fakeClient) Preview(_ context.Context, _ application.CommandRequest) (application.PreviewResult, error) {
	return f.preview, nil
}

func (f *fakeClient) Execute(_ context.Context, request application.CommandRequest) (application.ExecutionResult, error) {
	f.executed = request
	f.executeCalls++
	if f.executeErr != nil {
		return application.ExecutionResult{}, f.executeErr
	}
	if f.execution.CommandID == "" {
		f.execution.CommandID = "command"
	}
	return f.execution, nil
}
