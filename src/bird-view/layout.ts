import type { Edge, NodeHandle, Position } from "@xyflow/react";
import type { BirdViewFlowNode } from "./BirdViewNode";
import type { BirdViewEdge, BirdViewNode } from "./model";

export const BIRD_NODE_INITIAL_WIDTH = 260;
export const BIRD_NODE_INITIAL_HEIGHT = 142;
const HANDLE_SIZE = 7;
const BIRD_NODE_HANDLES: NodeHandle[] = [
  { id: null, type: "target", position: "left" as Position, x: -HANDLE_SIZE / 2, y: (BIRD_NODE_INITIAL_HEIGHT - HANDLE_SIZE) / 2, width: HANDLE_SIZE, height: HANDLE_SIZE },
  { id: null, type: "source", position: "right" as Position, x: BIRD_NODE_INITIAL_WIDTH - HANDLE_SIZE / 2, y: (BIRD_NODE_INITIAL_HEIGHT - HANDLE_SIZE) / 2, width: HANDLE_SIZE, height: HANDLE_SIZE },
];

export function semanticLevel(zoom: number): "overview" | "detail" {
  return zoom >= 0.72 ? "detail" : "overview";
}

export async function layoutBirdView(nodes: BirdViewNode[], edges: BirdViewEdge[], detailed: boolean): Promise<{ nodes: BirdViewFlowNode[]; edges: Edge[] }> {
  return {
    // The card has a fixed minimum size in CSS. Supplying its initial geometry
    // lets React Flow calculate persisted edges on the first paint, before its
    // ResizeObserver measurement arrives (or when that notification is missed).
    nodes: nodes.map((node) => ({ id: node.id, type: "birdViewNode", position: { x: node.positionX, y: node.positionY }, width: BIRD_NODE_INITIAL_WIDTH, height: BIRD_NODE_INITIAL_HEIGHT, initialWidth: BIRD_NODE_INITIAL_WIDTH, initialHeight: BIRD_NODE_INITIAL_HEIGHT, handles: BIRD_NODE_HANDLES.map((handle) => ({ ...handle })), data: { ...node, detailed } })),
    // Match the Workspace graph exactly: omitting an explicit type selects
    // React Flow's default bezier renderer for dependency and Gate edges.
    edges: edges.map((edge) => ({ id: edge.id, source: edge.fromNodeId, target: edge.toNodeId, label: detailed ? edge.label : undefined })),
  };
}
