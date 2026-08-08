import { useEffect, useRef, type CSSProperties, type RefCallback } from "react";
import { Maximize2, X } from "lucide-react";
import type { BacklogItem, Lane } from "../domain/model";
import type { GraphLayout } from "../graph/layout";
import {
  BACKLOG_RAIL_LEFT,
  BACKLOG_RAIL_WIDTH,
} from "./backlog-rail.config";

type BacklogRailProps = {
  lanes: Lane[];
  items: BacklogItem[];
  layout: GraphLayout;
  focusedLaneId?: string;
  laneColors: Record<string, string>;
  onExpand: () => void;
  onSelect: (item: BacklogItem) => void;
  expandButtonRef?: RefCallback<HTMLButtonElement>;
};

export function BacklogRail({ lanes, items, layout, focusedLaneId, laneColors, onExpand, onSelect, expandButtonRef }: BacklogRailProps) {
  return <div className="backlog-rail-layer">
    <div
      className="graph-column-heading backlog-rail-column-heading"
      style={{ left: BACKLOG_RAIL_LEFT, width: BACKLOG_RAIL_WIDTH }}
    >
      <div><span>BACKLOG</span><small>PHASE 미정</small></div>
      <button ref={expandButtonRef} type="button" aria-label="Open backlog list" title="Open backlog list" onClick={onExpand}>
        <Maximize2 size={13} />
      </button>
    </div>
    <div>
      {lanes.map((lane) => {
      const laneItems = items.filter((item) => item.status === "active" && item.laneId === lane.id).sort((a, b) => (a.position ?? 0) - (b.position ?? 0) || a.publicId - b.publicId);
      const top = layout.lanePositions.get(lane.id);
      const laneHeight = layout.laneHeights.get(lane.id);
      if (top === undefined || laneHeight === undefined) return null;
      // Respect the actual lane height instead of hiding work after two rows.
      const railHeight = laneHeight - 36;
      const visibleItemCount = Math.max(1, Math.floor((railHeight - 38) / 20));

      const dimmed = Boolean(focusedLaneId && focusedLaneId !== lane.id);
      return <section
        key={lane.id}
        className={`backlog-rail ${dimmed ? "dimmed" : ""}`}
        data-lane-id={lane.id}
        style={{
          left: BACKLOG_RAIL_LEFT,
          top: top + 18,
          width: BACKLOG_RAIL_WIDTH,
          height: laneHeight - 36,
          "--lane-color": laneColors[lane.id] ?? "#8c91a5",
        } as CSSProperties}
      >
        <header>
          <strong>UNASSIGNED</strong>
          <span>{laneItems.length} ITEMS</span>
        </header>
        <div className="backlog-rail-items">
          {laneItems.slice(0, visibleItemCount).map((item) =>
            <button type="button" className="backlog-rail-item" key={item.id} aria-label={`Open backlog B#${item.publicId}`} onClick={() => onSelect(item)}>
              <i aria-hidden="true" />
              <span>{item.title}</span>
              <small>B#{item.publicId}</small>
            </button>
          )}
          {laneItems.length > visibleItemCount && <small className="backlog-rail-more">+{laneItems.length - visibleItemCount} MORE</small>}
          {laneItems.length === 0 && <small className="backlog-rail-empty">No backlog items</small>}
        </div>
      </section>;
      })}
    </div>
  </div>;
}

type BacklogListProps = {
  lanes: Lane[];
  items: BacklogItem[];
  laneColors: Record<string, string>;
  onClose: () => void;
  onSelect: (item: BacklogItem) => void;
};

export function BacklogList({ lanes, items, laneColors, onClose, onSelect }: BacklogListProps) {
  const closeButtonRef = useRef<HTMLButtonElement>(null);
  useEffect(() => closeButtonRef.current?.focus(), []);

  return <section
    className="backlog-list"
    aria-labelledby="backlog-list-title"
    onKeyDown={(event) => {
      if (event.key !== "Escape") return;
      event.preventDefault();
      onClose();
    }}
  >
    <header className="backlog-list-header">
      <div>
        <span>LIVE WORKSPACE DATA</span>
        <h2 id="backlog-list-title">Lane backlogs</h2>
        <p>아직 Phase에 배정되지 않은 작업 후보를 Lane별로 모아봅니다.</p>
      </div>
      <button ref={closeButtonRef} type="button" aria-label="Back to graph" onClick={onClose}><X size={16} /> Back to graph</button>
    </header>
    <div className="backlog-list-grid">
      {lanes.map((lane) => {
        const laneItems = items.filter((item) => item.status === "active" && item.laneId === lane.id).sort((a, b) => (a.position ?? 0) - (b.position ?? 0) || a.publicId - b.publicId);
        return (
        <article
          className="backlog-list-lane"
          key={lane.id}
          aria-labelledby={`backlog-list-lane-${lane.id}`}
          style={{ "--lane-color": laneColors[lane.id] ?? "#8c91a5" } as CSSProperties}
        >
          <header>
            <div><i aria-hidden="true" /><h3 id={`backlog-list-lane-${lane.id}`}>{lane.name}</h3></div>
            <span>{laneItems.length} ITEMS</span>
          </header>
          <ul>
            {laneItems.map((item) =>
              <li key={item.id}>
                <button type="button" className="backlog-list-item" aria-label={`Open backlog B#${item.publicId}`} onClick={() => onSelect(item)}>
                <span>{item.title}</span>
                <small>B#{item.publicId} · PHASE 미정</small>
                </button>
              </li>
            )}
            {laneItems.length === 0 && <li className="backlog-list-empty"><span>No backlog items</span><small>PHASE 미정</small></li>}
          </ul>
        </article>);
      })}
    </div>
  </section>;
}
