package domain

import (
	"strings"
	"time"
)

type TaskStatus string

const (
	TaskPending     TaskStatus = "pending"
	TaskInProgress  TaskStatus = "in_progress"
	TaskImplemented TaskStatus = "implemented"
	TaskConfirmed   TaskStatus = "confirmed"
	TaskDiscarded   TaskStatus = "discarded"
)

var TaskStatuses = []TaskStatus{TaskPending, TaskInProgress, TaskImplemented, TaskConfirmed, TaskDiscarded}

type Task struct {
	ID                    string
	WorkspaceID           string
	LaneID                string
	PhasePosition         int
	Status                TaskStatus
	BlockedAt             *time.Time
	BlockerReason         string
	TerminalReason        string
	ImplementedAssessment string
}

func (t Task) Start() (Task, error) {
	return t.transition(TaskInProgress, t.Status == TaskPending)
}

func (t Task) ReportImplemented(assessment string) (Task, error) {
	if t.BlockedAt != nil {
		return t, &Violation{Code: CodeBlockedTask}
	}
	if strings.TrimSpace(assessment) == "" {
		return t, &Violation{Code: CodeInvalidStateTransition}
	}
	next, err := t.transition(TaskImplemented, t.Status == TaskInProgress)
	if err != nil {
		return t, err
	}
	next.ImplementedAssessment = strings.TrimSpace(assessment)
	return next, nil
}

func (t Task) Confirm() (Task, error) {
	return t.transition(TaskConfirmed, t.Status == TaskImplemented && t.BlockedAt == nil)
}

func (t Task) Discard(reason string) (Task, error) {
	valid := t.Status == TaskPending || t.Status == TaskInProgress || t.Status == TaskImplemented
	if strings.TrimSpace(reason) == "" {
		valid = false
	}
	next, err := t.transition(TaskDiscarded, valid)
	if err != nil {
		return t, err
	}
	next.BlockedAt = nil
	next.BlockerReason = ""
	return next, nil
}

func (t Task) Rework(reason string) (Task, error) {
	valid := t.Status == TaskImplemented && strings.TrimSpace(reason) != ""
	next, err := t.transition(TaskInProgress, valid)
	if err != nil {
		return t, err
	}
	next.ImplementedAssessment = ""
	return next, nil
}

func (t Task) Block(now time.Time, reason string) (Task, error) {
	if (t.Status != TaskPending && t.Status != TaskInProgress) || t.BlockedAt != nil || strings.TrimSpace(reason) == "" {
		return t, &Violation{Code: CodeInvalidStateTransition}
	}
	next := t
	next.BlockedAt = &now
	next.BlockerReason = strings.TrimSpace(reason)
	return next, nil
}

func (t Task) Unblock(reason string) (Task, error) {
	if t.BlockedAt == nil || strings.TrimSpace(reason) == "" || (t.Status != TaskPending && t.Status != TaskInProgress) {
		return t, &Violation{Code: CodeInvalidStateTransition}
	}
	next := t
	next.BlockedAt = nil
	next.BlockerReason = ""
	return next, nil
}

func (t Task) transition(status TaskStatus, valid bool) (Task, error) {
	if !valid {
		return t, &Violation{Code: CodeInvalidStateTransition}
	}
	next := t
	next.Status = status
	return next, nil
}
