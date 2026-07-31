// @vitest-environment jsdom

import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { GraphLayout } from "../graph/layout";
import { LaneAnchorColumn } from "./LaneAnchorColumn";

const layout: GraphLayout = {
  taskPositions: new Map(),
  gatePositions: new Map(),
  phaseRects: [],
  lanePositions: new Map([["adoption", 74], ["art", 228]]),
  laneHeights: new Map([["adoption", 154], ["art", 154]]),
  width: 1000,
  height: 424,
};

const lanes = [
  { id: "adoption", name: "Adoption", goal: "", summary: "", lifecycle: "active" as const },
  { id: "art", name: "Art", goal: "", summary: "", lifecycle: "active" as const },
];

describe("temporary lane anchor column", () => {
  it("renders a column heading without redundant row boundary guides", () => {
    const { container } = render(<LaneAnchorColumn lanes={lanes} layout={layout} />);

    expect(container.querySelector(".lane-anchor-column-heading")?.classList.contains("graph-column-heading")).toBe(true);
    expect(container.textContent).toContain("LANES");
    expect(container.textContent).toContain("2 ROWS");
    expect(container.querySelector(".lane-anchor-column-surface")?.getAttribute("style")).toContain("width: 216px");
    expect(container.querySelectorAll(".lane-anchor-row-slot")).toHaveLength(0);
  });

  it("is decorative and does not add duplicate navigation semantics", () => {
    const { container } = render(<LaneAnchorColumn lanes={lanes} layout={layout} />);

    expect(container.querySelector(".lane-anchor-column-layer")?.getAttribute("aria-hidden")).toBe("true");
  });
});
