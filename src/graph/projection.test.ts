import { describe, expect, it } from "vitest";
import { pilotReadyFixture } from "../fixtures/pilot-ready";
import { connectedTaskIds, visibleTaskIds } from "./projection";

describe("graph projection", () => {
  it("keeps every task in the multi-lane view", () => {
    expect(visibleTaskIds(pilotReadyFixture, { kind: "multi" }).size).toBe(pilotReadyFixture.tasks.length);
  });

  it("keeps the full horizontal graph in lane focus", () => {
    const ids = visibleTaskIds(pilotReadyFixture, { kind: "lane", id: "client" });
    expect(ids.size).toBe(pilotReadyFixture.tasks.length);
  });

  it("finds upstream and downstream dependencies", () => {
    expect(connectedTaskIds(pilotReadyFixture, "screen-design")).toEqual(new Set(["screen-design", "pilot-ui", "a11y"]));
  });
});
