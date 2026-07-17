package domain

import "strings"

type DependencyRef = Dependency

type TerminalUpdate struct {
	TaskID         string
	TerminalReason *string
}

type DependencyPatch struct {
	Remove          []DependencyRef
	Add             []DependencyRef
	TerminalUpdates []TerminalUpdate
}

type TerminalReasonChange struct {
	TaskID string
	Before string
	After  string
}

type DependencyPatchDiff struct {
	AddedDependencies       []Dependency
	RemovedDependencies     []Dependency
	TerminalReasonChanges   []TerminalReasonChange
	NewRootTaskIDs          []string
	NewLeafTaskIDs          []string
	BecameDanglingTaskIDs   []string
	ResolvedDanglingTaskIDs []string
}

type DependencyPatchPreview struct {
	Diff       DependencyPatchDiff
	Evaluation Evaluation
	candidate  *WorkspaceGraph
}

func (g *WorkspaceGraph) PreviewPatch(patch DependencyPatch) DependencyPatchPreview {
	candidate := g.clone()
	evaluation := Evaluation{}

	seenUpdates := map[string]bool{}
	for _, update := range patch.TerminalUpdates {
		task, exists := candidate.Tasks[update.TaskID]
		if !exists {
			evaluation.Errors = append(evaluation.Errors, Diagnostic{Code: CodeNotFound, EntityID: update.TaskID})
			continue
		}
		if seenUpdates[update.TaskID] {
			evaluation.Errors = append(evaluation.Errors, Diagnostic{Code: CodeInvalidDependencyPatch, EntityID: update.TaskID})
			continue
		}
		seenUpdates[update.TaskID] = true
		if update.TerminalReason != nil && strings.TrimSpace(*update.TerminalReason) == "" {
			evaluation.Errors = append(evaluation.Errors, Diagnostic{Code: CodeInvalidDependencyPatch, EntityID: update.TaskID})
			continue
		}
		if update.TerminalReason == nil {
			task.TerminalReason = ""
		} else {
			task.TerminalReason = strings.TrimSpace(*update.TerminalReason)
		}
		candidate.Tasks[update.TaskID] = task
	}
	for _, dependency := range patch.Remove {
		key := dependencyKey(dependency)
		if _, exists := candidate.Dependencies[key]; !exists {
			evaluation.Errors = append(evaluation.Errors, Diagnostic{Code: CodeInvalidDependencyPatch, EntityID: edgeID(dependency)})
			continue
		}
		delete(candidate.Dependencies, key)
	}
	for _, dependency := range patch.Add {
		key := dependencyKey(dependency)
		if _, exists := candidate.Dependencies[key]; exists {
			evaluation.Errors = append(evaluation.Errors, Diagnostic{Code: CodeDuplicateDependency, EntityID: edgeID(dependency)})
			continue
		}
		candidate.Dependencies[key] = dependency
	}
	evaluation.Errors = append(evaluation.Errors, candidate.validate()...)
	for _, dependency := range patch.Add {
		from, fromExists := candidate.Tasks[dependency.FromTaskID]
		to, toExists := candidate.Tasks[dependency.ToTaskID]
		if fromExists && toExists && from.WorkspaceID == to.WorkspaceID && from.PhasePosition > to.PhasePosition {
			evaluation.Warnings = append(evaluation.Warnings, Diagnostic{Code: CodePhaseOrderInversion, EntityID: edgeID(dependency)})
		}
	}
	diff := projectPatchDiff(g, candidate)
	for _, id := range diff.BecameDanglingTaskIDs {
		evaluation.Warnings = append(evaluation.Warnings, Diagnostic{Code: CodeDanglingPath, EntityID: id})
	}
	evaluation.sort()
	return DependencyPatchPreview{Diff: diff, Evaluation: evaluation, candidate: candidate}
}

func (g *WorkspaceGraph) ApplyPatch(patch DependencyPatch) DependencyPatchPreview {
	preview := g.PreviewPatch(patch)
	if !preview.Evaluation.HasErrors() {
		g.Tasks = preview.candidate.Tasks
		g.Dependencies = preview.candidate.Dependencies
		g.GateConditionTaskIDs = preview.candidate.GateConditionTaskIDs
	}
	preview.candidate = nil
	return preview
}

func (g *WorkspaceGraph) Connect(fromTaskID, toTaskID string) DependencyPatchPreview {
	return g.ApplyPatch(DependencyPatch{Add: []Dependency{{FromTaskID: fromTaskID, ToTaskID: toTaskID}}})
}

func (g *WorkspaceGraph) Disconnect(fromTaskID, toTaskID string) DependencyPatchPreview {
	return g.ApplyPatch(DependencyPatch{Remove: []Dependency{{FromTaskID: fromTaskID, ToTaskID: toTaskID}}})
}

func (g *WorkspaceGraph) SetTerminal(taskID, reason string) DependencyPatchPreview {
	return g.ApplyPatch(DependencyPatch{TerminalUpdates: []TerminalUpdate{{TaskID: taskID, TerminalReason: &reason}}})
}

func (g *WorkspaceGraph) ClearTerminal(taskID string) DependencyPatchPreview {
	return g.ApplyPatch(DependencyPatch{TerminalUpdates: []TerminalUpdate{{TaskID: taskID}}})
}

// DependencyGraph keeps the batch-1 connect surface while exposing constructor validation.
type DependencyGraph struct{ workspace *WorkspaceGraph }

func NewDependencyGraph(tasks []Task, edges []Dependency) (*DependencyGraph, Evaluation) {
	workspace, evaluation := NewWorkspaceGraph(tasks, edges, nil)
	if evaluation.HasErrors() {
		return nil, evaluation
	}
	return &DependencyGraph{workspace: workspace}, evaluation
}

func (g *DependencyGraph) Dependencies() []Dependency { return g.workspace.DependencyList() }

func (g *DependencyGraph) Connect(fromTaskID, toTaskID string) ([]Diagnostic, error) {
	preview := g.workspace.Connect(fromTaskID, toTaskID)
	return preview.Evaluation.Warnings, firstViolation(preview.Evaluation)
}
