export const INSPECTOR_MIN_WIDTH = 280;
export const INSPECTOR_MAX_WIDTH = 600;
export const INSPECTOR_DEFAULT_WIDTH = 330;

export function clampInspectorWidth(width: number): number {
  return Math.min(INSPECTOR_MAX_WIDTH, Math.max(INSPECTOR_MIN_WIDTH, width));
}

export function inspectorWidthFromPointer(startWidth: number, startX: number, currentX: number): number {
  return clampInspectorWidth(startWidth + startX - currentX);
}

export function inspectorWidthFromKey(width: number, key: string, largeStep = false): number {
  const step = largeStep ? 48 : 16;
  if (key === "ArrowLeft") return clampInspectorWidth(width + step);
  if (key === "ArrowRight") return clampInspectorWidth(width - step);
  if (key === "Home") return INSPECTOR_MIN_WIDTH;
  if (key === "End") return INSPECTOR_MAX_WIDTH;
  return width;
}
