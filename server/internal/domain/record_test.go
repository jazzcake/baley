package domain

import (
	"strings"
	"testing"
)

func TestTaskRecordRegistrationAndIdempotency(t *testing.T) {
	input := validRecordRegistration()
	record, err := NewTaskRecord("task-records", input, nil)
	if err != nil {
		t.Fatal(err)
	}
	if record.ID != input.ID || record.RelativePath != input.RelativePath || record.State != RecordReportedUncommitted {
		t.Fatalf("unexpected record: %+v", record)
	}
	if err := CompareRecordRegistration("task-records", record, input); err != nil {
		t.Fatalf("same registration rejected: %v", err)
	}
	upperHash := input
	upperHash.WorkingTreeHash = strings.ToUpper(input.WorkingTreeHash)
	if err := CompareRecordRegistration("task-records", record, upperHash); err != nil {
		t.Fatalf("same digest case variant rejected: %v", err)
	}

	changedHash := input
	changedHash.WorkingTreeHash = "sha256:" + repeatHex("b", 64)
	assertViolation(t, CompareRecordRegistration("task-records", record, changedHash), CodeRecordHashConflict)
	changedPath := input
	changedPath.RelativePath = "task-records/task-1/other.md"
	assertViolation(t, CompareRecordRegistration("task-records", record, changedPath), CodeRecordHashConflict)
	changedTask := input
	changedTask.TaskID = "other"
	assertViolation(t, CompareRecordRegistration("task-records", record, changedTask), CodeIdempotencyConflict)
	for name, mutate := range map[string]func(*RecordRegistration){
		"repository": func(value *RecordRegistration) { value.RepositoryID = "other" },
		"run":        func(value *RecordRegistration) { value.RunID = "other" },
		"type":       func(value *RecordRegistration) { value.Type = RecordHandoff },
		"summary":    func(value *RecordRegistration) { value.ShortSummary = "other" },
		"supersedes": func(value *RecordRegistration) { value.SupersedesRecordID = "other" },
	} {
		t.Run(name, func(t *testing.T) {
			changed := input
			mutate(&changed)
			assertViolation(t, CompareRecordRegistration("task-records", record, changed), CodeIdempotencyConflict)
		})
	}
}

func TestTaskRecordRejectsInvalidRegistration(t *testing.T) {
	input := validRecordRegistration()
	input.SupersedesRecordID = input.ID
	_, err := NewTaskRecord("task-records", input, nil)
	assertViolation(t, err, CodeInvalidStateTransition)
	input = validRecordRegistration()
	input.RelativePath = "/tmp/report.md"
	_, err = NewTaskRecord("task-records", input, nil)
	assertViolation(t, err, CodeInvalidRecordPath)
	input = validRecordRegistration()
	input.WorkingTreeHash = "sha256:short"
	_, err = NewTaskRecord("task-records", input, nil)
	assertViolation(t, err, CodeInvalidStateTransition)
}

func TestTaskRecordCommitLifecycle(t *testing.T) {
	record, _ := NewTaskRecord("task-records", validRecordRegistration(), nil)
	commit, blob := repeatHex("a", 40), repeatHex("b", 40)
	attached, outcome, err := record.AttachCommit(commit, blob)
	if err != nil || outcome != RunTransitionApplied || attached.State != RecordCommittedUnverified {
		t.Fatalf("attach failed: %+v %q %v", attached, outcome, err)
	}
	same, outcome, err := attached.AttachCommit(commit, blob)
	if err != nil || outcome != RunTransitionIdempotent || same != attached {
		t.Fatalf("retry not idempotent: %+v %q %v", same, outcome, err)
	}
	_, _, err = attached.AttachCommit(repeatHex("c", 40), blob)
	assertViolation(t, err, CodeRecordHashConflict)
	_, _, err = record.AttachCommit(repeatHex("a", 40), repeatHex("b", 64))
	assertViolation(t, err, CodeInvalidStateTransition)
	_, err = record.MarkVerified()
	assertViolation(t, err, CodeInvalidStateTransition)
	invalidState := record
	invalidState.State = RecordVerified
	_, _, err = invalidState.AttachCommit(commit, blob)
	assertViolation(t, err, CodeInvalidStateTransition)
	verified, err := attached.MarkVerified()
	if err != nil || verified.State != RecordVerified {
		t.Fatalf("verify failed: %+v %v", verified, err)
	}
}

func TestTaskRecordSupersessionRequiresValidSameTaskChain(t *testing.T) {
	baseInput := validRecordRegistration()
	baseInput.ID = "base"
	base, _ := NewTaskRecord("task-records", baseInput, nil)
	next := validRecordRegistration()
	next.ID = "next"
	next.SupersedesRecordID = "base"
	if _, err := NewTaskRecord("task-records", next, map[string]TaskRecord{"base": base}); err != nil {
		t.Fatalf("valid supersession rejected: %v", err)
	}
	_, err := NewTaskRecord("task-records", next, nil)
	assertViolation(t, err, CodeInvalidStateTransition)
	wrong := base
	wrong.TaskID = "other"
	_, err = NewTaskRecord("task-records", next, map[string]TaskRecord{"base": wrong})
	assertViolation(t, err, CodeInvalidStateTransition)
	cycle := base
	cycle.SupersedesRecordID = "next"
	_, err = NewTaskRecord("task-records", next, map[string]TaskRecord{"base": cycle, "next": {ID: "next"}})
	assertViolation(t, err, CodeInvalidStateTransition)
}

func validRecordRegistration() RecordRegistration {
	return RecordRegistration{ID: "record", WorkspaceID: "workspace", TaskID: "task", RunID: "run", Type: RecordCompletionReport, RepositoryID: "repository", RelativePath: "task-records/task-1/report.md", WorkingTreeHash: "sha256:" + repeatHex("a", 64), ShortSummary: " completion "}
}
func repeatHex(value string, count int) string {
	result := ""
	for len(result) < count {
		result += value
	}
	return result
}
