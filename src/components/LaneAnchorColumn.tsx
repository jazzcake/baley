import type { Lane } from "../domain/model";
import type { GraphLayout } from "../graph/layout";
import {
  LANE_ANCHOR_COLUMN_LEFT,
  LANE_ANCHOR_COLUMN_WIDTH,
  LANE_ANCHOR_HEADER_LEFT,
  LANE_ANCHOR_HEADER_WIDTH,
} from "./backlog-rail.config";

type LaneAnchorColumnProps = {
  lanes: Lane[];
  layout: GraphLayout;
};

export function LaneAnchorColumn({ lanes, layout }: LaneAnchorColumnProps) {
  return <div className="lane-anchor-column-layer" aria-hidden="true">
    <div
      className="lane-anchor-column-surface"
      style={{
        left: LANE_ANCHOR_COLUMN_LEFT,
        width: LANE_ANCHOR_COLUMN_WIDTH,
        height: layout.height,
      }}
    />
    <div
      className="graph-column-heading lane-anchor-column-heading"
      style={{ left: LANE_ANCHOR_HEADER_LEFT, width: LANE_ANCHOR_HEADER_WIDTH }}
    >
      <span>LANES</span>
      <small>{lanes.length} ROWS</small>
    </div>
  </div>;
}
