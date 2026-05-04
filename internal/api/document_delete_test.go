//go:build sqlite_fts5

package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/RandomCodeSpace/docsiq/internal/config"
	"github.com/RandomCodeSpace/docsiq/internal/store"
)

// seedDeleteFixture creates a router with two documents, two chunks
// each, claims and relationships rooted in each doc, and one entity
// shared across both docs plus one entity unique to doc-A. The shared
// entity must survive deletion of doc-A; the unique entity must not.
func seedDeleteFixture(t *testing.T) (http.Handler, *store.Store) {
	t.Helper()
	dir := t.TempDir()
	st, err := store.OpenForProject(dir, "_default")
	if err != nil {
		t.Fatalf("OpenForProject: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	ctx := context.Background()
	for _, d := range []*store.Document{
		{ID: "docA", Path: "/tmp/a.md", DocType: "md", FileHash: "ah", IsLatest: true},
		{ID: "docB", Path: "/tmp/b.md", DocType: "md", FileHash: "bh", IsLatest: true},
	} {
		if err := st.UpsertDocument(ctx, d); err != nil {
			t.Fatalf("UpsertDocument %s: %v", d.ID, err)
		}
	}

	chunks := []*store.Chunk{
		{ID: "cA1", DocID: "docA", ChunkIndex: 0, Content: "alpha-1"},
		{ID: "cA2", DocID: "docA", ChunkIndex: 1, Content: "alpha-2"},
		{ID: "cB1", DocID: "docB", ChunkIndex: 0, Content: "beta-1"},
	}
	if err := st.BatchInsertChunks(ctx, chunks); err != nil {
		t.Fatalf("BatchInsertChunks: %v", err)
	}
	if err := st.BatchUpsertEmbeddings(ctx, "test-model",
		[]string{"cA1", "cA2", "cB1"},
		[][]float32{{0.1, 0.2}, {0.3, 0.4}, {0.5, 0.6}}); err != nil {
		t.Fatalf("BatchUpsertEmbeddings: %v", err)
	}

	// Entities: shared (referenced by both docs) and unique-to-A.
	for _, e := range []*store.Entity{
		{ID: "eShared", Name: "Shared"},
		{ID: "eOnlyA", Name: "OnlyA"},
		{ID: "eOnlyB", Name: "OnlyB"},
	} {
		if err := st.UpsertEntity(ctx, e); err != nil {
			t.Fatalf("UpsertEntity %s: %v", e.ID, err)
		}
	}

	rels := []*store.Relationship{
		{ID: "rA1", SourceID: "eShared", TargetID: "eOnlyA", Predicate: "mentions", DocID: "docA"},
		{ID: "rA2", SourceID: "eOnlyA", TargetID: "eShared", Predicate: "rel", DocID: "docA"},
		{ID: "rB1", SourceID: "eShared", TargetID: "eOnlyB", Predicate: "mentions", DocID: "docB"},
	}
	if err := st.BatchInsertRelationships(ctx, rels); err != nil {
		t.Fatalf("BatchInsertRelationships: %v", err)
	}

	claims := []*store.Claim{
		{ID: "clA", EntityID: "eOnlyA", Claim: "claim-a", Status: "verified", DocID: "docA"},
		{ID: "clB", EntityID: "eOnlyB", Claim: "claim-b", Status: "verified", DocID: "docB"},
	}
	if err := st.BatchInsertClaims(ctx, claims); err != nil {
		t.Fatalf("BatchInsertClaims: %v", err)
	}

	cfg := &config.Config{}
	cfg.DataDir = dir
	h := NewRouter(nil, nil, cfg, nil,
		WithProjectStores(testSingleStore(dir, st, "_default")))
	return h, st
}

func TestDeleteDocumentHandler(t *testing.T) {
	t.Run("unknown_id_returns_404", func(t *testing.T) {
		h, _ := seedDeleteFixture(t)
		req := httptest.NewRequest(http.MethodDelete, "/api/documents/no-such-id", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404; body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("happy_path_returns_204_and_cleans_graph", func(t *testing.T) {
		h, st := seedDeleteFixture(t)
		req := httptest.NewRequest(http.MethodDelete, "/api/documents/docA", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204; body=%s", rec.Code, rec.Body.String())
		}
		if rec.Body.Len() != 0 {
			t.Errorf("body = %q, want empty for 204", rec.Body.String())
		}

		ctx := context.Background()

		// docA gone, docB preserved.
		if d, err := st.GetDocument(ctx, "docA"); err != nil || d != nil {
			t.Errorf("docA still present (err=%v, doc=%v)", err, d)
		}
		if d, err := st.GetDocument(ctx, "docB"); err != nil || d == nil {
			t.Errorf("docB unexpectedly removed (err=%v, doc=%v)", err, d)
		}

		// Chunks for docA gone, docB chunk preserved.
		aChunks, _ := st.ListChunksByDoc(ctx, "docA")
		if len(aChunks) != 0 {
			t.Errorf("docA chunks remain: %d", len(aChunks))
		}
		bChunks, _ := st.ListChunksByDoc(ctx, "docB")
		if len(bChunks) != 1 {
			t.Errorf("docB chunks count = %d, want 1", len(bChunks))
		}

		// Embeddings for docA chunks gone (cascade via FK).
		var embCount int
		row := st.DB().QueryRowContext(ctx,
			`SELECT COUNT(*) FROM embeddings WHERE chunk_id IN ('cA1','cA2')`)
		if err := row.Scan(&embCount); err != nil {
			t.Fatalf("embeddings count: %v", err)
		}
		if embCount != 0 {
			t.Errorf("docA embeddings remain: %d", embCount)
		}

		// Claims/relationships for docA gone, docB preserved.
		var relCount int
		_ = st.DB().QueryRowContext(ctx,
			`SELECT COUNT(*) FROM relationships WHERE doc_id='docA'`).Scan(&relCount)
		if relCount != 0 {
			t.Errorf("docA relationships remain: %d", relCount)
		}
		var bRelCount int
		_ = st.DB().QueryRowContext(ctx,
			`SELECT COUNT(*) FROM relationships WHERE doc_id='docB'`).Scan(&bRelCount)
		if bRelCount != 1 {
			t.Errorf("docB relationships count = %d, want 1", bRelCount)
		}
		var aClaimCount int
		_ = st.DB().QueryRowContext(ctx,
			`SELECT COUNT(*) FROM claims WHERE doc_id='docA'`).Scan(&aClaimCount)
		if aClaimCount != 0 {
			t.Errorf("docA claims remain: %d", aClaimCount)
		}

		// Orphan entity (eOnlyA) removed; shared (eShared) and
		// docB-only (eOnlyB) preserved.
		if e, err := st.GetEntity(ctx, "eOnlyA"); err != nil || e != nil {
			t.Errorf("orphan entity eOnlyA still present (err=%v, ent=%v)", err, e)
		}
		if e, err := st.GetEntity(ctx, "eShared"); err != nil || e == nil {
			t.Errorf("shared entity eShared was removed (err=%v, ent=%v)", err, e)
		}
		if e, err := st.GetEntity(ctx, "eOnlyB"); err != nil || e == nil {
			t.Errorf("docB-only entity eOnlyB was removed (err=%v, ent=%v)", err, e)
		}
	})

	t.Run("idempotent_after_delete", func(t *testing.T) {
		h, _ := seedDeleteFixture(t)
		req1 := httptest.NewRequest(http.MethodDelete, "/api/documents/docA", nil)
		rec1 := httptest.NewRecorder()
		h.ServeHTTP(rec1, req1)
		if rec1.Code != http.StatusNoContent {
			t.Fatalf("first delete status = %d, want 204", rec1.Code)
		}
		// Second delete of the same id must be 404, not 204.
		req2 := httptest.NewRequest(http.MethodDelete, "/api/documents/docA", nil)
		rec2 := httptest.NewRecorder()
		h.ServeHTTP(rec2, req2)
		if rec2.Code != http.StatusNotFound {
			t.Fatalf("second delete status = %d, want 404", rec2.Code)
		}
	})
}

// TestStoreDeleteDocumentCascade is a thin store-layer fence around
// the cascade transaction so a future schema change that breaks the
// graph cleanup fails here, not just at the HTTP boundary.
func TestStoreDeleteDocumentCascade(t *testing.T) {
	dir := t.TempDir()
	st, err := store.OpenForProject(dir, "tx")
	if err != nil {
		t.Fatalf("OpenForProject: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	ctx := context.Background()
	if err := st.UpsertDocument(ctx, &store.Document{
		ID: "d1", Path: "/p.md", DocType: "md", FileHash: "h1", IsLatest: true,
	}); err != nil {
		t.Fatalf("UpsertDocument: %v", err)
	}
	if err := st.BatchInsertChunks(ctx, []*store.Chunk{
		{ID: "ch1", DocID: "d1", ChunkIndex: 0, Content: "x"},
	}); err != nil {
		t.Fatalf("BatchInsertChunks: %v", err)
	}
	if err := st.UpsertEntity(ctx, &store.Entity{ID: "e1", Name: "E1"}); err != nil {
		t.Fatalf("UpsertEntity: %v", err)
	}
	if err := st.BatchInsertRelationships(ctx, []*store.Relationship{
		{ID: "r1", SourceID: "e1", TargetID: "e1", Predicate: "self", DocID: "d1"},
	}); err != nil {
		t.Fatalf("BatchInsertRelationships: %v", err)
	}
	if err := st.BatchInsertClaims(ctx, []*store.Claim{
		{ID: "cl1", EntityID: "e1", Claim: "c", Status: "v", DocID: "d1"},
	}); err != nil {
		t.Fatalf("BatchInsertClaims: %v", err)
	}

	affected, err := st.DeleteDocument(ctx, "d1")
	if err != nil {
		t.Fatalf("DeleteDocument: %v", err)
	}
	if affected != 1 {
		t.Errorf("affected = %d, want 1", affected)
	}

	// Everything tied to d1 must be gone, including the now-orphan
	// entity e1.
	for _, q := range []struct {
		name string
		sql  string
	}{
		{"chunks", `SELECT COUNT(*) FROM chunks WHERE doc_id='d1'`},
		{"relationships", `SELECT COUNT(*) FROM relationships WHERE doc_id='d1'`},
		{"claims", `SELECT COUNT(*) FROM claims WHERE doc_id='d1'`},
		{"entities", `SELECT COUNT(*) FROM entities WHERE id='e1'`},
		{"documents", `SELECT COUNT(*) FROM documents WHERE id='d1'`},
	} {
		var n int
		if err := st.DB().QueryRowContext(ctx, q.sql).Scan(&n); err != nil {
			t.Fatalf("count %s: %v", q.name, err)
		}
		if n != 0 {
			t.Errorf("%s count = %d, want 0", q.name, n)
		}
	}

	// Idempotent: deleting an unknown id is a 0-affected non-error.
	affected, err = st.DeleteDocument(ctx, "missing")
	if err != nil {
		t.Errorf("delete missing returned err: %v", err)
	}
	if affected != 0 {
		t.Errorf("delete missing affected = %d, want 0", affected)
	}
}
