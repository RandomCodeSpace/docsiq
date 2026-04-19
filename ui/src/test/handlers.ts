import { http, HttpResponse } from "msw";

export const handlers = [
  http.get("/api/stats", () =>
    HttpResponse.json({
      documents: 42,
      chunks: 512,
      entities: 380,
      relationships: 820,
      communities: 8,
      notes: 17,
      last_indexed: new Date().toISOString(),
    }),
  ),
  http.get("/api/projects", () =>
    HttpResponse.json([{ slug: "_default", name: "_default" }]),
  ),
];
