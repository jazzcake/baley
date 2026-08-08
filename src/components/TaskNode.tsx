import { Handle, Position, type NodeProps } from "@xyflow/react";
import { Check, CircleDot, Play, Wrench } from "lucide-react";
import { useLayoutEffect, useRef } from "react";
import { traceViewer } from "../debug/viewer-trace";
import type { TaskStatus } from "../domain/model";

export type TaskNodeData = { publicId: number; title: string; status: TaskStatus; lane: string; laneColor: string; dimmed: boolean; external: boolean };

const icons = { pending: CircleDot, in_progress: Play, implemented: Wrench, confirmed: Check, discarded: CircleDot };

export function TaskNode({ data, selected }: NodeProps) {
  const value = data as unknown as TaskNodeData;
  const Icon = icons[value.status];
  const nodeRef = useRef<HTMLElement>(null);
  const titleRef = useRef<HTMLElement>(null);

  useLayoutEffect(() => {
    const title = titleRef.current;
    const node = nodeRef.current;
    const status = node?.querySelector<HTMLElement>(".task-status");
    if (!title || !node || !status || title.scrollHeight <= title.clientHeight) return;
    traceViewer("task-card:title-clamped", {
      taskId: value.publicId,
      title: value.title,
      titleClientHeight: title.clientHeight,
      titleScrollHeight: title.scrollHeight,
      nodeClientHeight: node.clientHeight,
      statusOffsetTop: status.offsetTop,
    });
  }, [value.publicId, value.title]);

  return (
    <article ref={nodeRef} style={{ "--lane-color": value.laneColor } as React.CSSProperties} className={`task-node status-${value.status} ${selected ? "selected" : ""} ${value.dimmed ? "dimmed" : ""} ${value.external ? "external" : ""}`}>
      <Handle type="target" position={Position.Left} />
      <div className="task-node-top"><span className="task-lane"><i />{value.lane}</span><span className="task-id">#{value.publicId}</span></div>
      <strong ref={titleRef} className="task-title">{value.title}</strong>
      <span className="task-status"><Icon size={12} />{value.status === "implemented" ? "완료확인 대기" : value.status}</span>
      <Handle type="source" position={Position.Right} />
    </article>
  );
}
