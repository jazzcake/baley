import { describe, expect, it } from "vitest";
import { resolveGateReference } from "./App";
import type { Gate } from "./domain/model";

const gates: Gate[] = [
  { id: "stable-gate-id", publicId: 7, alias: "release-ready", name: "Release Ready", fromPhaseId: "build", toPhaseId: "release", status: "open" },
  { id: "G#8", publicId: 9, name: "Legacy Reserved Shape", fromPhaseId: "release", toPhaseId: "observe", status: "open" },
];

describe("Gate references", () => {
  it("resolves internal ID, G# public number, and alias", () => {
    expect(resolveGateReference(gates, "stable-gate-id")?.id).toBe("stable-gate-id");
    expect(resolveGateReference(gates, "G#7")?.id).toBe("stable-gate-id");
    expect(resolveGateReference(gates, "g#7")?.id).toBe("stable-gate-id");
    expect(resolveGateReference(gates, "RELEASE-READY")?.id).toBe("stable-gate-id");
    expect(resolveGateReference(gates, "G#8")).toBeUndefined();
  });
});
