import { Handle, Position, type NodeProps } from "@xyflow/react";
import { Check, CircleDot, LockKeyhole, Play } from "lucide-react";
import type { TaskStatus } from "../domain/model";

export type TaskNodeData = { title: string; status: TaskStatus; lane: string; laneColor: string; dimmed: boolean; laneFocused: boolean; external: boolean };

const icons = { done: Check, running: Play, blocked: LockKeyhole, ready: CircleDot };

export function TaskNode({ data, selected }: NodeProps) {
  const value = data as unknown as TaskNodeData;
  const Icon = icons[value.status];
  return (
    <article style={{ "--lane-color": value.laneColor } as React.CSSProperties} className={`task-node status-${value.status} ${selected ? "selected" : ""} ${value.dimmed ? "dimmed" : ""} ${value.laneFocused ? "lane-focused" : ""} ${value.external ? "external" : ""}`}>
      <Handle type="target" position={Position.Left} />
      <div className="task-node-top"><span className="task-lane"><i />{value.lane}</span></div>
      <strong>{value.title}</strong>
      <span className="task-status"><Icon size={12} />{value.status}</span>
      <Handle type="source" position={Position.Right} />
    </article>
  );
}
