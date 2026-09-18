package application

import "testing"

func TestConversationalDecisionAcceptsAgentDeclaredNaturalLanguageMeaning(t *testing.T) {
	preview := PreviewResult{ExpectedWorkspaceRevision: 1034, CommandHash: "sha256:fresh"}
	tests := []struct {
		name, command, action, statement, decisionID string
		taskID                                       int
		typed                                        any
	}{
		{"Korean compound confirm first", "task.confirm", "task.confirm", "#61·#62는 scope=task로 각각 confirm", "11111111-1111-4111-8111-111111111111", 61, taskConfirmArgs{TaskID: 61}},
		{"Korean compound confirm second", "task.confirm", "task.confirm", "#61·#62는 scope=task로 각각 confirm", "22222222-2222-4222-8222-222222222222", 62, taskConfirmArgs{TaskID: 62}},
		{"natural discard first", "task.discard", "task.discard", "이제 #61, #62 삭제해줘.", "33333333-3333-4333-8333-333333333333", 61, taskMutationArgs{TaskID: 61}},
		{"natural discard second", "task.discard", "task.discard", "이제 #61, #62 삭제해줘.", "44444444-4444-4444-8444-444444444444", 62, taskMutationArgs{TaskID: 62}},
		{"target inherited from exact conversation context", "task.discard", "task.discard", "그 두 작업은 삭제해 주세요", "55555555-5555-4555-8555-555555555555", 62, taskMutationArgs{TaskID: 62}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evidence := ConversationalDecisionEvidence{
				DecisionID: tt.decisionID, Source: "conversation", ConversationRef: "conversation:natural-language-test",
				Statement: tt.statement, Scope: "task", Action: tt.action, TaskID: tt.taskID,
				WorkspaceRevision: 1034, CommandHash: "sha256:fresh",
			}
			if reason := conversationalDecisionMismatchReason(tt.command, evidence, tt.typed, preview); reason != "" {
				t.Fatalf("natural-language decision rejected: reason=%q", reason)
			}
		})
	}
}

func TestConversationalDecisionMismatchReason(t *testing.T) {
	base := func() ConversationalDecisionEvidence {
		return ConversationalDecisionEvidence{
			DecisionID: "01a0b2d7-2da5-7b82-a829-7767b8b9df88", Source: "conversation",
			ConversationRef: "01a0b21e-94a2-7d73-a017-d359706eb1e0",
			Statement:       "#61·#62는 scope=task로 각각 confirm",
			Scope:           "task", Action: "task.confirm", TaskID: 61,
			WorkspaceRevision: 1017, CommandHash: "sha256:fresh",
		}
	}
	typed := taskConfirmArgs{TaskID: 61}
	preview := PreviewResult{ExpectedWorkspaceRevision: 1017, CommandHash: "sha256:fresh"}

	if reason := conversationalDecisionMismatchReason("task.confirm", base(), typed, preview); reason != "" {
		t.Fatalf("valid natural-language decision reason=%q", reason)
	}
	tests := []struct {
		name, want string
		change     func(*ConversationalDecisionEvidence)
	}{
		{"message identifier is not a decision UUID", "decision_id", func(value *ConversationalDecisionEvidence) {
			value.DecisionID = "msg_01a0b2d7-2e0e-73f0-b843-c36681df7dda"
		}},
		{"conversation reference is required but opaque", "conversation_ref", func(value *ConversationalDecisionEvidence) {
			value.ConversationRef = " "
		}},
		{"statement is required as opaque audit evidence", "statement", func(value *ConversationalDecisionEvidence) {
			value.Statement = " "
		}},
		{"evidence target must match command target", "task_id", func(value *ConversationalDecisionEvidence) {
			value.TaskID = 62
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value := base()
			tt.change(&value)
			if reason := conversationalDecisionMismatchReason("task.confirm", value, typed, preview); reason != tt.want {
				t.Fatalf("reason=%q want %q", reason, tt.want)
			}
		})
	}
}

func TestConversationalDiscardMismatchReason(t *testing.T) {
	evidence := ConversationalDecisionEvidence{
		DecisionID: "33333333-3333-4333-8333-333333333333", Source: "conversation",
		ConversationRef: "conversation:discard-61-62", Statement: "이제 #61, #62 삭제해줘.",
		Scope: "task", Action: "task.discard", TaskID: 61,
		WorkspaceRevision: 1034, CommandHash: "sha256:discard",
	}
	typed := taskMutationArgs{TaskID: 61, Reason: "사용자가 명시적으로 삭제를 결정함"}
	preview := PreviewResult{ExpectedWorkspaceRevision: 1034, CommandHash: "sha256:discard"}
	if reason := conversationalDecisionMismatchReason("task.discard", evidence, typed, preview); reason != "" {
		t.Fatalf("valid discard reason=%q", reason)
	}
	evidence.Action = "task.confirm"
	if reason := conversationalDecisionMismatchReason("task.discard", evidence, typed, preview); reason != "action" {
		t.Fatalf("cross-action reason=%q want action", reason)
	}
	evidence.Action = "task.discard"
	evidence.Scope = "all_awaiting_confirmation"
	if reason := conversationalDecisionMismatchReason("task.discard", evidence, typed, preview); reason != "scope" {
		t.Fatalf("discard all-awaiting reason=%q want scope", reason)
	}
}

func TestCommandEvaluationErrorMessageIncludesDecisionMismatchReason(t *testing.T) {
	diagnostic := Diagnostic{
		Code:    "decision_evidence_mismatch",
		Details: map[string]any{"reason": "statement"},
	}
	want := "command evaluation failed: decision_evidence_mismatch (statement)"
	if got := commandEvaluationErrorMessage(diagnostic); got != want {
		t.Fatalf("message=%q want %q", got, want)
	}
}
