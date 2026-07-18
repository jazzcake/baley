export type TaskStatus = "pending" | "in_progress" | "implemented" | "confirmed" | "discarded";
export type LaneLifecycle = "active" | "closed_out" | "discarded";
export type GateStatus = "open" | "ready" | "passed";
export type GateLinkKind = "required" | "reference" | "unlocks";

export type Phase = { id: string; name: string; order: number; state: "planned" | "active" | "completed" };
export type Lane = {
  id: string;
  name: string;
  goal: string;
  summary: string;
  lifecycle: LaneLifecycle;
};
export type Task = {
  id: string;
  publicId: number;
  laneId: string;
  phaseId: string;
  title: string;
  description: string;
  status: TaskStatus;
  blocker?: string;
  parentTaskId?: string;
};
export type Dependency = { id: string; fromTaskId: string; toTaskId: string };
export type Gate = {
  id: string;
  name: string;
  fromPhaseId: string;
  toPhaseId: string;
  status: GateStatus;
};
export type GateLink = { gateId: string; taskId: string; kind: GateLinkKind; satisfied?: boolean; satisfactionReason?: string };
export type WorkspaceFixture = {
	workspace: { id: string; name: string; revision: number; activePhaseId?: string };
  phases: Phase[];
  lanes: Lane[];
  tasks: Task[];
  dependencies: Dependency[];
  gates: Gate[];
  gateLinks: GateLink[];
	decisions: Array<{ action: string; entityType: string; entityId: string | number; expectedWorkspaceRevision: number }>;
};
