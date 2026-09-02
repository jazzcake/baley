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

type ConfirmationPreview = { command: CommandRequest; result: CommandPreview; scopeKey: string };

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
  const requestGenerationRef = useRef(0);
  const scopeKey = `${workspaceId}:${workspaceRevision}:${task.id}:${task.publicId}:${canApprove ? "approver" : "read-only"}`;
  const scopeRef = useRef(scopeKey);
  scopeRef.current = scopeKey;
  const [preview, setPreview] = useState<ConfirmationPreview>();
  const [acknowledgedWarnings, setAcknowledgedWarnings] = useState<string[]>([]);
  const [proceedReason, setProceedReason] = useState("");
  const [busy, setBusy] = useState<"preview" | "execute">();
  const [error, setError] = useState<string>();

  useEffect(() => {
    requestGenerationRef.current += 1;
    setPreview(undefined);
    setAcknowledgedWarnings([]);
    setProceedReason("");
    setBusy(undefined);
    setError(undefined);
  }, [canApprove, task.id, workspaceRevision]);

  const revokeIssuedGrant = async (grantId: string, reason: string) => {
    try {
      await revokeApprovalGrant(workspaceId, grantId, csrfToken);
      traceViewer("task-confirmation:grant-revoke-state", { workspaceId, taskId: task.publicId, grantId, reason, outcome: "revoked" });
    } catch (revokeError) {
      traceViewer("task-confirmation:grant-revoke-state", {
        workspaceId, taskId: task.publicId, grantId, reason, outcome: "failed",
        error: errorMessage(revokeError),
      });
    }
  };

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
    if (!canApprove) return;
    const requestScope = scopeKey;
    const requestGeneration = ++requestGenerationRef.current;
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
      if (scopeRef.current !== requestScope || requestGenerationRef.current !== requestGeneration) {
        traceViewer("task-confirmation:stale-preview-ignored", {
          requestScope, currentScope: scopeRef.current, requestGeneration,
          currentGeneration: requestGenerationRef.current,
        });
        return;
      }
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
      setPreview({ command, result, scopeKey: requestScope });
      setAcknowledgedWarnings([]);
      setProceedReason("");
    } catch (reason) {
      traceViewer("task-confirmation:request-failed", { stage: "preview", requestScope, error: errorMessage(reason) });
      if (scopeRef.current === requestScope && requestGenerationRef.current === requestGeneration) setError(errorMessage(reason));
    } finally {
      if (scopeRef.current === requestScope && requestGenerationRef.current === requestGeneration) setBusy(undefined);
    }
  };

  const approveAndExecute = async () => {
    if (!preview || !canApprove) return;
    const requestScope = scopeKey;
    const commandTaskId = preview.command.arguments.taskId;
    const commandWorkspaceId = preview.command.arguments.workspaceId;
    const previewMatchesCurrentTarget = preview.scopeKey === requestScope && scopeRef.current === requestScope &&
      preview.command.name === "task.confirm" && commandWorkspaceId === workspaceId && commandTaskId === task.publicId &&
      preview.command.envelope.expectedWorkspaceRevision === workspaceRevision &&
      (!preview.result.entityType || preview.result.entityType === "task") &&
      (!preview.result.entityId || preview.result.entityId === task.id);
    if (!previewMatchesCurrentTarget) {
      traceViewer("task-confirmation:target-mismatch-blocked", {
        requestScope, previewScope: preview.scopeKey, currentScope: scopeRef.current,
        calculatedTarget: { workspaceId, taskId: task.publicId, workspaceRevision },
        previewTarget: { commandWorkspaceId, commandTaskId, entityType: preview.result.entityType, entityId: preview.result.entityId },
      });
      setPreview(undefined);
      setError("The Task changed while you were reviewing it. Prepare a fresh confirmation.");
      return;
    }
    traceViewer("task-confirmation:event", {
      event: "confirm-task-once-click",
      calculatedTarget: { action: preview.command.name, workspaceId, taskId: task.publicId },
      reactState: { taskStatus: task.status, workspaceRevision },
      controllerState: { acknowledgedWarnings, proceedReasonPresent: Boolean(proceedReason.trim()) },
    });
    setBusy("execute");
    setError(undefined);
    let grantId = "";
    let stage: "grant" | "execute" = "grant";
    try {
      const grant = await issueApprovalGrant(workspaceId, {
        command: preview.command,
        acknowledgedWarningCodes: acknowledgedWarnings,
        proceedReason: proceedReason.trim(),
      }, csrfToken);
      grantId = grant.id;
      if (scopeRef.current !== requestScope) {
        await revokeIssuedGrant(grant.id, "target-changed-before-execute");
        grantId = "";
        traceViewer("task-confirmation:stale-grant-revoked", { requestScope, currentScope: scopeRef.current, grantId: grant.id });
        return;
      }
      traceViewer("task-confirmation:grant-state", {
        workspaceId, taskId: task.publicId, grantId, expiresAt: grant.expiresAt,
        commandHash: grant.commandHash, workspaceRevision: grant.workspaceRevision,
      });
      stage = "execute";
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
      traceViewer("task-confirmation:request-failed", { stage, requestScope, error: errorMessage(reason), grantIssued: Boolean(grantId) });
      if (grantId) await revokeIssuedGrant(grantId, `${stage}-failed`);
      if (scopeRef.current === requestScope) setError(errorMessage(reason));
    } finally {
      if (scopeRef.current === requestScope) setBusy(undefined);
    }
  };

  const blockingErrors = preview?.result.errors.filter((item) => item.code !== "human_approval_required") ?? [];
  const allWarningsAcknowledged = preview?.result.warnings.every((warning) => acknowledgedWarnings.includes(warning.code)) ?? false;
  const canExecute = Boolean(canApprove && preview && preview.scopeKey === scopeKey && blockingErrors.length === 0 && allWarningsAcknowledged &&
    (preview.result.warnings.length === 0 || proceedReason.trim()));
  const latestEvidence = acceptanceEvidence.reduce<AcceptanceEvidence | undefined>((latest, item) =>
    !latest || item.version > latest.version ? item : latest, undefined);
  const succeededRuns = runs.filter((run) => run.status === "succeeded").length;

  return <section ref={rootRef} className="task-confirmation" aria-label="Task confirmation">
    <div className="task-confirmation-heading"><span>HUMAN DECISION</span><strong>Review before confirming</strong></div>
    <p>Review this Task's implementation result and evidence. Confirmation records your decision; it does not grant the Agent any human authority.</p>
    {task.implementedAssessment && <blockquote className="task-confirmation-assessment">{task.implementedAssessment}</blockquote>}
    <dl className="task-confirmation-evidence">
      <div><dt>Implementation</dt><dd>{task.implementedAssessment ? "reported" : "missing"}</dd></div>
      <div><dt>Runs</dt><dd>{succeededRuns} succeeded</dd></div>
      <div><dt>Records</dt><dd>{records.length} indexed</dd></div>
      <div><dt>Acceptance evidence</dt><dd>{latestEvidence ? `v${latestEvidence.version} ${latestEvidence.verificationVerdict} / review ${latestEvidence.reviewVerdict}${latestEvidence.unresolvedBlockingCount ? ` / ${latestEvidence.unresolvedBlockingCount} blockers` : ""}` : "none reported"}</dd></div>
    </dl>
    {!canApprove && <p className="task-confirmation-note">An Approver or Owner role is required to confirm this Task.</p>}
    {error && <div className="form-error" role="alert">{error}</div>}
    {canApprove && !preview && <button className="primary-button" type="button" disabled={Boolean(busy)} onClick={() => void loadPreview()}>
      {busy === "preview" ? "Preparing confirmation..." : "Confirm task"}
    </button>}
    {canApprove && preview && <div className="task-confirmation-preview" data-task-confirm-preview>
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
