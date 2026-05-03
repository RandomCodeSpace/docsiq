import { useEffect, useRef, useState, useSyncExternalStore } from "react";

// UploadProgressEvent mirrors api/handlers.go uploadEvent. Keep the
// field names in sync — the SSE stream is the contract.
export interface UploadProgressEvent {
  job_id: string;
  file?: string;
  phase: string;
  chunks_done?: number;
  chunks_total?: number;
  message?: string;
  done: boolean;
  error?: string;
}

export interface UploadProgressState {
  jobId: string;
  file: string;
  phase: string;
  chunksDone: number;
  chunksTotal: number;
  message: string;
  done: boolean;
  error: string;
  history: UploadProgressEvent[];
}

const empty: UploadProgressState = {
  jobId: "",
  file: "",
  phase: "",
  chunksDone: 0,
  chunksTotal: 0,
  message: "",
  done: false,
  error: "",
  history: [],
};

function reduce(prev: UploadProgressState, evt: UploadProgressEvent): UploadProgressState {
  return {
    jobId: evt.job_id,
    file: evt.file ?? prev.file,
    phase: evt.phase,
    chunksDone: evt.chunks_done ?? 0,
    chunksTotal: evt.chunks_total ?? prev.chunksTotal,
    message: evt.message ?? "",
    done: evt.done,
    error: evt.error ?? "",
    history: [...prev.history, evt],
  };
}

// parseSseChunk extracts complete `data: {json}\n\n` frames from a
// streaming buffer. Returns the events parsed and the leftover tail
// (an unfinished frame). Exported for unit tests.
export function parseSseChunk(buffer: string): {
  events: UploadProgressEvent[];
  rest: string;
} {
  const events: UploadProgressEvent[] = [];
  let cursor = 0;
  while (cursor < buffer.length) {
    const sep = buffer.indexOf("\n\n", cursor);
    if (sep === -1) break;
    const frame = buffer.slice(cursor, sep);
    cursor = sep + 2;
    // SSE allows multi-line frames; we only care about `data: ...` lines.
    const dataLines: string[] = [];
    for (const line of frame.split("\n")) {
      if (line.startsWith("data: ")) {
        dataLines.push(line.slice(6));
      } else if (line.startsWith("data:")) {
        dataLines.push(line.slice(5));
      }
    }
    if (dataLines.length === 0) continue;
    const payload = dataLines.join("\n").trim();
    if (!payload) continue;
    try {
      const parsed = JSON.parse(payload) as UploadProgressEvent;
      // Skip plain-text fallback frames (legacy "queued: 1 files" etc.)
      if (typeof parsed === "object" && parsed && typeof parsed.phase === "string") {
        events.push(parsed);
      }
    } catch {
      // Plain-text legacy frame — surface it as a synthetic event so
      // older servers still produce *something*.
      events.push({
        job_id: "",
        phase: "message",
        message: payload,
        done: payload === "done" || payload.startsWith("error:"),
        error: payload.startsWith("error:") ? payload.slice(7).trim() : undefined,
      });
    }
  }
  return { events, rest: buffer.slice(cursor) };
}

// Tiny per-job store that React subscribes to via useSyncExternalStore.
// We use a class-free shape to keep the bundle slim.
type Listener = () => void;
function createStore() {
  const states = new Map<string, UploadProgressState>();
  const listeners = new Set<Listener>();
  return {
    get(jobId: string): UploadProgressState {
      return states.get(jobId) ?? { ...empty, jobId };
    },
    snapshot(): Map<string, UploadProgressState> {
      return states;
    },
    apply(jobId: string, evt: UploadProgressEvent) {
      const prev = states.get(jobId) ?? { ...empty, jobId };
      states.set(jobId, reduce(prev, evt));
      listeners.forEach((l) => l());
    },
    reset(jobId: string) {
      states.delete(jobId);
      listeners.forEach((l) => l());
    },
    subscribe(l: Listener) {
      listeners.add(l);
      return () => {
        listeners.delete(l);
      };
    },
  };
}

const sharedStore = createStore();

// useUploadProgress connects to GET /api/upload/progress for jobId.
// Returns the latest reduced state; `null` when jobId is empty/undefined.
// The connection auto-closes once the stream emits done:true.
export function useUploadProgress(jobId: string | null | undefined): UploadProgressState | null {
  const activeJobRef = useRef<string | null>(null);
  const [, setTick] = useState(0);

  const state = useSyncExternalStore(
    sharedStore.subscribe,
    () => (jobId ? sharedStore.get(jobId) : null),
    () => (jobId ? sharedStore.get(jobId) : null),
  );

  useEffect(() => {
    if (!jobId) {
      activeJobRef.current = null;
      return;
    }
    if (activeJobRef.current === jobId) return;
    activeJobRef.current = jobId;

    // EventSource for native SSE. credentials: include is implicit;
    // EventSource always sends cookies on same-origin.
    const url = `/api/upload/progress?job_id=${encodeURIComponent(jobId)}`;
    let closed = false;
    let buffer = "";

    // Prefer fetch + ReadableStream so we can handle the structured JSON
    // frame shape uniformly (EventSource auto-strips the `data: ` prefix
    // but we want the exact byte stream the handler emits to keep the
    // parser shared with tests).
    const controller = new AbortController();
    fetch(url, {
      method: "GET",
      credentials: "include",
      headers: { Accept: "text/event-stream" },
      signal: controller.signal,
    })
      .then(async (res) => {
        if (!res.ok || !res.body) {
          sharedStore.apply(jobId, {
            job_id: jobId,
            phase: "error",
            message: `progress stream failed: HTTP ${res.status}`,
            done: true,
            error: `HTTP ${res.status}`,
          });
          return;
        }
        const reader = res.body.getReader();
        const decoder = new TextDecoder();
        while (!closed) {
          const { value, done } = await reader.read();
          if (done) break;
          buffer += decoder.decode(value, { stream: true });
          const { events, rest } = parseSseChunk(buffer);
          buffer = rest;
          for (const evt of events) {
            sharedStore.apply(jobId, evt);
            if (evt.done) {
              closed = true;
              controller.abort();
              break;
            }
          }
        }
      })
      .catch((err) => {
        if (!closed && err?.name !== "AbortError") {
          sharedStore.apply(jobId, {
            job_id: jobId,
            phase: "error",
            message: String(err),
            done: true,
            error: String(err),
          });
        }
      });

    setTick((t) => t + 1);
    return () => {
      closed = true;
      controller.abort();
    };
  }, [jobId]);

  return state;
}

// resetUploadProgress drops the stored state for a job. Useful when the
// upload modal closes and the caller wants the next upload to start
// from a clean slate.
export function resetUploadProgress(jobId: string): void {
  sharedStore.reset(jobId);
}
