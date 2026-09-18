package application

import "testing"

func TestStatementSupportsConversationalDecision(t *testing.T) {
	tests := []struct {
		name, statement, scope string
		taskID                 int
		want                   bool
	}{
		{"confirm hash number", "confirm #110", "task", 110, true},
		{"confirm number", "confirm 110", "task", 110, true},
		{"confirm task number", "CONFIRM TASK #110!", "task", 110, true},
		{"complete hash number", "complete #110", "task", 110, true},
		{"complete number", "complete 110.", "task", 110, true},
		{"Korean number confirm request", "110번 확인해 주세요", "task", 110, true},
		{"Korean hash number complete request", "#110 완료해주세요!", "task", 110, true},
		{"Korean object complete request", "작업 #110을 완료해 주십시오", "task", 110, true},
		{"compound mixed decision confirms first Task", "#61, #62 confirm, #63 폐기, #64, #65도 폐기", "task", 61, true},
		{"compound mixed decision confirms second Task", "#61, #62 confirm, #63 폐기, #64, #65도 폐기", "task", 62, true},
		{"compound mixed decision does not confirm discarded Task", "#61, #62 confirm, #63 폐기, #64, #65도 폐기", "task", 63, false},
		{"compound mixed decision is not all-awaiting scope", "#61, #62 confirm, #63 폐기, #64, #65도 폐기", "all_awaiting_confirmation", 61, false},
		{"compound decision rejects unassigned trailing Task", "#61, #62 confirm, #63", "task", 61, false},
		{"compound decision rejects conflicting duplicate Task", "#61 confirm, #61 폐기", "task", 61, false},
		{"compound decision rejects unknown action", "#61, #62 ship", "task", 61, false},
		{"complete all awaiting", "complete all awaiting confirmation", "all_awaiting_confirmation", 110, true},
		{"confirm all awaiting tasks", "confirm all tasks awaiting confirmation!", "all_awaiting_confirmation", 110, true},
		{"Korean all awaiting completion", "확인 대기 중인 모든 작업을 완료해 주세요", "all_awaiting_confirmation", 110, true},
		{"Korean all awaiting alternate order", "확인 대기 중인 태스크를 모두 완료해주세요", "all_awaiting_confirmation", 110, true},
		{"Korean practical all completion", "완료 확인 대기 사항들 전부 완료처리", "all_awaiting_confirmation", 110, true},
		{"Korean practical all confirmation", "완료 확인 대기 건 전부 확인해주세요", "all_awaiting_confirmation", 110, true},
		{"Korean counted direct approval", "완료 확인 25건, 직접 처리하세요. 제가 승인했습니다.", "all_awaiting_confirmation", 110, true},
		{"English reverse all completion", "implemented tasks awaiting confirmation all complete", "all_awaiting_confirmation", 110, true},
		{"vague affirmation", "yes", "task", 110, false},
		{"question", "can you confirm #110?", "task", 110, false},
		{"question without punctuation", "can you confirm #110", "task", 110, false},
		{"Korean permission question", "#110 확인해도 될까요", "task", 110, false},
		{"Korean question punctuation", "110번 완료할까요?", "task", 110, false},
		{"wrong Task", "confirm #111", "task", 110, false},
		{"multiple Tasks", "confirm #110 and #111", "task", 110, false},
		{"cross-target comma", "confirm #110, #111", "task", 110, false},
		{"English negation", "do not confirm #110", "task", 110, false},
		{"English contraction negation", "don't complete #110", "task", 110, false},
		{"English curly contraction negation", "don’t approve #110", "task", 110, false},
		{"English cannot", "cannot confirm #110", "task", 110, false},
		{"English exclusion", "confirm #110 except if tests fail", "task", 110, false},
		{"English qualifier", "confirm #110 if Alice agrees", "task", 110, false},
		{"English hedging", "probably confirm #110", "task", 110, false},
		{"Korean suffix negation", "#110 확인하지 마", "task", 110, false},
		{"Korean separated negation", "#110 확인 안 함", "task", 110, false},
		{"Korean prohibited", "#110 확인하면 안 돼", "task", 110, false},
		{"Korean qualifier", "문제 없으면 #110 완료해 주세요", "task", 110, false},
		{"Korean exclusion", "#111 제외하고 #110 완료해 주세요", "task", 110, false},
		{"negated all", "do not complete all awaiting confirmation", "all_awaiting_confirmation", 110, false},
		{"question all", "can you complete all awaiting confirmation?", "all_awaiting_confirmation", 110, false},
		{"qualified all", "complete all awaiting confirmation if safe", "all_awaiting_confirmation", 110, false},
		{"excluded Task all", "complete all awaiting confirmation except #110", "all_awaiting_confirmation", 110, false},
		{"cross-target all", "complete all awaiting confirmation and #110", "all_awaiting_confirmation", 110, false},
		{"hash target with counted approval", "완료 확인 #25건, 직접 처리하세요. 제가 승인했습니다.", "all_awaiting_confirmation", 110, false},
		{"Korean negated all", "확인 대기 중인 모든 작업을 완료하면 안 돼", "all_awaiting_confirmation", 110, false},
		{"Korean excluded all", "#110 제외하고 확인 대기 중인 작업을 모두 완료해 주세요", "all_awaiting_confirmation", 110, false},
		{"scope mismatch single", "confirm #110", "all_awaiting_confirmation", 110, false},
		{"scope mismatch all", "complete all awaiting confirmation", "task", 110, false},
		{"unknown scope", "confirm #110", "ambiguous", 110, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := statementSupportsConversationalDecision(tt.statement, tt.scope, tt.taskID); got != tt.want {
				t.Fatalf("statementSupportsConversationalDecision(%q, %q, %d)=%v want %v", tt.statement, tt.scope, tt.taskID, got, tt.want)
			}
		})
	}
}

