import type { WorkspaceFixture } from "../domain/model";

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
