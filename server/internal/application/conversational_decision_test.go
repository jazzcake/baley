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
