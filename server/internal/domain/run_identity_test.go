package domain

import (
	"testing"
	"time"
)

const testClientRunID = "00000000-0000-4000-8000-000000000099"

func TestRunStartIdentityComparison(t *testing.T) {
	base := RunStartIdentity{WorkspaceID: "workspace", TaskID: "task", ClientRunID: testClientRunID, Kind: RunImplementation, ParentRunID: "parent", TargetRunID: "target"}
	if err := CompareRunStartIdentity(base, base); err != nil {
		t.Fatalf("same identity rejected: %v", err)
	}
	mutations := map[string]func(*RunStartIdentity){
		"workspace": func(value *RunStartIdentity) { value.WorkspaceID = "other" },
		"task":      func(value *RunStartIdentity) { value.TaskID = "other" },
		"client run": func(value *RunStartIdentity) {
			value.ClientRunID = "00000000-0000-4000-8000-000000000098"
		},
		"kind":   func(value *RunStartIdentity) { value.Kind = RunReviewResponse },
		"parent": func(value *RunStartIdentity) { value.ParentRunID = "other" },
		"target": func(value *RunStartIdentity) { value.TargetRunID = "other" },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			requested := base
			mutate(&requested)
			assertViolation(t, CompareRunStartIdentity(base, requested), CodeIdempotencyConflict)
		})
	}
}

func TestRunStartIdentityValidation(t *testing.T) {
	valid := RunStartIdentity{WorkspaceID: "workspace", TaskID: "task", ClientRunID: testClientRunID, Kind: RunImplementation}
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*RunStartIdentity){
		"workspace":     func(value *RunStartIdentity) { value.WorkspaceID = "" },
		"task":          func(value *RunStartIdentity) { value.TaskID = " " },
		"client":        func(value *RunStartIdentity) { value.ClientRunID = "" },
		"client format": func(value *RunStartIdentity) { value.ClientRunID = "client-run" },
		"kind":          func(value *RunStartIdentity) { value.Kind = RunKind("unknown") },
	} {
		t.Run(name, func(t *testing.T) {
			value := valid
			mutate(&value)
			assertViolation(t, value.Validate(), CodeInvalidStateTransition)
		})
	}
}

func TestRunLeaseHeartbeatAndExpiry(t *testing.T) {
	now := testRunTime()
	run := runningRun(now)
	next, err := run.Heartbeat("secret-token", run.Version, now.Add(time.Minute), 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if !next.HeartbeatAt.Equal(now.Add(time.Minute)) || !next.LeaseExpiresAt.Equal(now.Add(15*time.Minute)) || next.Version != run.Version+1 {
		t.Fatalf("heartbeat did not extend lease: %+v", next)
	}
	if run.IsLeaseExpired(now.Add(5*time.Minute - time.Nanosecond)) {
		t.Fatal("lease must remain valid strictly before expiry")
	}
	if !run.IsLeaseExpired(now.Add(5 * time.Minute)) {
		t.Fatal("lease must be expired at its boundary")
	}
}

func TestRunHeartbeatRejectsTimeRegressionAndNeverShortensLease(t *testing.T) {
	now := testRunTime()
	run := runningRun(now)
	_, err := run.Heartbeat("secret-token", run.Version, now.Add(-time.Nanosecond), time.Minute)
	assertViolation(t, err, CodeInvalidStateTransition)
	next, err := run.Heartbeat("secret-token", run.Version, now.Add(time.Minute), time.Nanosecond)
	if err != nil {
		t.Fatal(err)
	}
	if !next.LeaseExpiresAt.After(run.LeaseExpiresAt) {
		t.Fatalf("heartbeat shortened or preserved lease: before=%v after=%v", run.LeaseExpiresAt, next.LeaseExpiresAt)
	}
}

func TestHeartbeatAndTerminalTransitionsShareVersionCAS(t *testing.T) {
	now := testRunTime()
	run := runningRun(now)
	heartbeated, err := run.Heartbeat("secret-token", run.Version, now.Add(time.Minute), time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = heartbeated.Terminate(RunSucceeded, run.Version, now.Add(2*time.Minute), "done")
	assertViolation(t, err, CodeStaleRunVersion)

	terminated, _, err := run.Terminate(RunSucceeded, run.Version, now.Add(time.Minute), "done")
	if err != nil {
		t.Fatal(err)
	}
	_, err = terminated.Heartbeat("secret-token", terminated.Version, now.Add(2*time.Minute), time.Minute)
	assertViolation(t, err, CodeInvalidStateTransition)
}

func TestRunHeartbeatRejectsInvalidCredentialsAndCAS(t *testing.T) {
	now := testRunTime()
	run := runningRun(now)
	_, err := run.Heartbeat("wrong-token", run.Version, now.Add(time.Minute), time.Minute)
	assertViolation(t, err, CodeRunLeaseMismatch)
	_, err = run.Heartbeat("secret-token", run.Version-1, now.Add(time.Minute), time.Minute)
	assertViolation(t, err, CodeStaleRunVersion)
	_, err = run.Heartbeat("secret-token", run.Version, run.LeaseExpiresAt, time.Minute)
	assertViolation(t, err, CodeInvalidStateTransition)
	_, err = run.Heartbeat("secret-token", run.Version, now.Add(time.Minute), 0)
	assertViolation(t, err, CodeInvalidStateTransition)
}

func TestExpiredInterruptionCompetesByRunVersion(t *testing.T) {
	now := testRunTime()
	run := runningRun(now)
	interrupted, outcome, err := run.InterruptExpired(run.Version, run.LeaseExpiresAt, "lease expired")
	if err != nil || outcome != RunTransitionApplied || interrupted.Status != RunInterrupted {
		t.Fatalf("expired run not interrupted: run=%+v outcome=%q err=%v", interrupted, outcome, err)
	}
	_, _, err = run.InterruptExpired(run.Version-1, run.LeaseExpiresAt, "lease expired")
	assertViolation(t, err, CodeStaleRunVersion)
	_, _, err = run.InterruptExpired(run.Version, run.LeaseExpiresAt.Add(-time.Nanosecond), "early")
	assertViolation(t, err, CodeInvalidStateTransition)
}

func TestHashLeaseTokenDoesNotExposeRawToken(t *testing.T) {
	hash := HashLeaseToken("secret-token")
	if hash == "" || hash == "secret-token" || hash != HashLeaseToken("secret-token") || hash == HashLeaseToken("different") {
		t.Fatalf("unexpected lease token hash: %q", hash)
	}
	if HashLeaseToken(" ") != "" {
		t.Fatal("blank token should not produce a usable hash")
	}
}

func testRunTime() time.Time {
	return time.Date(2026, 7, 18, 10, 0, 0, 0, time.UTC)
}
