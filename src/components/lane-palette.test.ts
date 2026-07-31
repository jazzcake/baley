import { describe, expect, it } from "vitest";
import { laneColorMap, LANE_ACCENT_PALETTE } from "./lane-palette";

describe("lane accent palette", () => {
  it("keeps a twelve-color unique palette", () => {
    expect(LANE_ACCENT_PALETTE).toHaveLength(12);
    expect(new Set(LANE_ACCENT_PALETTE).size).toBe(12);
  });

  it("assigns accents in lane order", () => {
    const colors = laneColorMap([
      { id: "adoption" },
      { id: "art" },
      { id: "client" },
      { id: "server" },
    ]);

    expect(colors).toEqual({
      adoption: "#A25DDC",
      art: "#FDAB3D",
      client: "#579BFC",
      server: "#00A887",
    });
  });

  it("cycles after the palette is exhausted", () => {
    const lanes = Array.from(
      { length: LANE_ACCENT_PALETTE.length + 1 },
      (_, index) => ({ id: `lane-${index}` }),
    );
    const colors = laneColorMap(lanes);

    expect(colors["lane-0"]).toBe(colors[`lane-${LANE_ACCENT_PALETTE.length}`]);
  });
});
