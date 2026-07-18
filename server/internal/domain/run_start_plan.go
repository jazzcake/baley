package domain

import (
	"strings"
	"time"
)

type RunStartRequest struct {
	RunID           string
	Identity        RunStartIdentity
	OperatorActorID string
	SessionRef      string
	LeaseToken      string
	LeaseDuration   time.Duration
	Now             time.Time
	TargetRun       *Run
}

type PlannedEvent struct {
	Type       string
	EntityType string
	EntityID   string
	Payload    map[string]any
}

type RunStartPlan struct {
	Task   Task
	Run    Run
	Events []PlannedEvent
}

func PlanRunStart(workspaceState WorkspaceState, task Task, phaseState PhaseState, predecessors []Task, request RunStartRequest) (RunStartPlan, Evaluation) {
	evaluation := EvaluateRunStart(task, request.Identity.Kind, phaseState, predecessors)
	if workspaceState != WorkspaceActive {
		evaluation.Errors = append(evaluation.Errors, Diagnostic{Code: CodeInvalidStateTransition, EntityID: request.Identity.WorkspaceID})
	}
	if strings.TrimSpace(request.Identity.ClientRunID) == "" {
		evaluation.Errors = append(evaluation.Errors, Diagnostic{Code: CodeInvalidStateTransition, EntityID: request.Identity.ClientRunID})
	}
	if request.Identity.WorkspaceID != task.WorkspaceID || request.Identity.TaskID != task.ID {
		evaluation.Errors = append(evaluation.Errors, Diagnostic{Code: CodeInvalidStateTransition, EntityID: task.ID})
	}
	if strings.TrimSpace(request.RunID) == "" || strings.TrimSpace(request.OperatorActorID) == "" || strings.TrimSpace(request.LeaseToken) == "" || request.LeaseDuration <= 0 || request.Now.IsZero() {
		evaluation.Errors = append(evaluation.Errors, Diagnostic{Code: CodeInvalidStateTransition, EntityID: request.RunID})
	}
	if task.Status == TaskConfirmed || task.Status == TaskDiscarded {
		evaluation.Errors = append(evaluation.Errors, Diagnostic{Code: CodeInvalidStateTransition, EntityID: task.ID})
	}
	if request.Identity.Kind == RunIndependentAgentReview && request.Identity.TargetRunID != "" {
		target := request.TargetRun
		if target == nil || target.ID != request.Identity.TargetRunID || target.ID == request.RunID || target.WorkspaceID != request.Identity.WorkspaceID || target.TaskID != request.Identity.TaskID || target.Kind != RunImplementation {
			evaluation.Errors = append(evaluation.Errors, Diagnostic{Code: CodeInvalidStateTransition, EntityID: request.Identity.TargetRunID})
		}
	}
	evaluation.sort()
	if evaluation.HasErrors() {
		return RunStartPlan{}, evaluation
	}

	nextTask := task
	events := make([]PlannedEvent, 0, 2)
	if task.Status == TaskPending {
		started, err := task.Start()
		if err != nil {
			evaluation.Errors = append(evaluation.Errors, Diagnostic{Code: violationCode(err), EntityID: task.ID})
			evaluation.sort()
			return RunStartPlan{}, evaluation
		}
		nextTask = started
		events = append(events, PlannedEvent{
			Type:       "task.started",
			EntityType: "task",
			EntityID:   task.ID,
			Payload:    map[string]any{"taskId": task.ID, "clientRunId": request.Identity.ClientRunID},
		})
	}

	run := Run{
		ID:              request.RunID,
		WorkspaceID:     request.Identity.WorkspaceID,
		TaskID:          request.Identity.TaskID,
		ClientRunID:     request.Identity.ClientRunID,
		Kind:            request.Identity.Kind,
		Status:          RunRunning,
		OperatorActorID: request.OperatorActorID,
		SessionRef:      strings.TrimSpace(request.SessionRef),
		ParentRunID:     request.Identity.ParentRunID,
		TargetRunID:     request.Identity.TargetRunID,
		LeaseTokenHash:  HashLeaseToken(request.LeaseToken),
		HeartbeatAt:     request.Now,
		LeaseExpiresAt:  request.Now.Add(request.LeaseDuration),
		Version:         1,
		StartedAt:       request.Now,
	}
	events = append(events, PlannedEvent{
		Type:       "run.started",
		EntityType: "run",
		EntityID:   run.ID,
		Payload: map[string]any{
			"runId":       run.ID,
			"taskId":      run.TaskID,
			"clientRunId": run.ClientRunID,
			"kind":        run.Kind,
		},
	})
	return RunStartPlan{Task: nextTask, Run: run, Events: events}, evaluation
}

func violationCode(err error) string {
	if violation, ok := err.(*Violation); ok {
		return violation.Code
	}
	return CodeInvalidStateTransition
}
