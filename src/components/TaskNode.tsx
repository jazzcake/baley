import { Handle, Position, type NodeProps } from "@xyflow/react";
import { Check, CircleDot, Play, Wrench } from "lucide-react";
import type { TaskStatus } from "../domain/model";

export type TaskNodeData = { publicId: number; title: string; status: TaskStatus; lane: string; laneColor: string; dimmed: boolean; external: boolean };

const icons = { pending: CircleDot, in_progress: Play, implemented: Wrench, confirmed: Check, discarded: CircleDot };

export function TaskNode({ data, selected }: NodeProps) {
  const value = data as unknown as TaskNodeData;
  const Icon = icons[value.status];
  return (
    <article style={{ "--lane-color": value.laneColor } as React.CSSProperties} className={`task-node status-${value.status} ${selected ? "selected" : ""} ${value.dimmed ? "dimmed" : ""} ${value.external ? "external" : ""}`}>
      <Handle type="target" position={Position.Left} />
      <div className="task-node-top"><span className="task-lane"><i />{value.lane}</span><span className="task-id">#{value.publicId}</span></div>
      <strong>{value.title}</strong>
      <span className="task-status"><Icon size={12} />{value.status === "implemented" ? "완료확인 대기" : value.status}</span>
      <Handle type="source" position={Position.Right} />
    </article>
  );
}
