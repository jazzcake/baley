import { Handle, Position, type Node, type NodeProps } from "@xyflow/react";
import { CheckCircle2, CircleDot, PauseCircle } from "lucide-react";
import type { BirdViewNode as BirdViewNodeModel } from "./model";

export type BirdViewNodeData = BirdViewNodeModel & { detailed: boolean };
export type BirdViewFlowNode = Node<BirdViewNodeData, "birdViewNode">;

export function BirdViewNode({ data, selected }: NodeProps<BirdViewFlowNode>) {
  const Icon = data.status === "achieved" ? CheckCircle2 : data.status === "parked" ? PauseCircle : CircleDot;
  const content = <>
    <Handle type="target" position={Position.Left} />
    <span className={`bird-node-kicker bird-node-status-${data.status}`}><Icon size={14} /> {data.status}</span>
    <strong>{data.title}</strong>
    {data.detailed && <p>{data.summary || "No summary yet."}</p>}
    <Handle type="source" position={Position.Right} />
  </>;
  return <article className={`bird-node bird-node-${data.status} ${selected ? "selected" : ""}`} data-status={data.status} aria-label={`${data.title}, ${data.status}`}>{content}</article>;
}
