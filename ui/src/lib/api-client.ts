import type { ApiError } from "@/types/api";

// Before cookies are set the first time, the UI may have been shipped a
// one-shot bearer via the meta tag (legacy). We exchange it for a cookie
// exactly once, then never read or send the key again. If no meta tag
// exists (production path), we rely entirely on cookies already set by
// the operator's OOB provisioning (e.g. `docsiq login`).
let sessionReady: Promise<void> | null = null;

function readOneShotBearerFromMeta(): string | null {
  if (typeof document === "undefined") return null;
  const m = document.querySelector('meta[name="docsiq-api-key"]');
  const v = m?.getAttribute("content");
  return v && v.length > 0 ? v : null;
}

async function establishSession(bearer: string): Promise<void> {
  const m = document.querySelector('meta[name="docsiq-api-key"]');
  m?.parentElement?.removeChild(m);

  const res = await fetch("/api/session", {
    method: "POST",
    credentials: "include",
    headers: { Authorization: `Bearer ${bearer}` },
  });
  if (!res.ok) {
    let body: ApiError = { error: `HTTP ${res.status}` };
    try { body = await res.json(); } catch { /* non-json */ }
    throw new ApiErrorResponse(res.status, body);
  }
}

export function initAuth(): void {
  const bearer = readOneShotBearerFromMeta();
  sessionReady = bearer ? establishSession(bearer) : Promise.resolve();
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
  if (sessionReady) await sessionReady;
  const headers = new Headers(init.headers);
  if (init.body && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }
  const res = await fetch(path, { ...init, headers, credentials: "include" });
  if (!res.ok) {
    let body: ApiError = { error: `HTTP ${res.status}` };
    try {
      body = await res.json();
    } catch {
      /* non-json */
    }
    throw new ApiErrorResponse(res.status, body);
  }
  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}
