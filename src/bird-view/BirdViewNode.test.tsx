// @vitest-environment jsdom

import React from "react";
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { BirdViewNode } from "./BirdViewNode";

vi.mock("@xyflow/react", () => ({
  Handle: ({ type }: { type: string }) => <i data-handle={type} />,
  Position: { Left: "left", Right: "right" },
}));

afterEach(cleanup);

describe("Bird View node visual contract", () => {
  it.each(["active", "achieved", "parked"] as const)("exposes the %s status through Baley card semantics", (status) => {
    const props = {
      data: { id: "n", title: "Outcome", summary: "Summary", content: "", status, detailed: true },
      selected: true,
    } as unknown as React.ComponentProps<typeof BirdViewNode>;
    render(<BirdViewNode {...props} />);
    const card = screen.getByRole("article", { name: `Outcome, ${status}` });
    expect([...card.classList]).toEqual(expect.arrayContaining(["bird-node", `bird-node-${status}`, "selected"]));
    expect(card.getAttribute("data-status")).toBe(status);
    expect(screen.getByText(status).classList.contains(`bird-node-status-${status}`)).toBe(true);
    expect(card.querySelectorAll("[data-handle]")).toHaveLength(2);
  });

  it("uses an English empty-summary fallback", () => {
    const props = {
      data: { id: "n", title: "Outcome", summary: "", content: "", status: "active", detailed: true },
      selected: false,
    } as unknown as React.ComponentProps<typeof BirdViewNode>;
    render(<BirdViewNode {...props} />);
    expect(screen.getByText("No summary yet.")).toBeTruthy();
  });
});
