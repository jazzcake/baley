import { describe, expect, it } from "vitest";
import { pilotReadyFixture } from "../fixtures/pilot-ready";
import { layoutGraph, NODE_HEIGHT, NODE_WIDTH, rectanglesOverlap } from "./layout";

describe("phase-aware graph layout", () => {
  it("keeps task nodes inside their phase containers without overlap", async () => {
    const visible = new Set(pilotReadyFixture.tasks.map((task) => task.id));
    const layout = await layoutGraph(pilotReadyFixture, visible);
    const boxes = pilotReadyFixture.tasks.map((task) => {
      const point = layout.taskPositions.get(task.id);
      const phase = layout.phaseRects.find((rect) => rect.id === task.phaseId);
      expect(point).toBeDefined();
      expect(phase).toBeDefined();
      expect(point!.x).toBeGreaterThanOrEqual(phase!.x);
      expect(point!.x + NODE_WIDTH).toBeLessThanOrEqual(phase!.x + phase!.width);
      return { id: task.id, x: point!.x, y: point!.y, width: NODE_WIDTH, height: NODE_HEIGHT };
    });

    for (let index = 0; index < boxes.length; index += 1) {
      for (let other = index + 1; other < boxes.length; other += 1) {
        expect(rectanglesOverlap(boxes[index]!, boxes[other]!)).toBe(false);
      }
    }
  });

  it("places every gate in the empty corridor between phases", async () => {
    const layout = await layoutGraph(
      pilotReadyFixture,
      new Set(pilotReadyFixture.tasks.map((task) => task.id)),
    );
    const build = layout.phaseRects.find((phase) => phase.id === "build")!;
    const validate = layout.phaseRects.find((phase) => phase.id === "validate")!;
    const gate = layout.gatePositions.get("pilot-ready")!;
    expect(gate.x).toBeGreaterThanOrEqual(build.x + build.width);
    expect(gate.x + 210).toBeLessThanOrEqual(validate.x);
  });
});
