import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { useProjectStore } from "@/stores/project";
import { apiFetch } from "@/lib/api-client";
import { useEffect, useMemo, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { qk } from "@/hooks/api/keys";
import {
  resetUploadProgress,
  useUploadProgress,
  type UploadProgressEvent,
} from "@/hooks/api/useUploadProgress";

interface UploadResponse {
  job_id: string;
  status: string;
}

interface PerFileState {
  phase: string;
  message: string;
  chunksDone: number;
  chunksTotal: number;
  error: string;
}

const PHASE_LABELS: Record<string, string> = {
  queued: "Queued",
  load: "Loading",
  chunk: "Chunking",
  indexing: "Indexing",
  embed: "Embedding",
  extract_entities: "Extracting entities",
  extract_relationships: "Linking relationships",
  extract_claims: "Extracting claims",
  structure: "Summarising",
  finalize: "Finalising graph",
  done: "Done",
  error: "Failed",
  cancelled: "Cancelled",
};

function reduceFiles(events: UploadProgressEvent[]): Map<string, PerFileState> {
  const byFile = new Map<string, PerFileState>();
  for (const evt of events) {
    const key = evt.file || "(job)";
    const prev = byFile.get(key) ?? {
      phase: "",
      message: "",
      chunksDone: 0,
      chunksTotal: 0,
      error: "",
    };
    byFile.set(key, {
      phase: evt.phase,
      message: evt.message ?? prev.message,
      chunksDone: evt.chunks_done ?? prev.chunksDone,
      chunksTotal: evt.chunks_total ?? prev.chunksTotal,
      error: evt.error ?? prev.error,
    });
  }
  return byFile;
}

export function UploadModal({ open, onOpenChange }: { open: boolean; onOpenChange: (v: boolean) => void }) {
  const project = useProjectStore((s) => s.slug);
  const qc = useQueryClient();
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [jobId, setJobId] = useState<string | null>(null);

  const progress = useUploadProgress(jobId);

  // When the SSE stream finishes (success or failure), refresh the
  // documents list and stop showing "uploading…". The modal stays open
  // so the user can read the final phase before dismissing.
  useEffect(() => {
    if (!progress?.done) return;
    setBusy(false);
    qc.invalidateQueries({ queryKey: qk.docs(project) });
    qc.invalidateQueries({ queryKey: qk.stats(project) });
  }, [progress?.done, qc, project]);

  // Reset state when the modal closes so a re-open starts clean.
  useEffect(() => {
    if (open) return;
    if (jobId) resetUploadProgress(jobId);
    setJobId(null);
    setErr(null);
  }, [open, jobId]);

  async function onFiles(files: FileList | null) {
    if (!files || files.length === 0) return;
    setBusy(true);
    setErr(null);
    setJobId(null);
    try {
      const fd = new FormData();
      for (const f of Array.from(files)) fd.append("files", f, f.name);
      const res = await apiFetch<UploadResponse>(
        `/api/upload?project=${encodeURIComponent(project)}`,
        { method: "POST", body: fd },
      );
      setJobId(res.job_id);
    } catch (e) {
      setErr((e as Error).message);
      setBusy(false);
    }
  }

  const perFile = useMemo(
    () => (progress ? reduceFiles(progress.history) : new Map<string, PerFileState>()),
    [progress],
  );

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Upload documents</DialogTitle>
        </DialogHeader>
        <input
          type="file"
          multiple
          disabled={busy}
          onChange={(e) => onFiles(e.currentTarget.files)}
          className="block w-full text-sm"
          aria-label="Choose files to upload"
        />
        {err && (
          <p className="text-xs text-destructive" role="alert">
            {err}
          </p>
        )}
        {jobId && (
          <ul
            className="mt-3 flex flex-col gap-2 text-xs"
            aria-live="polite"
            aria-busy={busy}
          >
            {[...perFile.entries()].map(([file, state]) => (
              <li
                key={file}
                className="flex flex-col gap-1 rounded border border-border bg-muted/40 px-3 py-2"
              >
                <div className="flex items-center justify-between gap-2">
                  <span className="truncate font-medium text-foreground" title={file}>
                    {file}
                  </span>
                  <span
                    className={
                      state.phase === "error"
                        ? "text-destructive"
                        : state.phase === "done"
                          ? "text-emerald-600 dark:text-emerald-400"
                          : "text-muted-foreground"
                    }
                  >
                    {PHASE_LABELS[state.phase] ?? state.phase}
                  </span>
                </div>
                {state.chunksTotal > 0 && state.phase === "embed" && (
                  <div className="text-muted-foreground">
                    embedding {state.chunksDone}/{state.chunksTotal} chunks
                  </div>
                )}
                {state.message && state.phase !== "embed" && (
                  <div className="text-muted-foreground truncate" title={state.message}>
                    {state.message}
                  </div>
                )}
                {state.error && (
                  <div className="text-destructive break-words" role="alert">
                    {state.error}
                  </div>
                )}
              </li>
            ))}
          </ul>
        )}
        {busy && !jobId && <p className="text-xs text-muted-foreground">Uploading…</p>}
      </DialogContent>
    </Dialog>
  );
}
