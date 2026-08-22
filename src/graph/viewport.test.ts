import { describe, expect, it } from "vitest";
import { fitViewportToCanvas, focusViewportOnAnchor, zoomViewportAtCenter } from "./viewport";

describe("canvas viewport calculations", () => {
  it("places a graph anchor at the requested visible-canvas fraction", () => {
    expect(focusViewportOnAnchor({ x: 976, y: 347 }, 900, 700, 1, 0.22, 0.5)).toEqual({
      x: -778,
      y: 3,
      zoom: 1,
    });
  });

  it("clamps focus fractions to the visible canvas", () => {
    expect(focusViewportOnAnchor({ x: 100, y: 100 }, 800, 600, 0.5, 2, -1)).toEqual({
      x: 750,
      y: -50,
      zoom: 0.5,
    });
  });
  it("zooms around the visible canvas center", () => {
    expect(zoomViewportAtCenter({ x: -200, y: -100, zoom: 1 }, 1.2, 1000, 600, 0.55, 1.55)).toEqual({
      x: -340,
      y: -180,
      zoom: 1.2,
    });
  });

  it("fits the full layout inside the padded canvas", () => {
    const viewport = fitViewportToCanvas(1200, 600, 1000, 700, 0.55, 1.55, 32)!;
    expect(viewport.zoom).toBeCloseTo(0.78);
    expect(viewport.x).toBeCloseTo(32);
    expect(viewport.y).toBeCloseTo(116);
  });

  it("allows wide canvases to fit below the former 0.55 minimum", () => {
    expect(fitViewportToCanvas(4000, 600, 1000, 700, 0.25, 1.55, 32)?.zoom).toBeCloseTo(0.25);
  });

  it("rejects unmeasured canvas dimensions", () => {
    expect(zoomViewportAtCenter({ x: 0, y: 0, zoom: 1 }, 1.2, 0, 600, 0.55, 1.55)).toBeUndefined();
    expect(fitViewportToCanvas(1200, 600, 0, 700, 0.55, 1.55)).toBeUndefined();
  });
});
