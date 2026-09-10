package application

import "testing"

func TestStatementSupportsConversationalDecision(t *testing.T) {
	tests := []struct {
		name, statement, scope string
		taskID                 int
		want                   bool
	}{
		{"exact English Task", "confirm #110", "task", 110, true},
		{"exact Korean Task", "#110 확인 완료", "task", 110, true},
		{"all awaiting", "complete all awaiting confirmation", "all_awaiting_confirmation", 110, true},
		{"vague affirmation", "yes", "task", 110, false},
		{"wrong Task", "confirm #111", "task", 110, false},
		{"multiple Tasks", "confirm #110 and #111", "task", 110, false},
		{"English negation", "do not confirm #110", "task", 110, false},
		{"English contraction negation", "don't complete #110", "task", 110, false},
		{"English curly contraction negation", "don’t approve #110", "task", 110, false},
		{"Korean suffix negation", "#110 확인하지 마", "task", 110, false},
		{"Korean separated negation", "#110 확인 안 함", "task", 110, false},
		{"negated all", "do not complete all awaiting confirmation", "all_awaiting_confirmation", 110, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := statementSupportsConversationalDecision(tt.statement, tt.scope, tt.taskID); got != tt.want {
				t.Fatalf("statementSupportsConversationalDecision(%q, %q, %d)=%v want %v", tt.statement, tt.scope, tt.taskID, got, tt.want)
			}
		})
	}
}
