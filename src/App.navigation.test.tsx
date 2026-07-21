// @vitest-environment jsdom

import React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { fetchGraph } from "./api/client";
import { pilotReadyFixture } from "./fixtures/pilot-ready";
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
  beforeEach(() => {
    window.history.replaceState({}, "", "/lanes/client?task=pilot-ui");
    vi.mocked(fetchGraph).mockResolvedValue(pilotReadyFixture);
  });

  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it("navigates to Home from the Baley logo", async () => {
    render(<App />);
    fireEvent.click(await screen.findByRole("button", { name: "Go to Home" }));
    await waitFor(() => expect(window.location.pathname + window.location.search).toBe("/"));
    expect(screen.getByRole("heading", { name: pilotReadyFixture.workspace.name })).toBeTruthy();
  });

  it("navigates to Workspace Home from the workspace label", async () => {
    render(<App />);
    fireEvent.click(await screen.findByRole("button", { name: "Go to Workspace Home" }));
    await waitFor(() => expect(window.location.pathname + window.location.search).toBe("/"));
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
});
