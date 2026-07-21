export type CanvasViewport = { x: number; y: number; zoom: number };

export function zoomViewportAtCenter(
  current: CanvasViewport,
  factor: number,
  width: number,
  height: number,
  minZoom: number,
  maxZoom: number,
): CanvasViewport | undefined {
  if (width <= 0 || height <= 0 || current.zoom <= 0) return undefined;
  const zoom = Math.min(maxZoom, Math.max(minZoom, current.zoom * factor));
  const centerX = width / 2;
  const centerY = height / 2;
  const graphX = (centerX - current.x) / current.zoom;
  const graphY = (centerY - current.y) / current.zoom;
  return { x: centerX - graphX * zoom, y: centerY - graphY * zoom, zoom };
}

export function fitViewportToCanvas(
  contentWidth: number,
  contentHeight: number,
  width: number,
  height: number,
  minZoom: number,
  maxZoom: number,
  padding = 32,
): CanvasViewport | undefined {
  if (contentWidth <= 0 || contentHeight <= 0 || width <= 0 || height <= 0) return undefined;
  const availableWidth = Math.max(1, width - padding * 2);
  const availableHeight = Math.max(1, height - padding * 2);
  const zoom = Math.min(maxZoom, Math.max(minZoom, Math.min(availableWidth / contentWidth, availableHeight / contentHeight)));
  return {
    x: (width - contentWidth * zoom) / 2,
    y: (height - contentHeight * zoom) / 2,
    zoom,
  };
}
