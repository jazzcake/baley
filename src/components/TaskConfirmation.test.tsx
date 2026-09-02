// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  executeCommand,
  issueApprovalGrant,
  previewCommand,
  revokeApprovalGrant,
} from "../api/auth";
import type { Task } from "../domain/model";
import { TaskConfirmation } from "./TaskConfirmation";

vi.mock("../api/auth", () => ({
  previewCommand: vi.fn(),
  issueApprovalGrant: vi.fn(),
  executeCommand: vi.fn(),
  revokeApprovalGrant: vi.fn(),
}));
vi.mock("../debug/viewer-trace", () => ({ traceViewer: vi.fn() }));

const task: Task = {
  id: "task-internal",
  publicId: 159,
  laneId: "server",
  phaseId: "multi-user-operations",
  title: "Long-lived login",
  description: "Keep the browser session active.",
  currentSummary: "Google login remains active for normal long-term use.",
  implementedAssessment: "Tests, deployment, and live session verification passed.",
  status: "implemented",
};

const baseProps = {
  workspaceId: "workspace",
  workspaceRevision: 12,
  task,
  csrfToken: "csrf",
  canApprove: true,
  runs: [{ id: "run", taskId: task.id, kind: "implementation", status: "succeeded", startedAt: "2026-09-02T00:00:00Z" }],
  records: [{ id: "record", taskId: task.id, recordType: "completion-report", repositoryId: "repo", relativePath: "report.md", state: "committed", shortSummary: "passed" }],
  acceptanceEvidence: [{ id: "evidence", taskId: task.id, version: 1, completionReportRecordId: "record", verificationVerdict: "passed" as const, independentReviewRecordId: "review", reviewVerdict: "pass" as const, unresolvedBlockingCount: 0, reportedByActorId: "agent" }],
};

describe("TaskConfirmation", () => {
  beforeEach(() => {
    vi.stubGlobal("requestAnimationFrame", (callback: FrameRequestCallback) => window.setTimeout(() => callback(0), 0));
    vi.stubGlobal("cancelAnimationFrame", (id: number) => window.clearTimeout(id));
    vi.mocked(previewCommand).mockResolvedValue({
      commandHash: "sha256:command",
      expectedWorkspaceRevision: 12,
      requiredCapability: "task:approve",
      projectedDiff: { status: "confirmed" },
      errors: [{ code: "human_approval_required", message: "Human confirmation is required." }],
      warnings: [],
      advisories: [],
      entityType: "task",
      entityId: task.id,
    });
    vi.mocked(issueApprovalGrant).mockResolvedValue({
      id: "grant",
      expiresAt: "2026-09-02T00:01:00Z",
      commandHash: "sha256:command",
      workspaceRevision: 12,
    });
    vi.mocked(executeCommand).mockResolvedValue({ commandId: "command", workspaceRevision: 13, eventIds: ["event"] });
    vi.mocked(revokeApprovalGrant).mockResolvedValue(undefined);
  });

  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
    vi.unstubAllGlobals();
  });

  it("shows readable evidence and withholds the action from a non-approver", () => {
    render(<TaskConfirmation {...baseProps} canApprove={false} onConfirmed={vi.fn()} />);

    expect(screen.getByText("Tests, deployment, and live session verification passed.")).toBeTruthy();
    expect(screen.getByText("1 succeeded")).toBeTruthy();
    expect(screen.getByText("1 indexed")).toBeTruthy();
    expect(screen.getByText("1 passed")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Confirm task" })).toBeNull();
    expect(screen.getByText(/Approver or Owner/)).toBeTruthy();
  });

  it("uses a fresh unapproved preview, a browser grant, and one exact execution", async () => {
    const onConfirmed = vi.fn();
    render(<TaskConfirmation {...baseProps} onConfirmed={onConfirmed} />);

    fireEvent.click(screen.getByRole("button", { name: "Confirm task" }));
    await screen.findByRole("button", { name: "Confirm task once" });

    const previewedCommand = vi.mocked(previewCommand).mock.calls[0]![0];
    expect(previewedCommand).toMatchObject({
      name: "task.confirm",
      arguments: { workspaceId: "workspace", taskId: 159 },
      envelope: { expectedWorkspaceRevision: 12 },
    });
    expect(previewedCommand.envelope.approvalGrantId).toBeUndefined();
    expect(previewedCommand.envelope.humanApprovalAttestation).toBeUndefined();

    fireEvent.click(screen.getByRole("button", { name: "Confirm task once" }));
    await waitFor(() => expect(onConfirmed).toHaveBeenCalledWith(expect.objectContaining({ workspaceRevision: 13 })));

    expect(issueApprovalGrant).toHaveBeenCalledWith("workspace", {
      command: previewedCommand,
      acknowledgedWarningCodes: [],
      proceedReason: "",
    }, "csrf");
    expect(executeCommand).toHaveBeenCalledWith(expect.objectContaining({
      name: "task.confirm",
      envelope: expect.objectContaining({ approvalGrantId: "grant", expectedWorkspaceRevision: 12 }),
    }), "csrf");
  });

  it("requires every warning and a reason before the final confirmation", async () => {
    vi.mocked(previewCommand).mockResolvedValueOnce({
      commandHash: "sha256:warning",
      expectedWorkspaceRevision: 12,
      requiredCapability: "task:approve",
      projectedDiff: {},
      errors: [{ code: "human_approval_required", message: "Human confirmation is required." }],
      warnings: [{ code: "dangling_path", message: "This Task has no successor." }],
      advisories: [],
    });
    render(<TaskConfirmation {...baseProps} onConfirmed={vi.fn()} />);

    fireEvent.click(screen.getByRole("button", { name: "Confirm task" }));
    const executeButton = await screen.findByRole("button", { name: "Confirm task once" });
    expect((executeButton as HTMLButtonElement).disabled).toBe(true);
    fireEvent.click(screen.getByRole("checkbox"));
    expect((executeButton as HTMLButtonElement).disabled).toBe(true);
    fireEvent.change(screen.getByRole("textbox", { name: "Why is it safe to proceed?" }), { target: { value: "Intentional terminal Task." } });
    expect((executeButton as HTMLButtonElement).disabled).toBe(false);
  });

  it("revokes an issued grant when execution fails", async () => {
    vi.mocked(executeCommand).mockRejectedValueOnce(new Error("revision changed"));
    render(<TaskConfirmation {...baseProps} onConfirmed={vi.fn()} />);

    fireEvent.click(screen.getByRole("button", { name: "Confirm task" }));
    fireEvent.click(await screen.findByRole("button", { name: "Confirm task once" }));

    expect((await screen.findByRole("alert")).textContent).toContain("revision changed");
    await waitFor(() => expect(revokeApprovalGrant).toHaveBeenCalledWith("workspace", "grant", "csrf"));
  });
});
