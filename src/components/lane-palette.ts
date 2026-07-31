import type { Lane } from "../domain/model";

// Presentation-only palette; lane order determines the repeating accent.
export const LANE_ACCENT_PALETTE = [
  "#A25DDC", // violet
  "#FDAB3D", // amber
  "#579BFC", // blue
  "#00A887", // teal
  "#E2445C", // rose
  "#00A9BD", // cyan
  "#7FBA00", // lime
  "#6161FF", // indigo
  "#E16B2D", // orange
  "#D64FA5", // magenta
  "#2F9E72", // emerald
  "#667085", // slate
] as const;

export function laneColorMap(lanes: Array<Pick<Lane, "id">>): Record<string, string> {
  return Object.fromEntries(
    lanes.map((lane, index) => [
      lane.id,
      LANE_ACCENT_PALETTE[index % LANE_ACCENT_PALETTE.length]!,
    ]),
  );
}
