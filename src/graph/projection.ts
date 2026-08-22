import type { Dependency, WorkspaceFixture } from "../domain/model";

export type ViewSpec = { kind: "multi" } | { kind: "lane"; id: string } | { kind: "gate"; id: string };

export function canvasKey(view: ViewSpec): string {
  return view.kind === "gate" ? `gate:${view.id}` : "workspace";
}

export function defaultGateFocusId(fixture: WorkspaceFixture): string | undefined {
  const activePhaseId = fixture.workspace.activePhaseId;
  if (!activePhaseId) return fixture.gates[0]?.id;
  return fixture.gates.find((gate) => gate.fromPhaseId === activePhaseId && gate.status !== "passed")?.id;
}

export function visibleTaskIds(fixture: WorkspaceFixture, view: ViewSpec): Set<string> {
  if (view.kind === "gate") {
    return new Set(fixture.gateLinks.filter((link) => link.gateId === view.id).map((link) => link.taskId));
  }
  return new Set(fixture.tasks.map((task) => task.id));
}

export function laneFocusTaskIds(fixture: WorkspaceFixture, laneId: string): Set<string> {
  return new Set(
    fixture.tasks
      .filter((task) => task.laneId === laneId)
      .map((task) => task.id),
  );
}

export function connectedTaskIds(fixture: WorkspaceFixture, taskId: string): Set<string> {
  const result = new Set([taskId]);
  const walk = (id: string, direction: "up" | "down") => {
    fixture.dependencies.forEach((edge) => {
      const next = direction === "up" && edge.toTaskId === id ? edge.fromTaskId : direction === "down" && edge.fromTaskId === id ? edge.toTaskId : undefined;
      if (next && !result.has(next)) { result.add(next); walk(next, direction); }
    });
  };
  walk(taskId, "up");
  walk(taskId, "down");
  return result;
}

export type PhaseSummary = { id: string; phaseId: string; laneId: string; taskIds: string[]; completedTaskCount: number; dependencyCount: number };
export type PhasePresentation = { collapsedPhaseIds: Set<string>; hiddenTaskIds: Set<string>; summaries: PhaseSummary[]; summaryIdForTaskId: Map<string, string>; laneIdForTaskId: Map<string, string> };
export type ProjectedDependency = { id: string; source: string; target: string; dependencyIds: string[]; sourceLaneId?: string; targetLaneId?: string };
export function summaryNodeId(phaseId: string, laneId: string): string { return `phase-summary:${phaseId}:${laneId}`; }
export function phasePresentation(fixture: WorkspaceFixture, requested: ReadonlySet<string>): PhasePresentation {
  const completed = new Set(fixture.phases.filter((phase) => phase.state === "completed").map((phase) => phase.id));
  const collapsedPhaseIds = new Set([...requested].filter((id) => completed.has(id)));
  const hiddenTaskIds = new Set<string>(); const summaryIdForTaskId = new Map<string, string>();
  const summaries = fixture.phases.filter((phase) => collapsedPhaseIds.has(phase.id)).flatMap((phase) => fixture.lanes.map((lane) => {
    const taskIds = fixture.tasks.filter((task) => task.phaseId === phase.id && task.laneId === lane.id).map((task) => task.id);
    const id = summaryNodeId(phase.id, lane.id); taskIds.forEach((taskId) => { hiddenTaskIds.add(taskId); summaryIdForTaskId.set(taskId, id); });
    const taskSet = new Set(taskIds);
    return { id, phaseId: phase.id, laneId: lane.id, taskIds, completedTaskCount: fixture.tasks.filter((task) => taskSet.has(task.id) && task.status === "confirmed").length, dependencyCount: fixture.dependencies.filter((edge) => taskSet.has(edge.fromTaskId) || taskSet.has(edge.toTaskId)).length };
  }));
  return { collapsedPhaseIds, hiddenTaskIds, summaries, summaryIdForTaskId, laneIdForTaskId: new Map(fixture.tasks.map((task) => [task.id, task.laneId])) };
}
// Tree rendering policy: docs/adr/0001-tree-dag-layout-and-visual-transitive-reduction.md
export function transitiveReduction<T>(
  edges: readonly T[],
  sourceOf: (edge: T) => string,
  targetOf: (edge: T) => string,
): T[] {
  const adjacency = new Map<string, Set<string>>();
  for (const edge of edges) {
    const source = sourceOf(edge);
    const target = targetOf(edge);
    const targets = adjacency.get(source) ?? new Set<string>();
    targets.add(target);
    adjacency.set(source, targets);
  }

  return edges.filter((edge) => {
    const source = sourceOf(edge);
    const target = targetOf(edge);
    if (source === target) return true;
    const visited = new Set([source]);
    const stack = [...(adjacency.get(source) ?? [])].filter((candidate) => candidate !== target);
    while (stack.length) {
      const current = stack.pop()!;
      if (current === target) return false;
      if (visited.has(current)) continue;
      visited.add(current);
      for (const next of adjacency.get(current) ?? []) {
        if (!visited.has(next)) stack.push(next);
      }
    }
    return true;
  });
}

export function projectDependencies(dependencies: Dependency[], visible: ReadonlySet<string>, presentation: PhasePresentation): ProjectedDependency[] {
  const aggregate = new Map<string, ProjectedDependency>();
  for (const dependency of dependencies) {
    if (!visible.has(dependency.fromTaskId) || !visible.has(dependency.toTaskId)) continue;
    const source = presentation.summaryIdForTaskId.get(dependency.fromTaskId) ?? dependency.fromTaskId;
    const target = presentation.summaryIdForTaskId.get(dependency.toTaskId) ?? dependency.toTaskId;
    if (source === target) continue;
    const key = `${source}→${target}`; const existing = aggregate.get(key);
    const sourceLaneId = presentation.laneIdForTaskId.get(dependency.fromTaskId);
    const targetLaneId = presentation.laneIdForTaskId.get(dependency.toTaskId);
    if (existing) existing.dependencyIds.push(dependency.id);
    else aggregate.set(key, { id: `summary-dependency:${key}`, source, target, dependencyIds: [dependency.id], sourceLaneId, targetLaneId });
  }
  return [...aggregate.values()].sort((first, second) => first.id.localeCompare(second.id));
}
