// @vitest-environment jsdom

import React from "react";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { applyNodeChanges } from "@xyflow/react";
import { BIRD_VIEW_PLANNING_GRID, BirdViewRoutes } from "./BirdViewRoutes";
import { executeBirdViewCommand, fetchBirdViewGraph, fetchBirdViewNodeFocus, listBirdViews } from "./api";
import type { BirdViewNodeFocus } from "./model";

vi.mock("../auth/AuthProvider", () => ({ useAuth: () => ({ state: { status: "authenticated", account: { actorId: "actor" }, csrfToken: "csrf", memberships: [] } }) }));
vi.mock("./api", () => ({ listBirdViews: vi.fn(), fetchBirdViewGraph: vi.fn(), fetchBirdViewNodeFocus: vi.fn(), executeBirdViewCommand: vi.fn() }));

const renderedFlows: Array<Record<string, unknown>> = [];
const renderedBackgrounds: Array<Record<string, unknown>> = [];
let flowMountCount = 0;
let flowUnmountCount = 0;

vi.mock("@xyflow/react", () => ({
  Background: (props: Record<string, unknown>) => { renderedBackgrounds.push(props); return <svg className={String(props.className ?? "")} data-testid="flow-background" data-pattern-id={String(props.id ?? "")} data-variant={String(props.variant ?? "dots")} />; },
  BackgroundVariant: { Lines: "lines", Dots: "dots", Cross: "cross" },
  Controls: () => null,
  MiniMap: () => null,
  Handle: ({ type }: { type: string }) => <i data-handle={type} />,
  Position: { Left: "left", Right: "right" },
  applyNodeChanges: vi.fn((_changes: unknown, nodes: unknown) => nodes),
  applyEdgeChanges: (_changes: unknown, edges: unknown) => edges,
  ReactFlow: function MockReactFlow({ children, ...props }: { children: React.ReactNode } & Record<string, unknown>) {
    renderedFlows.push(props);
    React.useEffect(() => { flowMountCount += 1; return () => { flowUnmountCount += 1; }; }, []);
    return <div data-testid="bird-flow">{children}</div>;
  },
}));

type RenderedNode = { id: string; type?: string; position: { x: number; y: number }; width?: number; height?: number; className?: string; data: { label?: string; kind?: string } };
type RenderedEdge = { id: string; source: string; target: string; className?: string; data?: { relation?: string; locked?: boolean } };

const graph = {
  birdView: { id: "v", title: "Map", description: "", status: "active" as const, revision: 4, createdAt: "", updatedAt: "" },
  nodes: [
    { id: "a", title: "Outcome A", summary: "Summary A", content: "", status: "active" as const, positionX: 120, positionY: 240, createdAt: "", updatedAt: "" },
    { id: "b", title: "Outcome B", summary: "Summary B", content: "", status: "active" as const, positionX: 520, positionY: 260, createdAt: "", updatedAt: "" },
  ],
  edges: [{ id: "overview-edge", fromNodeId: "a", toNodeId: "b", label: "supports", createdAt: "" }],
};

