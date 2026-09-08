import { describe, expect, it } from "vitest";
import { layoutBirdView, semanticLevel } from "./layout";

describe("Bird View semantic layouts", () => {
  it("switches at the explicit two-level zoom boundary", () => {
    expect(semanticLevel(0.4)).toBe("overview");
    expect(semanticLevel(0.72)).toBe("detail");
  });

  it("lays out cyclic Bird View edges without treating them as a Task DAG", async () => {
    const nodes = ["a", "b"].map((id, index) => ({ id, title: id, summary: "", content: "", status: "active" as const, positionX: index * 320, positionY: index * 120, createdAt: "", updatedAt: "" }));
    const edges = [
      { id: "ab", fromNodeId: "a", toNodeId: "b", label: "leads", createdAt: "" },
      { id: "ba", fromNodeId: "b", toNodeId: "a", label: "informs", createdAt: "" },
    ];
    const result = await layoutBirdView(nodes, edges, true);
    expect(result.nodes).toHaveLength(2);
    expect(result.nodes[0]).toMatchObject({ width: 260, height: 142, initialWidth: 260, initialHeight: 142, handles: [{ type: "target" }, { type: "source" }] });
    expect(result.edges.map((edge) => [edge.source, edge.target])).toEqual([["a", "b"], ["b", "a"]]);
    expect(result.edges.every((edge) => edge.type === undefined)).toBe(true);
  });
});
