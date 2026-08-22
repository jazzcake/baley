import { Handle, Position, type NodeProps } from "@xyflow/react";
import { Diamond, Milestone } from "lucide-react";

type GateData = { title: string; publicId: number; alias?: string; gateId: string; status: string; summary: string; dimmed: boolean; compact?: boolean };

export function GateNode({ data, selected }: NodeProps) {
  const value = data as unknown as GateData;
  return <article className={`gate-node ${value.compact ? "compact" : ""} ${selected ? "selected" : ""} ${value.dimmed ? "dimmed" : ""}`}>
    <Handle type="target" position={Position.Left} />
    {!value.compact && <div className="gate-icon"><Milestone size={18} /></div>}
    <div><span><Diamond size={9} /> G#{value.publicId} · PHASE GATE · {value.status}</span><strong>{value.compact ? "Passed" : value.title}</strong>{!value.compact && <small>{value.alias ?? value.gateId} · {value.status === "ready" ? "Approval pending" : value.summary}</small>}</div>
    <Handle type="source" position={Position.Right} />
  </article>;
}