const focusPayload: BirdViewNodeFocus = {
  birdView: graph.birdView,
  node: graph.nodes[0]!,
  workspaces: [{
    workspace: { id: "w", name: "Workspace", state: "active", revision: 1, activePhaseId: "build" },
    phases: [
      { id: "build", name: "Build", state: "active", position: 1 },
      { id: "validate", name: "Validate", state: "planned", position: 2 },
      { id: "unrelated-phase", name: "Unrelated phase", state: "planned", position: 3 },
    ],
    lanes: [{ id: "lane", name: "Client", state: "active" }],
    tasks: [
      { id: "condition", publicId: 7, laneId: "lane", phaseId: "build", title: "Bound condition", description: "", status: "confirmed" },
      { id: "entry", publicId: 8, laneId: "lane", phaseId: "validate", title: "Bound entry", description: "", status: "pending" },
      { id: "unbound", publicId: 9, laneId: "lane", phaseId: "unrelated-phase", title: "Unbound task", description: "", status: "pending" },
    ],
    dependencies: [
      { fromTaskId: "condition", toTaskId: "entry" },
      { fromTaskId: "condition", toTaskId: "unbound" },
    ],
    gates: [
      { id: "ready", publicId: 4, name: "Release ready", fromPhaseId: "build", toPhaseId: "validate", status: "open", conditions: [{ taskId: "condition" }, { taskId: "unbound" }], entryTasks: [{ taskId: "entry" }, { taskId: "unbound" }] },
      { id: "implicit", publicId: 5, name: "Adjacent only", fromPhaseId: "validate", toPhaseId: "unrelated-phase", status: "open", conditions: [{ taskId: "entry" }], entryTasks: [] },
    ],
    backlogItems: [{ id: "backlog", publicId: 3, laneId: "lane", title: "Unbound backlog", description: "", status: "active", position: 1 }],
    bindings: [
      { nodeId: "a", workspaceId: "w", targetId: "condition", targetType: "task", state: "pinned", targetTitle: "Bound condition", targetStatus: "confirmed" },
      { nodeId: "a", workspaceId: "w", targetId: "entry", targetType: "task", state: "suggested", targetTitle: "Bound entry", targetStatus: "pending" },
      { nodeId: "a", workspaceId: "w", targetId: "ready", targetType: "gate", state: "pinned", targetTitle: "Release ready", targetStatus: "open", targetPublicId: 4 },
    ],
  }],
};

let animationFrameId = 0;
const animationFrames = new Map<number, FrameRequestCallback>();
const currentFlow = () => renderedFlows[renderedFlows.length - 1]!;
const currentNodes = () => currentFlow().nodes as RenderedNode[];
const currentEdges = () => currentFlow().edges as RenderedEdge[];
const focusPhase = () => screen.getByTestId("bird-flow").parentElement?.getAttribute("data-focus-phase");
const renderRoutes = (path: string) => render(<MemoryRouter initialEntries={[path]}><Routes><Route path="/bird-views/*" element={<BirdViewRoutes />} /></Routes></MemoryRouter>);

beforeEach(() => {
  vi.stubGlobal("requestAnimationFrame", vi.fn((callback: FrameRequestCallback) => { const id = ++animationFrameId; animationFrames.set(id, callback); return id; }));
  vi.stubGlobal("cancelAnimationFrame", vi.fn((id: number) => { animationFrames.delete(id); }));
});

afterEach(() => {
  cleanup();
  renderedFlows.length = 0;
  renderedBackgrounds.length = 0;
  animationFrames.clear();
  animationFrameId = 0;
  flowMountCount = 0;
  flowUnmountCount = 0;
  vi.clearAllMocks();
  vi.useRealTimers();
  vi.unstubAllGlobals();
  delete (window as Window & { __BALEY_BIRD_VIEW_TRACE__?: unknown }).__BALEY_BIRD_VIEW_TRACE__;
});

