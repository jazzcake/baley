import { describe, expect, it } from "vitest";
import { resolveAuthMode } from "./AuthProvider";

describe("Viewer auth mode cutover", () => {
  it("defaults production builds to enforced and keeps local development compatible", () => {
    expect(resolveAuthMode(undefined, true)).toBe("enforced");
    expect(resolveAuthMode("", true)).toBe("enforced");
    expect(resolveAuthMode(undefined, false)).toBe("legacy");
    expect(resolveAuthMode("legacy", true)).toBe("legacy");
    expect(resolveAuthMode("enforced", false)).toBe("enforced");
  });
});
