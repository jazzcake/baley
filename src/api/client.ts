import type { WorkspaceFixture } from "../domain/model";

type GraphDTO = {
  workspace: WorkspaceFixture["workspace"];
  phases: Array<{ id: string; name: string; position: number; state: "planned" | "active" | "completed" }>;
  lanes: Array<{ id: string; name: string; state: "active" | "closed_out" | "discarded" }>;
  tasks: Array<{ id: string; publicId: number; laneId: string; phaseId: string; parentTaskId?: string; title: string; description: string; currentSummary?: string; nextAction?: string; terminalReason?: string; implementedAssessment?: string; status: WorkspaceFixture["tasks"][number]["status"]; blockerReason?: string }>;
  dependencies: Array<{ fromTaskId: string; toTaskId: string }>;
  gates: Array<{ id: string; name: string; fromPhaseId: string; toPhaseId: string; status: "open" | "ready" | "passed"; conditions: Array<{ id: string; taskId: string; satisfied: boolean; satisfactionReason: string }> }>;
  decisions: WorkspaceFixture["decisions"];
  runs: NonNullable<WorkspaceFixture["runs"]>;
  records: NonNullable<WorkspaceFixture["records"]>;
};

const baseURL = (import.meta.env.VITE_BALEY_API_URL || "http://127.0.0.1:8080").replace(/\/$/, "");
export const workspaceID = import.meta.env.VITE_BALEY_WORKSPACE_ID || "00000000-0000-4000-8000-000000000001";

export async function fetchGraph(signal?: AbortSignal): Promise<WorkspaceFixture> {
  const response = await fetch(`${baseURL}/v1/workspaces/${encodeURIComponent(workspaceID)}/graph`, { signal });
  if (!response.ok) throw new Error(`Baley server returned HTTP ${response.status}`);
  const dto = await response.json() as GraphDTO;
  return {
    workspace: dto.workspace,
    phases: dto.phases.map((phase) => ({ id: phase.id, name: phase.name, order: phase.position, state: phase.state })),
    lanes: dto.lanes.map((lane) => ({ id: lane.id, name: lane.name, lifecycle: lane.state, goal: "", summary: "" })),
    tasks: dto.tasks.map((task) => ({ ...task, blocker: task.blockerReason })),
    dependencies: dto.dependencies.map((edge, index) => ({ id: `dependency-${index}`, ...edge })),
    gates: dto.gates.map(({ conditions: _conditions, ...gate }) => gate),
    gateLinks: dto.gates.flatMap((gate) => gate.conditions.map((condition) => ({ gateId: gate.id, taskId: condition.taskId, kind: "required" as const, satisfied: condition.satisfied, satisfactionReason: condition.satisfactionReason }))),
    decisions: dto.decisions,
    runs: dto.runs ?? [],
    records: dto.records ?? [],
  };
}