func TestConversationalDecisionMismatchReason(t *testing.T) {
	base := func() ConversationalDecisionEvidence {
		return ConversationalDecisionEvidence{
			DecisionID: "01a0b2d7-2da5-7b82-a829-7767b8b9df88", Source: "conversation",
			ConversationRef: "01a0b21e-94a2-7d73-a017-d359706eb1e0",
			Statement:       "#61, #62 confirm, #63 폐기, #64, #65도 폐기",
			Scope:           "task", Action: "task.confirm", TaskID: 61,
			WorkspaceRevision: 1017, CommandHash: "sha256:fresh",
		}
	}
	typed := taskConfirmArgs{TaskID: 61}
	preview := PreviewResult{ExpectedWorkspaceRevision: 1017, CommandHash: "sha256:fresh"}

	if reason := conversationalDecisionMismatchReason("task.confirm", base(), typed, preview); reason != "" {
		t.Fatalf("valid compound decision reason=%q", reason)
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
		{"all scope cannot describe selected Tasks", "statement_scope", func(value *ConversationalDecisionEvidence) {
			value.Scope = "all_awaiting_confirmation"
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

func TestStatementSupportsConversationalTaskDiscard(t *testing.T) {
	tests := []struct {
		name, statement string
		taskID          int
		want            bool
	}{
		{"English discard", "discard task #63", 63, true},
		{"English delete", "delete #63", 63, true},
		{"Korean discard", "#63 폐기", 63, true},
		{"Korean delete together", "Task #63을 삭제합시다", 63, true},
		{"Korean delete request", "63번 작업을 삭제해 주세요", 63, true},
		{"compound discard first", "#61, #62 confirm, #63 폐기, #64, #65도 삭제", 63, true},
		{"compound discard inherited", "#61, #62 confirm, #63 폐기, #64, #65도 삭제", 64, true},
		{"compound discard last", "#61, #62 confirm, #63 폐기, #64, #65도 삭제", 65, true},
		{"confirm target is not discard", "#61, #62 confirm, #63 폐기", 61, false},
		{"wrong target", "#64 삭제", 63, false},
		{"explicit contextual delete", "삭제합시다", 63, true},
		{"explicit contextual discard", "폐기해 주세요", 63, true},
		{"vague affirmation", "좋아요", 63, false},
		{"question", "#63 삭제할까요?", 63, false},
		{"negation", "#63 삭제하지 마", 63, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := statementSupportsConversationalTaskAction(tt.statement, "task", "task.discard", tt.taskID); got != tt.want {
				t.Fatalf("discard statement=%q task=%d got=%v want=%v", tt.statement, tt.taskID, got, tt.want)
			}
		})
	}
}

func TestConversationalDiscardMismatchReason(t *testing.T) {
	evidence := ConversationalDecisionEvidence{
		DecisionID: "33333333-3333-4333-8333-333333333333", Source: "conversation",
		ConversationRef: "turn:discard-63", Statement: "#63 삭제합시다",
		Scope: "task", Action: "task.discard", TaskID: 63,
		WorkspaceRevision: 1034, CommandHash: "sha256:discard",
	}
	typed := taskMutationArgs{TaskID: 63, Reason: "사용자가 명시적으로 삭제를 결정함"}
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
		Details: map[string]any{"reason": "statement_scope"},
	}
	want := "command evaluation failed: decision_evidence_mismatch (statement_scope)"
	if got := commandEvaluationErrorMessage(diagnostic); got != want {
		t.Fatalf("message=%q want %q", got, want)
	}
}
