import { Handle, Position, type NodeProps } from "@xyflow/react";
import { Diamond, Milestone } from "lucide-react";

export function GateNode({ data, selected }: NodeProps) {
  const value = data as unknown as { title: string; status: string; summary: string; dimmed: boolean };
  return (
    <article className={`gate-node ${selected ? "selected" : ""} ${value.dimmed ? "dimmed" : ""}`}>
      <Handle type="target" position={Position.Left} />
      <div className="gate-icon"><Milestone size={18} /></div>
      <div><span><Diamond size={9} /> PHASE GATE · {value.status}</span><strong>{value.title}</strong><small>{value.status === "ready" ? "통과 승인 대기" : value.summary}</small></div>
      <Handle type="source" position={Position.Right} />
    </article>
  );
}
