export class APIError extends Error {
  constructor(
    message: string,
    readonly status: number,
    readonly code: string,
    readonly retryAfter?: string,
  ) {
    super(message);
    this.name = "APIError";
  }
}

const baseURL = (import.meta.env.VITE_BALEY_API_URL || "http://127.0.0.1:8080").replace(/\/$/, "");

type APIErrorBody = {
  error?: {
    code?: string;
    message?: string;
  };
};

export async function requestJSON<T>(
  path: string,
  init: RequestInit = {},
  csrfToken?: string,
): Promise<T> {
  const headers = new Headers(init.headers);
  if (init.body !== undefined && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }
  if (csrfToken) headers.set("X-Baley-CSRF", csrfToken);

  const response = await fetch(`${baseURL}${path}`, {
    ...init,
    credentials: "include",
    headers,
  });

  if (!response.ok) {
    let body: APIErrorBody | undefined;
    try {
      body = await response.json() as APIErrorBody;
    } catch {
      // The typed status remains useful when an intermediary returned non-JSON.
    }
    throw new APIError(
      body?.error?.message || `Baley server returned HTTP ${response.status}`,
      response.status,
      body?.error?.code || "http_error",
      response.headers.get("Retry-After") ?? undefined,
    );
  }

  if (response.status === 204) return undefined as T;
  return await response.json() as T;
}