describe("Bird View V2 routes", () => {
  it("replaces the V1 card list with a direct canvas entry action", async () => {
    vi.mocked(listBirdViews).mockResolvedValue([]);
    renderRoutes("/bird-views");
    expect(await screen.findByRole("button", { name: /Create Bird View/ })).toBeTruthy();
    expect(screen.queryByText(/PRIVATE PLANNING GRAPH/)).toBeNull();
  });

  it("enables free-form node dragging and handle connections", async () => {
    vi.mocked(fetchBirdViewGraph).mockResolvedValue(graph);
    renderRoutes("/bird-views/v");
    await waitFor(() => expect(renderedFlows.length).toBeGreaterThan(0));
    expect(currentFlow()).toMatchObject({ nodesDraggable: true, nodesConnectable: true, edgesReconnectable: false, connectOnClick: false, deleteKeyCode: null });
    expect(currentNodes()[0]!.position).toEqual({ x: 120, y: 240 });
    expect(screen.getByRole("navigation", { name: "Bird View tools" }).textContent).toContain("BIRD VIEW / REVISION 4");
    expect(screen.getByRole("button", { name: /Node/ }).classList.contains("bird-v2-add-node")).toBe(true);
    const traces = (window as Window & { __BALEY_BIRD_VIEW_TRACE__?: Array<{ event: string; details: Record<string, unknown> }> }).__BALEY_BIRD_VIEW_TRACE__;
    expect(traces?.find((item) => item.event === "graph-load:calculated")?.details.apiGraphPayload).toEqual(graph);
  });

  it("renders an upper-only two-scale planning grid instead of the Workspace dot grammar", async () => {
    vi.mocked(fetchBirdViewGraph).mockResolvedValue(graph);
    renderRoutes("/bird-views/v");

    const backgrounds = await screen.findAllByTestId("flow-background");
    expect(backgrounds).toHaveLength(2);
    expect(screen.getByTestId("bird-flow").parentElement?.getAttribute("data-pattern")).toBe("planning-grid");
    expect(backgrounds.map((item) => ({ id: item.dataset.patternId, variant: item.dataset.variant, className: item.getAttribute("class") }))).toEqual([
      { id: BIRD_VIEW_PLANNING_GRID.minor.id, variant: "lines", className: "bird-v2-planning-grid bird-v2-planning-grid-minor" },
      { id: BIRD_VIEW_PLANNING_GRID.major.id, variant: "lines", className: "bird-v2-planning-grid bird-v2-planning-grid-major" },
    ]);
    expect(renderedBackgrounds.map(({ id, variant, gap, lineWidth, color }) => ({ id, variant, gap, lineWidth, color }))).toEqual([
      { ...BIRD_VIEW_PLANNING_GRID.minor, variant: "lines" },
      { ...BIRD_VIEW_PLANNING_GRID.major, variant: "lines" },
    ]);
  });

  it("uses the established Baley inspector pattern for selected Bird nodes", async () => {
    vi.useFakeTimers();
    vi.mocked(fetchBirdViewGraph).mockResolvedValue(graph);
    renderRoutes("/bird-views/v");
    await act(async () => { await Promise.resolve(); await Promise.resolve(); });
    const node = currentNodes().find((item) => item.id === "a")!;
    act(() => { (currentFlow().onNodeClick as Function)({ isTrusted: true, detail: 1 }, node); vi.advanceTimersByTime(220); });
    const inspector = screen.getByRole("complementary", { name: "Edit Bird View node" });
    expect(inspector.classList.contains("bird-v2-editor")).toBe(true);
    expect(inspector.classList.contains("inspector")).toBe(true);
    expect(screen.getByText("BIRD VIEW INSPECTOR")).toBeTruthy();
    expect(screen.getByRole("button", { name: /Save changes/ }).classList.contains("primary-button")).toBe(true);
  });

  it("keeps one ReactFlow and the same Bird anchor through enter, focus, two-stage exit, and exact viewport restoration", async () => {
    vi.useFakeTimers();
    vi.mocked(fetchBirdViewGraph).mockResolvedValue(graph);
    vi.mocked(fetchBirdViewNodeFocus).mockResolvedValue(focusPayload);
    renderRoutes("/bird-views/v");
    await act(async () => { await Promise.resolve(); await Promise.resolve(); });

    const viewport = { x: -90, y: -40, zoom: 1.25 };
    const anchorPosition = { x: 98, y: 109 };
    const controller = {
      getViewport: () => viewport,
      screenToFlowPosition: vi.fn(() => anchorPosition),
      getNodes: () => currentNodes(),
      getEdges: () => currentEdges(),
      setViewport: vi.fn(),
      fitView: vi.fn(),
    };
    act(() => (currentFlow().onInit as Function)(controller));
    const assertSingleFlowAndAnchor = (position: { x: number; y: number }) => {
      expect(screen.getAllByTestId("bird-flow")).toHaveLength(1);
      expect(flowMountCount).toBe(1);
      expect(flowUnmountCount).toBe(0);
      expect(currentNodes().find((node) => node.id === "a")).toMatchObject({ id: "a", type: "birdViewNode", position });
    };

    expect(focusPhase()).toBe("overview");
    assertSingleFlowAndAnchor({ x: 120, y: 240 });
    expect(screen.getByRole("button", { name: /Node/ })).toBeTruthy();
    const overviewAnchor = currentNodes().find((node) => node.id === "a")!;
    act(() => (currentFlow().onNodeDoubleClick as Function)({ isTrusted: true, detail: 2 }, overviewAnchor));
    expect(screen.queryByRole("button", { name: /Node/ })).toBeNull();
    await act(async () => { await Promise.resolve(); await Promise.resolve(); });

    expect(focusPhase()).toBe("entering");
    expect(screen.queryByRole("button", { name: /Node/ })).toBeNull();
    assertSingleFlowAndAnchor(anchorPosition);
    expect(controller.screenToFlowPosition).toHaveBeenCalledWith({ x: 32, y: 96 });
    expect(currentNodes().find((node) => node.id === "b")).toMatchObject({ className: "bird-focus-fading" });
    expect(currentEdges()).toEqual([expect.objectContaining({ id: "overview-edge", className: "bird-focus-fading" })]);

    act(() => vi.advanceTimersByTime(260));
    expect(focusPhase()).toBe("focused");
    expect(screen.queryByRole("button", { name: /Node/ })).toBeNull();
    assertSingleFlowAndAnchor(anchorPosition);
    expect(currentNodes().map((node) => node.id)).toEqual(["a", "phase:build", "phase:validate", "task:condition", "task:entry", "gate:ready"]);
    expect(currentNodes().filter((node) => node.id !== "a")).toEqual(expect.arrayContaining([
      expect.objectContaining({ id: "phase:build", type: "focusCard", className: "bird-focus-subset", data: expect.objectContaining({ kind: "phase", label: "Build" }) }),
      expect.objectContaining({ id: "phase:validate", type: "focusCard", className: "bird-focus-subset", data: expect.objectContaining({ kind: "phase", label: "Validate" }) }),
      expect.objectContaining({ id: "task:condition", type: "focusCard", data: expect.objectContaining({ label: "#7 Bound condition" }) }),
      expect.objectContaining({ id: "task:entry", type: "focusCard", data: expect.objectContaining({ label: "#8 Bound entry" }) }),
      expect.objectContaining({ id: "gate:ready", type: "focusCard", data: expect.objectContaining({ label: "G#4 Release ready" }) }),
    ]));
    expect(currentNodes().some((node) => node.id === "b" || node.id === "task:unbound" || node.id === "phase:unrelated-phase" || node.id === "gate:implicit")).toBe(false);
    expect(currentEdges()).toEqual([
      expect.objectContaining({ id: "dependency:condition:entry", source: "task:condition", target: "task:entry", className: "dependency-edge", data: { relation: "dependency" } }),
      expect.objectContaining({ id: "gate:ready:condition:condition", source: "task:condition", target: "gate:ready", className: "gate-edge gate-edge-required", data: { relation: "required" } }),
      expect.objectContaining({ id: "gate:ready:entry:entry", source: "gate:ready", target: "task:entry", className: "gate-edge gate-edge-locked", data: { relation: "unlocks", locked: true } }),
    ]);
    vi.mocked(applyNodeChanges).mockClear();
    act(() => (currentFlow().onNodesChange as Function)([{ id: "task:condition", type: "dimensions", dimensions: { width: 250, height: 72 } }]));
    expect(applyNodeChanges).not.toHaveBeenCalled();
    expect(screen.queryByText(/Workspace graph/)).toBeNull();
    expect(screen.queryByLabelText("Edit Bird View node")).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "Exit focus" }));
    expect(focusPhase()).toBe("exiting-start");
    expect(screen.queryByRole("button", { name: /Node/ })).toBeNull();
    assertSingleFlowAndAnchor({ x: 120, y: 240 });
    expect(currentNodes().find((node) => node.id === "b")).toMatchObject({ className: "bird-focus-fading" });
    expect(currentEdges()).toEqual([expect.objectContaining({ id: "overview-edge", className: "bird-focus-fading" })]);

    act(() => vi.advanceTimersByTime(16));
    expect(focusPhase()).toBe("exiting");
    expect(screen.queryByRole("button", { name: /Node/ })).toBeNull();
    assertSingleFlowAndAnchor({ x: 120, y: 240 });
    expect(currentNodes().find((node) => node.id === "b")).toMatchObject({ className: "bird-focus-fading" });
    expect(currentEdges()).toEqual([expect.objectContaining({ id: "overview-edge", className: "bird-focus-fading" })]);

    act(() => vi.advanceTimersByTime(260));
    expect(focusPhase()).toBe("overview");
    assertSingleFlowAndAnchor({ x: 120, y: 240 });
    expect(currentNodes().map((node) => ({ id: node.id, position: node.position, className: node.className }))).toEqual([
      { id: "a", position: { x: 120, y: 240 }, className: undefined },
      { id: "b", position: { x: 520, y: 260 }, className: undefined },
    ]);
    expect(currentEdges()).toEqual([expect.objectContaining({ id: "overview-edge", className: undefined })]);
    expect(controller.setViewport).toHaveBeenLastCalledWith(viewport, { duration: 0 });
    expect(fetchBirdViewNodeFocus).toHaveBeenCalledWith("v", "a");
    expect(screen.queryByRole("button", { name: "Exit focus" })).toBeNull();
    expect(screen.getByRole("button", { name: /Node/ })).toBeTruthy();
    const transitionTargets = ((window as Window & { __BALEY_BIRD_VIEW_TRACE__?: Array<{ event: string; details: { calculatedTargetState?: { focusPhase?: string } } }> }).__BALEY_BIRD_VIEW_TRACE__ ?? [])
      .filter((item) => item.event === "focus-transition")
      .map((item) => item.details.calculatedTargetState?.focusPhase);
    expect(transitionTargets).toEqual(expect.arrayContaining(["entering", "focused", "exiting-start", "exiting", "overview"]));
  });

  it("keeps the initial linked subset inside a 1762x1039 desktop viewport", async () => {
    vi.useFakeTimers();
    vi.stubGlobal("matchMedia", vi.fn(() => ({ matches: true })));
    vi.mocked(fetchBirdViewGraph).mockResolvedValue(graph);
    vi.mocked(fetchBirdViewNodeFocus).mockResolvedValue(focusPayload);
    renderRoutes("/bird-views/v");
    await act(async () => { await Promise.resolve(); await Promise.resolve(); });

    // This reproduces a centered overview node whose upper-left focus target is
    // negative in Flow space even though it is at (32, 96) on screen.
    const viewport = { x: 352, y: 384, zoom: 1.6 };
    const anchorPosition = { x: -200, y: -180 };
    const controller = { getViewport: () => viewport, screenToFlowPosition: () => anchorPosition, getNodes: () => currentNodes(), getEdges: () => currentEdges(), setViewport: vi.fn(), fitView: vi.fn() };
    act(() => (currentFlow().onInit as Function)(controller));
    act(() => (currentFlow().onNodeDoubleClick as Function)({ isTrusted: true, detail: 2 }, currentNodes().find((node) => node.id === "a")));
    await act(async () => { await Promise.resolve(); await Promise.resolve(); });

    expect(focusPhase()).toBe("focused");
    expect(screen.queryByRole("button", { name: /Node/ })).toBeNull();
    const subset = currentNodes().filter((node) => node.id !== "a");
    const screenRight = Math.max(...subset.map((node) => (node.position.x + (node.width ?? 250)) * viewport.zoom + viewport.x));
    const screenBottom = Math.max(...subset.map((node) => (node.position.y + (node.height ?? 72)) * viewport.zoom + viewport.y));
    expect(screenRight).toBeLessThanOrEqual(1762 - 32);
    expect(screenBottom).toBeLessThanOrEqual(1039 - 32);
    expect(currentNodes().find((node) => node.id === "gate:ready")?.position.y).toBe(anchorPosition.y + 216);
  });

  it("skips enter and exit delays when reduced motion is requested", async () => {
    vi.useFakeTimers();
    vi.stubGlobal("matchMedia", vi.fn(() => ({ matches: true })));
    vi.mocked(fetchBirdViewGraph).mockResolvedValue(graph);
    vi.mocked(fetchBirdViewNodeFocus).mockResolvedValue(focusPayload);
    renderRoutes("/bird-views/v");
    await act(async () => { await Promise.resolve(); await Promise.resolve(); });
    const viewport = { x: 11, y: 22, zoom: 0.8 };
    const controller = { getViewport: () => viewport, screenToFlowPosition: () => ({ x: 32, y: 96 }), getNodes: () => currentNodes(), getEdges: () => currentEdges(), setViewport: vi.fn(), fitView: vi.fn() };
    act(() => (currentFlow().onInit as Function)(controller));
    const timeoutSpy = vi.spyOn(window, "setTimeout");
    act(() => (currentFlow().onNodeDoubleClick as Function)({ isTrusted: true, detail: 2 }, currentNodes().find((node) => node.id === "a")));
    await act(async () => { await Promise.resolve(); await Promise.resolve(); });

    expect(focusPhase()).toBe("focused");
    expect(timeoutSpy.mock.calls.some((call) => call[1] === 260)).toBe(false);
    fireEvent.click(screen.getByRole("button", { name: "Exit focus" }));
    expect(focusPhase()).toBe("overview");
    expect(timeoutSpy.mock.calls.some((call) => call[1] === 260)).toBe(false);
    expect(controller.setViewport).toHaveBeenLastCalledWith(viewport, { duration: 0 });
    expect(flowMountCount).toBe(1);
    expect(flowUnmountCount).toBe(0);
  });

  it("persists final drag coordinates through the shared command boundary", async () => {
    vi.mocked(fetchBirdViewGraph).mockResolvedValue(graph);
    vi.mocked(executeBirdViewCommand).mockResolvedValue({ commandId: "c", birdViewRevision: 5, eventIds: [], projection: {}, idempotent: false });
    renderRoutes("/bird-views/v");
    await waitFor(() => expect(renderedFlows.length).toBeGreaterThan(0));
    act(() => (currentFlow().onNodeDragStop as Function)({}, { id: "a", position: { x: 444, y: 555 } }));
    await waitFor(() => expect(executeBirdViewCommand).toHaveBeenCalled());
    expect(vi.mocked(executeBirdViewCommand).mock.calls[0]![0]).toMatchObject({ name: "bird_view.node.update", arguments: { birdViewId: "v", nodeId: "a", positionX: 444, positionY: 555 }, envelope: { expectedBirdViewRevision: 4 } });
  });

  it("uses the Workspace default bezier grammar for newly connected edges", async () => {
    vi.mocked(fetchBirdViewGraph).mockResolvedValue(graph);
    vi.mocked(executeBirdViewCommand).mockResolvedValue({ commandId: "c", birdViewRevision: 5, eventIds: [], projection: {}, idempotent: false });
    renderRoutes("/bird-views/v");
    await waitFor(() => expect(renderedFlows.length).toBeGreaterThan(0));
    act(() => (currentFlow().onConnect as Function)({ source: "a", target: "new-target" }));
    await waitFor(() => expect(currentEdges()).toHaveLength(2));
    expect(currentEdges().find((edge) => edge.id !== "overview-edge")).not.toHaveProperty("type");
  });
});
