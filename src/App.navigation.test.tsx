// @vitest-environment jsdom

import React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { fetchGraph } from "./api/client";
import { pilotReadyFixture } from "./fixtures/pilot-ready";
import { layoutGraph, type GraphLayout } from "./graph/layout";
import App from "./App";

vi.mock("./api/client", () => ({ fetchGraph: vi.fn() }));
vi.mock("./graph/layout", () => ({
  laneBandRect: vi.fn(),
  laneLabelTop: vi.fn(),
  layoutGraph: vi.fn(async () => undefined),
}));
vi.mock("@xyflow/react", () => ({
  Background: () => null,
  Panel: ({ children, ...props }: { children: React.ReactNode }) => React.createElement("div", props, children),
  ReactFlow: ({ children, viewport, fitView, panOnDrag }: { children: React.ReactNode; viewport?: unknown; fitView?: unknown; panOnDrag?: boolean }) => React.createElement("div", { "data-testid": "graph", "data-controlled": String(Boolean(viewport)), "data-auto-fit": String(Boolean(fitView)), "data-drag-disabled": String(panOnDrag === false) }, children),
  ViewportPortal: ({ children }: { children: React.ReactNode }) => React.createElement(React.Fragment, null, children),
  useReactFlow: () => ({ zoomIn: vi.fn(), zoomOut: vi.fn(), fitView: vi.fn() }),
  useStore: (selector: (state: unknown) => unknown) => selector({ transform: [0, 0, 1], minZoom: 0.55, maxZoom: 1.55 }),
  useStoreApi: () => ({ getState: () => ({ transform: [0, 0, 1], minZoom: 0.55, maxZoom: 1.55, width: 1200, height: 700, panZoom: { setViewport: vi.fn() } }) }),
}));

describe("Home navigation entry points", () => {
  const backlogLayout: GraphLayout = {
    taskPositions: new Map(),
    gatePositions: new Map(),
    phaseRects: [],
    lanePositions: new Map(pilotReadyFixture.lanes.map((lane, index) => [lane.id, 74 + index * 154])),
    laneHeights: new Map(pilotReadyFixture.lanes.map((lane) => [lane.id, 154])),
    width: 1200,
    height: 740,
  };

  beforeEach(() => {
    vi.stubEnv("VITE_BALEY_WORKSPACE_ID", pilotReadyFixture.workspace.id);
    window.history.replaceState({}, "", "/workspaces/pilot/lanes/client?task=pilot-ui");
    vi.mocked(fetchGraph).mockResolvedValue(pilotReadyFixture);
    vi.mocked(layoutGraph).mockResolvedValue(backlogLayout);
  });

  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
    vi.unstubAllEnvs();
  });

  it("navigates to Home from the Baley logo", async () => {
    render(<App />);
    fireEvent.click(await screen.findByRole("button", { name: "Go to Home" }));
    await waitFor(() => expect(window.location.pathname + window.location.search).toBe("/workspaces/pilot"));
    expect(screen.getByRole("heading", { name: pilotReadyFixture.workspace.name })).toBeTruthy();
  });

  it("navigates to Workspace Home from the workspace label", async () => {
    render(<App />);
    fireEvent.click(await screen.findByRole("button", { name: "Go to Workspace Home" }));
    await waitFor(() => expect(window.location.pathname + window.location.search).toBe("/workspaces/pilot"));
    expect(screen.getByRole("heading", { name: pilotReadyFixture.workspace.name })).toBeTruthy();
  });

  it("uses React Flow's native uncontrolled draggable viewport", async () => {
    render(<App />);
    const canvas = await screen.findByTestId("graph");
    expect(canvas.getAttribute("data-controlled")).toBe("false");
    expect(canvas.getAttribute("data-auto-fit")).toBe("false");
    expect(canvas.getAttribute("data-drag-disabled")).toBe("false");
    expect(screen.getByLabelText("Viewport controls")).toBeTruthy();
  });

  it("keeps the graph mounted and restores focus after backlog list mode", async () => {
    render(<App />);
    const graph = await screen.findByTestId("graph");
    const openButton = await screen.findByRole("button", { name: "Open backlog list" });

    fireEvent.click(openButton);
    expect(await screen.findByRole("heading", { name: "Lane backlogs" })).toBeTruthy();
    expect(screen.getByTestId("graph")).toBe(graph);
    expect(graph.parentElement?.getAttribute("inert")).not.toBeNull();
    expect(document.activeElement).toBe(screen.getByRole("button", { name: "Back to graph" }));

    fireEvent.click(screen.getByRole("button", { name: "Back to graph" }));
    await waitFor(() => expect(screen.queryByRole("heading", { name: "Lane backlogs" })).toBeNull());
    await waitFor(() => expect(document.activeElement).toBe(openButton));
    expect(screen.getByTestId("graph")).toBe(graph);
  });

  it("exposes each lane anchor as a native navigation button", async () => {
    render(<App />);
    const artLane = await screen.findByRole("button", { name: "Open Art lane" });

    expect(artLane.tagName).toBe("BUTTON");
    fireEvent.click(artLane);
    await waitFor(() => expect(window.location.pathname).toBe("/workspaces/pilot/lanes/art"));
  });
});
