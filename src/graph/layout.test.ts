import { describe, expect, it } from "vitest";
import { pilotReadyFixture } from "../fixtures/pilot-ready";
import { BACKLOG_RAIL_GUTTER_WIDTH } from "../components/backlog-rail.config";
import type { WorkspaceFixture } from "../domain/model";
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

  it("keeps the temporary backlog gutter outside the first phase", async () => {
    const layout = await layoutGraph(
      pilotReadyFixture,
      new Set(pilotReadyFixture.tasks.map((task) => task.id)),
    );

    expect(layout.phaseRects[0]?.x).toBe(180 + BACKLOG_RAIL_GUTTER_WIDTH);
  });

  it("does not reserve the temporary backlog gutter when the rail is hidden", async () => {
    const layout = await layoutGraph(
      pilotReadyFixture,
      new Set(pilotReadyFixture.tasks.map((task) => task.id)),
      false,
    );

    expect(layout.phaseRects[0]?.x).toBe(180);
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


  it("aligns a gate with the median center of its linked Tasks", async () => {
    const layout = await layoutGraph(
      pilotReadyFixture,
      new Set(pilotReadyFixture.tasks.map((task) => task.id)),
    );
    const centers = pilotReadyFixture.gateLinks
      .filter((link) => link.gateId === "pilot-ready")
      .map((link) => layout.taskPositions.get(link.taskId)!.y + NODE_HEIGHT / 2)
      .sort((left, right) => left - right);
    const middle = Math.floor(centers.length / 2);
    const expectedCenter = centers.length % 2 === 0
      ? (centers[middle - 1]! + centers[middle]!) / 2
      : centers[middle]!;
    expect(layout.gatePositions.get("pilot-ready")!.y + 94 / 2).toBe(expectedCenter);
  });
  it("keeps sibling tasks in the same layer 4px apart", async () => {
    const layout = await layoutGraph(
      pilotReadyFixture,
      new Set(pilotReadyFixture.tasks.map((task) => task.id)),
    );
    const pilotUi = layout.taskPositions.get("pilot-ui")!;
    const accessibility = layout.taskPositions.get("a11y")!;
    expect(Math.abs(pilotUi.y - accessibility.y) - NODE_HEIGHT).toBe(4);
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

  it("keeps Flow geometry unchanged when the mode is explicit", async () => {
    const visible = new Set(pilotReadyFixture.tasks.map((task) => task.id));
    const implicit = await layoutGraph(pilotReadyFixture, visible);
    const explicit = await layoutGraph(pilotReadyFixture, visible, true, new Set(), "flow");
    expect([...explicit.taskPositions]).toEqual([...implicit.taskPositions]);
    expect(explicit.phaseRects).toEqual(implicit.phaseRects);
    expect([...explicit.lanePositions]).toEqual([...implicit.lanePositions]);
    expect([...explicit.laneHeights]).toEqual([...implicit.laneHeights]);
  });

  it("lays out Tree depth left-to-right and stacks same-depth siblings vertically", async () => {
    const visible = new Set(pilotReadyFixture.tasks.map((task) => task.id));
    const layout = await layoutGraph(pilotReadyFixture, visible, true, new Set(), "tree");
    const parent = layout.taskPositions.get("screen-design")!;
    const firstChild = layout.taskPositions.get("pilot-ui")!;
    const secondChild = layout.taskPositions.get("a11y")!;
    expect(parent.x + NODE_WIDTH).toBeLessThan(firstChild.x);
    expect(firstChild.x).toBe(secondChild.x);
    expect(Math.abs(firstChild.y - secondChild.y)).toBeGreaterThanOrEqual(NODE_HEIGHT);
  });

  it("keeps every Tree task within its original Lane band", async () => {
    const visible = new Set(pilotReadyFixture.tasks.map((task) => task.id));
    const layout = await layoutGraph(pilotReadyFixture, visible, true, new Set(), "tree");
    for (const task of pilotReadyFixture.tasks) {
      const position = layout.taskPositions.get(task.id)!;
      const laneTop = layout.lanePositions.get(task.laneId)!;
      const laneHeight = layout.laneHeights.get(task.laneId)!;
      expect(position.y).toBeGreaterThanOrEqual(laneTop + LANE_BAND_INSET_Y);
      expect(position.y + NODE_HEIGHT).toBeLessThanOrEqual(laneTop + laneHeight - LANE_BAND_INSET_Y);
    }
  });

  it("makes the live DayTripper #30 and #39 dependency shapes legible in Tree mode", async () => {
    const dayTripperSlice: WorkspaceFixture = {
      workspace: { id: "day-tripper", name: "DayTripper", revision: 424, activePhaseId: "foundation-proof" },
      phases: [{ id: "foundation-proof", name: "Foundation Proof", order: 0, state: "active" }],
      lanes: [{ id: "data-pipeline", name: "data-pipeline", goal: "", summary: "", lifecycle: "active" }],
      tasks: [30, 32, 33, 34, 39, 40, 41, 42].map((publicId) => ({
        id: `task-${publicId}`,
        publicId,
        laneId: "data-pipeline",
        phaseId: "foundation-proof",
        title: `Task #${publicId}`,
        description: "",
        status: "pending" as const,
      })),
      dependencies: [
        { id: "30-34", fromTaskId: "task-30", toTaskId: "task-34" },
        { id: "30-32", fromTaskId: "task-30", toTaskId: "task-32" },
        { id: "30-33", fromTaskId: "task-30", toTaskId: "task-33" },
        { id: "39-40", fromTaskId: "task-39", toTaskId: "task-40" },
        { id: "40-41", fromTaskId: "task-40", toTaskId: "task-41" },
        { id: "40-42", fromTaskId: "task-40", toTaskId: "task-42" },
      ],
      backlogItems: [],
      gates: [],
      gateLinks: [],
      decisions: [],
    };
    const visible = new Set(dayTripperSlice.tasks.map((task) => task.id));
    const first = await layoutGraph(dayTripperSlice, visible, true, new Set(), "tree");
    const second = await layoutGraph(dayTripperSlice, visible, true, new Set(), "tree");
    expect([...second.taskPositions]).toEqual([...first.taskPositions]);

    const root30 = first.taskPositions.get("task-30")!;
    const root39 = first.taskPositions.get("task-39")!;
    expect(root30.x).toBe(root39.x);
    const family30 = ["task-30", "task-32", "task-33", "task-34"].map((id) => first.taskPositions.get(id)!);
    const family39 = ["task-39", "task-40", "task-41", "task-42"].map((id) => first.taskPositions.get(id)!);
    expect(Math.max(...family30.map((point) => point.y))).toBeLessThan(Math.min(...family39.map((point) => point.y)));


    for (const [parentId, childIds] of [
      ["task-30", ["task-34", "task-32", "task-33"]],
      ["task-40", ["task-41", "task-42"]],
    ] as const) {
      const parent = first.taskPositions.get(parentId)!;
      const children = childIds.map((id) => first.taskPositions.get(id)!);
      expect(children.every((child) => parent.x + NODE_WIDTH < child.x)).toBe(true);
      expect(new Set(children.map((child) => child.x)).size).toBe(1);
      expect(new Set(children.map((child) => child.y)).size).toBe(children.length);
    }
    const task40 = first.taskPositions.get("task-40")!;
    const task41 = first.taskPositions.get("task-41")!;
    const task42 = first.taskPositions.get("task-42")!;
    const children30 = ["task-32", "task-33", "task-34"].map((id) => first.taskPositions.get(id)!);
    expect(root30.y).toBe(children30[1]!.y);
    expect(root39.y).toBe(task40.y);
    expect(task40.y + NODE_HEIGHT / 2).toBe((task41.y + task42.y + NODE_HEIGHT) / 2);
    expect(root39.x + NODE_WIDTH).toBeLessThan(task40.x);
    expect(task40.x + NODE_WIDTH).toBeLessThan(task41.x);

    const boxes = [...first.taskPositions.entries()].map(([id, point]) => ({ id, ...point, width: NODE_WIDTH, height: NODE_HEIGHT }));
    for (let index = 0; index < boxes.length; index += 1) {
      for (let other = index + 1; other < boxes.length; other += 1) {
        expect(rectanglesOverlap(boxes[index]!, boxes[other]!)).toBe(false);
      }
    }
  });
});
