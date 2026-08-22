// @vitest-environment jsdom

import React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { fetchGraph } from "./api/client";
import { pilotReadyFixture } from "./fixtures/pilot-ready";
import { layoutGraph, type GraphLayout } from "./graph/layout";
import App from "./App";

const panZoomSetViewport = vi.hoisted(() => vi.fn(() => Promise.resolve({})));
const setViewportState = vi.hoisted(() => vi.fn());
const renderedViewport = vi.hoisted(() => ({ style: { transform: "translate(0px, 0px) scale(1)" } }));
const renderedRenderer = vi.hoisted(() => ({ __zoom: undefined as unknown }));
const canvasDomNode = vi.hoisted(() => ({
  clientWidth: 1200,
  clientHeight: 700,
  querySelector: vi.fn((selector: string) => selector === ".react-flow__viewport" ? renderedViewport : selector === ".react-flow__renderer" ? renderedRenderer : undefined),
}));

vi.mock("./api/client", () => ({ fetchGraph: vi.fn() }));
vi.mock("./graph/layout", () => ({
  NODE_WIDTH: 190,
  NODE_HEIGHT: 110,
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
  useStore: (selector: (state: unknown) => unknown) => selector({ transform: [0, 0, 1], minZoom: 0.55, maxZoom: 1.55, width: 0, height: 0, domNode: canvasDomNode, panZoom: { setViewport: panZoomSetViewport } }),
  useStoreApi: () => ({ getState: () => ({ transform: [0, 0, 1], minZoom: 0.55, maxZoom: 1.55, width: 0, height: 0, domNode: canvasDomNode, nodeLookup: new Map(pilotReadyFixture.tasks.map((task) => [task.id, {}])), panZoom: { setViewport: panZoomSetViewport } }), setState: setViewportState }),
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
    vi.stubEnv("VITE_BALEY_AUTH_MODE", "legacy");
    vi.stubEnv("VITE_BALEY_WORKSPACE_ID", pilotReadyFixture.workspace.id);
    window.history.replaceState({}, "", "/workspaces/pilot/lanes/client?task=pilot-ui");
    vi.mocked(fetchGraph).mockResolvedValue(pilotReadyFixture);
    vi.mocked(layoutGraph).mockResolvedValue(backlogLayout);
    panZoomSetViewport.mockClear();
    setViewportState.mockClear();
    renderedViewport.style.transform = "translate(0px, 0px) scale(1)";
    window.localStorage.clear();
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

  it("defaults to Flow and persists a Tree selection for the Workspace", async () => {
    render(<App />);
    const flow = await screen.findByRole("button", { name: "Flow" });
    const tree = screen.getByRole("button", { name: "Tree" });
    expect(flow.getAttribute("aria-pressed")).toBe("true");
    expect(tree.getAttribute("aria-pressed")).toBe("false");

    fireEvent.click(tree);

    await waitFor(() => expect(tree.getAttribute("aria-pressed")).toBe("true"));
    expect(window.localStorage.getItem("baley:layout-mode:pilot")).toBe("tree");
    await waitFor(() => expect(layoutGraph).toHaveBeenLastCalledWith(
      expect.anything(),
      expect.any(Set),
      true,
      expect.any(Set),
      "tree",
    ));
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

  it("opens the full backlog from the header and shows the selected item in Inspector", async () => {
    const backlogItem = { id: "backlog-brief", publicId: 1, laneId: "client", title: "Pilot backlog brief", description: "Review the pilot backlog.", status: "active" as const, position: 1, promotedTaskId: null, promotedTaskPublicId: null };
    vi.mocked(fetchGraph).mockResolvedValue({ ...pilotReadyFixture, backlogItems: [backlogItem] });
    render(<App />);

    fireEvent.click(await screen.findByRole("button", { name: "Open full backlog" }));
    fireEvent.click(await screen.findByRole("button", { name: "Open backlog B#1" }));

    expect(await screen.findByText("BACKLOG INSPECTOR")).toBeTruthy();
    expect(screen.getByRole("heading", { name: backlogItem.title })).toBeTruthy();
    await waitFor(() => expect(window.location.search).toBe(`?backlog=${backlogItem.id}`));
  });

  it("opens the full backlog and Inspector together from a rail item", async () => {
    const backlogItem = { id: "backlog-rail", publicId: 2, laneId: "client", title: "Rail backlog brief", description: "Open from the rail.", status: "active" as const, position: 1, promotedTaskId: null, promotedTaskPublicId: null };
    vi.mocked(fetchGraph).mockResolvedValue({ ...pilotReadyFixture, backlogItems: [backlogItem] });
    render(<App />);

    fireEvent.click(await screen.findByRole("button", { name: "Open backlog B#2" }));

    expect(await screen.findByRole("heading", { name: "Lane backlogs" })).toBeTruthy();
    expect(screen.getByText("BACKLOG INSPECTOR")).toBeTruthy();
    await waitFor(() => expect(window.location.search).toBe(`?backlog=${backlogItem.id}`));
  });

  it("exposes each lane anchor as a native navigation button", async () => {
    render(<App />);
    const artLane = await screen.findByRole("button", { name: "Open Art lane" });

    expect(artLane.tagName).toBe("BUTTON");
    fireEvent.click(artLane);
    await waitFor(() => expect(window.location.pathname).toBe("/workspaces/pilot/lanes/art"));
  });

  it("shows a public-ID result and focuses the clicked Task", async () => {
    const pilotTask = pilotReadyFixture.tasks.find((task) => task.publicId === 104)!;
    vi.mocked(layoutGraph).mockResolvedValue({
      ...backlogLayout,
      taskPositions: new Map([[pilotTask.id, { x: 500, y: 240 }]]),
    });
    render(<App />);

    const search = await screen.findByRole("combobox", { name: "Task 검색" });
    fireEvent.change(search, { target: { value: "#104" } });
    fireEvent.click(await screen.findByRole("option", { name: /#104.*Pilot UI/ }));

    await waitFor(() => expect(window.location.pathname + window.location.search).toBe(`/workspaces/pilot?task=${pilotTask.id}`));
    await waitFor(() => expect(panZoomSetViewport).toHaveBeenCalledWith({ x: 5, y: 55, zoom: 1 }, { duration: 0 }));
    expect(setViewportState).toHaveBeenCalledWith({ transform: [5, 55, 1] });
    expect(renderedViewport.style.transform).toBe("translate(5px, 55px) scale(1)");
  });
});
