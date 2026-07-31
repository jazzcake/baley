package domain

import (
	"strings"
	"time"
)

type GateStatus string

const (
	GateOpen        GateStatus = "open"
	GateReadyStatus GateStatus = "ready"
	GatePassed      GateStatus = "passed"
)

type Phase struct {
	ID          string
	WorkspaceID string
	Position    int
	State       PhaseState
}

type Gate struct {
	ID               string
	PublicID         int
	Alias            string
	WorkspaceID      string
	FromPhaseID      string
	ToPhaseID        string
	PassedAt         *time.Time
	CriteriaRevision int64
}

type GateTaskCondition struct {
	WorkspaceID string
	GateID      string
	LinkID      string
	TaskID      string
	TaskStatus  TaskStatus
	Passed      bool
	PassReason  string
}

type GateEntryTask struct {
	WorkspaceID     string
	GateID          string
	TaskID          string
	SelectionSource string
}

func NormalizeGateAlias(value string) (string, error) {
	alias := strings.ToLower(strings.TrimSpace(value))
	if alias == "" {
		return "", nil
	}
	if len(alias) > 64 || alias[0] == '-' || alias[len(alias)-1] == '-' {
		return "", &Violation{Code: CodeInvalidStateTransition}
	}
	previousDash := false
	for _, char := range alias {
		valid := char >= 'a' && char <= 'z' || char >= '0' && char <= '9' || char == '-'
		if !valid || char == '-' && previousDash {
			return "", &Violation{Code: CodeInvalidStateTransition}
		}
		previousDash = char == '-'
	}
	return alias, nil
}

func IsReservedGatePublicReference(value string) bool {
	reference := strings.TrimSpace(value)
	if len(reference) < 3 || !strings.EqualFold(reference[:2], "G#") {
		return false
	}
	digits := reference[2:]
	if digits[0] < '1' || digits[0] > '9' {
		return false
	}
	for index := 1; index < len(digits); index++ {
		if digits[index] < '0' || digits[index] > '9' {
			return false
		}
	}
	return true
}

func GateReady(conditions []GateTaskCondition) bool {
	if len(conditions) == 0 {
		return false
	}
	for _, condition := range conditions {
		if condition.TaskStatus != TaskConfirmed && !condition.Passed {
			return false
		}
	}
	return true
}

func GateStatusFor(gate Gate, conditions []GateTaskCondition) GateStatus {
	if gate.PassedAt != nil {
		return GatePassed
	}
	if GateReady(conditions) {
		return GateReadyStatus
	}
	return GateOpen
}

func PassGateTask(condition GateTaskCondition, reason string) (GateTaskCondition, error) {
	if condition.Passed || strings.TrimSpace(reason) == "" {
		return condition, &Violation{Code: CodeInvalidStateTransition}
	}
	condition.Passed = true
	condition.PassReason = strings.TrimSpace(reason)
	return condition, nil
}

func RevokeGateTaskPass(condition GateTaskCondition, reason string) (GateTaskCondition, error) {
	if !condition.Passed || strings.TrimSpace(reason) == "" {
		return condition, &Violation{Code: CodeInvalidStateTransition}
	}
	condition.Passed = false
	condition.PassReason = ""
	return condition, nil
}

type GateTransition struct {
	From     Phase
	To       Phase
	PassedAt time.Time
}

func PlanGatePass(gate Gate, from, to Phase, conditions []GateTaskCondition, now time.Time) (GateTransition, error) {
	if gate.PassedAt != nil || from.ID != gate.FromPhaseID || to.ID != gate.ToPhaseID || from.State != PhaseActive || to.State != PhasePlanned || to.Position != from.Position+1 {
		return GateTransition{}, &Violation{Code: CodeGateNotCurrent}
	}
	if len(conditions) == 0 {
		return GateTransition{}, &Violation{Code: CodeGateHasNoTasks}
	}
	if !GateReady(conditions) {
		return GateTransition{}, &Violation{Code: CodeGateNotReady}
	}
	from.State = PhaseCompleted
	to.State = PhaseActive
	return GateTransition{From: from, To: to, PassedAt: now}, nil
}
