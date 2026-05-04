import { useState } from "react";
import { toast } from "sonner";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { useDeleteDoc } from "@/hooks/api/useDocs";

interface Props {
  open: boolean;
  onOpenChange: (next: boolean) => void;
  project: string;
  docId: string;
  docLabel: string;
  // Called after a successful delete settles. Use this to navigate
  // away from a now-stale detail route.
  onDeleted?: () => void;
}

// DeleteDocumentDialog renders a destructive confirm flow. The actual
// mutation lives in useDeleteDoc (which invalidates the relevant query
// keys); this component is purely UI + success/error toasting and
// closing the dialog cleanly. We deliberately keep the mutation
// outside the dialog so callers can pre-warm or share state if needed.
export function DeleteDocumentDialog({
  open,
  onOpenChange,
  project,
  docId,
  docLabel,
  onDeleted,
}: Props) {
  const del = useDeleteDoc(project);
  const [busy, setBusy] = useState(false);

  async function onConfirm() {
    setBusy(true);
    try {
      await del.mutateAsync(docId);
      toast.success("Document deleted", {
        description: docLabel,
      });
      onOpenChange(false);
      onDeleted?.();
    } catch (e) {
      const msg = (e as Error).message || "Unknown error";
      toast.error("Delete failed", { description: msg });
    } finally {
      setBusy(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Delete this document?</DialogTitle>
          <DialogDescription>
            This permanently removes <span className="font-medium text-foreground">{docLabel}</span>{" "}
            from this project, including its chunks, embeddings, and
            any entities or claims that came only from it. This cannot
            be undone.
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <DialogClose asChild>
            <Button type="button" variant="outline" disabled={busy}>
              Cancel
            </Button>
          </DialogClose>
          <Button
            type="button"
            variant="destructive"
            onClick={onConfirm}
            disabled={busy}
          >
            {busy ? "Deleting…" : "Delete"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
