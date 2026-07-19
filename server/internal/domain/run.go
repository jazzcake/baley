package domain

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"strings"
	"time"
)

type RunStatus string

const (
	RunRunning     RunStatus = "running"
	RunSucceeded   RunStatus = "succeeded"
	RunFailed      RunStatus = "failed"
	RunInterrupted RunStatus = "interrupted"
	RunCancelled   RunStatus = "cancelled"
)

var RunStatuses = []RunStatus{RunRunning, RunSucceeded, RunFailed, RunInterrupted, RunCancelled}
var RunTerminalStatuses = []RunStatus{RunSucceeded, RunFailed, RunInterrupted, RunCancelled}

type Run struct {
	ID              string
	WorkspaceID     string
	TaskID          string
	ClientRunID     string
	Kind            RunKind
	Status          RunStatus
	OperatorActorID string
	SessionRef      string
	ParentRunID     string
	TargetRunID     string
	LeaseTokenHash  string
	HeartbeatAt     time.Time
	LeaseExpiresAt  time.Time
	Version         int64
	StartedAt       time.Time
	EndedAt         *time.Time
	ResultSummary   string
	ErrorSummary    string
}

type RunTransitionOutcome string

const (
	RunTransitionApplied    RunTransitionOutcome = "applied"
	RunTransitionIdempotent RunTransitionOutcome = "idempotent"
)

type RunCorrection struct {
	PreviousStatus        RunStatus
	PreviousResultSummary string
	PreviousErrorSummary  string
	PreviousEndedAt       *time.Time
	NewStatus             RunStatus
	NewResultSummary      string
	NewErrorSummary       string
	NewEndedAt            *time.Time
	Reason                string
}

func (r Run) Terminate(status RunStatus, expectedVersion int64, now time.Time, summary string) (Run, RunTransitionOutcome, error) {
	resultSummary, errorSummary, err := terminalSummaries(status, summary)
	if err != nil {
		return r, "", err
	}
	if isTerminalRunStatus(r.Status) {
		if r.Status == status && r.ResultSummary == resultSummary && r.ErrorSummary == errorSummary {
			return r, RunTransitionIdempotent, nil
		}
		return r, "", &Violation{Code: CodeIdempotencyConflict}
	}
	if r.Status != RunRunning {
		return r, "", &Violation{Code: CodeInvalidStateTransition}
	}
	if r.Version != expectedVersion {
		return r, "", &Violation{Code: CodeStaleRunVersion}
	}
	next := r
	next.Status = status
	next.ResultSummary = resultSummary
	next.ErrorSummary = errorSummary
	next.EndedAt = timePointer(now)
	next.Version++
	return next, RunTransitionApplied, nil
}

func (r Run) CorrectTerminal(status RunStatus, expectedVersion int64, now time.Time, summary, reason string) (Run, RunCorrection, error) {
	trimmedReason := strings.TrimSpace(reason)
	if !isTerminalRunStatus(r.Status) || trimmedReason == "" {
		return r, RunCorrection{}, &Violation{Code: CodeInvalidStateTransition}
	}
	if r.Version != expectedVersion {
		return r, RunCorrection{}, &Violation{Code: CodeStaleRunVersion}
	}
	resultSummary, errorSummary, err := terminalSummaries(status, summary)
	if err != nil {
		return r, RunCorrection{}, err
	}
	correction := RunCorrection{
		PreviousStatus:        r.Status,
		PreviousResultSummary: r.ResultSummary,
		PreviousErrorSummary:  r.ErrorSummary,
		PreviousEndedAt:       r.EndedAt,
		NewStatus:             status,
		NewResultSummary:      resultSummary,
		NewErrorSummary:       errorSummary,
		NewEndedAt:            timePointer(now),
		Reason:                trimmedReason,
	}
	next := r
	next.Status = status
	next.ResultSummary = resultSummary
	next.ErrorSummary = errorSummary
	next.EndedAt = timePointer(now)
	next.Version++
	return next, correction, nil
}

func (r Run) Heartbeat(rawToken string, expectedVersion int64, now time.Time, extension time.Duration) (Run, error) {
	if r.Status != RunRunning || extension <= 0 || now.Before(r.HeartbeatAt) || !now.Before(r.LeaseExpiresAt) {
		return r, &Violation{Code: CodeInvalidStateTransition}
	}
	if r.Version != expectedVersion {
		return r, &Violation{Code: CodeStaleRunVersion}
	}
	if !leaseTokenMatches(r.LeaseTokenHash, rawToken) {
		return r, &Violation{Code: CodeRunLeaseMismatch}
	}
	next := r
	next.HeartbeatAt = now
	next.LeaseExpiresAt = r.LeaseExpiresAt.Add(extension)
	next.Version++
	return next, nil
}

func (r Run) IsLeaseExpired(now time.Time) bool {
	return r.Status == RunRunning && !now.Before(r.LeaseExpiresAt)
}

func (r Run) InterruptExpired(expectedVersion int64, now time.Time, summary string) (Run, RunTransitionOutcome, error) {
	if !r.IsLeaseExpired(now) {
		return r, "", &Violation{Code: CodeInvalidStateTransition}
	}
	return r.Terminate(RunInterrupted, expectedVersion, now, summary)
}

func HashLeaseToken(rawToken string) string {
	if strings.TrimSpace(rawToken) == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(rawToken))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func terminalSummaries(status RunStatus, summary string) (string, string, error) {
	trimmed := strings.TrimSpace(summary)
	if !isTerminalRunStatus(status) || trimmed == "" {
		return "", "", &Violation{Code: CodeInvalidStateTransition}
	}
	if status == RunSucceeded {
		return trimmed, "", nil
	}
	return "", trimmed, nil
}

func isTerminalRunStatus(status RunStatus) bool {
	for _, candidate := range RunTerminalStatuses {
		if status == candidate {
			return true
		}
	}
	return false
}

func leaseTokenMatches(expectedHash, rawToken string) bool {
	actualHash := HashLeaseToken(rawToken)
	if expectedHash == "" || actualHash == "" || len(expectedHash) != len(actualHash) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expectedHash), []byte(actualHash)) == 1
}

func timePointer(value time.Time) *time.Time { return &value }
