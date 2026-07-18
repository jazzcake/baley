package domain

import "strings"

type RunStartIdentity struct {
	WorkspaceID string
	TaskID      string
	ClientRunID string
	Kind        RunKind
	ParentRunID string
	TargetRunID string
}

func (i RunStartIdentity) Validate() error {
	if strings.TrimSpace(i.WorkspaceID) == "" || strings.TrimSpace(i.TaskID) == "" || strings.TrimSpace(i.ClientRunID) == "" || !validRunKind(i.Kind) {
		return &Violation{Code: CodeInvalidStateTransition}
	}
	return nil
}

func (i RunStartIdentity) Matches(other RunStartIdentity) bool {
	return i.WorkspaceID == other.WorkspaceID &&
		i.TaskID == other.TaskID &&
		i.ClientRunID == other.ClientRunID &&
		i.Kind == other.Kind &&
		i.ParentRunID == other.ParentRunID &&
		i.TargetRunID == other.TargetRunID
}

func CompareRunStartIdentity(existing, requested RunStartIdentity) error {
	if err := requested.Validate(); err != nil {
		return err
	}
	if err := existing.Validate(); err != nil {
		return err
	}
	if !existing.Matches(requested) {
		return &Violation{Code: CodeIdempotencyConflict}
	}
	return nil
}
