import { useEffect, useRef, useState } from "react";
import {
  executeCommand,
  previewCommand,
  type CommandExecution,
  type CommandPreview,
  type CommandRequest,
} from "../api/auth";
import { traceViewer } from "../debug/viewer-trace";
import type { Phase, Task } from "../domain/model";

type Draft =
  | { kind: "update"; title: string; description: string; currentSummary: string }
  | { kind: "move"; targetPhaseId: string };

type Prepared = { command: CommandRequest; result: CommandPreview; draft: Draft; scopeKey: string };

const terminal = (task: Task) => task.status === "confirmed" || task.status === "discarded";
const messageFor = (value: unknown) => value instanceof Error ? value.message : "The command could not be completed.";
const commandID = () => globalThis.crypto?.randomUUID?.() ?? `browser-${Date.now()}-${Math.random().toString(16).slice(2)}`;

export function TaskCommandEditor({
  workspaceId,
  workspaceRevision,
  task,
  phases,
  csrfToken,
  canOperate,
  onExecuted,
}: {
  workspaceId: string;
  workspaceRevision: number;
  task: Task;
  phases: Phase[];
  csrfToken: string;
  canOperate: boolean;
  onExecuted: (execution: CommandExecution, draft: Draft) => void;
}) {
  const [title, setTitle] = useState(task.title);
  const [description, setDescription] = useState(task.description);
  const [currentSummary, setCurrentSummary] = useState(task.currentSummary ?? "");
  const moveTargets = phases.filter((phase) => phase.state !== "completed" && phase.id !== task.phaseId);
  const [targetPhaseId, setTargetPhaseId] = useState(moveTargets[0]?.id ?? "");
  const [prepared, setPrepared] = useState<Prepared>();
  const [busy, setBusy] = useState<"preview" | "execute">();
  const [error, setError] = useState<string>();
  const [acknowledgedWarnings, setAcknowledgedWarnings] = useState<string[]>([]);
  const [proceedReason, setProceedReason] = useState("");
  const requestGeneration = useRef(0);
  const scopeKey = `${workspaceId}:${workspaceRevision}:${task.id}:${task.status}:${canOperate}`;
  const scopeRef = useRef(scopeKey);
  scopeRef.current = scopeKey;

  useEffect(() => {
    setTitle(task.title);
    setDescription(task.description);
    setCurrentSummary(task.currentSummary ?? "");
    setTargetPhaseId(phases.find((phase) => phase.state !== "completed" && phase.id !== task.phaseId)?.id ?? "");
    setPrepared(undefined);
    setError(undefined);
    requestGeneration.current += 1;
  }, [phases, task.currentSummary, task.description, task.id, task.phaseId, task.status, task.title, workspaceRevision]);

  useEffect(() => {
    const frame = window.requestAnimationFrame(() => traceViewer("task-command-editor:dom-rendered", {
      userEvent: "react-commit",
      calculatedTarget: prepared ? { command: prepared.command.name, taskId: task.publicId } : "edit-form",
      reactState: { canOperate, terminal: terminal(task), busy, hasPreview: Boolean(prepared), error: Boolean(error) },
      controllerState: { workspaceRevision, requestGeneration: requestGeneration.current },
      renderedDom: {
        editorPresent: Boolean(document.querySelector(`[data-task-command-editor="${task.id}"]`)),
        previewPresent: Boolean(document.querySelector("[data-task-command-preview]")),
        errorPresent: Boolean(document.querySelector("[data-task-command-error]")),
      },
    }));
    return () => window.cancelAnimationFrame(frame);
  }, [busy, canOperate, error, prepared, task, workspaceRevision]);

  if (!canOperate || terminal(task)) {
    return <section className="command-hint" data-task-command-editor={task.id}>
      <strong>{terminal(task) ? "Task is terminal" : "View-only membership"}</strong>
      <p>{terminal(task) ? "Confirmed and discarded Tasks cannot be edited or moved." : "Task editing requires the workspace:operate capability."}</p>
    </section>;
  }

  const prepare = async (draft: Draft) => {
    const generation = ++requestGeneration.current;
    const requestedScope = scopeKey;
    const args = draft.kind === "move"
      ? { workspaceId, taskId: task.publicId, targetPhaseId: draft.targetPhaseId }
      : {
          workspaceId,
          taskId: task.publicId,
          ...(draft.title !== task.title ? { title: draft.title } : {}),
          ...(draft.description !== task.description ? { description: draft.description } : {}),
          ...(draft.currentSummary !== (task.currentSummary ?? "") ? { currentSummary: draft.currentSummary } : {}),
        };
    const command: CommandRequest = {
      name: draft.kind === "move" ? "task.move" : "task.update",
      arguments: args,
      envelope: { idempotencyKey: commandID(), expectedWorkspaceRevision: workspaceRevision },
    };
    traceViewer("task-command-editor:event", {
      userEvent: draft.kind === "move" ? "review-phase-move-click" : "review-task-changes-click",
      calculatedTarget: { command: command.name, arguments: args },
      reactState: { taskId: task.publicId, taskStatus: task.status, taskPhaseId: task.phaseId, workspaceRevision },
      controllerState: { generation, scopeKey: requestedScope },
      renderedDom: { selectedTask: document.querySelector(".react-flow__node.selected")?.getAttribute("data-id") },
    });
    setBusy("preview");
    setError(undefined);
    try {
      const result = await previewCommand(command, csrfToken);
      if (scopeRef.current !== requestedScope || requestGeneration.current !== generation) {
        traceViewer("task-command-editor:stale-preview-ignored", { requestedScope, currentScope: scopeRef.current, generation, currentGeneration: requestGeneration.current });
        return;
      }
      setPrepared({ command, result, draft, scopeKey: requestedScope });
      setAcknowledgedWarnings([]);
      setProceedReason("");
      traceViewer("task-command-editor:preview-state", {
        calculatedTarget: { command: command.name, entityType: result.entityType, entityId: result.entityId },
        reactState: { taskId: task.publicId, workspaceRevision },
        controllerState: { commandHash: result.commandHash, errorCodes: result.errors.map((item) => item.code), warningCodes: result.warnings.map((item) => item.code) },
        renderedDom: "next-react-commit",
      });
    } catch (cause) {
      if (scopeRef.current === requestedScope && requestGeneration.current === generation) setError(messageFor(cause));
      traceViewer("task-command-editor:request-failed", { stage: "preview", requestedScope, error: messageFor(cause) });
    } finally {
      if (scopeRef.current === requestedScope && requestGeneration.current === generation) setBusy(undefined);
    }
  };

  const execute = async () => {
    if (!prepared) return;
    const requestedScope = scopeKey;
    const targetMatches = prepared.scopeKey === requestedScope && scopeRef.current === requestedScope &&
      prepared.command.arguments.workspaceId === workspaceId && prepared.command.arguments.taskId === task.publicId &&
      prepared.command.envelope.expectedWorkspaceRevision === workspaceRevision &&
      (!prepared.result.entityType || prepared.result.entityType === "task") &&
      (!prepared.result.entityId || prepared.result.entityId === task.id);
    if (!targetMatches) {
      setPrepared(undefined);
      setError("The Task or Workspace revision changed. Review a fresh command before applying it.");
      traceViewer("task-command-editor:target-mismatch-blocked", { requestedScope, previewScope: prepared.scopeKey, taskId: task.publicId, workspaceRevision });
      return;
    }
    traceViewer("task-command-editor:event", {
      userEvent: "apply-reviewed-command-click",
      calculatedTarget: { command: prepared.command.name, arguments: prepared.command.arguments },
      reactState: { canOperate, taskStatus: task.status, workspaceRevision },
      controllerState: { acknowledgedWarnings, proceedReasonPresent: Boolean(proceedReason.trim()) },
      renderedDom: { previewPresent: Boolean(document.querySelector("[data-task-command-preview]")) },
    });
    setBusy("execute");
    setError(undefined);
    try {
      const execution = await executeCommand({
        ...prepared.command,
        envelope: {
          ...prepared.command.envelope,
          acknowledgedWarningCodes: acknowledgedWarnings,
          ...(proceedReason.trim() ? { proceedReason: proceedReason.trim() } : {}),
        },
      }, csrfToken);
      if (scopeRef.current !== requestedScope) return;
      traceViewer("task-command-editor:execution-state", {
        calculatedTarget: { command: prepared.command.name, taskId: task.publicId },
        reactState: { priorWorkspaceRevision: workspaceRevision },
        controllerState: { commandId: execution.commandId, workspaceRevision: execution.workspaceRevision },
        renderedDom: "next-react-commit",
      });
      onExecuted(execution, prepared.draft);
      setPrepared(undefined);
    } catch (cause) {
      setPrepared(undefined);
      setError(`${messageFor(cause)} The graph remains server-authored; review again after its latest revision loads.`);
      traceViewer("task-command-editor:request-failed", { stage: "execute", requestedScope, error: messageFor(cause), recovery: "discard-preview-and-poll-server-graph" });
    } finally {
      if (scopeRef.current === requestedScope) setBusy(undefined);
    }
  };

  const changed = title.trim() && (title !== task.title || description !== task.description || currentSummary !== (task.currentSummary ?? ""));
  const blockingErrors = prepared?.result.errors ?? [];
  const allWarningsAcknowledged = prepared?.result.warnings.every((warning) => acknowledgedWarnings.includes(warning.code)) ?? false;
  const canExecute = Boolean(prepared && blockingErrors.length === 0 && allWarningsAcknowledged && (prepared.result.warnings.length === 0 || proceedReason.trim()));

  return <section className="task-command-editor" data-task-command-editor={task.id}>
    <div className="task-command-editor-heading"><strong>Human task controls</strong><span>Preview / execute</span></div>
    <p>Task content and Phase are editable. Canvas positions remain automatic and are not persisted.</p>
    <label>Title<input value={title} onChange={(event) => { setTitle(event.currentTarget.value); setPrepared(undefined); }} /></label>
    <label>Description<textarea value={description} onChange={(event) => { setDescription(event.currentTarget.value); setPrepared(undefined); }} /></label>
    <label>Current summary<textarea value={currentSummary} onChange={(event) => { setCurrentSummary(event.currentTarget.value); setPrepared(undefined); }} /></label>
    <button type="button" className="quiet-button task-command-action" disabled={!changed || Boolean(busy)} onClick={() => void prepare({ kind: "update", title: title.trim(), description, currentSummary })}>
      {busy === "preview" ? "Checking..." : "Review task changes"}
    </button>
    <div className="task-phase-move">
      <label>Move to Phase<select value={targetPhaseId} onChange={(event) => { setTargetPhaseId(event.currentTarget.value); setPrepared(undefined); }}>{moveTargets.map((phase) => <option key={phase.id} value={phase.id}>{phase.name}</option>)}</select></label>
      <button type="button" className="quiet-button task-command-action" disabled={!targetPhaseId || Boolean(busy)} onClick={() => void prepare({ kind: "move", targetPhaseId })}>Review Phase move</button>
    </div>
    {error && <div className="form-error" role="alert" data-task-command-error>{error}</div>}
    {prepared && <div className="task-command-preview" data-task-command-preview>
      <strong>{prepared.command.name === "task.move" ? "Phase move preview" : "Task update preview"}</strong>
      <span>Workspace revision {prepared.result.expectedWorkspaceRevision} · {prepared.result.requiredCapability}</span>
      {prepared.result.errors.map((item) => <div className="form-error" key={item.code}><strong>{item.code}</strong><span>{item.message}</span></div>)}
      {prepared.result.warnings.map((warning) => <label className="task-command-warning" key={warning.code}><input type="checkbox" checked={acknowledgedWarnings.includes(warning.code)} onChange={(event) => setAcknowledgedWarnings((current) => event.currentTarget.checked ? [...current, warning.code] : current.filter((code) => code !== warning.code))} />{warning.message}</label>)}
      {prepared.result.warnings.length > 0 && <label>Why is it safe to proceed?<textarea value={proceedReason} onChange={(event) => setProceedReason(event.currentTarget.value)} /></label>}
      <details><summary>Projected server change</summary><pre>{JSON.stringify(prepared.result.projectedDiff, null, 2)}</pre></details>
      <div><button type="button" className="primary-button" disabled={!canExecute || Boolean(busy)} onClick={() => void execute()}>{busy === "execute" ? "Applying..." : "Apply reviewed command"}</button><button type="button" className="text-link" disabled={Boolean(busy)} onClick={() => setPrepared(undefined)}>Cancel</button></div>
    </div>}
  </section>;
}

export type { Draft as TaskCommandDraft };
