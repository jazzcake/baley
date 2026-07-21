import { describe, expect, it } from "vitest";
import { pilotReadyFixture } from "../fixtures/pilot-ready";
import { laneBandRect, laneLabelTop, layoutGraph, LANE_BAND_INSET_Y, LANE_CONTENT_BREATHING_ROOM_Y, LANE_HEIGHT, LANE_LABEL_HEIGHT, NODE_HEIGHT, NODE_WIDTH, rectanglesOverlap } from "./layout";

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

    for (const task of pilotReadyFixture.tasks) {
      const point = layout.taskPositions.get(task.id)!;
      const laneTop = layout.lanePositions.get(task.laneId)!;
      const laneHeight = layout.laneHeights.get(task.laneId)!;
      expect(point.y).toBeGreaterThanOrEqual(laneTop + LANE_BAND_INSET_Y + LANE_CONTENT_BREATHING_ROOM_Y);
      expect(point.y + NODE_HEIGHT).toBeLessThanOrEqual(laneTop + laneHeight - LANE_BAND_INSET_Y - LANE_CONTENT_BREATHING_ROOM_Y);
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

  it("projects a focused lane band across the full virtual canvas", async () => {
    const layout = await layoutGraph(
      pilotReadyFixture,
      new Set(pilotReadyFixture.tasks.map((task) => task.id)),
    );
    const band = laneBandRect(layout, "client")!;
    expect(band).toEqual({
      id: "client",
      x: 0,
      y: layout.lanePositions.get("client")! + LANE_BAND_INSET_Y,
      width: layout.width,
      height: layout.laneHeights.get("client")! - LANE_BAND_INSET_Y * 2,
    });
    expect(band.height).toBeGreaterThanOrEqual(
      LANE_CONTENT_BREATHING_ROOM_Y * 2 + NODE_HEIGHT,
    );
    expect(laneLabelTop(layout, "client")).toBe(
      layout.lanePositions.get("client")! + (layout.laneHeights.get("client")! - LANE_LABEL_HEIGHT) / 2,
    );
    expect(layout.laneHeights.get("client")).toBeGreaterThan(LANE_HEIGHT);
  });
});
