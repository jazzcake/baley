import { describe, expect, it } from "vitest";
import { pilotReadyFixture } from "../fixtures/pilot-ready";
import { validateFixture } from "./validate-fixture";

describe("validateFixture", () => {
  it("accepts the Pilot Ready fixture", () => expect(() => validateFixture(pilotReadyFixture)).not.toThrow());
  it("rejects dependency cycles", () => {
    const fixture = { ...pilotReadyFixture, dependencies: [...pilotReadyFixture.dependencies, { id: "cycle", fromTaskId: "api-build", toTaskId: "api-design" }] };
    expect(() => validateFixture(fixture)).toThrow("cycle");
  });
  it("allows dependencies across lane and phase boundaries", () => {
    const fixture = {
      ...pilotReadyFixture,
      dependencies: [
        ...pilotReadyFixture.dependencies,
        { id: "cross-phase", fromTaskId: "findings", toTaskId: "user-test" },
      ],
    };
    expect(() => validateFixture(fixture)).not.toThrow();
  });
});
