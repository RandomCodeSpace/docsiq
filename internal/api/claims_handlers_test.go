package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/RandomCodeSpace/docsiq/internal/config"
	"github.com/RandomCodeSpace/docsiq/internal/store"
)

// newClaimsRouter builds a router with a seeded store that has entities
// and claims, so the REST layer can be exercised end-to-end.
func newClaimsRouter(t *testing.T) http.Handler {
	t.Helper()
	dir := t.TempDir()
	st, err := store.OpenForProject(dir, "testproj")
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	ctx := context.Background()
	if err := st.UpsertDocument(ctx, &store.Document{
		ID: "d1", Path: "/tmp/d1.md", DocType: "md", FileHash: "d1h", IsLatest: true,
	}); err != nil {
		t.Fatalf("UpsertDocument: %v", err)
	}
	if err := st.UpsertEntity(ctx, &store.Entity{ID: "e1", Name: "Alpha"}); err != nil {
		t.Fatalf("UpsertEntity: %v", err)
	}
	claims := []*store.Claim{
		{ID: "c1", EntityID: "e1", Claim: "fact-1", Status: "verified", DocID: "d1"},
		{ID: "c2", EntityID: "e1", Claim: "fact-2", Status: "pending", DocID: "d1"},
	}
	if err := st.BatchInsertClaims(ctx, claims); err != nil {
		t.Fatalf("BatchInsertClaims: %v", err)
	}

	cfg := &config.Config{}
	cfg.DataDir = dir
	return NewRouter(nil, nil, cfg, nil,
		WithProjectStores(testSingleStore(dir, st, "_default", "testproj")))
}

func TestClaimsHandlers(t *testing.T) {
	t.Run("claims_for_entity_happy_path", func(t *testing.T) {
		h := newClaimsRouter(t)
		req := httptest.NewRequest(http.MethodGet, "/api/entities/e1/claims", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		var claims []store.Claim
		if err := json.NewDecoder(rec.Body).Decode(&claims); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(claims) != 2 {
			t.Errorf("len = %d, want 2", len(claims))
		}
	})

	t.Run("claims_for_unknown_entity_returns_empty_array", func(t *testing.T) {
		h := newClaimsRouter(t)
		req := httptest.NewRequest(http.MethodGet, "/api/entities/nope/claims", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		body := rec.Body.String()
		if body != "[]\n" {
			t.Errorf("body = %q, want %q", body, "[]\n")
		}
	})

	t.Run("list_claims_status_filter", func(t *testing.T) {
		h := newClaimsRouter(t)
		req := httptest.NewRequest(http.MethodGet, "/api/claims?status=verified", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		var claims []store.Claim
		if err := json.NewDecoder(rec.Body).Decode(&claims); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(claims) != 1 {
			t.Errorf("len = %d, want 1", len(claims))
		}
		if len(claims) > 0 && claims[0].Status != "verified" {
			t.Errorf("status = %q, want verified", claims[0].Status)
		}
	})

	t.Run("list_claims_limit_bound", func(t *testing.T) {
		h := newClaimsRouter(t)
		req := httptest.NewRequest(http.MethodGet, "/api/claims?limit=1", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		var claims []store.Claim
		if err := json.NewDecoder(rec.Body).Decode(&claims); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(claims) != 1 {
			t.Errorf("len = %d, want 1", len(claims))
		}
	})
}
