export type TaskStatus = "done" | "running" | "blocked" | "ready";
export type LaneLifecycle = "active" | "close-out" | "discard";
export type GateStatus = "open" | "ready" | "passed" | "reopened";
export type GateLinkKind = "required" | "reference" | "unlocks";

export type Phase = { id: string; name: string; order: number };
export type Lane = {
  id: string;
  name: string;
  goal: string;
  summary: string;
  lifecycle: LaneLifecycle;
};
export type Task = {
  id: string;
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
export type GateLink = { gateId: string; taskId: string; kind: GateLinkKind };
export type WorkspaceFixture = {
  phases: Phase[];
  lanes: Lane[];
  tasks: Task[];
  dependencies: Dependency[];
  gates: Gate[];
  gateLinks: GateLink[];
};
