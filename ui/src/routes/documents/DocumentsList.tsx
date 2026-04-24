import { Link } from "react-router-dom";
import { useState } from "react";
import { useDocs } from "@/hooks/api/useDocs";
import { useProjectStore } from "@/stores/project";
import { formatRelativeTime } from "@/lib/format";
import { Button } from "@/components/ui/button";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { UploadModal } from "./UploadModal";
import { EmptyState, ErrorState, LoadingSkeleton } from "@/components/empty";

export default function DocumentsList() {
  const project = useProjectStore((s) => s.slug);
  const { data, isLoading, error, refetch } = useDocs(project);
  const docs = data ?? [];
  const err = error as Error | null | undefined;
  const [uploadOpen, setUploadOpen] = useState(false);

  return (
    <div className="docs-page">
      <div className="docs-page-head">
        <h1 className="docs-page-title">Documents</h1>
        <Button onClick={() => setUploadOpen(true)}>Upload</Button>
      </div>
      <UploadModal open={uploadOpen} onOpenChange={setUploadOpen} />
      {isLoading ? (
        <LoadingSkeleton label="Loading documents" rows={6} />
      ) : err ? (
        <ErrorState
          title="Documents failed to load"
          message={err.message || "Unknown error"}
          onRetry={() => refetch()}
        />
      ) : docs.length === 0 ? (
        <EmptyState
          title="No documents yet"
          description="Upload a PDF, DOCX, or web page to get started."
        />
      ) : (
        <div className="table-scroll">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Title</TableHead>
                <TableHead>Type</TableHead>
                <TableHead>Updated</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {docs.map((d) => (
                <TableRow key={d.id}>
                  <TableCell>
                    <Link to={`/docs/${d.id}`} className="text-foreground underline decoration-dotted">
                      {d.title || d.path}
                    </Link>
                  </TableCell>
                  <TableCell className="text-muted-foreground">{d.doc_type}</TableCell>
                  <TableCell className="text-muted-foreground">
                    {formatRelativeTime(d.updated_at * 1000)}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
    </div>
  );
}
