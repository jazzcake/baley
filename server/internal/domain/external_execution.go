package domain

import (
	"strings"
	"time"
)

type ExternalExecutionStatus string

const (
	ExternalExecutionCreating ExternalExecutionStatus = "creating"
	ExternalExecutionActive   ExternalExecutionStatus = "active"
	ExternalExecutionReview   ExternalExecutionStatus = "review"
	ExternalExecutionSettled  ExternalExecutionStatus = "settled"
	ExternalExecutionLost     ExternalExecutionStatus = "lost"
)

var ExternalExecutionStatuses = []ExternalExecutionStatus{
	ExternalExecutionCreating, ExternalExecutionActive, ExternalExecutionReview,
	ExternalExecutionSettled, ExternalExecutionLost,
}

type ExternalExecutionSettlementReason string

const (
	ExternalExecutionCompleted            ExternalExecutionSettlementReason = "completed"
	ExternalExecutionAbandoned            ExternalExecutionSettlementReason = "abandoned"
	ExternalExecutionRejected             ExternalExecutionSettlementReason = "rejected"
	ExternalExecutionSuperseded           ExternalExecutionSettlementReason = "superseded"
	ExternalExecutionCreationFailed       ExternalExecutionSettlementReason = "creation_failed"
	ExternalExecutionDeletedAfterRecovery ExternalExecutionSettlementReason = "external_deleted_after_recovery"
)

var ExternalExecutionSettlementReasons = []ExternalExecutionSettlementReason{
	ExternalExecutionCompleted, ExternalExecutionAbandoned, ExternalExecutionRejected,
	ExternalExecutionSuperseded, ExternalExecutionCreationFailed, ExternalExecutionDeletedAfterRecovery,
}

type ExternalExecution struct {
	ID, WorkspaceID, TaskID, Provider, ExternalID, ProviderInstanceID, HostID string
	Status                                                                    ExternalExecutionStatus
	AttemptNumber                                                             int
	ClientExecutionID, ContextSnapshotHash, LastTerminalHandle                string
	StartedAt                                                                 time.Time
	LastObservedAt, SettledAt                                                 *time.Time
	SettlementReason                                                          ExternalExecutionSettlementReason
	CreatedByActorID                                                          string
}

func (e ExternalExecution) IsOpen() bool { return e.Status != ExternalExecutionSettled }

func ReserveExternalExecution(id, workspaceID, taskID, clientExecutionID, provider, contextSnapshotHash, actorID string, attempt int, now time.Time) (ExternalExecution, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if id == "" || workspaceID == "" || taskID == "" || clientExecutionID == "" || provider != "orca" || actorID == "" || attempt <= 0 || now.IsZero() {
		return ExternalExecution{}, &Violation{Code: CodeInvalidStateTransition}
	}
	return ExternalExecution{ID: id, WorkspaceID: workspaceID, TaskID: taskID, Provider: provider, Status: ExternalExecutionCreating, AttemptNumber: attempt, ClientExecutionID: clientExecutionID, ContextSnapshotHash: strings.TrimSpace(contextSnapshotHash), StartedAt: now, CreatedByActorID: actorID}, nil
}

func (e ExternalExecution) Attach(externalID, instanceID, hostID, terminalHandle string, now time.Time) (ExternalExecution, error) {
	if e.Status != ExternalExecutionCreating || strings.TrimSpace(externalID) == "" || strings.TrimSpace(instanceID) == "" || strings.TrimSpace(hostID) == "" || now.IsZero() {
		return e, &Violation{Code: CodeExternalExecutionInvalidTransition}
	}
	e.ExternalID, e.ProviderInstanceID, e.HostID = strings.TrimSpace(externalID), strings.TrimSpace(instanceID), strings.TrimSpace(hostID)
	e.LastTerminalHandle = strings.TrimSpace(terminalHandle)
	e.Status, e.LastObservedAt = ExternalExecutionActive, timePointer(now)
	return e, nil
}

func (e ExternalExecution) MarkReview(now time.Time) (ExternalExecution, error) {
	if e.Status != ExternalExecutionActive || now.IsZero() {
		return e, &Violation{Code: CodeExternalExecutionInvalidTransition}
	}
	e.Status, e.LastObservedAt = ExternalExecutionReview, timePointer(now)
	return e, nil
}

func (e ExternalExecution) MarkLost(now time.Time) (ExternalExecution, error) {
	if (e.Status != ExternalExecutionCreating && e.Status != ExternalExecutionActive && e.Status != ExternalExecutionReview) || now.IsZero() {
		return e, &Violation{Code: CodeExternalExecutionInvalidTransition}
	}
	e.Status, e.LastObservedAt = ExternalExecutionLost, timePointer(now)
	return e, nil
}

func (e ExternalExecution) Reconnect(externalID, instanceID, hostID, terminalHandle string, now time.Time) (ExternalExecution, error) {
	if e.Status != ExternalExecutionLost || strings.TrimSpace(externalID) == "" || strings.TrimSpace(instanceID) == "" || strings.TrimSpace(hostID) == "" || now.IsZero() {
		return e, &Violation{Code: CodeExternalExecutionInvalidTransition}
	}
	e.ExternalID, e.ProviderInstanceID, e.HostID = strings.TrimSpace(externalID), strings.TrimSpace(instanceID), strings.TrimSpace(hostID)
	e.LastTerminalHandle, e.Status, e.LastObservedAt = strings.TrimSpace(terminalHandle), ExternalExecutionActive, timePointer(now)
	return e, nil
}

func (e ExternalExecution) Settle(reason ExternalExecutionSettlementReason, now time.Time) (ExternalExecution, error) {
	if !e.IsOpen() || !validSettlementReason(reason) || now.IsZero() {
		return e, &Violation{Code: CodeExternalExecutionInvalidTransition}
	}
	e.Status, e.SettlementReason, e.SettledAt = ExternalExecutionSettled, reason, timePointer(now)
	return e, nil
}

func ValidateExternalExecutionLock(existing []ExternalExecution, taskID, provider string) Evaluation {
	evaluation := Evaluation{}
	for _, execution := range existing {
		if execution.TaskID == taskID && execution.Provider == provider && execution.IsOpen() {
			evaluation.Errors = append(evaluation.Errors, Diagnostic{Code: CodeExternalExecutionAlreadyActive, EntityID: execution.ID})
		}
	}
	evaluation.sort()
	return evaluation
}

func validSettlementReason(reason ExternalExecutionSettlementReason) bool {
	for _, value := range ExternalExecutionSettlementReasons {
		if value == reason {
			return true
		}
	}
	return false
}
