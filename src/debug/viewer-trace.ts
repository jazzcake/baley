export function traceViewer(event: string, details: Record<string, unknown>) {
  if (import.meta.env.DEV) console.info(`[Baley viewer] ${event}`, details);
}
