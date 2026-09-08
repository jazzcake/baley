// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { executeCommand, previewCommand } from "../api/auth";
import type { Phase, Task } from "../domain/model";
import { TaskCommandEditor, type TaskCommandDraft } from "./TaskCommandEditor";

vi.mock("../api/auth", () => ({ previewCommand: vi.fn(), executeCommand: vi.fn() }));
vi.mock("../debug/viewer-trace", () => ({ traceViewer: vi.fn() }));

const task: Task = { id: "task-7", publicId: 7, laneId: "client", phaseId: "build", title: "Original", description: "Before", currentSummary: "Working", status: "in_progress" };
const phases: Phase[] = [
  { id: "plan", name: "Plan", order: 0, state: "completed" },
  { id: "build", name: "Build", order: 1, state: "active" },
  { id: "ship", name: "Ship", order: 2, state: "planned" },
];

function editor(overrides: Partial<Parameters<typeof TaskCommandEditor>[0]> = {}) {
  const onExecuted = vi.fn<(execution: { commandId: string; workspaceRevision: number; eventIds: string[] }, draft: TaskCommandDraft) => void>();
  render(<TaskCommandEditor workspaceId="workspace" workspaceRevision={12} task={task} phases={phases} csrfToken="csrf" canOperate onExecuted={onExecuted} {...overrides} />);
  return onExecuted;
}

describe("TaskCommandEditor", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(previewCommand).mockImplementation(async (command) => ({
      commandHash: "sha256:preview",
      expectedWorkspaceRevision: 12,
      requiredCapability: "workspace:operate",
      projectedDiff: { command: command.name },
      errors: [], warnings: [], advisories: [], entityType: "task", entityId: task.id,
    }));
    vi.mocked(executeCommand).mockResolvedValue({ commandId: "command", workspaceRevision: 13, eventIds: ["event"] });
  });
  afterEach(cleanup);

  it("previews and executes a human task.update through the shared command boundary", async () => {
    const onExecuted = editor();
    fireEvent.change(screen.getByLabelText("Title"), { target: { value: "Updated title" } });
    fireEvent.change(screen.getByLabelText("Current summary"), { target: { value: "Updated summary" } });
    fireEvent.click(screen.getByRole("button", { name: "Review task changes" }));

    await waitFor(() => expect(previewCommand).toHaveBeenCalledWith(expect.objectContaining({
      name: "task.update",
      arguments: { workspaceId: "workspace", taskId: 7, title: "Updated title", currentSummary: "Updated summary" },
      envelope: expect.objectContaining({ expectedWorkspaceRevision: 12 }),
    }), "csrf"));
    fireEvent.click(screen.getByRole("button", { name: "Apply reviewed command" }));
    await waitFor(() => expect(executeCommand).toHaveBeenCalledWith(expect.objectContaining({ name: "task.update" }), "csrf"));
    expect(onExecuted).toHaveBeenCalledWith(expect.objectContaining({ workspaceRevision: 13 }), expect.objectContaining({ kind: "update", title: "Updated title" }));
  });

  it("offers only valid Phase targets and executes task.move without XY persistence", async () => {
    const onExecuted = editor();
    expect(screen.queryByRole("option", { name: "Plan" })).toBeNull();
    expect(screen.queryByRole("option", { name: "Build" })).toBeNull();
    expect(screen.getByRole("option", { name: "Ship" })).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Review Phase move" }));
    await waitFor(() => expect(previewCommand).toHaveBeenCalledWith(expect.objectContaining({
      name: "task.move", arguments: { workspaceId: "workspace", taskId: 7, targetPhaseId: "ship" },
    }), "csrf"));
    fireEvent.click(screen.getByRole("button", { name: "Apply reviewed command" }));
    await waitFor(() => expect(onExecuted).toHaveBeenCalledWith(expect.anything(), { kind: "move", targetPhaseId: "ship" }));
    expect(JSON.stringify(vi.mocked(executeCommand).mock.calls[0]?.[0])).not.toContain("position");
  });

  it("guards view-only membership and terminal Tasks", () => {
    const { rerender } = render(<TaskCommandEditor workspaceId="workspace" workspaceRevision={12} task={task} phases={phases} csrfToken="csrf" canOperate={false} onExecuted={vi.fn()} />);
    expect(screen.getByText("View-only membership")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Review task changes" })).toBeNull();
    rerender(<TaskCommandEditor workspaceId="workspace" workspaceRevision={12} task={{ ...task, status: "confirmed" }} phases={phases} csrfToken="csrf" canOperate onExecuted={vi.fn()} />);
    expect(screen.getByText("Task is terminal")).toBeTruthy();
  });

  it("discards a failed execution preview and preserves server-authored state for retry", async () => {
    vi.mocked(executeCommand).mockRejectedValueOnce(new Error("revision changed"));
    editor();
    fireEvent.change(screen.getByLabelText("Title"), { target: { value: "Retry me" } });
    fireEvent.click(screen.getByRole("button", { name: "Review task changes" }));
    fireEvent.click(await screen.findByRole("button", { name: "Apply reviewed command" }));
    expect((await screen.findByRole("alert")).textContent).toContain("revision changed");
    expect(document.querySelector("[data-task-command-preview]")).toBeNull();
    expect((screen.getByLabelText("Title") as HTMLInputElement).value).toBe("Retry me");
  });
});
