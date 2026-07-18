package domain

import (
	"testing"
	"time"
)

func TestRunTerminalTransitions(t *testing.T) {
	now := time.Date(2026, 7, 18, 10, 0, 0, 0, time.UTC)
	tests := []struct {
		name          string
		status        RunStatus
		summary       string
		wantResult    string
		wantError     string
		wantViolation string
	}{
		{name: "succeeded", status: RunSucceeded, summary: " implementation complete ", wantResult: "implementation complete"},
		{name: "failed", status: RunFailed, summary: " tests failed ", wantError: "tests failed"},
		{name: "interrupted", status: RunInterrupted, summary: " lease expired ", wantError: "lease expired"},
		{name: "cancelled", status: RunCancelled, summary: " user cancelled ", wantError: "user cancelled"},
		{name: "running is not terminal", status: RunRunning, summary: "invalid", wantViolation: CodeInvalidStateTransition},
		{name: "blank summary", status: RunSucceeded, summary: "  ", wantViolation: CodeInvalidStateTransition},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			run := runningRun(now)
			next, outcome, err := run.Terminate(tt.status, run.Version, now.Add(time.Minute), tt.summary)
			if tt.wantViolation != "" {
				assertViolation(t, err, tt.wantViolation)
				if next != run || outcome != "" {
					t.Fatalf("failed transition mutated run: next=%+v outcome=%q", next, outcome)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if outcome != RunTransitionApplied || next.Status != tt.status || next.ResultSummary != tt.wantResult || next.ErrorSummary != tt.wantError {
				t.Fatalf("unexpected terminal result: next=%+v outcome=%q", next, outcome)
			}
			if next.Version != run.Version+1 || next.EndedAt == nil || !next.EndedAt.Equal(now.Add(time.Minute)) {
				t.Fatalf("terminal metadata not updated: %+v", next)
			}
		})
	}
}

func TestRunTerminalRetryIsIdempotentOnlyForSameOutcome(t *testing.T) {
	now := time.Date(2026, 7, 18, 10, 0, 0, 0, time.UTC)
	run, _, err := runningRun(now).Terminate(RunSucceeded, 4, now.Add(time.Minute), "done")
	if err != nil {
		t.Fatal(err)
	}
	same, outcome, err := run.Terminate(RunSucceeded, 4, now.Add(2*time.Minute), " done ")
	if err != nil || outcome != RunTransitionIdempotent || same != run {
		t.Fatalf("same terminal retry should be idempotent: run=%+v outcome=%q err=%v", same, outcome, err)
	}
	_, _, err = run.Terminate(RunFailed, run.Version, now.Add(2*time.Minute), "late failure")
	assertViolation(t, err, CodeIdempotencyConflict)
	_, _, err = run.Terminate(RunSucceeded, run.Version, now.Add(2*time.Minute), "different summary")
	assertViolation(t, err, CodeIdempotencyConflict)
}

func TestRunTerminalTransitionRejectsStaleVersion(t *testing.T) {
	run := runningRun(time.Now())
	next, _, err := run.Terminate(RunSucceeded, run.Version-1, time.Now(), "done")
	assertViolation(t, err, CodeStaleRunVersion)
	if next != run {
		t.Fatal("stale transition mutated run")
	}
}

func TestRunManualCorrectionRecordsPreviousAndNewValues(t *testing.T) {
	now := time.Date(2026, 7, 18, 10, 0, 0, 0, time.UTC)
	failed, _, err := runningRun(now).Terminate(RunFailed, 4, now.Add(time.Minute), "transient error")
	if err != nil {
		t.Fatal(err)
	}
	corrected, correction, err := failed.CorrectTerminal(RunSucceeded, failed.Version, now.Add(2*time.Minute), "verified success", "operator corrected imported result")
	if err != nil {
		t.Fatal(err)
	}
	if correction.PreviousStatus != RunFailed || correction.PreviousErrorSummary != "transient error" || correction.NewStatus != RunSucceeded || correction.NewResultSummary != "verified success" || correction.Reason == "" {
		t.Fatalf("correction lacks audit values: %+v", correction)
	}
	if corrected.Status != RunSucceeded || corrected.Version != failed.Version+1 {
		t.Fatalf("correction not applied: %+v", corrected)
	}
	_, _, err = failed.CorrectTerminal(RunSucceeded, failed.Version, now, "done", " ")
	assertViolation(t, err, CodeInvalidStateTransition)
	_, _, err = runningRun(now).CorrectTerminal(RunFailed, 4, now, "failed", "manual")
	assertViolation(t, err, CodeInvalidStateTransition)
}

func runningRun(now time.Time) Run {
	return Run{
		ID:             "run-1",
		Status:         RunRunning,
		LeaseTokenHash: HashLeaseToken("secret-token"),
		HeartbeatAt:    now,
		LeaseExpiresAt: now.Add(5 * time.Minute),
		Version:        4,
		StartedAt:      now,
	}
}
