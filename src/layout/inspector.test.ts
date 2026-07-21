import { describe, expect, it } from "vitest";
import { INSPECTOR_DEFAULT_WIDTH, INSPECTOR_MAX_WIDTH, INSPECTOR_MIN_WIDTH, inspectorWidthFromKey, inspectorWidthFromPointer } from "./inspector";

describe("inspector sizing", () => {
  it("grows when the left edge is dragged left and shrinks when dragged right", () => {
    expect(inspectorWidthFromPointer(INSPECTOR_DEFAULT_WIDTH, 500, 420)).toBe(410);
    expect(inspectorWidthFromPointer(INSPECTOR_DEFAULT_WIDTH, 500, 560)).toBe(INSPECTOR_MIN_WIDTH);
  });

  it("clamps pointer resizing to the supported range", () => {
    expect(inspectorWidthFromPointer(INSPECTOR_DEFAULT_WIDTH, 500, -1000)).toBe(INSPECTOR_MAX_WIDTH);
    expect(inspectorWidthFromPointer(INSPECTOR_DEFAULT_WIDTH, 500, 2000)).toBe(INSPECTOR_MIN_WIDTH);
  });

  it("supports accessible keyboard resizing", () => {
    expect(inspectorWidthFromKey(330, "ArrowLeft")).toBe(346);
    expect(inspectorWidthFromKey(330, "ArrowRight", true)).toBe(282);
    expect(inspectorWidthFromKey(330, "Home")).toBe(INSPECTOR_MIN_WIDTH);
    expect(inspectorWidthFromKey(330, "End")).toBe(INSPECTOR_MAX_WIDTH);
    expect(inspectorWidthFromKey(330, "Enter")).toBe(330);
  });
});
