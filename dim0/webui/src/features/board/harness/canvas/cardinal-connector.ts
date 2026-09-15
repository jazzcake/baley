import {
  hitTestPoint,
  projectToNodeBoundary,
  type CanvasStore,
  type EdgeEnd,
  type Node,
  type NodeId,
  type Vec2,
} from "@canvas-harness/core"


export const CARDINAL_CONNECTOR_SIDES = ["n", "e", "s", "w"] as const

export type CardinalConnectorSide = (typeof CARDINAL_CONNECTOR_SIDES)[number]


/**
 * Return the node-local anchor for a cardinal connector handle.
 * Local coordinates keep the attachment exact when a node is rotated.
 */
export const cardinalConnectorSource = (
  node: Node,
  side: CardinalConnectorSide,
): EdgeEnd => {
  const offsets: Record<CardinalConnectorSide, Vec2> = {
    n: { x: node.w / 2, y: 0 },
    e: { x: node.w, y: node.h / 2 },
    s: { x: node.w / 2, y: node.h },
    w: { x: 0, y: node.h / 2 },
  }

  return { nodeId: node.id, localOffset: offsets[side] }
}


/**
 * Resolve a connector target like canvas-harness's Arrow tool: attach to a
 * node boundary when the pointer is over a node, otherwise keep a world point.
 */
export const connectorEndFromWorldPoint = (
  store: CanvasStore,
  world: Vec2,
): { end: EdgeEnd; nodeId: NodeId | null } => {
  const hit = hitTestPoint(store, world, store.getCamera().z)
  if (hit?.kind === "body") {
    const node = store.getNode(hit.nodeId)
    if (node) {
      return {
        end: { nodeId: node.id, localOffset: projectToNodeBoundary(world, node) },
        nodeId: node.id,
      }
    }
  }

  return { end: { worldPoint: world }, nodeId: null }
}


/** Direction of the visual triangle before applying the node's rotation. */
export const cardinalConnectorAngle = (side: CardinalConnectorSide): number => {
  const angles: Record<CardinalConnectorSide, number> = {
    n: 0,
    e: 90,
    s: 180,
    w: -90,
  }
  return angles[side]
}
