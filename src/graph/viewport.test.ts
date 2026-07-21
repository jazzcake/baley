import { describe, expect, it } from "vitest";
import { fitViewportToCanvas, zoomViewportAtCenter } from "./viewport";

describe("canvas viewport calculations", () => {
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

  it("rejects unmeasured canvas dimensions", () => {
    expect(zoomViewportAtCenter({ x: 0, y: 0, zoom: 1 }, 1.2, 0, 600, 0.55, 1.55)).toBeUndefined();
    expect(fitViewportToCanvas(1200, 600, 0, 700, 0.55, 1.55)).toBeUndefined();
  });
});
