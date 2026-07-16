import type { WorkspaceFixture } from "../domain/model";

export type ViewSpec = { kind: "multi" } | { kind: "lane"; id: string } | { kind: "gate"; id: string };

export function visibleTaskIds(fixture: WorkspaceFixture, view: ViewSpec): Set<string> {
  if (view.kind === "multi") return new Set(fixture.tasks.map((task) => task.id));
  if (view.kind === "lane") {
    return new Set(fixture.tasks.map((task) => task.id));
  }
  return new Set(fixture.gateLinks.filter((link) => link.gateId === view.id).map((link) => link.taskId));
}

export function laneFocusTaskIds(fixture: WorkspaceFixture, laneId: string): Set<string> {
  const own = fixture.tasks.filter((task) => task.laneId === laneId).map((task) => task.id);
  const ownSet = new Set(own);
  const relatedGates = new Set(
    fixture.gateLinks
      .filter((link) => ownSet.has(link.taskId))
      .map((link) => link.gateId),
  );
  const gateContext = fixture.gateLinks
    .filter((link) => relatedGates.has(link.gateId))
    .map((link) => link.taskId);
  return new Set([...own, ...gateContext]);
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
