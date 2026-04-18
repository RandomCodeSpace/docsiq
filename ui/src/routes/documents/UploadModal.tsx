import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { useProjectStore } from "@/stores/project";
import { apiFetch } from "@/lib/api-client";
import { useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { qk } from "@/hooks/api/keys";

export function UploadModal({ open, onOpenChange }: { open: boolean; onOpenChange: (v: boolean) => void }) {
  const project = useProjectStore((s) => s.slug);
  const qc = useQueryClient();
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  async function onFiles(files: FileList | null) {
    if (!files || files.length === 0) return;
    setBusy(true); setErr(null);
    try {
      const fd = new FormData();
      for (const f of Array.from(files)) fd.append("files", f, f.name);
      await apiFetch(`/api/upload?project=${encodeURIComponent(project)}`, { method: "POST", body: fd });
      qc.invalidateQueries({ queryKey: qk.docs(project) });
      qc.invalidateQueries({ queryKey: qk.stats(project) });
      onOpenChange(false);
    } catch (e) {
      setErr((e as Error).message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader><DialogTitle>Upload documents</DialogTitle></DialogHeader>
        <input
          type="file"
          multiple
          onChange={(e) => onFiles(e.currentTarget.files)}
          className="block w-full text-sm"
        />
        {busy && <p className="text-xs text-muted-foreground">Uploading…</p>}
        {err && <p className="text-xs text-destructive">{err}</p>}
      </DialogContent>
    </Dialog>
  );
}
