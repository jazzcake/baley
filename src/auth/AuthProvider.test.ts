import { describe, expect, it } from "vitest";
import { resolveAuthMode } from "./AuthProvider";

describe("Viewer auth mode cutover", () => {
  it("defaults every build to enforced and requires an explicit legacy opt-in", () => {
    expect(resolveAuthMode(undefined, true)).toBe("enforced");
    expect(resolveAuthMode("", true)).toBe("enforced");
    expect(resolveAuthMode(undefined, false)).toBe("enforced");
    expect(resolveAuthMode("legacy", true)).toBe("legacy");
    expect(resolveAuthMode("enforced", false)).toBe("enforced");
  });
});
