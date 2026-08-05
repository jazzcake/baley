import type { WorkspaceFixture } from "../domain/model";
import { requestJSON } from "./http";

type GraphDTO = {
  workspace: WorkspaceFixture["workspace"];
  phases: Array<{ id: string; name: string; position: number; state: "planned" | "active" | "completed" }>;
  lanes: Array<{ id: string; name: string; state: "active" | "closed_out" | "discarded" }>;
  tasks: Array<{ id: string; publicId: number; laneId: string; phaseId: string; parentTaskId?: string; title: string; description: string; currentSummary?: string; nextAction?: string; terminalReason?: string; implementedAssessment?: string; status: WorkspaceFixture["tasks"][number]["status"]; blockerReason?: string; requestedAcceptanceMode?: "delegated" | "human_required" | "inherit"; effectiveAcceptanceMode?: "delegated" | "human_required"; acceptancePolicyVersion?: string; evidenceProfileId?: string; acceptanceEvaluation?: { eligible: boolean; reasons: string[] } }>;
  backlogItems?: WorkspaceFixture["backlogItems"];
  dependencies: Array<{ fromTaskId: string; toTaskId: string }>;
  gates: Array<{ id: string; publicId?: number; alias?: string; name: string; fromPhaseId: string; toPhaseId: string; status: "open" | "ready" | "passed"; conditions: Array<{ id: string; taskId: string; satisfied: boolean; satisfactionReason: string }>; entryTasks: Array<{ taskId: string; selectionSource: "explicit" | "automatic" }> }>;
  decisions: WorkspaceFixture["decisions"];
  runs: NonNullable<WorkspaceFixture["runs"]>;
  externalExecutions?: NonNullable<WorkspaceFixture["externalExecutions"]>;
  records: NonNullable<WorkspaceFixture["records"]>;
  acceptanceEvidence?: NonNullable<WorkspaceFixture["acceptanceEvidence"]>;
};

export const workspaceID = import.meta.env.VITE_BALEY_WORKSPACE_ID || "00000000-0000-4000-8000-000000000001";

export async function fetchGraph(workspaceId: string, signal?: AbortSignal): Promise<WorkspaceFixture> {
  const dto = await requestJSON<GraphDTO>(
    `/v1/workspaces/${encodeURIComponent(workspaceId)}/graph`,
    { signal },
  );
  const phasePositions = new Map(dto.phases.map((phase) => [phase.id, phase.position]));
  const fallbackGateNumbers = new Map(
    [...dto.gates]
      .sort((left, right) => (phasePositions.get(left.fromPhaseId) ?? Number.MAX_SAFE_INTEGER) - (phasePositions.get(right.fromPhaseId) ?? Number.MAX_SAFE_INTEGER) || left.id.localeCompare(right.id))
      .map((gate, index) => [gate.id, index + 1]),
  );
  return {
    workspace: dto.workspace,
    phases: dto.phases.map((phase) => ({ id: phase.id, name: phase.name, order: phase.position, state: phase.state })),
    lanes: dto.lanes.map((lane) => ({ id: lane.id, name: lane.name, lifecycle: lane.state, goal: "", summary: "" })),
    tasks: dto.tasks.map((task) => ({ ...task, blocker: task.blockerReason })),
    backlogItems: (dto.backlogItems ?? []).filter((item) => item.status === "active").sort((a, b) => a.laneId.localeCompare(b.laneId) || (a.position ?? 0) - (b.position ?? 0) || a.publicId - b.publicId),
    dependencies: dto.dependencies.map((edge, index) => ({ id: `dependency-${index}`, ...edge })),
    gates: dto.gates.map(({ conditions: _conditions, entryTasks: _entryTasks, ...gate }) => ({
      ...gate,
      publicId: gate.publicId ?? fallbackGateNumbers.get(gate.id)!,
    })),
    gateLinks: dto.gates.flatMap((gate) => [
      ...gate.conditions.map((condition) => ({ gateId: gate.id, taskId: condition.taskId, kind: "required" as const, satisfied: condition.satisfied, satisfactionReason: condition.satisfactionReason })),
      ...gate.entryTasks.map((entry) => ({ gateId: gate.id, taskId: entry.taskId, kind: "unlocks" as const, satisfactionReason: entry.selectionSource })),
    ]),
    decisions: dto.decisions,
    runs: dto.runs ?? [],
    externalExecutions: dto.externalExecutions ?? [],
    records: dto.records ?? [],
    acceptanceEvidence: dto.acceptanceEvidence ?? [],
  };
}
