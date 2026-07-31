// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { GraphLayout } from "../graph/layout";
import { BacklogList, BacklogRail } from "./BacklogRail";

const layout: GraphLayout = {
  taskPositions: new Map(),
  gatePositions: new Map(),
  phaseRects: [],
  lanePositions: new Map([["client", 74], ["server", 228]]),
  laneHeights: new Map([["client", 154], ["server", 154]]),
  width: 1000,
  height: 424,
};

const lanes = [
  { id: "client", name: "Client", goal: "", summary: "", lifecycle: "active" as const },
  { id: "server", name: "Server", goal: "", summary: "", lifecycle: "active" as const },
];
const items = [
  { id: "b1", publicId: 1, laneId: "client", title: "First", description: "", status: "active" as const, position: 1, promotedTaskId: null, promotedTaskPublicId: null },
  { id: "b2", publicId: 2, laneId: "client", title: "Second", description: "", status: "active" as const, position: 2, promotedTaskId: null, promotedTaskPublicId: null },
  { id: "b3", publicId: 3, laneId: "client", title: "Third", description: "", status: "active" as const, position: 3, promotedTaskId: null, promotedTaskPublicId: null },
  { id: "b4", publicId: 4, laneId: "server", title: "Server", description: "", status: "active" as const, position: 1, promotedTaskId: null, promotedTaskPublicId: null },
];

describe("live lane backlog rail", () => {
  afterEach(cleanup);

  it("renders ordered phase-unassigned items for every lane", () => {
    const { container } = render(<BacklogRail lanes={lanes} items={items} layout={layout} laneColors={{ client: "#579bfc", server: "#00a887" }} onExpand={() => undefined} />);

    expect(container.querySelector(".backlog-rail-column-heading")?.classList.contains("graph-column-heading")).toBe(true);
    expect(container.querySelector("[data-lane-id='client']")).toBeTruthy();
    expect(container.querySelector("[data-lane-id='server']")).toBeTruthy();
    expect(container.textContent).toContain("PHASE 미정");
    expect(container.querySelectorAll(".backlog-rail-item")).toHaveLength(3);
    expect(container.textContent).toContain("+1 MORE");
    expect(container.textContent).toContain("B#1");
  });

  it("dims rails outside the focused lane", () => {
    const { container } = render(<BacklogRail lanes={lanes} items={items} layout={layout} focusedLaneId="client" laneColors={{}} onExpand={() => undefined} />);

    expect(container.querySelector("[data-lane-id='client']")?.classList.contains("dimmed")).toBe(false);
    expect(container.querySelector("[data-lane-id='server']")?.classList.contains("dimmed")).toBe(true);
  });

  it("opens the expanded backlog list from the rail heading", () => {
    const onExpand = vi.fn();
    render(<BacklogRail lanes={lanes} items={items} layout={layout} laneColors={{}} onExpand={onExpand} />);

    fireEvent.click(screen.getByRole("button", { name: "Open backlog list" }));
    expect(onExpand).toHaveBeenCalledOnce();
  });

  it("renders every live item by lane in list mode and returns to the graph", () => {
    const onClose = vi.fn();
    render(<BacklogList lanes={lanes} items={items} laneColors={{}} onClose={onClose} />);

    expect(screen.getByRole("heading", { name: "Lane backlogs" })).toBeTruthy();
    expect(screen.getByRole("heading", { name: "Client" })).toBeTruthy();
    expect(screen.getAllByRole("listitem")).toHaveLength(items.length);
    expect(document.activeElement).toBe(screen.getByRole("button", { name: "Back to graph" }));
    fireEvent.keyDown(screen.getByRole("button", { name: "Back to graph" }), { key: "Escape" });
    expect(onClose).toHaveBeenCalledOnce();
  });

  it("closes list mode from its button", () => {
    const onClose = vi.fn();
    render(<BacklogList lanes={lanes} items={items} laneColors={{}} onClose={onClose} />);

    fireEvent.click(screen.getByRole("button", { name: "Back to graph" }));
    expect(onClose).toHaveBeenCalledOnce();
  });
});
