package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/RandomCodeSpace/docsiq/internal/config"
	"github.com/RandomCodeSpace/docsiq/internal/llm"
	"github.com/RandomCodeSpace/docsiq/internal/store"
)

// nopProvider is a zero-behavior LLM stub used to build a Pipeline for
// tests that exercise non-LLM paths (indexing gated on ExtractGraph=false
// and ExtractClaims=false, plus Prune). All methods return a one-dim
// zero vector or an empty string — nothing reaches the network.
type nopProvider struct{}

func (nopProvider) Name() string    { return "nop" }
func (nopProvider) ModelID() string { return "nop-0" }
func (nopProvider) Complete(_ context.Context, _ string, _ ...llm.Option) (string, error) {
	return "", nil
}
func (nopProvider) Embed(_ context.Context, _ string) ([]float32, error) {
	return []float32{0}, nil
}
func (nopProvider) EmbedBatch(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i := range out {
		out[i] = []float32{0}
	}
	return out, nil
}

// buildTestPipeline constructs a Pipeline with extraction disabled so
// indexing a real file doesn't touch the LLM.
func buildTestPipeline(t *testing.T) (*Pipeline, *store.Store, *config.Config, string) {
	t.Helper()
	dir := t.TempDir()
	st, err := store.OpenForProject(dir, "testproj")
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	cfg := &config.Config{DataDir: dir}
	cfg.Indexing.ChunkSize = 256
	cfg.Indexing.ChunkOverlap = 32
	cfg.Indexing.BatchSize = 4
	cfg.Indexing.Workers = 1
	cfg.Indexing.ExtractGraph = false
	cfg.Indexing.ExtractClaims = false
	cfg.Indexing.MaxGleanings = 0

	pl := New(st, nopProvider{}, cfg)
	return pl, st, cfg, dir
}

func TestIncrementalReIndex(t *testing.T) {
	t.Run("unchanged_file_is_skipped", func(t *testing.T) {
		pl, st, _, dir := buildTestPipeline(t)
		f := filepath.Join(dir, "doc.md")
		if err := os.WriteFile(f, []byte("# hello\nworld"), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		ctx := context.Background()
		if err := pl.IndexPath(ctx, f, IndexOptions{}); err != nil {
			t.Fatalf("first IndexPath: %v", err)
		}

		first, err := st.GetDocumentByPath(ctx, f)
		if err != nil {
			t.Fatalf("GetDocumentByPath: %v", err)
		}
		if first == nil {
			t.Fatal("doc not indexed on first pass")
		}

		// Second pass with the same file — hash + mtime match → no new version.
		if err := pl.IndexPath(ctx, f, IndexOptions{}); err != nil {
			t.Fatalf("second IndexPath: %v", err)
		}
		second, err := st.GetDocumentByPath(ctx, f)
		if err != nil {
			t.Fatalf("GetDocumentByPath: %v", err)
		}
		if second.ID != first.ID {
			t.Errorf("doc ID changed on skip: %s → %s", first.ID, second.ID)
		}
		if second.Version != first.Version {
			t.Errorf("version bumped on skip: %d → %d", first.Version, second.Version)
		}
	})

	t.Run("edited_file_is_reindexed", func(t *testing.T) {
		pl, st, _, dir := buildTestPipeline(t)
		f := filepath.Join(dir, "doc.md")
		if err := os.WriteFile(f, []byte("v1 content"), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		ctx := context.Background()
		if err := pl.IndexPath(ctx, f, IndexOptions{}); err != nil {
			t.Fatalf("first IndexPath: %v", err)
		}
		first, _ := st.GetDocumentByPath(ctx, f)

		// Edit — must change both content and mtime. Add a small sleep
		// so mtime advances on filesystems with 1s granularity.
		time.Sleep(1100 * time.Millisecond)
		if err := os.WriteFile(f, []byte("v2 content is longer"), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		if err := pl.IndexPath(ctx, f, IndexOptions{}); err != nil {
			t.Fatalf("second IndexPath: %v", err)
		}
		second, _ := st.GetDocumentByPath(ctx, f)
		if second.ID == first.ID {
			t.Error("doc not re-indexed after edit (same ID)")
		}
		if second.FileHash == first.FileHash {
			t.Error("file_hash unchanged after edit")
		}
	})

	t.Run("force_flag_bypasses_mtime_cache", func(t *testing.T) {
		// Setup: index a file, then modify its content so the cache
		// has divergent mtime vs. the new contents. --force must cause
		// the new contents to be ingested (whereas without --force the
		// mtime mismatch would still trigger re-ingest — but with matching
		// hash short-circuit may skip). Here the hash differs, so --force
		// is proven to take the ingest path rather than the cache path.
		pl, st, _, dir := buildTestPipeline(t)
		f := filepath.Join(dir, "doc.md")
		if err := os.WriteFile(f, []byte("v1"), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		ctx := context.Background()
		if err := pl.IndexPath(ctx, f, IndexOptions{}); err != nil {
			t.Fatalf("first IndexPath: %v", err)
		}
		first, _ := st.GetDocumentByPath(ctx, f)

		// Edit — now hash AND mtime both differ.
		time.Sleep(1100 * time.Millisecond)
		if err := os.WriteFile(f, []byte("v2 forced"), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		if err := pl.IndexPath(ctx, f, IndexOptions{Force: true}); err != nil {
			t.Fatalf("forced IndexPath: %v", err)
		}
		second, _ := st.GetDocumentByPath(ctx, f)
		if second.ID == first.ID {
			t.Error("--force did not re-ingest a modified file")
		}
	})
}

func TestPruneRemovesMissingFiles(t *testing.T) {
	pl, st, _, dir := buildTestPipeline(t)
	f := filepath.Join(dir, "to-delete.md")
	if err := os.WriteFile(f, []byte("doomed"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	ctx := context.Background()
	if err := pl.IndexPath(ctx, f, IndexOptions{}); err != nil {
		t.Fatalf("IndexPath: %v", err)
	}

	// Remove the file on disk, then prune.
	if err := os.Remove(f); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	n, err := pl.Prune(ctx)
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if n != 1 {
		t.Errorf("removed = %d, want 1", n)
	}
	got, _ := st.GetDocumentByPath(ctx, f)
	if got != nil {
		t.Error("pruned document still present")
	}
}
