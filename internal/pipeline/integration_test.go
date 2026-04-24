//go:build integration && sqlite_fts5

package pipeline_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/RandomCodeSpace/docsiq/internal/config"
	"github.com/RandomCodeSpace/docsiq/internal/embedder"
	"github.com/RandomCodeSpace/docsiq/internal/llm/mock"
	"github.com/RandomCodeSpace/docsiq/internal/pipeline"
	"github.com/RandomCodeSpace/docsiq/internal/search"
	"github.com/RandomCodeSpace/docsiq/internal/store"
)

// TestPipeline_IndexAndSearch_EndToEnd drives pipeline.New().IndexPath()
// followed by Finalize() against a 5-file markdown corpus using a
// deterministic mock LLM provider, then asserts:
//   - SQLite documents, chunks, embeddings row counts are in the
//     expected bands,
//   - entity / relationship counts reflect mock extraction,
//   - a LocalSearch for a known substring returns >=1 hit from the
//     correct document.
//
// Runs under the integration build tag so it stays out of the default
// `go test ./...` path; the CI test-integration job runs it with -race.
func TestPipeline_IndexAndSearch_EndToEnd(t *testing.T) {
	t.Parallel()

	// 1. Temp dir for the SQLite DB (OpenForProject constructs
	// <dataDir>/projects/<slug>/docsiq.db for us).
	dataDir := t.TempDir()
	st, err := store.OpenForProject(dataDir, "itest")
	if err != nil {
		t.Fatalf("store.OpenForProject: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	// 2. Resolve the corpus relative to this test file; filepath.Abs
	// resolves against the test binary's cwd which is the package dir
	// (internal/pipeline), so ../../testdata/pipeline is correct.
	corpus, err := filepath.Abs(filepath.Join("..", "..", "testdata", "pipeline"))
	if err != nil {
		t.Fatalf("resolve corpus: %v", err)
	}

	// 3. Minimal config; values low to keep wall-clock predictable
	// under -race.
	cfg := &config.Config{
		DataDir:        dataDir,
		DefaultProject: "itest",
		LLM:            config.LLMConfig{Provider: "none"}, // unused; we inject the mock directly
		Indexing: config.IndexingConfig{
			BatchSize:     4,
			ChunkSize:     512,
			ChunkOverlap:  64,
			ExtractGraph:  true,
			ExtractClaims: false,
			MaxGleanings:  0,
		},
		Community: config.CommunityConfig{
			MinCommunitySize: 1,
			MaxLevels:        2,
		},
	}

	// 4. Mock provider — no network, deterministic.
	prov := mock.New(mock.DefaultDims)
	pl := pipeline.New(st, prov, cfg)

	// 5. Drive the indexer with a 120s deadline; a real deadlock will
	// blow past this and fail loud.
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	if err := pl.IndexPath(ctx, corpus, pipeline.IndexOptions{
		Workers: 2,
		Verbose: false,
	}); err != nil {
		t.Fatalf("IndexPath: %v", err)
	}

	// 6. Run Finalize (Phases 3-4: community detection + summaries).
	if err := pl.Finalize(ctx, false); err != nil {
		t.Fatalf("Finalize: %v", err)
	}

	// 7. Row-count assertions — bands, not exact values, so chunker
	// re-tuning doesn't break this test.
	docCount := countRows(ctx, t, st, `SELECT count(*) FROM documents WHERE is_latest = 1`)
	if docCount != 5 {
		t.Errorf("document count: want 5, got %d", docCount)
	}

	chunkCount := countRows(ctx, t, st, `SELECT count(*) FROM chunks`)
	if chunkCount < 5 || chunkCount > 50 {
		t.Errorf("chunk count: want 5..50, got %d", chunkCount)
	}

	embCount := countRows(ctx, t, st, `SELECT count(*) FROM embeddings`)
	if embCount != chunkCount {
		t.Errorf("embedding count: want %d (= chunk count), got %d", chunkCount, embCount)
	}

	// Mock returns 2 entities per extraction prompt; extractor runs
	// at least once per chunk; dedup collapses duplicates. Lower
	// bound 2, upper bound 2*chunkCount is a generous band.
	entityCount := countRows(ctx, t, st, `SELECT count(*) FROM entities`)
	if entityCount < 2 || entityCount > 2*chunkCount {
		t.Errorf("entity count: want 2..%d, got %d", 2*chunkCount, entityCount)
	}

	relCount := countRows(ctx, t, st, `SELECT count(*) FROM relationships`)
	if relCount < 1 {
		t.Errorf("relationship count: want >=1, got %d", relCount)
	}

	// 8. Search assertion: "Apollo" appears in alpha.md, beta.md,
	// gamma.md (indirectly via missions), and epsilon.md. LocalSearch
	// must return >=1 chunk whose content references the corpus.
	emb := embedder.New(prov, cfg.Indexing.BatchSize)
	if emb == nil {
		t.Fatal("embedder.New returned nil for non-nil provider")
	}
	result, err := search.LocalSearch(ctx, st, emb, nil, "Apollo program", 5, 0)
	if err != nil {
		t.Fatalf("LocalSearch: %v", err)
	}
	if len(result.Chunks) == 0 {
		t.Fatal("LocalSearch returned 0 chunks; expected >=1")
	}
	var gotApollo bool
	for _, c := range result.Chunks {
		if strings.Contains(strings.ToLower(c.Chunk.Content), "apollo") {
			gotApollo = true
			break
		}
	}
	if !gotApollo {
		t.Errorf("LocalSearch returned %d chunks but none mentioned Apollo", len(result.Chunks))
	}
}

// countRows runs a `SELECT count(*) ...` and fails the test on error.
func countRows(ctx context.Context, t *testing.T, st *store.Store, q string) int {
	t.Helper()
	var n int
	if err := st.DB().QueryRowContext(ctx, q).Scan(&n); err != nil {
		t.Fatalf("countRows %q: %v", q, err)
	}
	return n
}
