import { useEffect, useRef, useState } from "react";
import {
  executeCommand,
  issueApprovalGrant,
  previewCommand,
  revokeApprovalGrant,
  type CommandExecution,
  type CommandPreview,
  type CommandRequest,
} from "../api/auth";
import { traceViewer } from "../debug/viewer-trace";
import type { Task, TaskRecord, Run, AcceptanceEvidence } from "../domain/model";

type ConfirmationPreview = { command: CommandRequest; result: CommandPreview };

function commandID(): string {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") return crypto.randomUUID();
  return `viewer-${Date.now()}-${Math.random().toString(16).slice(2)}`;
}

function errorMessage(reason: unknown): string {
  return reason instanceof Error ? reason.message : "Task confirmation failed.";
}

export function TaskConfirmation({
  workspaceId,
  workspaceRevision,
  task,
  csrfToken,
  canApprove,
  runs,
  records,
  acceptanceEvidence,
  onConfirmed,
}: {
  workspaceId: string;
  workspaceRevision: number;
  task: Task;
  csrfToken: string;
  canApprove: boolean;
  runs: Run[];
  records: TaskRecord[];
  acceptanceEvidence: AcceptanceEvidence[];
  onConfirmed: (execution: CommandExecution) => void;
}) {
  const rootRef = useRef<HTMLElement>(null);
  const [preview, setPreview] = useState<ConfirmationPreview>();
  const [acknowledgedWarnings, setAcknowledgedWarnings] = useState<string[]>([]);
  const [proceedReason, setProceedReason] = useState("");
  const [busy, setBusy] = useState<"preview" | "execute">();
  const [error, setError] = useState<string>();

  useEffect(() => {
    setPreview(undefined);
    setAcknowledgedWarnings([]);
    setProceedReason("");
    setError(undefined);
  }, [task.id, workspaceRevision]);

  useEffect(() => {
    const frame = window.requestAnimationFrame(() => traceViewer("task-confirmation:dom-rendered", {
      taskId: task.publicId,
      workspaceId,
      renderedDom: {
        rootPresent: Boolean(rootRef.current),
        reviewVisible: Boolean(rootRef.current?.querySelector("[data-task-confirm-preview]")),
        confirmButtonVisible: Boolean(rootRef.current?.querySelector("[data-task-confirm-execute]")),
      },
      applicationState: { hasPreview: Boolean(preview), busy, errorPresent: Boolean(error) },
    }));
    return () => window.cancelAnimationFrame(frame);
  }, [busy, error, preview, task.publicId, workspaceId]);

  const loadPreview = async () => {
    const command: CommandRequest = {
      name: "task.confirm",
      arguments: { workspaceId, taskId: task.publicId },
      envelope: { idempotencyKey: commandID(), expectedWorkspaceRevision: workspaceRevision },
    };
    traceViewer("task-confirmation:event", {
      event: "confirm-task-click",
      calculatedTarget: { action: command.name, workspaceId, taskId: task.publicId, workspaceRevision },
      reactState: {
        taskStatus: task.status,
        implementedAssessment: task.implementedAssessment,
        runCount: runs.length,
        recordCount: records.length,
        acceptanceEvidenceCount: acceptanceEvidence.length,
      },
      controllerState: { preview: "not-requested", busy },
    });
    setBusy("preview");
    setError(undefined);
    try {
      const result = await previewCommand(command, csrfToken);
      traceViewer("task-confirmation:preview-state", {
        workspaceId,
        calculatedTarget: { action: command.name, entityType: result.entityType, entityId: result.entityId },
        commandHash: result.commandHash,
        workspaceRevision: result.expectedWorkspaceRevision,
        controllerState: {
          warningCodes: result.warnings.map((item) => item.code),
          errorCodes: result.errors.map((item) => item.code),
        },
      });
      setPreview({ command, result });
      setAcknowledgedWarnings([]);
      setProceedReason("");
    } catch (reason) {
      setError(errorMessage(reason));
    } finally {
      setBusy(undefined);
    }
  };

  const approveAndExecute = async () => {
    if (!preview) return;
    traceViewer("task-confirmation:event", {
      event: "confirm-task-once-click",
      calculatedTarget: { action: preview.command.name, workspaceId, taskId: task.publicId },
      reactState: { taskStatus: task.status, workspaceRevision },
      controllerState: { acknowledgedWarnings, proceedReasonPresent: Boolean(proceedReason.trim()) },
    });
    setBusy("execute");
    setError(undefined);
    let grantId = "";
    try {
      const grant = await issueApprovalGrant(workspaceId, {
        command: preview.command,
        acknowledgedWarningCodes: acknowledgedWarnings,
        proceedReason: proceedReason.trim(),
      }, csrfToken);
      grantId = grant.id;
      traceViewer("task-confirmation:grant-state", {
        workspaceId, taskId: task.publicId, grantId, expiresAt: grant.expiresAt,
        commandHash: grant.commandHash, workspaceRevision: grant.workspaceRevision,
      });
      const result = await executeCommand({
        ...preview.command,
        envelope: {
          ...preview.command.envelope,
          expectedWorkspaceRevision: grant.workspaceRevision,
          acknowledgedWarningCodes: acknowledgedWarnings,
          ...(proceedReason.trim() ? { proceedReason: proceedReason.trim() } : {}),
          approvalGrantId: grant.id,
        },
      }, csrfToken);
      traceViewer("task-confirmation:execution-state", {
        workspaceId, taskId: task.publicId, commandId: result.commandId,
        workspaceRevision: result.workspaceRevision, approvalProtocol: result.approvalProtocol,
      });
      onConfirmed(result);
    } catch (reason) {
      if (grantId) void revokeApprovalGrant(workspaceId, grantId, csrfToken).catch(() => undefined);
      setError(errorMessage(reason));
    } finally {
      setBusy(undefined);
    }
  };

  const blockingErrors = preview?.result.errors.filter((item) => item.code !== "human_approval_required") ?? [];
  const allWarningsAcknowledged = preview?.result.warnings.every((warning) => acknowledgedWarnings.includes(warning.code)) ?? false;
  const canExecute = Boolean(preview && blockingErrors.length === 0 && allWarningsAcknowledged &&
    (preview.result.warnings.length === 0 || proceedReason.trim()));
  const passedEvidence = acceptanceEvidence.filter((item) =>
    item.verificationVerdict === "passed" && item.reviewVerdict === "pass" && item.unresolvedBlockingCount === 0).length;
  const succeededRuns = runs.filter((run) => run.status === "succeeded").length;

  return <section ref={rootRef} className="task-confirmation" aria-label="Task confirmation">
    <div className="task-confirmation-heading"><span>HUMAN DECISION</span><strong>Ready to confirm</strong></div>
    <p>Review this Task's implementation result and evidence. Confirmation records your decision; it does not grant the Agent any human authority.</p>
    {task.implementedAssessment && <blockquote className="task-confirmation-assessment">{task.implementedAssessment}</blockquote>}
    <dl className="task-confirmation-evidence">
      <div><dt>Implementation</dt><dd>{task.implementedAssessment ? "reported" : "missing"}</dd></div>
      <div><dt>Runs</dt><dd>{succeededRuns} succeeded</dd></div>
      <div><dt>Records</dt><dd>{records.length} indexed</dd></div>
      <div><dt>Acceptance evidence</dt><dd>{passedEvidence} passed</dd></div>
    </dl>
    {!canApprove && <p className="task-confirmation-note">An Approver or Owner role is required to confirm this Task.</p>}
    {error && <div className="form-error" role="alert">{error}</div>}
    {canApprove && !preview && <button className="primary-button" type="button" disabled={Boolean(busy)} onClick={() => void loadPreview()}>
      {busy === "preview" ? "Preparing confirmation..." : "Confirm task"}
    </button>}
    {preview && <div className="task-confirmation-preview" data-task-confirm-preview>
      <strong>Confirm Task #{task.publicId} once</strong>
      <p>The server checked revision {preview.result.expectedWorkspaceRevision} and bound this decision to the exact Task state.</p>
      {preview.result.errors.map((item) => <div className={item.code === "human_approval_required" ? "approval-required" : "form-error"} key={item.code}>
        <strong>{item.code === "human_approval_required" ? "Your confirmation is required" : item.code}</strong><span>{item.message}</span>
      </div>)}
      {preview.result.warnings.length > 0 && <fieldset className="warning-list"><legend>Review warnings</legend>
        {preview.result.warnings.map((warning) => <label key={warning.code}><input type="checkbox" checked={acknowledgedWarnings.includes(warning.code)} onChange={(event) => {
          const checked = event.currentTarget.checked;
          setAcknowledgedWarnings((current) => checked ? [...new Set([...current, warning.code])] : current.filter((code) => code !== warning.code));
        }} /><span><strong>{warning.code}</strong>{warning.message}</span></label>)}
      </fieldset>}
      {preview.result.warnings.length > 0 && <label className="command-input-label">Why is it safe to proceed?<textarea className="proceed-reason" value={proceedReason} onChange={(event) => setProceedReason(event.currentTarget.value)} /></label>}
      <button className="primary-button" data-task-confirm-execute type="button" disabled={busy === "execute" || !canExecute} onClick={() => void approveAndExecute()}>
        {busy === "execute" ? "Confirming..." : "Confirm task once"}
      </button>
      <button className="quiet-button task-confirmation-cancel" type="button" disabled={Boolean(busy)} onClick={() => setPreview(undefined)}>Cancel</button>
    </div>}
  </section>;
}
