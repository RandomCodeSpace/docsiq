package search

import (
	"context"
	"testing"

	"github.com/RandomCodeSpace/docsiq/internal/embedder"
	"github.com/RandomCodeSpace/docsiq/internal/llm"
	"github.com/RandomCodeSpace/docsiq/internal/store"
	"github.com/RandomCodeSpace/docsiq/internal/vectorindex"
)

// mockProvider is a deterministic stand-in for llm.Provider. Embed returns
// a per-text canonical vector so the tests can assert known top-K results
// without an external API.
type mockProvider struct {
	modelID string
	table   map[string][]float32

	completeCalls int
	embedCalls    int
}

func (m *mockProvider) Name() string      { return "mock" }
func (m *mockProvider) ModelID() string   { return m.modelID }
func (m *mockProvider) BatchCeiling() int { return 0 }
func (m *mockProvider) Complete(ctx context.Context, prompt string, opts ...llm.Option) (string, error) {
	m.completeCalls++
	return "answer: " + prompt[:min(len(prompt), 32)], nil
}
func (m *mockProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	m.embedCalls++
	if v, ok := m.table[text]; ok {
		return v, nil
	}
	// Fallback: zero vector. Tests only query keys they seeded.
	return []float32{0, 0, 0, 0}, nil
}
func (m *mockProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		v, _ := m.Embed(ctx, t)
		out[i] = v
	}
	return out, nil
}

func seedCorpus(t *testing.T) (*store.Store, *embedder.Embedder, *mockProvider) {
	t.Helper()
	dir := t.TempDir()
	st, err := store.OpenForProject(dir, "testproj")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	mp := &mockProvider{
		modelID: "mock-embed",
		table: map[string][]float32{
			"alpha":         {1, 0, 0, 0},
			"beta":          {0, 1, 0, 0},
			"gamma":         {0, 0, 1, 0},
			"delta":         {0, 0, 0, 1},
			"almost alpha":  {0.98, 0.1, 0, 0}, // query — closest to alpha
		},
	}
	emb := embedder.New(mp, 8)

	ctx := context.Background()
	for name := range mp.table {
		if name == "almost alpha" {
			continue // query-only
		}
		if err := st.UpsertDocument(ctx, &store.Document{
			ID: "d-" + name, Path: name, Title: name, DocType: "txt",
			FileHash: "h-" + name, // file_hash has a UNIQUE constraint
			IsLatest: true,
		}); err != nil {
			t.Fatalf("UpsertDocument %s: %v", name, err)
		}
		if err := st.InsertChunk(ctx, &store.Chunk{ID: "c-" + name, DocID: "d-" + name, ChunkIndex: 0, Content: name}); err != nil {
			t.Fatalf("InsertChunk %s: %v", name, err)
		}
		if err := st.UpsertEmbedding(ctx, "c-"+name, "mock-embed", mp.table[name]); err != nil {
			t.Fatalf("UpsertEmbedding %s: %v", name, err)
		}
	}
	return st, emb, mp
}

// TestLocalSearch_BruteForceAndIndexAgree verifies that identical top-1 hits
// come out of both paths — a regression guard for the HNSW wiring.
func TestLocalSearch_BruteForceAndIndexAgree(t *testing.T) {
	st, emb, _ := seedCorpus(t)
	ctx := context.Background()

	// Brute-force path (idx=nil)
	bf, err := LocalSearch(ctx, st, emb, nil, "almost alpha", 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(bf.Chunks) == 0 || bf.Chunks[0].Chunk.ID != "c-alpha" {
		t.Fatalf("brute-force top-1: %+v", bf.Chunks)
	}

	// HNSW path
	idx, err := vectorindex.BuildFromStore(ctx, st)
	if err != nil {
		t.Fatal(err)
	}
	hn, err := LocalSearch(ctx, st, emb, idx, "almost alpha", 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(hn.Chunks) == 0 || hn.Chunks[0].Chunk.ID != "c-alpha" {
		t.Fatalf("hnsw top-1: %+v", hn.Chunks)
	}
}

func TestLocalSearch_BruteForceFallback_NilIndex(t *testing.T) {
	st, emb, _ := seedCorpus(t)
	ctx := context.Background()
	res, err := LocalSearch(ctx, st, emb, nil, "alpha", 2, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Chunks) == 0 {
		t.Fatalf("no chunks returned")
	}
}

// TestGlobalSearch_PerProjectProvider verifies that GlobalSearch uses the
// provider it receives (the per-project override resolver is tested
// elsewhere). Here we count Complete() calls on a mock override provider
// to confirm it, and not some other provider, was invoked.
func TestGlobalSearch_UsesInjectedProvider(t *testing.T) {
	st, emb, _ := seedCorpus(t)
	override := &mockProvider{modelID: "mock-embed", table: map[string][]float32{"q": {1, 0, 0, 0}}}
	// Seed a community so GlobalSearch has something to synthesize.
	ctx := context.Background()
	_ = st.UpsertCommunity(ctx, &store.Community{ID: "cm1", Level: 0, Title: "alpha cluster", Summary: "stuff about alpha", Vector: []float32{1, 0, 0, 0}})

	_, err := GlobalSearch(ctx, st, emb, override, "q", 0)
	if err != nil {
		t.Fatal(err)
	}
	if override.completeCalls != 1 {
		t.Fatalf("override.completeCalls=%d, want 1 (the injected provider must be used)", override.completeCalls)
	}
}
