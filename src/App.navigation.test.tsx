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
  Controls: () => React.createElement("div", { "data-testid": "controls" }),
  Panel: ({ children }: { children: React.ReactNode }) => React.createElement("div", null, children),
  ReactFlow: ({ viewport, onViewportChange }: { viewport: { x: number; y: number; zoom: number }; onViewportChange: (viewport: { x: number; y: number; zoom: number }) => void }) => React.createElement("button", { "aria-label": "Simulate viewport change", "data-testid": "graph", "data-zoom": viewport.zoom, onClick: () => onViewportChange({ x: 12, y: 24, zoom: 1.25 }) }),
  ViewportPortal: ({ children }: { children: React.ReactNode }) => React.createElement(React.Fragment, null, children),
  useReactFlow: () => ({ zoomIn: vi.fn(), zoomOut: vi.fn(), fitView: vi.fn() }),
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

  it("keeps the rendered canvas synchronized with viewport changes", async () => {
    render(<App />);
    const canvas = await screen.findByRole("button", { name: "Simulate viewport change" });
    expect(canvas.getAttribute("data-zoom")).toBe("1");
    fireEvent.click(canvas);
    await waitFor(() => expect(canvas.getAttribute("data-zoom")).toBe("1.25"));
  });
});
