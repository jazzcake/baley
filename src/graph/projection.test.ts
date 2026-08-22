import { describe, expect, it } from "vitest";
import { pilotReadyFixture } from "../fixtures/pilot-ready";
import { canvasKey, connectedTaskIds, defaultGateFocusId, laneFocusTaskIds, phasePresentation, projectDependencies, transitiveReduction, visibleTaskIds } from "./projection";

describe("graph projection", () => {
  it("preserves the same canvas between multi-lane and lane focus", () => {
    expect(canvasKey({ kind: "multi" })).toBe("workspace");
    expect(canvasKey({ kind: "lane", id: "client" })).toBe("workspace");
    expect(canvasKey({ kind: "gate", id: "pilot-ready" })).toBe("gate:pilot-ready");
  });
  it("keeps every task in the multi-lane view", () => {
    expect(visibleTaskIds(pilotReadyFixture, { kind: "multi" }).size).toBe(pilotReadyFixture.tasks.length);
  });

  it("keeps the full graph in lane focus so the renderer can dim other lanes", () => {
    const ids = visibleTaskIds(pilotReadyFixture, { kind: "lane", id: "client" });
    expect(ids.size).toBe(pilotReadyFixture.tasks.length);
  });

  it("keeps only tasks attached to the focused gate", () => {
    const ids = visibleTaskIds(pilotReadyFixture, { kind: "gate", id: "pilot-ready" });
    expect(ids).toEqual(new Set(["api-build", "pilot-ui", "assets", "findings", "user-test"]));
  });

  it("selects the active Phase outgoing Gate instead of an earlier future Gate", () => {
    const fixture = {
      ...pilotReadyFixture,
      workspace: { ...pilotReadyFixture.workspace, activePhaseId: "validate" },
      gates: [
        { id: "release-ready", publicId: 3, name: "Release Ready", fromPhaseId: "release", toPhaseId: "ship", status: "open" as const },
        { id: "validate-ready", publicId: 2, name: "Validate Ready", fromPhaseId: "validate", toPhaseId: "release", status: "open" as const },
      ],
    };
    expect(defaultGateFocusId(fixture)).toBe("validate-ready");
  });

  it("finds upstream and downstream dependencies", () => {
    expect(connectedTaskIds(pilotReadyFixture, "screen-design")).toEqual(new Set(["screen-design", "pilot-ui", "a11y"]));
  });

  it("only highlights tasks owned by the focused lane", () => {
    expect([...laneFocusTaskIds(pilotReadyFixture, "server")]).toEqual([
      "api-design",
      "api-build",
    ]);
  });
  it("restores task-to-task dependency endpoints when a collapsed phase is expanded", () => {
    const fixture = {
      ...pilotReadyFixture,
      phases: pilotReadyFixture.phases.map((phase) => phase.id === "build" ? { ...phase, state: "completed" as const } : phase),
      dependencies: [...pilotReadyFixture.dependencies, { id: "d6", fromTaskId: "api-build", toTaskId: "user-test" }],
    };
    const visible = new Set(fixture.tasks.map((task) => task.id));
    const collapsed = phasePresentation(fixture, new Set(["build"]));
    const expanded = phasePresentation(fixture, new Set());

    expect(projectDependencies(fixture.dependencies, visible, collapsed)).toContainEqual(expect.objectContaining({ source: "phase-summary:build:server", target: "user-test" }));
    expect(projectDependencies(fixture.dependencies, visible, expanded)).toContainEqual(expect.objectContaining({ source: "api-build", target: "user-test" }));
  });

  it("hides direct dependency edges when a longer visible path preserves reachability", () => {
    const edges = [
      { id: "133-136", source: "133", target: "136" },
      { id: "136-123", source: "136", target: "123" },
      { id: "133-123", source: "133", target: "123" },
      { id: "134-136", source: "134", target: "136" },
      { id: "134-123", source: "134", target: "123" },
      { id: "standalone", source: "other", target: "123" },
    ];

    expect(transitiveReduction(edges, (edge) => edge.source, (edge) => edge.target).map((edge) => edge.id)).toEqual([
      "133-136",
      "136-123",
      "134-136",
      "standalone",
    ]);
  });

});
