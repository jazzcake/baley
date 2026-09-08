import { requestJSON } from "../api/http";
import type { BirdView, BirdViewCommand, BirdViewExecution, BirdViewGraph, BirdViewNodeFocus } from "./model";

export async function listBirdViews(signal?: AbortSignal) {
  const response = await requestJSON<{ items: BirdView[] }>("/v1/bird-views", { signal });
  return response.items;
}

export function fetchBirdViewGraph(id: string, signal?: AbortSignal) {
  return requestJSON<BirdViewGraph>(`/v1/bird-views/${encodeURIComponent(id)}/graph`, { signal });
}

export function fetchBirdViewNodeFocus(id: string, nodeId: string, signal?: AbortSignal) {
  return requestJSON<BirdViewNodeFocus>(`/v1/bird-views/${encodeURIComponent(id)}/nodes/${encodeURIComponent(nodeId)}/focus`, { signal });
}

export function executeBirdViewCommand(command: BirdViewCommand, csrfToken: string) {
  return requestJSON<BirdViewExecution>("/v1/commands/execute", {
    method: "POST",
    body: JSON.stringify(command),
  }, csrfToken);
}
