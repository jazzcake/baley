import ELK from "elkjs/lib/elk.bundled.js";
import type { Dependency, Task, WorkspaceFixture } from "../domain/model";
import { BACKLOG_RAIL_GUTTER_WIDTH } from "../components/backlog-rail.config";
import type { LayoutMode } from "./layout-mode";
import { transitiveReduction } from "./projection";

const elk = new ELK();
export const NODE_WIDTH = 190;
export const NODE_HEIGHT = 110;
export const SUMMARY_NODE_WIDTH = 190;
export const SUMMARY_NODE_HEIGHT = 84;
const LANE_LABEL_WIDTH = 180;
const PHASE_PADDING_X = 32;
const PHASE_HEADER_HEIGHT = 74;
export const LANE_HEIGHT = 154;
export const LANE_BAND_INSET_Y = 18;
export const LANE_CONTENT_BREATHING_ROOM_Y = 18;
export const LANE_LABEL_HEIGHT = 68;
const GATE_CORRIDOR_WIDTH = 250;
const COMPACT_GATE_CORRIDOR_WIDTH = 112;
const GATE_NODE_WIDTH = 210;
const GATE_NODE_HEIGHT = 94;
const MIN_PHASE_WIDTH = 340;
const TREE_NODE_GAP_Y = 24;
const TREE_COMPONENT_GAP_Y = 40;

