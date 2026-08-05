package domain

import (
	"testing"
	"time"
)

func TestExternalExecutionLifecycleKeepsLostLockedUntilSettled(t *testing.T) {
	now := time.Date(2026, 8, 5, 1, 2, 3, 0, time.UTC)
	execution, err := ReserveExternalExecution("execution-1", "workspace-1", "task-1", "11111111-1111-4111-8111-111111111111", "orca", "sha256:context", "actor-1", 1, now)
	if err != nil {
		t.Fatal(err)
	}
	execution, err = execution.Attach("worktree-1", "instance-1", "local", "terminal-1", now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	execution, err = execution.MarkLost(now.Add(2 * time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if !execution.IsOpen() {
		t.Fatal("lost execution must keep the Task lock")
	}
	if evaluation := ValidateExternalExecutionLock([]ExternalExecution{execution}, "task-1", "orca"); !evaluation.HasErrors() || evaluation.Errors[0].Code != CodeExternalExecutionAlreadyActive {
		t.Fatalf("expected lock error: %+v", evaluation)
	}
	execution, err = execution.Settle(ExternalExecutionAbandoned, now.Add(3*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if execution.IsOpen() || ValidateExternalExecutionLock([]ExternalExecution{execution}, "task-1", "orca").HasErrors() {
		t.Fatal("settled execution must release the Task lock")
	}
}

func TestExternalExecutionReconnectsOnlyFromLost(t *testing.T) {
	now := time.Now().UTC()
	execution, _ := ReserveExternalExecution("execution-1", "workspace-1", "task-1", "11111111-1111-4111-8111-111111111111", "orca", "", "actor-1", 1, now)
	if _, err := execution.Reconnect("worktree", "instance", "local", "terminal", now); err == nil {
		t.Fatal("creating execution must not reconnect")
	}
	execution, _ = execution.MarkLost(now.Add(time.Second))
	reconnected, err := execution.Reconnect("worktree", "instance", "local", "terminal", now.Add(2*time.Second))
	if err != nil || reconnected.Status != ExternalExecutionActive {
		t.Fatalf("reconnect failed: %+v %v", reconnected, err)
	}
}
