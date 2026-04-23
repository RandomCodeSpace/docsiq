import { describe, it, expect, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "@/test/msw";
import { apiFetch, ApiErrorResponse, initAuth } from "../api-client";

describe("apiFetch", () => {
  it("returns parsed json on 200", async () => {
    server.use(http.get("/api/ok", () => HttpResponse.json({ hello: "world" })));
    const body = await apiFetch<{ hello: string }>("/api/ok");
    expect(body.hello).toBe("world");
  });

  it("throws ApiErrorResponse on 4xx with error + request_id", async () => {
    server.use(
      http.get("/api/bad", () =>
        HttpResponse.json({ error: "nope", request_id: "req-123" }, { status: 400 }),
      ),
    );
    try {
      await apiFetch("/api/bad");
      throw new Error("should not reach");
    } catch (e) {
      expect(e).toBeInstanceOf(ApiErrorResponse);
      expect((e as ApiErrorResponse).status).toBe(400);
      expect((e as ApiErrorResponse).requestId).toBe("req-123");
      expect((e as ApiErrorResponse).message).toBe("nope");
    }
  });

  it("handles 204 no-content", async () => {
    server.use(http.delete("/api/x", () => new HttpResponse(null, { status: 204 })));
    const r = await apiFetch("/api/x", { method: "DELETE" });
    expect(r).toBeUndefined();
  });

  it("sends credentials: 'include' on every fetch", async () => {
    const spy = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response("{}", { status: 200, headers: { "content-type": "application/json" } }),
    );
    await apiFetch("/api/stats");
    const init = (spy.mock.calls[0][1] ?? {}) as RequestInit;
    expect(init.credentials).toBe("include");
    spy.mockRestore();
  });

  it("does not set Authorization header on data-path fetch even when a key exists in a meta tag", async () => {
    const meta = document.createElement("meta");
    meta.setAttribute("name", "docsiq-api-key");
    meta.setAttribute("content", "s3cret");
    document.head.appendChild(meta);
    // Spy must be installed BEFORE initAuth() so the /api/session exchange
    // is captured by the mock and not passed through to MSW (which has no
    // handler for it and would throw).
    const spy = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response("{}", { status: 200, headers: { "content-type": "application/json" } }),
    );
    try {
      initAuth();
      await apiFetch("/api/stats");
      const statsCall = spy.mock.calls.find((c) => c[0] === "/api/stats");
      expect(statsCall).toBeDefined();
      const init = (statsCall![1] ?? {}) as RequestInit;
      const hdrs = new Headers(init.headers);
      expect(hdrs.has("Authorization")).toBe(false);
    } finally {
      spy.mockRestore();
      if (meta.parentElement) document.head.removeChild(meta);
    }
  });
});
