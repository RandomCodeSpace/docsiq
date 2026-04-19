import type { ApiError } from "@/types/api";

let bearer: string | null = null;

function readBearerFromMeta(): string | null {
  if (typeof document === "undefined") return null;
  const m = document.querySelector('meta[name="docsiq-api-key"]');
  const v = m?.getAttribute("content");
  return v && v.length > 0 ? v : null;
}

export function initAuth() {
  bearer = readBearerFromMeta();
}

export class ApiErrorResponse extends Error {
  status: number;
  requestId?: string;
  constructor(status: number, body: ApiError) {
    super(body.error);
    this.status = status;
    this.requestId = body.request_id;
  }
}

export async function apiFetch<T>(
  path: string,
  init: RequestInit = {},
): Promise<T> {
  const headers = new Headers(init.headers);
  if (bearer) headers.set("Authorization", `Bearer ${bearer}`);
  if (init.body && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }
  const res = await fetch(path, { ...init, headers });
  if (!res.ok) {
    let body: ApiError = { error: `HTTP ${res.status}` };
    try { body = await res.json(); } catch { /* non-json */ }
    throw new ApiErrorResponse(res.status, body);
  }
  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}
