export type LayoutMode = "flow" | "tree";

const STORAGE_PREFIX = "baley:layout-mode:";

type LayoutModeStorage = Pick<Storage, "getItem" | "setItem">;

export function layoutModeStorageKey(workspaceId: string): string {
  return `${STORAGE_PREFIX}${workspaceId}`;
}

export function readLayoutMode(workspaceId: string, storage: LayoutModeStorage): LayoutMode {
  try {
    return storage.getItem(layoutModeStorageKey(workspaceId)) === "tree" ? "tree" : "flow";
  } catch {
    return "flow";
  }
}

export function writeLayoutMode(workspaceId: string, mode: LayoutMode, storage: LayoutModeStorage): boolean {
  try {
    storage.setItem(layoutModeStorageKey(workspaceId), mode);
    return true;
  } catch {
    return false;
  }
}
