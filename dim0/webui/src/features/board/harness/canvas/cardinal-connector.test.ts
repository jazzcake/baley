import { describe, expect, it } from "vitest"
import { asClientId, asNodeId, createCanvasStore, type Node } from "@canvas-harness/core"
import {
  CARDINAL_CONNECTOR_SIDES,
  CARDINAL_CONNECTOR_OUTWARD_OFFSET_PX,
  cardinalConnectorAngle,
  cardinalConnectorScreenOffset,
  cardinalConnectorSource,
  connectorEndFromWorldPoint,
} from "./cardinal-connector"


const node: Node = {
  id: asNodeId("source"),
  type: "rect",
  x: 100,
  y: 200,
  w: 80,
  h: 40,
  angle: 0,
  z: 0,
  groups: [],
}


describe("cardinal connector handles", () => {
  it("exposes only the four cardinal sides", () => {
    expect(CARDINAL_CONNECTOR_SIDES).toEqual(["n", "e", "s", "w"])
  })

  it.each([
    ["n", { x: 40, y: 0 }],
    ["e", { x: 80, y: 20 }],
    ["s", { x: 40, y: 40 }],
    ["w", { x: 0, y: 20 }],
  ] as const)("anchors %s exactly on the node boundary", (side, localOffset) => {
    expect(cardinalConnectorSource(node, side)).toEqual({
      nodeId: node.id,
      localOffset,
    })
  })

  it("rotates each triangle toward its side", () => {
    expect(CARDINAL_CONNECTOR_SIDES.map(cardinalConnectorAngle)).toEqual([0, 90, 180, -90])
  })

  it("moves cardinal triangles six screen pixels outside the selection line", () => {
    expect(CARDINAL_CONNECTOR_OUTWARD_OFFSET_PX).toBe(6)
    expect(cardinalConnectorScreenOffset("n", 0)).toEqual({ x: 0, y: -6 })
    expect(cardinalConnectorScreenOffset("e", 0).x).toBeCloseTo(6)
    expect(cardinalConnectorScreenOffset("e", 0).y).toBeCloseTo(0)
    expect(cardinalConnectorScreenOffset("s", 0).y).toBeCloseTo(6)
    expect(cardinalConnectorScreenOffset("w", 0).x).toBeCloseTo(-6)
  })

  it("rotates the outward offset with the node", () => {
    const offset = cardinalConnectorScreenOffset("n", Math.PI / 2)
    expect(offset.x).toBeCloseTo(6)
    expect(offset.y).toBeCloseTo(0)
  })

  it("attaches a target hit to its nearest node boundary", () => {
    const store = createCanvasStore({ clientId: asClientId("test") })
    store.addNode(node)

    expect(connectorEndFromWorldPoint(store, { x: 120, y: 210 })).toEqual({
      end: { nodeId: node.id, localOffset: { x: 20, y: 0 } },
      nodeId: node.id,
    })
  })

  it("keeps an empty target as a free world point", () => {
    const store = createCanvasStore({ clientId: asClientId("test") })
    const world = { x: 500, y: 600 }

    expect(connectorEndFromWorldPoint(store, world)).toEqual({
      end: { worldPoint: world },
      nodeId: null,
    })
  })
})
