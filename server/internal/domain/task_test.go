package domain

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestTaskAllowedLifecycle(t *testing.T) {
	task := Task{ID: "task", Status: TaskPending}
	var err error
	if task, err = task.Start(); err != nil || task.Status != TaskInProgress {
		t.Fatalf("start: task=%+v err=%v", task, err)
	}
	if task, err = task.ReportImplemented("verified"); err != nil || task.Status != TaskImplemented {
		t.Fatalf("report: task=%+v err=%v", task, err)
	}
	if task, err = task.Rework("review changes"); err != nil || task.Status != TaskInProgress {
		t.Fatalf("rework: task=%+v err=%v", task, err)
	}
	if task, err = task.ReportImplemented("verified again"); err != nil {
		t.Fatal(err)
	}
	if task, err = task.Confirm(); err != nil || task.Status != TaskConfirmed {
		t.Fatalf("confirm: task=%+v err=%v", task, err)
	}

	for _, status := range []TaskStatus{TaskPending, TaskInProgress, TaskImplemented} {
		got, err := (Task{Status: status}).Discard("no longer needed")
		if err != nil || got.Status != TaskDiscarded {
			t.Errorf("discard from %s: task=%+v err=%v", status, got, err)
		}
	}
}

func TestTaskUpdateChangesOnlyRequestedContentAndRejectsTerminalTasks(t *testing.T) {
	description := "updated description"
	summary := "People can see the outcome first."
	original := Task{ID: "task", Title: "original title", Description: "original description", CurrentSummary: "keep", NextAction: "keep", Status: TaskPending}
	updated, err := original.Update(nil, &description, &summary)
	if err != nil || updated.Title != original.Title || updated.Description != description || updated.CurrentSummary != summary || updated.NextAction != original.NextAction {
		t.Fatalf("description-only update = %+v, %v", updated, err)
	}
	if _, err = original.Update(nil, nil, nil); err == nil {
		t.Fatal("empty content update accepted")
	}
	for _, status := range []TaskStatus{TaskConfirmed, TaskDiscarded} {
		if _, err = (Task{ID: "task", Title: "original", Status: status}).Update(&description, nil, nil); err == nil {
			t.Fatalf("terminal %s update accepted", status)
		}
	}
}

func TestTaskRejectsUnlistedTransitionsWithoutMutation(t *testing.T) {
	tests := []struct {
		name      string
		task      Task
		operation func(Task) (Task, error)
	}{
		{"start-in-progress", Task{Status: TaskInProgress}, func(v Task) (Task, error) { return v.Start() }},
		{"report-pending", Task{Status: TaskPending}, func(v Task) (Task, error) { return v.ReportImplemented("assessment") }},
		{"confirm-pending", Task{Status: TaskPending}, func(v Task) (Task, error) { return v.Confirm() }},
		{"rework-pending", Task{Status: TaskPending}, func(v Task) (Task, error) { return v.Rework("reason") }},
		{"discard-confirmed", Task{Status: TaskConfirmed}, func(v Task) (Task, error) { return v.Discard("reason") }},
		{"discard-discarded", Task{Status: TaskDiscarded}, func(v Task) (Task, error) { return v.Discard("reason") }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := test.operation(test.task)
			assertViolation(t, err, CodeInvalidStateTransition)
			if !reflect.DeepEqual(got, test.task) {
				t.Fatalf("failed transition mutated task: before=%+v after=%+v", test.task, got)
			}
		})
	}
}

func TestTaskRequiresAssessmentAndReasons(t *testing.T) {
	_, err := (Task{Status: TaskInProgress}).ReportImplemented(" ")
	assertViolation(t, err, CodeInvalidStateTransition)
	_, err = (Task{Status: TaskPending}).Discard("")
	assertViolation(t, err, CodeInvalidStateTransition)
	_, err = (Task{Status: TaskImplemented}).Rework("")
	assertViolation(t, err, CodeInvalidStateTransition)
}

func TestTaskBlockerLifecycle(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	for _, status := range []TaskStatus{TaskPending, TaskInProgress} {
		original := Task{ID: "task", Status: status}
		blocked, err := original.Block(now, "waiting for input")
		if err != nil || blocked.BlockedAt == nil || blocked.BlockerReason != "waiting for input" {
			t.Fatalf("block %s: task=%+v err=%v", status, blocked, err)
		}
		unblocked, err := blocked.Unblock("input arrived")
		if err != nil || unblocked.BlockedAt != nil || unblocked.BlockerReason != "" {
			t.Fatalf("unblock %s: task=%+v err=%v", status, unblocked, err)
		}
	}
	for _, status := range []TaskStatus{TaskImplemented, TaskConfirmed, TaskDiscarded} {
		_, err := (Task{Status: status}).Block(now, "reason")
		assertViolation(t, err, CodeInvalidStateTransition)
	}
	_, err := (Task{Status: TaskPending}).Block(now, " ")
	assertViolation(t, err, CodeInvalidStateTransition)
	_, err = (Task{Status: TaskPending}).Unblock("reason")
	assertViolation(t, err, CodeInvalidStateTransition)
}

func TestBlockedTaskCannotReportImplemented(t *testing.T) {
	now := time.Now()
	task := Task{Status: TaskInProgress, BlockedAt: &now, BlockerReason: "blocked"}
	got, err := task.ReportImplemented("done")
	assertViolation(t, err, CodeBlockedTask)
	if !reflect.DeepEqual(got, task) {
		t.Fatal("failed report mutated task")
	}
}

func TestDiscardClearsBlockerMetadata(t *testing.T) {
	now := time.Now()
	task := Task{Status: TaskInProgress, BlockedAt: &now, BlockerReason: "obsolete blocker"}
	discarded, err := task.Discard("work cancelled")
	if err != nil {
		t.Fatal(err)
	}
	if discarded.Status != TaskDiscarded || discarded.BlockedAt != nil || discarded.BlockerReason != "" {
		t.Fatalf("discarded task retained blocker: %+v", discarded)
	}
}

func assertViolation(t *testing.T, err error, code string) {
	t.Helper()
	var violation *Violation
	if !errors.As(err, &violation) || violation.Code != code {
		t.Fatalf("expected %s, got %v", code, err)
	}
}
