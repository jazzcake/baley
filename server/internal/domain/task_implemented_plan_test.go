package domain

import (
	"testing"
	"time"
)

func TestPlanTaskReportImplementedProjectsWarningsEventAndDecision(t *testing.T) {
	task := Task{ID: "task", WorkspaceID: "workspace", Status: TaskInProgress}
	records := []TaskRecord{
		{ID: "plan", WorkspaceID: "workspace", TaskID: "task", Type: RecordDetailedPlan},
		{ID: "other-task-report", WorkspaceID: "workspace", TaskID: "other", Type: RecordCompletionReport},
	}
	plan := PlanTaskReportImplemented(task, " verified implementation ", records, true, 42, WarningAcknowledgement{
		Codes:         []string{CodeMissingIndependentReview, CodeMissingCompletionReport, CodeDanglingPath},
		ProceedReason: "review will be attached before confirmation",
	})
	if plan.Evaluation.HasErrors() || plan.Task.Status != TaskImplemented || plan.Task.ImplementedAssessment != "verified implementation" {
		t.Fatalf("unexpected plan: %+v", plan)
	}
	for _, code := range []string{CodeMissingIndependentReview, CodeMissingCompletionReport, CodeDanglingPath} {
		if !hasDiagnostic(plan.Evaluation.Warnings, code) {
			t.Errorf("missing warning %s: %+v", code, plan.Evaluation.Warnings)
		}
	}
	if hasDiagnostic(plan.Evaluation.Warnings, CodeMissingDetailedPlan) {
		t.Fatal("present detailed plan reported missing")
	}
	if plan.Event.Type != "task.implemented_reported" || plan.Decision.Action != "task.confirm" || plan.Decision.TaskID != task.ID || plan.Decision.ExpectedWorkspaceRevision != 42 {
		t.Fatalf("event or decision missing: %+v", plan)
	}
}

func TestPlanTaskReportImplementedRecordCombinations(t *testing.T) {
	task := Task{ID: "task", WorkspaceID: "workspace", Status: TaskInProgress}
	all := []TaskRecord{
		{WorkspaceID: "workspace", TaskID: "task", Type: RecordDetailedPlan},
		{WorkspaceID: "workspace", TaskID: "task", Type: RecordIndependentReview},
		{WorkspaceID: "workspace", TaskID: "task", Type: RecordCompletionReport},
	}
	plan := PlanTaskReportImplemented(task, "done", all, false, 7, WarningAcknowledgement{})
	if len(plan.Evaluation.Warnings) != 0 {
		t.Fatalf("complete records produced warnings: %+v", plan.Evaluation.Warnings)
	}
	if len(plan.Decision.Warnings) != 0 {
		t.Fatalf("decision warning snapshot differs: %+v", plan.Decision)
	}
}

func TestPlanTaskReportImplementedRejectsMissingAssessmentAndBlocker(t *testing.T) {
	task := Task{ID: "task", WorkspaceID: "workspace", Status: TaskInProgress}
	plan := PlanTaskReportImplemented(task, " ", nil, false, 1, WarningAcknowledgement{})
	if !hasDiagnostic(plan.Evaluation.Errors, CodeInvalidStateTransition) || plan.Event.Type != "" || plan.Decision.Action != "" || plan.Task.Status != TaskInProgress {
		t.Fatalf("missing assessment accepted or mutated: %+v", plan)
	}
	now := time.Now()
	task.BlockedAt, task.BlockerReason = &now, "waiting"
	plan = PlanTaskReportImplemented(task, "done", nil, false, 1, WarningAcknowledgement{})
	if !hasDiagnostic(plan.Evaluation.Errors, CodeBlockedTask) || plan.Task.Status != TaskInProgress {
		t.Fatalf("blocked Task accepted: %+v", plan)
	}
}

func TestPlanTaskReportImplementedDecisionBindsRevision(t *testing.T) {
	task := Task{ID: "task", WorkspaceID: "workspace", Status: TaskInProgress}
	acknowledgement := WarningAcknowledgement{
		Codes:         []string{CodeMissingDetailedPlan, CodeMissingIndependentReview, CodeMissingCompletionReport},
		ProceedReason: "records are being prepared",
	}
	first := PlanTaskReportImplemented(task, "done", nil, false, 10, acknowledgement)
	second := PlanTaskReportImplemented(task, "done", nil, false, 11, acknowledgement)
	if first.Decision.ExpectedWorkspaceRevision == second.Decision.ExpectedWorkspaceRevision {
		t.Fatal("decision is not revision-bound")
	}
}

func TestPlanTaskReportImplementedRequiresCompleteWarningAcknowledgement(t *testing.T) {
	task := Task{ID: "task", WorkspaceID: "workspace", Status: TaskInProgress}
	plan := PlanTaskReportImplemented(task, "done", nil, false, 1, WarningAcknowledgement{
		Codes:         []string{CodeMissingDetailedPlan},
		ProceedReason: "partial acknowledgement",
		Enforce:       true,
	})
	if !plan.Evaluation.HasErrors() || plan.Task.Status != TaskInProgress || plan.Event.Type != "" || plan.Decision.Action != "" {
		t.Fatalf("partial acknowledgement allowed mutation: %+v", plan)
	}
}

func TestPlanTaskReportImplementedProceedReasonIsOptional(t *testing.T) {
	task := Task{ID: "task", WorkspaceID: "workspace", Status: TaskInProgress}
	plan := PlanTaskReportImplemented(task, "done", nil, false, 2, WarningAcknowledgement{
		Codes:   []string{CodeMissingDetailedPlan, CodeMissingIndependentReview, CodeMissingCompletionReport},
		Enforce: true,
	})
	if plan.Evaluation.HasErrors() || plan.Task.Status != TaskImplemented {
		t.Fatalf("optional proceed reason was required: %+v", plan)
	}
}
