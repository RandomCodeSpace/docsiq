import { useParams, useNavigate } from "react-router-dom";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useWriteNote, useNote } from "@/hooks/api/useNotes";
import { useProjectStore } from "@/stores/project";
import { useEffect } from "react";

const schema = z.object({
  content: z.string().min(1, "Content cannot be empty"),
  author: z.string().optional(),
  tagsRaw: z.string().optional(),
});
type FormData = z.infer<typeof schema>;

export default function NoteEditor() {
  const { key } = useParams();
  const project = useProjectStore((s) => s.slug);
  const nav = useNavigate();
  const { data: existing } = useNote(project, key);
  const write = useWriteNote(project);

  const { register, handleSubmit, formState, reset } = useForm<FormData>({
    resolver: zodResolver(schema),
    defaultValues: { content: "", author: "", tagsRaw: "" },
  });

  useEffect(() => {
    if (existing) {
      reset({
        content: existing.content,
        author: existing.author ?? "",
        tagsRaw: existing.tags.join(", "),
      });
    }
  }, [existing, reset]);

  const onSubmit = async (data: FormData) => {
    if (!key) return;
    const tags = (data.tagsRaw ?? "").split(",").map((t) => t.trim()).filter(Boolean);
    await write.mutateAsync({ key, content: data.content, author: data.author || undefined, tags });
    nav(`/notes/${encodeURIComponent(key)}`);
  };

  return (
    <form onSubmit={handleSubmit(onSubmit)} className="p-8 max-w-[620px] mx-auto space-y-4">
      <h1 className="text-xl font-semibold">Edit {key}</h1>
      <textarea
        {...register("content")}
        rows={20}
        className="w-full font-mono text-sm p-3 bg-[var(--color-surface-1)] border border-[var(--color-border)] rounded-md"
        aria-label="Note content"
      />
      {formState.errors.content && (
        <p className="text-xs text-[var(--color-semantic-error)]">
          {formState.errors.content.message}
        </p>
      )}
      <input
        {...register("author")}
        placeholder="Author (optional)"
        className="w-full px-3 py-2 bg-[var(--color-surface-1)] border border-[var(--color-border)] rounded-md text-sm"
      />
      <input
        {...register("tagsRaw")}
        placeholder="Tags, comma-separated"
        className="w-full px-3 py-2 bg-[var(--color-surface-1)] border border-[var(--color-border)] rounded-md text-sm"
      />
      <div className="flex gap-2">
        <button
          type="submit"
          disabled={formState.isSubmitting}
          className="px-3 py-1.5 bg-[var(--color-accent)] text-[var(--color-accent-contrast)] rounded-md text-sm"
        >
          Save
        </button>
        <button
          type="button"
          onClick={() => nav(-1)}
          className="px-3 py-1.5 border border-[var(--color-border-strong)] rounded-md text-sm"
        >
          Cancel
        </button>
      </div>
    </form>
  );
}
