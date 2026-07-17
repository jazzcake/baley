package domain

import (
	"sort"
	"strings"
)

type Dependency struct {
	FromTaskID string
	ToTaskID   string
}

type DependencyKey struct {
	FromTaskID string
	ToTaskID   string
}

type WorkspaceGraph struct {
	Tasks                map[string]Task
	Dependencies         map[DependencyKey]Dependency
	GateConditionTaskIDs map[string]struct{}
}

func NewWorkspaceGraph(tasks []Task, dependencies []Dependency, gateTaskIDs []string) (*WorkspaceGraph, Evaluation) {
	g := &WorkspaceGraph{
		Tasks:                make(map[string]Task, len(tasks)),
		Dependencies:         make(map[DependencyKey]Dependency, len(dependencies)),
		GateConditionTaskIDs: make(map[string]struct{}, len(gateTaskIDs)),
	}
	evaluation := Evaluation{}
	for _, task := range tasks {
		if _, exists := g.Tasks[task.ID]; exists || task.ID == "" {
			evaluation.Errors = append(evaluation.Errors, Diagnostic{Code: CodeInvalidDependencyPatch, EntityID: task.ID})
			continue
		}
		g.Tasks[task.ID] = task
	}
	for _, dependency := range dependencies {
		key := dependencyKey(dependency)
		if _, exists := g.Dependencies[key]; exists {
			evaluation.Errors = append(evaluation.Errors, Diagnostic{Code: CodeDuplicateDependency, EntityID: edgeID(dependency)})
			continue
		}
		g.Dependencies[key] = dependency
	}
	for _, id := range gateTaskIDs {
		g.GateConditionTaskIDs[id] = struct{}{}
	}
	evaluation.Errors = append(evaluation.Errors, g.validate()...)
	for _, dependency := range g.Dependencies {
		from, fromExists := g.Tasks[dependency.FromTaskID]
		to, toExists := g.Tasks[dependency.ToTaskID]
		if fromExists && toExists && from.WorkspaceID == to.WorkspaceID && from.PhasePosition > to.PhasePosition {
			evaluation.Warnings = append(evaluation.Warnings, Diagnostic{Code: CodePhaseOrderInversion, EntityID: edgeID(dependency)})
		}
	}
	evaluation.sort()
	return g, evaluation
}

func (g *WorkspaceGraph) clone() *WorkspaceGraph {
	clone := &WorkspaceGraph{
		Tasks:                make(map[string]Task, len(g.Tasks)),
		Dependencies:         make(map[DependencyKey]Dependency, len(g.Dependencies)),
		GateConditionTaskIDs: make(map[string]struct{}, len(g.GateConditionTaskIDs)),
	}
	for id, task := range g.Tasks {
		clone.Tasks[id] = task
	}
	for key, dependency := range g.Dependencies {
		clone.Dependencies[key] = dependency
	}
	for id := range g.GateConditionTaskIDs {
		clone.GateConditionTaskIDs[id] = struct{}{}
	}
	return clone
}

func (g *WorkspaceGraph) DependencyList() []Dependency {
	result := make([]Dependency, 0, len(g.Dependencies))
	for _, dependency := range g.Dependencies {
		result = append(result, dependency)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].FromTaskID != result[j].FromTaskID {
			return result[i].FromTaskID < result[j].FromTaskID
		}
		return result[i].ToTaskID < result[j].ToTaskID
	})
	return result
}

func (g *WorkspaceGraph) validate() []Diagnostic {
	errors := []Diagnostic{}
	for id := range g.GateConditionTaskIDs {
		if _, exists := g.Tasks[id]; !exists {
			errors = append(errors, Diagnostic{Code: CodeNotFound, EntityID: id})
		}
	}
	for _, dependency := range g.Dependencies {
		from, fromExists := g.Tasks[dependency.FromTaskID]
		to, toExists := g.Tasks[dependency.ToTaskID]
		if !fromExists || !toExists {
			errors = append(errors, Diagnostic{Code: CodeNotFound, EntityID: edgeID(dependency)})
			continue
		}
		if dependency.FromTaskID == dependency.ToTaskID {
			errors = append(errors, Diagnostic{Code: CodeSelfDependency, EntityID: dependency.FromTaskID})
		}
		if from.WorkspaceID != to.WorkspaceID {
			errors = append(errors, Diagnostic{Code: CodeCrossWorkspaceDependency, EntityID: edgeID(dependency)})
		}
	}
	if hasCycle(g.DependencyList()) {
		errors = append(errors, Diagnostic{Code: CodeDependencyCycle})
	}
	for id, task := range g.Tasks {
		if task.TerminalReason == "" {
			continue
		}
		if strings.TrimSpace(task.TerminalReason) == "" {
			errors = append(errors, Diagnostic{Code: CodeInvalidDependencyPatch, EntityID: id})
			continue
		}
		_, gateCondition := g.GateConditionTaskIDs[id]
		if gateCondition || g.hasOutgoing(id) {
			errors = append(errors, Diagnostic{Code: CodeTerminalPathConflict, EntityID: id})
		}
	}
	return errors
}

func (g *WorkspaceGraph) hasIncoming(id string) bool {
	for key := range g.Dependencies {
		if key.ToTaskID == id {
			return true
		}
	}
	return false
}

func (g *WorkspaceGraph) hasOutgoing(id string) bool {
	for key := range g.Dependencies {
		if key.FromTaskID == id {
			return true
		}
	}
	return false
}

func (g *WorkspaceGraph) isDangling(id string) bool {
	task := g.Tasks[id]
	_, gate := g.GateConditionTaskIDs[id]
	return !g.hasOutgoing(id) && !gate && task.TerminalReason == ""
}

func dependencyKey(d Dependency) DependencyKey {
	return DependencyKey{FromTaskID: d.FromTaskID, ToTaskID: d.ToTaskID}
}
func edgeID(d Dependency) string { return d.FromTaskID + "->" + d.ToTaskID }

func hasCycle(edges []Dependency) bool {
	adjacency := make(map[string][]string)
	for _, edge := range edges {
		adjacency[edge.FromTaskID] = append(adjacency[edge.FromTaskID], edge.ToTaskID)
	}
	visiting := map[string]bool{}
	visited := map[string]bool{}
	var visit func(string) bool
	visit = func(id string) bool {
		if visiting[id] {
			return true
		}
		if visited[id] {
			return false
		}
		visiting[id] = true
		for _, next := range adjacency[id] {
			if visit(next) {
				return true
			}
		}
		visiting[id] = false
		visited[id] = true
		return false
	}
	for id := range adjacency {
		if visit(id) {
			return true
		}
	}
	return false
}
