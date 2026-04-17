package vectorindex

import (
	"context"
	"testing"

	"github.com/RandomCodeSpace/docsiq/internal/store"
)

// openTempStore spins up a fresh SQLite store in a temp dir.
func openTempStore(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	st, err := store.OpenForProject(dir, "testproj")
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func seedChunk(t *testing.T, st *store.Store, id, docID string, vec []float32, model string) {
	t.Helper()
	ctx := context.Background()
	if err := st.UpsertDocument(ctx, &store.Document{
		ID: docID, Path: docID, Title: docID, DocType: "txt",
		FileHash: "h-" + docID + "-" + id, // file_hash is UNIQUE
		IsLatest: true,
	}); err != nil {
		t.Fatalf("UpsertDocument: %v", err)
	}
	if err := st.InsertChunk(ctx, &store.Chunk{ID: id, DocID: docID, ChunkIndex: 0, Content: "hello", TokenCount: 1}); err != nil {
		t.Fatalf("InsertChunk: %v", err)
	}
	if err := st.UpsertEmbedding(ctx, id, model, vec); err != nil {
		t.Fatalf("UpsertEmbedding: %v", err)
	}
}

func TestBuildFromStore_Empty(t *testing.T) {
	st := openTempStore(t)
	idx, err := BuildFromStore(context.Background(), st)
	if err != nil {
		t.Fatal(err)
	}
	if idx.Size() != 0 {
		t.Fatalf("Size()=%d, want 0 on empty store", idx.Size())
	}
}

func TestBuildFromStore_RoundTrip(t *testing.T) {
	st := openTempStore(t)
	model := "test-embed"

	seedChunk(t, st, "c1", "doc1", []float32{1, 0, 0, 0}, model)
	seedChunk(t, st, "c2", "doc1", []float32{0, 1, 0, 0}, model)
	seedChunk(t, st, "c3", "doc1", []float32{0, 0, 1, 0}, model)

	idx, err := BuildFromStore(context.Background(), st)
	if err != nil {
		t.Fatal(err)
	}
	if idx.Size() != 3 {
		t.Fatalf("Size()=%d, want 3", idx.Size())
	}

	// Query with the c2 vector — must come back as the top hit.
	hits, err := idx.Search([]float32{0, 1, 0, 0}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].ID != "c2" {
		t.Fatalf("top-1 hit=%+v, want c2", hits)
	}
}

func TestBuildFromStore_NilStore(t *testing.T) {
	if _, err := BuildFromStore(context.Background(), nil); err == nil {
		t.Fatalf("nil store: want error, got nil")
	}
}
