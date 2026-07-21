import ELK from "elkjs/lib/elk.bundled.js";
import type { WorkspaceFixture } from "../domain/model";

const elk = new ELK();
export const NODE_WIDTH = 190;
export const NODE_HEIGHT = 110;
const LANE_LABEL_WIDTH = 180;
const PHASE_PADDING_X = 32;
const PHASE_HEADER_HEIGHT = 74;
export const LANE_HEIGHT = 154;
export const LANE_BAND_INSET_Y = 18;
export const LANE_LABEL_HEIGHT = 52;
const GATE_CORRIDOR_WIDTH = 250;
const MIN_PHASE_WIDTH = 340;

export type Point = { x: number; y: number };
export type LayoutRect = { id: string; x: number; y: number; width: number; height: number };
export type GraphLayout = {
  taskPositions: Map<string, Point>;
  gatePositions: Map<string, Point>;
  phaseRects: LayoutRect[];
  lanePositions: Map<string, number>;
  laneHeights: Map<string, number>;
  width: number;
  height: number;
};

export function laneBandRect(layout: GraphLayout, laneId: string): LayoutRect | undefined {
  const y = layout.lanePositions.get(laneId);
  const height = layout.laneHeights.get(laneId);
  return y === undefined || height === undefined ? undefined : { id: laneId, x: 0, y: y + LANE_BAND_INSET_Y, width: layout.width, height: height - LANE_BAND_INSET_Y * 2 };
}

export function laneLabelTop(layout: GraphLayout, laneId: string): number | undefined {
  const y = layout.lanePositions.get(laneId);
  const height = layout.laneHeights.get(laneId);
  return y === undefined || height === undefined ? undefined : y + (height - LANE_LABEL_HEIGHT) / 2;
}

type LocalPhaseLayout = {
  phaseId: string;
  laneNodes: Map<string, Map<string, Point>>;
  laneContentHeights: Map<string, number>;
  contentWidth: number;
};

async function layoutPhase(
  fixture: WorkspaceFixture,
  phaseId: string,
  taskIds: Set<string>,
): Promise<LocalPhaseLayout> {
  const laneNodes = new Map<string, Map<string, Point>>();
  const laneContentHeights = new Map<string, number>();
  let contentWidth = 0;

  for (const lane of fixture.lanes) {
    const tasks = fixture.tasks.filter(
      (task) => taskIds.has(task.id) && task.phaseId === phaseId && task.laneId === lane.id,
    );
    if (!tasks.length) continue;

    const taskSet = new Set(tasks.map((task) => task.id));
    const graph = await elk.layout({
      id: `${phaseId}-${lane.id}`,
      layoutOptions: {
        "elk.algorithm": "layered",
        "elk.direction": "RIGHT",
        "elk.spacing.nodeNode": "36",
        "elk.layered.spacing.nodeNodeBetweenLayers": "70",
        "elk.padding": "[top=0,left=0,bottom=0,right=0]",
      },
      children: tasks.map((task) => ({ id: task.id, width: NODE_WIDTH, height: NODE_HEIGHT })),
      edges: fixture.dependencies
        .filter((edge) => taskSet.has(edge.fromTaskId) && taskSet.has(edge.toTaskId))
        .map((edge) => ({ id: edge.id, sources: [edge.fromTaskId], targets: [edge.toTaskId] })),
    });

    const positions = new Map<string, Point>();
    let contentHeight = NODE_HEIGHT;
    graph.children?.forEach((node) => {
      positions.set(node.id, { x: node.x ?? 0, y: node.y ?? 0 });
      contentWidth = Math.max(contentWidth, (node.x ?? 0) + NODE_WIDTH);
      contentHeight = Math.max(contentHeight, (node.y ?? 0) + NODE_HEIGHT);
    });
    laneNodes.set(lane.id, positions);
    laneContentHeights.set(lane.id, contentHeight);
  }

  return { phaseId, laneNodes, laneContentHeights, contentWidth };
}

export async function layoutGraph(
  fixture: WorkspaceFixture,
  taskIds: Set<string>,
): Promise<GraphLayout> {
  const phases = [...fixture.phases].sort((a, b) => a.order - b.order);
  const localLayouts = await Promise.all(
    phases.map((phase) => layoutPhase(fixture, phase.id, taskIds)),
  );
  const taskPositions = new Map<string, Point>();
  const gatePositions = new Map<string, Point>();
  const phaseRects: LayoutRect[] = [];
  const laneHeights = new Map(fixture.lanes.map((lane) => {
    const contentHeight = Math.max(NODE_HEIGHT, ...localLayouts.map((local) => local.laneContentHeights.get(lane.id) ?? 0));
    return [lane.id, Math.max(LANE_HEIGHT, contentHeight + LANE_BAND_INSET_Y * 2)] as const;
  }));
  const lanePositions = new Map<string, number>();
  let cursorY = PHASE_HEADER_HEIGHT;
  fixture.lanes.forEach((lane) => {
    lanePositions.set(lane.id, cursorY);
    cursorY += laneHeights.get(lane.id) ?? LANE_HEIGHT;
  });
  const height = cursorY + 42;

  let cursorX = LANE_LABEL_WIDTH;
  localLayouts.forEach((local, phaseIndex) => {
    const phaseWidth = Math.max(
      MIN_PHASE_WIDTH,
      local.contentWidth + PHASE_PADDING_X * 2,
    );
    phaseRects.push({
      id: local.phaseId,
      x: cursorX,
      y: 0,
      width: phaseWidth,
      height,
    });

    for (const [laneId, positions] of local.laneNodes) {
      const laneY = lanePositions.get(laneId) ?? PHASE_HEADER_HEIGHT;
      const laneHeight = laneHeights.get(laneId) ?? LANE_HEIGHT;
      const contentHeight = local.laneContentHeights.get(laneId) ?? NODE_HEIGHT;
      for (const [taskId, point] of positions) {
        taskPositions.set(taskId, {
          x: cursorX + PHASE_PADDING_X + point.x,
          y: laneY + (laneHeight - contentHeight) / 2 + point.y,
        });
      }
    }

    cursorX += phaseWidth;
    if (phaseIndex < localLayouts.length - 1) {
      const fromPhaseId = local.phaseId;
      const gate = fixture.gates.find(
        (candidate) =>
          candidate.fromPhaseId === fromPhaseId &&
          phases[phaseIndex + 1]?.id === candidate.toPhaseId,
      );
      if (gate) {
        gatePositions.set(gate.id, {
          x: cursorX + (GATE_CORRIDOR_WIDTH - 210) / 2,
          y: PHASE_HEADER_HEIGHT + ((fixture.lanes.length * LANE_HEIGHT) - 94) / 2,
        });
      }
      cursorX += GATE_CORRIDOR_WIDTH;
    }
  });

  return {
    taskPositions,
    gatePositions,
    phaseRects,
    lanePositions,
    laneHeights,
    width: cursorX + 48,
    height,
  };
}

export function rectanglesOverlap(
  first: { x: number; y: number; width: number; height: number },
  second: { x: number; y: number; width: number; height: number },
): boolean {
  return !(
    first.x + first.width <= second.x ||
    second.x + second.width <= first.x ||
    first.y + first.height <= second.y ||
    second.y + second.height <= first.y
  );
}
