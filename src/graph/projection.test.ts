import { describe, expect, it } from "vitest";
import { pilotReadyFixture } from "../fixtures/pilot-ready";
import { canvasKey, connectedTaskIds, defaultGateFocusId, laneFocusTaskIds, visibleTaskIds } from "./projection";

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
        { id: "release-ready", name: "Release Ready", fromPhaseId: "release", toPhaseId: "ship", status: "open" as const },
        { id: "validate-ready", name: "Validate Ready", fromPhaseId: "validate", toPhaseId: "release", status: "open" as const },
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
});
