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
  Panel: ({ children }: { children: React.ReactNode }) => React.createElement("div", null, children),
  ReactFlow: ({ children }: { children: React.ReactNode }) => React.createElement("div", { "data-testid": "graph" }, React.createElement("div", { className: "react-flow__viewport" }, children)),
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

  it("changes the rendered viewport through the app-owned zoom controls", async () => {
    render(<App />);
    const zoom = await screen.findByRole("status", { name: "Current zoom" });
    const canvas = document.querySelector<HTMLElement>(".graph-canvas")!;
    Object.defineProperty(canvas, "clientWidth", { configurable: true, value: 1200 });
    Object.defineProperty(canvas, "clientHeight", { configurable: true, value: 700 });
    expect(zoom.textContent).toBe("100%");
    fireEvent.click(screen.getByRole("button", { name: "Zoom in" }));
    await waitFor(() => expect(zoom.textContent).toBe("120%"));
    expect(document.querySelector<HTMLElement>(".react-flow__viewport")?.style.transform).toContain("scale(1.2)");
    expect(document.querySelector<HTMLElement>(".react-flow__viewport")?.style.transform).not.toContain("NaN");
  });
});
