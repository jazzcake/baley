package domain

type RunKind string

const (
	RunDetailedPlanning       RunKind = "detailed_planning"
	RunImplementation         RunKind = "implementation"
	RunIndependentAgentReview RunKind = "independent_agent_review"
	RunReviewResponse         RunKind = "review_response"
	RunCompletionReporting    RunKind = "completion_reporting"
)

var RunKinds = []RunKind{RunDetailedPlanning, RunImplementation, RunIndependentAgentReview, RunReviewResponse, RunCompletionReporting}

type PhaseState string

const (
	PhasePlanned   PhaseState = "planned"
	PhaseActive    PhaseState = "active"
	PhaseCompleted PhaseState = "completed"
)

func EvaluateRunStart(task Task, kind RunKind, phaseState PhaseState, predecessors []Task) Evaluation {
	evaluation := Evaluation{}
	if !validRunKind(kind) {
		evaluation.Errors = append(evaluation.Errors, Diagnostic{Code: CodeInvalidStateTransition, EntityID: string(kind)})
		evaluation.sort()
		return evaluation
	}
	if phaseState != PhaseActive && !(kind == RunDetailedPlanning && phaseState == PhasePlanned) {
		evaluation.Errors = append(evaluation.Errors, Diagnostic{Code: CodePhaseInactive, EntityID: task.ID})
	}
	blockedKind := kind == RunImplementation || kind == RunReviewResponse
	if blockedKind && task.BlockedAt != nil {
		evaluation.Errors = append(evaluation.Errors, Diagnostic{Code: CodeBlockedTask, EntityID: task.ID})
	}
	if blockedKind {
		for _, predecessor := range predecessors {
			if predecessor.Status != TaskImplemented && predecessor.Status != TaskConfirmed {
				evaluation.Errors = append(evaluation.Errors, Diagnostic{Code: CodeUnresolvedDependency, EntityID: predecessor.ID})
			}
		}
	}
	evaluation.sort()
	return evaluation
}

func validRunKind(kind RunKind) bool {
	for _, candidate := range RunKinds {
		if kind == candidate {
			return true
		}
	}
	return false
}
