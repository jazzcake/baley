export type BirdView = {
  id: string;
  title: string;
  description: string;
  status: "active" | "archived";
  revision: number;
  createdAt: string;
  updatedAt: string;
  archivedAt?: string;
};

export type BirdViewNode = {
  id: string;
  title: string;
  summary: string;
  content: string;
  status: "active" | "achieved" | "parked";
  positionX: number;
  positionY: number;
  createdAt: string;
  updatedAt: string;
};

export type BirdViewEdge = {
  id: string;
  fromNodeId: string;
  toNodeId: string;
  label: string;
  createdAt: string;
};

export type BirdViewBinding = {
  nodeId: string;
  workspaceId: string;
  targetId: string;
  targetType: "phase" | "gate" | "task" | "backlog";
  state: "suggested" | "pinned" | "excluded";
  targetTitle: string;
  targetStatus: string;
  targetPublicId?: number;
};

export type BirdViewGraph = { birdView: BirdView; nodes: BirdViewNode[]; edges: BirdViewEdge[] };

export type BirdViewCommand = {
  name: string;
  arguments: Record<string, unknown>;
  envelope: {
    idempotencyKey: string;
    expectedBirdViewRevision?: number;
    executedByActorId?: string;
  };
};

export type BirdViewExecution = {
  commandId: string;
  birdViewRevision: number;
  eventIds: string[];
  projection: unknown;
  idempotent: boolean;
};

export type BirdViewFocusWorkspace = {
  workspace: { id: string; name: string; state: string; revision: number; activePhaseId?: string };
  phases: Array<{ id: string; name: string; state: string; position: number }>;
  lanes: Array<{ id: string; name: string; goal?: string; summary?: string; state: string }>;
  tasks: Array<{ id: string; publicId: number; laneId: string; phaseId: string; title: string; description: string; status: "pending" | "in_progress" | "implemented" | "confirmed" | "discarded" }>;
  dependencies: Array<{ fromTaskId: string; toTaskId: string }>;
  gates: Array<{ id: string; publicId: number; alias?: string; name: string; fromPhaseId: string; toPhaseId: string; status: string; conditions: Array<{ taskId: string }>; entryTasks: Array<{ taskId: string }> }>;
  backlogItems: Array<{ id: string; publicId: number; laneId: string; title: string; description: string; status: string; position: number | null }>;
  bindings: BirdViewBinding[];
};

export type BirdViewNodeFocus = { birdView: BirdView; node: BirdViewNode; workspaces: BirdViewFocusWorkspace[] };
