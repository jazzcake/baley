import type { WorkspaceFixture } from "./model";

export function validateFixture(fixture: WorkspaceFixture): void {
  const collections = [fixture.phases, fixture.lanes, fixture.tasks, fixture.gates];
  if (collections.some((items) => new Set(items.map((item) => item.id)).size !== items.length)) throw new Error("Fixture IDs must be unique within each entity type");
  const graphNodeIds = [...fixture.tasks, ...fixture.gates].map((item) => item.id);
  if (new Set(graphNodeIds).size !== graphNodeIds.length) throw new Error("Task and Gate IDs must be unique graph node IDs");
  const lanes = new Set(fixture.lanes.map((lane) => lane.id));
  const phases = new Set(fixture.phases.map((phase) => phase.id));
  const tasks = new Set(fixture.tasks.map((task) => task.id));
  const gates = new Set(fixture.gates.map((gate) => gate.id));
  for (const task of fixture.tasks) {
    if (!lanes.has(task.laneId) || !phases.has(task.phaseId)) throw new Error(`Invalid task reference: ${task.id}`);
  }
  for (const dependency of fixture.dependencies) {
    if (!tasks.has(dependency.fromTaskId) || !tasks.has(dependency.toTaskId)) throw new Error(`Invalid dependency: ${dependency.id}`);
  }
  for (const link of fixture.gateLinks) {
    if (!gates.has(link.gateId) || !tasks.has(link.taskId)) throw new Error(`Invalid gate link: ${link.gateId}/${link.taskId}`);
  }
  const adjacency = new Map<string, string[]>();
  fixture.dependencies.forEach(({ fromTaskId, toTaskId }) => adjacency.set(fromTaskId, [...(adjacency.get(fromTaskId) ?? []), toTaskId]));
  const visiting = new Set<string>();
  const visited = new Set<string>();
  const visit = (id: string) => {
    if (visiting.has(id)) throw new Error("Task dependency cycle detected");
    if (visited.has(id)) return;
    visiting.add(id);
    (adjacency.get(id) ?? []).forEach(visit);
    visiting.delete(id);
    visited.add(id);
  };
  fixture.tasks.forEach((task) => visit(task.id));
}
