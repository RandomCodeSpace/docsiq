import { describe, it, expect } from "vitest";
import { parseSseChunk } from "../useUploadProgress";

describe("parseSseChunk", () => {
  it("parses a single complete JSON SSE frame", () => {
    const input =
      `data: {"job_id":"job-1","phase":"queued","done":false,"message":"queued: 1 files"}\n\n`;
    const { events, rest } = parseSseChunk(input);
    expect(rest).toBe("");
    expect(events).toHaveLength(1);
    expect(events[0]).toMatchObject({
      job_id: "job-1",
      phase: "queued",
      done: false,
      message: "queued: 1 files",
    });
  });

  it("parses multiple frames in one chunk and preserves order", () => {
    const input =
      `data: {"job_id":"j","phase":"chunk","chunks_total":5,"done":false}\n\n` +
      `data: {"job_id":"j","phase":"embed","chunks_done":3,"chunks_total":5,"done":false}\n\n` +
      `data: {"job_id":"j","phase":"done","done":true}\n\n`;
    const { events, rest } = parseSseChunk(input);
    expect(rest).toBe("");
    expect(events).toHaveLength(3);
    expect(events.map((e) => e.phase)).toEqual(["chunk", "embed", "done"]);
    expect(events[1]?.chunks_done).toBe(3);
    expect(events[2]?.done).toBe(true);
  });

  it("returns the unfinished tail when a frame is split across chunks", () => {
    const head = `data: {"job_id":"j","phase":"chunk","done":false}\n\ndata: {"job_id":"j",`;
    const { events, rest } = parseSseChunk(head);
    expect(events).toHaveLength(1);
    expect(rest).toBe(`data: {"job_id":"j",`);

    const tail = rest + `"phase":"done","done":true}\n\n`;
    const second = parseSseChunk(tail);
    expect(second.events).toHaveLength(1);
    expect(second.events[0]?.phase).toBe("done");
    expect(second.rest).toBe("");
  });

  it("treats plain-text legacy frames as synthetic message events", () => {
    const input = `data: queued: 1 files\n\ndata: error: boom\n\n`;
    const { events } = parseSseChunk(input);
    expect(events).toHaveLength(2);
    expect(events[0]?.phase).toBe("message");
    expect(events[0]?.message).toBe("queued: 1 files");
    expect(events[0]?.done).toBe(false);
    expect(events[1]?.phase).toBe("message");
    expect(events[1]?.done).toBe(true);
    expect(events[1]?.error).toBe("boom");
  });

  it("ignores non-data SSE lines (comments, retry, id)", () => {
    const input =
      `: heartbeat\nretry: 1000\nid: 42\ndata: {"job_id":"j","phase":"done","done":true}\n\n`;
    const { events } = parseSseChunk(input);
    expect(events).toHaveLength(1);
    expect(events[0]?.phase).toBe("done");
  });
});
