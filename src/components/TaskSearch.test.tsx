// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { Task } from "../domain/model";
import { TaskSearch, taskSearchResults } from "./TaskSearch";

const tasks: Task[] = [
  { id: "pilot", publicId: 125, laneId: "adoption", phaseId: "pilot", title: "Day Tripper Pilot 온보딩", description: "", status: "pending" },
  { id: "review", publicId: 124, laneId: "adoption", phaseId: "enablement", title: "Embedding Enablement 수용 검증", description: "", status: "confirmed" },
  { id: "another", publicId: 126, laneId: "adoption", phaseId: "pilot", title: "Pilot 실행과 복원", description: "", status: "in_progress" },
];

afterEach(cleanup);

describe("taskSearchResults", () => {
  it("uses an exact public ID when the query starts with #", () => {
    expect(taskSearchResults(tasks, "#125").map((task) => task.id)).toEqual(["pilot"]);
    expect(taskSearchResults(tasks, "#12")).toEqual([]);
    expect(taskSearchResults(tasks, "#pilot")).toEqual([]);
  });

  it("matches title text partially and without case sensitivity", () => {
    expect(taskSearchResults(tasks, "pilot").map((task) => task.id)).toEqual(["another", "pilot"]);
    expect(taskSearchResults(tasks, "수용 검증").map((task) => task.id)).toEqual(["review"]);
  });
});

describe("TaskSearch", () => {
  it("selects the first result with Enter", () => {
    const onSelect = vi.fn();
    render(<TaskSearch tasks={tasks} onSelect={onSelect} />);

    const input = screen.getByRole("combobox", { name: "Task 검색" });
    fireEvent.change(input, { target: { value: "#125" } });
    expect(screen.getByRole("option", { name: /#125.*Day Tripper/ })).toBeTruthy();
    fireEvent.keyDown(input, { key: "Enter" });

    expect(onSelect).toHaveBeenCalledWith(tasks[0], "#125");
    expect((input as HTMLInputElement).value).toBe("#125");
  });

  it("supports arrow-key result selection", () => {
    const onSelect = vi.fn();
    render(<TaskSearch tasks={tasks} onSelect={onSelect} />);

    const input = screen.getByRole("combobox", { name: "Task 검색" });
    fireEvent.change(input, { target: { value: "pilot" } });
    fireEvent.keyDown(input, { key: "ArrowDown" });
    fireEvent.keyDown(input, { key: "Enter" });

    expect(onSelect).toHaveBeenCalledWith(tasks[0], "pilot");
  });
});
