import type { ApiError } from "@/types/api";
import { useAuthStore } from "@/stores/auth";

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

// FormData/Blob/URLSearchParams/streams/buffers carry their own framing —
// the browser sets Content-Type (with the multipart boundary, etc.) when
// fetch builds the request. Defaulting to application/json here would clobber
// that boundary and produce an unparseable body.
function isBrowserManagedBody(body: BodyInit): boolean {
  if (typeof FormData !== "undefined" && body instanceof FormData) return true;
  if (typeof Blob !== "undefined" && body instanceof Blob) return true;
  if (typeof URLSearchParams !== "undefined" && body instanceof URLSearchParams) return true;
  if (typeof ReadableStream !== "undefined" && body instanceof ReadableStream) return true;
  if (body instanceof ArrayBuffer) return true;
  if (ArrayBuffer.isView(body)) return true;
  return false;
}

// Shared low-level request helper. Centralises the auth-required signal
// so a 401 from any HTTP surface (apiFetch on /api/*, mcpRequest on
// /mcp/*) flips the same auth store the AuthRequiredBanner reads from.
// Returns the raw Response so callers that need headers or non-JSON
// framing (e.g. MCP's SSE event-stream + Mcp-Session-Id) can handle it.
async function rawRequest(path: string, init: RequestInit = {}): Promise<Response> {
  if (sessionReady) await sessionReady;
  const headers = new Headers(init.headers);
  if (init.body && !headers.has("Content-Type") && !isBrowserManagedBody(init.body)) {
    headers.set("Content-Type", "application/json");
  }
  const res = await fetch(path, { ...init, headers, credentials: "include" });
  if (res.status === 401) {
    useAuthStore.getState().signalUnauthorized();
  }
  return res;
}

export async function apiFetch<T>(
  path: string,
  init: RequestInit = {},
): Promise<T> {
  const res = await rawRequest(path, init);
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

// MCP can't use apiFetch() directly because it needs Mcp-Session-Id from
// response headers and may receive an SSE text/event-stream body that the
// generic JSON shape can't expose. mcpRequest mirrors apiFetch's
// session-bootstrap, Content-Type defaulting, credentials, and shared
// 401 → auth-store gate, then hands the caller the raw Response for
// MCP-specific framing.
export async function mcpRequest(path: string, init: RequestInit = {}): Promise<Response> {
  return rawRequest(path, init);
}
