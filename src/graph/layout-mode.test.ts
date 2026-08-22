import { describe, expect, it } from "vitest";
import { layoutModeStorageKey, readLayoutMode, writeLayoutMode } from "./layout-mode";

describe("Workspace layout mode persistence", () => {
  it("defaults invalid and missing values to flow", () => {
    const values = new Map<string, string>();
    const storage = { getItem: (key: string) => values.get(key) ?? null, setItem: (key: string, value: string) => void values.set(key, value) };
    expect(readLayoutMode("one", storage)).toBe("flow");
    values.set(layoutModeStorageKey("one"), "diagonal");
    expect(readLayoutMode("one", storage)).toBe("flow");
  });

  it("persists Tree independently for each Workspace", () => {
    const values = new Map<string, string>();
    const storage = { getItem: (key: string) => values.get(key) ?? null, setItem: (key: string, value: string) => void values.set(key, value) };
    expect(writeLayoutMode("one", "tree", storage)).toBe(true);
    expect(readLayoutMode("one", storage)).toBe("tree");
    expect(readLayoutMode("two", storage)).toBe("flow");
  });

  it("fails safely when storage is unavailable", () => {
    const storage = { getItem: () => { throw new Error("blocked"); }, setItem: () => { throw new Error("blocked"); } };
    expect(readLayoutMode("one", storage)).toBe("flow");
    expect(writeLayoutMode("one", "tree", storage)).toBe(false);
  });
});
