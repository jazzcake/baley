package domain

import (
	"sort"
	"strings"
)

type TaskDecision struct {
	Action                    string
	TaskID                    string
	ExpectedWorkspaceRevision int64
	Warnings                  []Diagnostic
}

type TaskImplementedPlan struct {
	Task       Task
	Evaluation Evaluation
	Event      PlannedEvent
	Decision   TaskDecision
}

type WarningAcknowledgement struct {
	Codes         []string
	ProceedReason string
	Enforce       bool
}

func PlanTaskReportImplemented(task Task, assessment string, records []TaskRecord, dangling bool, resultingWorkspaceRevision int64, acknowledgement WarningAcknowledgement) TaskImplementedPlan {
	plan := TaskImplementedPlan{Task: task, Evaluation: Evaluation{}}
	if resultingWorkspaceRevision <= 0 {
		plan.Evaluation.Errors = append(plan.Evaluation.Errors, Diagnostic{Code: CodeInvalidStateTransition, EntityID: task.WorkspaceID})
		return plan
	}
	implemented, err := task.ReportImplemented(assessment)
	if err != nil {
		plan.Evaluation.Errors = append(plan.Evaluation.Errors, Diagnostic{Code: violationCode(err), EntityID: task.ID})
		plan.Evaluation.sort()
		return plan
	}
	present := map[RecordType]bool{}
	for _, record := range records {
		if record.WorkspaceID == task.WorkspaceID && record.TaskID == task.ID {
			present[record.Type] = true
		}
	}
	for _, required := range []struct {
		typeValue RecordType
		code      string
	}{
		{RecordDetailedPlan, CodeMissingDetailedPlan},
		{RecordIndependentReview, CodeMissingIndependentReview},
		{RecordCompletionReport, CodeMissingCompletionReport},
	} {
		if !present[required.typeValue] {
			plan.Evaluation.Warnings = append(plan.Evaluation.Warnings, Diagnostic{Code: required.code, EntityID: task.ID})
		}
	}
	if dangling {
		plan.Evaluation.Warnings = append(plan.Evaluation.Warnings, Diagnostic{Code: CodeDanglingPath, EntityID: task.ID})
	}
	plan.Evaluation.sort()
	warningCodes := make([]string, 0, len(plan.Evaluation.Warnings))
	warningSet := make(map[string]bool, len(plan.Evaluation.Warnings))
	for _, warning := range plan.Evaluation.Warnings {
		warningCodes = append(warningCodes, warning.Code)
		warningSet[warning.Code] = true
	}
	acknowledged := make(map[string]bool, len(acknowledgement.Codes))
	for _, code := range acknowledgement.Codes {
		if acknowledged[code] {
			plan.Evaluation.Errors = append(plan.Evaluation.Errors, Diagnostic{Code: CodeInvalidStateTransition, EntityID: code})
			continue
		}
		if !warningSet[code] {
			plan.Evaluation.Errors = append(plan.Evaluation.Errors, Diagnostic{Code: CodeInvalidStateTransition, EntityID: code})
			continue
		}
		acknowledged[code] = true
	}
	if acknowledgement.Enforce {
		for _, code := range warningCodes {
			if !acknowledged[code] {
				plan.Evaluation.Errors = append(plan.Evaluation.Errors, Diagnostic{Code: CodeInvalidStateTransition, EntityID: code})
			}
		}
	}
	plan.Evaluation.sort()
	if plan.Evaluation.HasErrors() {
		return plan
	}
	acknowledgedCodes := append([]string{}, acknowledgement.Codes...)
	sort.Strings(acknowledgedCodes)
	plan.Task = implemented
	plan.Event = PlannedEvent{
		Type:       "task.implemented_reported",
		EntityType: "task",
		EntityID:   task.ID,
		Payload: map[string]any{
			"taskId":                   task.ID,
			"assessment":               implemented.ImplementedAssessment,
			"warnings":                 warningCodes,
			"acknowledgedWarningCodes": acknowledgedCodes,
			"proceedReason":            strings.TrimSpace(acknowledgement.ProceedReason),
		},
	}
	plan.Decision = TaskDecision{
		Action:                    "task.confirm",
		TaskID:                    task.ID,
		ExpectedWorkspaceRevision: resultingWorkspaceRevision,
		Warnings:                  append([]Diagnostic(nil), plan.Evaluation.Warnings...),
	}
	return plan
}
