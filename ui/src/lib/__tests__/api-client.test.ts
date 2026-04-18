import { describe, it, expect } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "@/test/msw";
import { apiFetch, ApiErrorResponse } from "../api-client";

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
});
