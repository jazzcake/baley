package domain

import (
	"fmt"
	"sort"
)

const (
	CodeNotFound                 = "not_found"
	CodeInvalidStateTransition   = "invalid_state_transition"
	CodeSelfDependency           = "self_dependency"
	CodeDuplicateDependency      = "duplicate_dependency"
	CodeDependencyCycle          = "dependency_cycle"
	CodeCrossWorkspaceDependency = "cross_workspace_dependency"
	CodeInvalidDependencyPatch   = "invalid_dependency_patch"
	CodeTerminalPathConflict     = "terminal_path_conflict"
	CodeUnresolvedDependency     = "unresolved_dependency"
	CodeBlockedTask              = "blocked_task"
	CodePhaseInactive            = "phase_inactive"
	CodeGateNotReady             = "gate_not_ready"
	CodeGateHasNoTasks           = "gate_has_no_tasks"
	CodeGateTaskWrongPhase       = "gate_task_wrong_phase"
	CodeGateNotCurrent           = "gate_not_current"
	CodeHumanApprovalRequired    = "human_approval_required"
	CodeHumanApprovalMismatch    = "human_approval_mismatch"
	CodeStaleRevision            = "stale_revision"
	CodeIdempotencyConflict      = "idempotency_conflict"
	CodeDanglingPath             = "dangling_path"
	CodePhaseOrderInversion      = "phase_order_inversion"
)

var UsedDiagnosticCodes = []string{
	CodeNotFound,
	CodeInvalidStateTransition,
	CodeSelfDependency,
	CodeDuplicateDependency,
	CodeDependencyCycle,
	CodeCrossWorkspaceDependency,
	CodeInvalidDependencyPatch,
	CodeTerminalPathConflict,
	CodeUnresolvedDependency,
	CodeBlockedTask,
	CodePhaseInactive,
	CodeGateNotReady,
	CodeGateHasNoTasks,
	CodeGateTaskWrongPhase,
	CodeGateNotCurrent,
	CodeHumanApprovalRequired,
	CodeHumanApprovalMismatch,
	CodeStaleRevision,
	CodeIdempotencyConflict,
	CodeDanglingPath,
	CodePhaseOrderInversion,
}

type Diagnostic struct {
	Code     string         `json:"code"`
	EntityID string         `json:"entityId,omitempty"`
	Details  map[string]any `json:"details,omitempty"`
}

type Evaluation struct {
	Errors     []Diagnostic `json:"errors"`
	Warnings   []Diagnostic `json:"warnings"`
	Advisories []Diagnostic `json:"advisories"`
}

func (e Evaluation) HasErrors() bool { return len(e.Errors) != 0 }

func (e *Evaluation) sort() {
	sortDiagnostics(e.Errors)
	sortDiagnostics(e.Warnings)
	sortDiagnostics(e.Advisories)
}

func sortDiagnostics(values []Diagnostic) {
	sort.SliceStable(values, func(i, j int) bool {
		if values[i].Code != values[j].Code {
			return values[i].Code < values[j].Code
		}
		return values[i].EntityID < values[j].EntityID
	})
}

type Violation struct {
	Code string
}

func (v *Violation) Error() string {
	return fmt.Sprintf("Baley domain violation: %s", v.Code)
}

func firstViolation(e Evaluation) error {
	if len(e.Errors) == 0 {
		return nil
	}
	for _, code := range []string{CodeNotFound, CodeSelfDependency, CodeDuplicateDependency, CodeCrossWorkspaceDependency, CodeTerminalPathConflict, CodeDependencyCycle, CodeInvalidDependencyPatch} {
		for _, diagnostic := range e.Errors {
			if diagnostic.Code == code {
				return &Violation{Code: code}
			}
		}
	}
	return &Violation{Code: e.Errors[0].Code}
}