export type Point = { x: number; y: number };
export type LayoutRect = { id: string; x: number; y: number; width: number; height: number };
export type GraphLayout = {
  taskPositions: Map<string, Point>;
  summaryPositions?: Map<string, Point>;
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

// Decision context and invariants: docs/adr/0001-tree-dag-layout-and-visual-transitive-reduction.md
function layoutTreeLane(
  tasks: Task[],
  dependencies: Dependency[],
  depthByTaskId: ReadonlyMap<string, number>,
): { positions: Map<string, Point>; contentHeight: number; contentWidth: number } {
  const taskById = new Map(tasks.map((task) => [task.id, task]));
  const compareTasks = (firstId: string, secondId: string) => {
    const first = taskById.get(firstId)!;
    const second = taskById.get(secondId)!;
    return first.publicId - second.publicId || first.id.localeCompare(second.id);
  };
  const predecessors = new Map(tasks.map((task) => [task.id, [] as string[]]));
  const successors = new Map(tasks.map((task) => [task.id, [] as string[]]));
  const neighbors = new Map(tasks.map((task) => [task.id, [] as string[]]));
  for (const dependency of dependencies) {
    if (!taskById.has(dependency.fromTaskId) || !taskById.has(dependency.toTaskId)) continue;
    predecessors.get(dependency.toTaskId)!.push(dependency.fromTaskId);
    successors.get(dependency.fromTaskId)!.push(dependency.toTaskId);
    neighbors.get(dependency.fromTaskId)!.push(dependency.toTaskId);
    neighbors.get(dependency.toTaskId)!.push(dependency.fromTaskId);
  }
  for (const values of [...predecessors.values(), ...successors.values(), ...neighbors.values()]) {
    values.sort(compareTasks);
  }

  const components: string[][] = [];
  const visited = new Set<string>();
  for (const task of [...tasks].sort((first, second) => compareTasks(first.id, second.id))) {
    if (visited.has(task.id)) continue;
    const component: string[] = [];
    const stack = [task.id];
    visited.add(task.id);
    while (stack.length) {
      const taskId = stack.pop()!;
      component.push(taskId);
      for (const neighborId of neighbors.get(taskId) ?? []) {
        if (visited.has(neighborId)) continue;
        visited.add(neighborId);
        stack.push(neighborId);
      }
    }
    component.sort(compareTasks);
    components.push(component);
  }

  const positions = new Map<string, Point>();
  let cursorY = 0;
  let contentWidth = NODE_WIDTH;
  const averageRank = (taskId: string, links: ReadonlyMap<string, string[]>, ranks: ReadonlyMap<string, number>) => {
    const values = (links.get(taskId) ?? []).flatMap((linkedId) => {
      const rank = ranks.get(linkedId);
      return rank === undefined ? [] : [rank];
    });
    return values.length ? values.reduce((sum, value) => sum + value, 0) / values.length : undefined;
  };

  for (const component of components) {
    const layers = new Map<number, string[]>();
    for (const taskId of component) {
      const depth = depthByTaskId.get(taskId) ?? 0;
      const layer = layers.get(depth) ?? [];
      layer.push(taskId);
      layers.set(depth, layer);
    }
    const depths = [...layers.keys()].sort((first, second) => first - second);
    for (const layer of layers.values()) layer.sort(compareTasks);
    const buildRanks = () => {
      const ranks = new Map<string, number>();
      for (const depth of depths) {
        layers.get(depth)!.forEach((taskId, index) => ranks.set(taskId, index));
      }
      return ranks;
    };
    const reorderLayer = (depth: number, links: ReadonlyMap<string, string[]>, ranks: ReadonlyMap<string, number>) => {
      const layer = layers.get(depth)!;
      const previousIndex = new Map(layer.map((taskId, index) => [taskId, index]));
      layer.sort((firstId, secondId) => {
        const firstRank = averageRank(firstId, links, ranks);
        const secondRank = averageRank(secondId, links, ranks);
        if (firstRank !== undefined || secondRank !== undefined) {
          if (firstRank === undefined) return 1;
          if (secondRank === undefined) return -1;
          if (firstRank !== secondRank) return firstRank - secondRank;
        }
        return (previousIndex.get(firstId) ?? 0) - (previousIndex.get(secondId) ?? 0) || compareTasks(firstId, secondId);
      });
    };

    let ranks = buildRanks();
    for (let iteration = 0; iteration < 4; iteration += 1) {
      for (const depth of depths) {
        reorderLayer(depth, predecessors, ranks);
        ranks = buildRanks();
      }
      for (const depth of [...depths].reverse()) {
        reorderLayer(depth, successors, ranks);
        ranks = buildRanks();
      }
    }

    const maxLayerCount = Math.max(...[...layers.values()].map((layer) => layer.length));
    const blockHeight = maxLayerCount * NODE_HEIGHT + Math.max(0, maxLayerCount - 1) * TREE_NODE_GAP_Y;
    for (const depth of depths) {
      const layer = layers.get(depth)!;
      const layerHeight = layer.length * NODE_HEIGHT + Math.max(0, layer.length - 1) * TREE_NODE_GAP_Y;
      const layerTop = cursorY + (blockHeight - layerHeight) / 2;
      layer.forEach((taskId, index) => {
        positions.set(taskId, {
          x: depth * (NODE_WIDTH + 70),
          y: layerTop + index * (NODE_HEIGHT + TREE_NODE_GAP_Y),
        });
      });
      contentWidth = Math.max(contentWidth, depth * (NODE_WIDTH + 70) + NODE_WIDTH);
    }
    cursorY += blockHeight + TREE_COMPONENT_GAP_Y;
  }

  return {
    positions,
    contentHeight: Math.max(NODE_HEIGHT, cursorY - (components.length ? TREE_COMPONENT_GAP_Y : 0)),
    contentWidth,
  };
}

async function layoutPhase(
  fixture: WorkspaceFixture,
  phaseId: string,
  taskIds: Set<string>,
  mode: LayoutMode,
): Promise<LocalPhaseLayout> {
  const laneNodes = new Map<string, Map<string, Point>>();
  const laneContentHeights = new Map<string, number>();
  let contentWidth = 0;
  const phaseTasks = fixture.tasks.filter((task) => taskIds.has(task.id) && task.phaseId === phaseId);
  const phaseTaskSet = new Set(phaseTasks.map((task) => task.id));
  const depthByTaskId = new Map(phaseTasks.map((task) => [task.id, 0]));
  const treeDependencies = mode === "tree"
    ? transitiveReduction(
        fixture.dependencies.filter((dependency) => phaseTaskSet.has(dependency.fromTaskId) && phaseTaskSet.has(dependency.toTaskId)),
        (dependency) => dependency.fromTaskId,
        (dependency) => dependency.toTaskId,
      )
    : fixture.dependencies;


  if (mode === "tree") {
    const indegree = new Map(phaseTasks.map((task) => [task.id, 0]));
    const successors = new Map(phaseTasks.map((task) => [task.id, [] as string[]]));
    for (const dependency of treeDependencies) {
      if (!phaseTaskSet.has(dependency.fromTaskId) || !phaseTaskSet.has(dependency.toTaskId)) continue;
      indegree.set(dependency.toTaskId, (indegree.get(dependency.toTaskId) ?? 0) + 1);
      successors.get(dependency.fromTaskId)?.push(dependency.toTaskId);
    }
    const taskOrder = (firstId: string, secondId: string) => {
      const first = phaseTasks.find((task) => task.id === firstId)!;
      const second = phaseTasks.find((task) => task.id === secondId)!;
      return first.publicId - second.publicId || first.id.localeCompare(second.id);
    };
    const queue = phaseTasks
      .filter((task) => indegree.get(task.id) === 0)
      .map((task) => task.id)
      .sort(taskOrder);
    while (queue.length) {
      const taskId = queue.shift()!;
      for (const successorId of [...(successors.get(taskId) ?? [])].sort(taskOrder)) {
        depthByTaskId.set(successorId, Math.max(
          depthByTaskId.get(successorId) ?? 0,
          (depthByTaskId.get(taskId) ?? 0) + 1,
        ));
        const remaining = (indegree.get(successorId) ?? 0) - 1;
        indegree.set(successorId, remaining);
        if (remaining === 0) {
          queue.push(successorId);
          queue.sort(taskOrder);
        }
      }
    }
  }

  for (const lane of fixture.lanes) {
    const tasks = phaseTasks.filter((task) => task.laneId === lane.id);
    if (!tasks.length) continue;
    if (mode === "tree") {
      const treeLayout = layoutTreeLane(tasks, treeDependencies, depthByTaskId);
      laneNodes.set(lane.id, treeLayout.positions);
      laneContentHeights.set(lane.id, treeLayout.contentHeight);
      contentWidth = Math.max(contentWidth, treeLayout.contentWidth);
      continue;
    }

    const taskSet = new Set(tasks.map((task) => task.id));
    const graph = await elk.layout({
      id: `${phaseId}-${lane.id}`,
      layoutOptions: {
        "elk.algorithm": "layered",
        "elk.direction": "RIGHT",
        "elk.spacing.nodeNode": "4",
        "elk.layered.spacing.nodeNodeBetweenLayers": "70",
        "elk.layered.considerModelOrder.strategy": "NONE",
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
      contentHeight = Math.max(contentHeight, (node.y ?? 0) + NODE_HEIGHT);
    });
    for (const point of positions.values()) contentWidth = Math.max(contentWidth, point.x + NODE_WIDTH);
    laneNodes.set(lane.id, positions);
    laneContentHeights.set(lane.id, contentHeight);
  }

  return { phaseId, laneNodes, laneContentHeights, contentWidth };
}

export async function layoutGraph(
  fixture: WorkspaceFixture,
  taskIds: Set<string>,
  reserveBacklogRail = true,
  collapsedPhaseIds: ReadonlySet<string> = new Set(),
  mode: LayoutMode = "flow",
): Promise<GraphLayout> {
  const phases = [...fixture.phases].sort((a, b) => a.order - b.order);
  const localLayouts = await Promise.all(
    phases.map((phase) => layoutPhase(fixture, phase.id, collapsedPhaseIds.has(phase.id) ? new Set() : taskIds, mode)),
  );
  const taskPositions = new Map<string, Point>();
  const summaryPositions = new Map<string, Point>();
  const gatePositions = new Map<string, Point>();
  const phaseRects: LayoutRect[] = [];
  const laneHeights = new Map(fixture.lanes.map((lane) => {
    const contentHeight = Math.max(NODE_HEIGHT, ...localLayouts.map((local) => local.laneContentHeights.get(lane.id) ?? 0));
    return [lane.id, Math.max(
      LANE_HEIGHT,
      contentHeight + (LANE_BAND_INSET_Y + LANE_CONTENT_BREATHING_ROOM_Y) * 2,
    )] as const;
  }));
  const lanePositions = new Map<string, number>();
  let cursorY = PHASE_HEADER_HEIGHT;
  fixture.lanes.forEach((lane) => {
    lanePositions.set(lane.id, cursorY);
    cursorY += laneHeights.get(lane.id) ?? LANE_HEIGHT;
  });
  const height = cursorY + 42;

  // TEMP UI experiment: reserve a phase-free gutter for the lane backlog rail.
  let cursorX = LANE_LABEL_WIDTH + (reserveBacklogRail ? BACKLOG_RAIL_GUTTER_WIDTH : 0);
  localLayouts.forEach((local, phaseIndex) => {
    const collapsed = collapsedPhaseIds.has(local.phaseId);
    const phaseWidth = collapsed ? SUMMARY_NODE_WIDTH + PHASE_PADDING_X * 2 : Math.max(
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

    for (const lane of fixture.lanes) {
      const laneId = lane.id;
      if (collapsed) { const laneY = lanePositions.get(laneId) ?? PHASE_HEADER_HEIGHT; const laneHeight = laneHeights.get(laneId) ?? LANE_HEIGHT; summaryPositions.set(`phase-summary:${local.phaseId}:${laneId}`, { x: cursorX + PHASE_PADDING_X, y: laneY + (laneHeight - SUMMARY_NODE_HEIGHT) / 2 }); continue; }
      const positions = local.laneNodes.get(laneId);
      if (!positions) continue;
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
          x: cursorX + (GATE_CORRIDOR_WIDTH - GATE_NODE_WIDTH) / 2,
          y: PHASE_HEADER_HEIGHT + (height - PHASE_HEADER_HEIGHT - GATE_NODE_HEIGHT) / 2,
        });
      }
      cursorX += GATE_CORRIDOR_WIDTH;
    }
  });

  // A Gate is a relationship junction, so align it to the median of its linked
  // Tasks rather than the geometric center of every Lane. Median placement is
  // stable when an outlying task is added; the viewport center remains a safe
  // fallback when none of its linked Tasks are rendered.
  for (const gate of fixture.gates) {
    const position = gatePositions.get(gate.id);
    if (!position) continue;
    const centers = fixture.gateLinks
      .filter((link) => link.gateId === gate.id)
      .map((link) => taskPositions.get(link.taskId)?.y)
      .filter((y): y is number => y !== undefined)
      .map((y) => y + NODE_HEIGHT / 2)
      .sort((left, right) => left - right);
    const middle = Math.floor(centers.length / 2);
    const centerY = centers.length === 0
      ? PHASE_HEADER_HEIGHT + (height - PHASE_HEADER_HEIGHT) / 2
      : centers.length % 2 === 0
        ? (centers[middle - 1]! + centers[middle]!) / 2
        : centers[middle]!;
    position.y = Math.max(PHASE_HEADER_HEIGHT, Math.min(height - GATE_NODE_HEIGHT - 42, centerY - GATE_NODE_HEIGHT / 2));
  }
  return {
    taskPositions,
    summaryPositions,
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
