export function traceViewer(event: string, details: Record<string, unknown>) {
  if (!import.meta.env.DEV) return;
  const traceWindow = window as Window & { __BALEY_VIEWER_TRACE__?: Array<{ event: string; details: Record<string, unknown> }> };
  traceWindow.__BALEY_VIEWER_TRACE__ = [...(traceWindow.__BALEY_VIEWER_TRACE__ ?? []).slice(-99), { event, details }];
  console.info(`[Baley viewer] ${event}`, details);
}
