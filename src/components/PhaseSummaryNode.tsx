import { Handle, Position, type NodeProps } from "@xyflow/react";

export function PhaseSummaryNode({ data }: NodeProps) {
  const value = data as unknown as {
    phaseName: string;
    laneName: string;
    taskCount: number;
    completedTaskCount: number;
    dependencyCount: number;
    onExpand: () => void;
  };
  const empty = value.taskCount === 0;

  return (
    <button
      type="button"
      className={`phase-summary-node ${empty ? "empty" : ""}`}
      aria-label={`Expand ${value.phaseName} ${value.laneName} lane summary`}
      aria-expanded="false"
      onClick={(event) => {
        event.stopPropagation();
        value.onExpand();
      }}
    >
      {!empty && <Handle type="target" position={Position.Left} />}
      <span>{value.laneName} LANE</span>
      <strong>{empty ? "No task" : `${value.taskCount} tasks`}</strong>
      {!empty && <small>{value.completedTaskCount} confirmed{" · "}{value.dependencyCount} dependencies</small>}
      {!empty && <Handle type="source" position={Position.Right} />}
    </button>
  );
}