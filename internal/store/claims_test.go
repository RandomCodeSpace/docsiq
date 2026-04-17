package store

import (
	"context"
	"testing"
)

func newClaimsStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := OpenForProject(dir, "testproj")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func seedClaims(t *testing.T, s *Store) {
	t.Helper()
	ctx := context.Background()
	// Seed a document and an entity so the FK targets exist (FKs are on).
	if err := s.UpsertDocument(ctx, &Document{
		ID: "d1", Path: "/tmp/d1.md", DocType: "md", FileHash: "d1h", IsLatest: true,
	}); err != nil {
		t.Fatalf("UpsertDocument: %v", err)
	}
	if err := s.UpsertEntity(ctx, &Entity{ID: "e1", Name: "Alpha"}); err != nil {
		t.Fatalf("UpsertEntity: %v", err)
	}
	claims := []*Claim{
		{ID: "c1", EntityID: "e1", Claim: "alpha works", Status: "verified", DocID: "d1"},
		{ID: "c2", EntityID: "e1", Claim: "alpha is fast", Status: "pending", DocID: "d1"},
		{ID: "c3", EntityID: "e1", Claim: "alpha costs", Status: "verified", DocID: "d1"},
	}
	if err := s.BatchInsertClaims(ctx, claims); err != nil {
		t.Fatalf("BatchInsertClaims: %v", err)
	}
}

func TestClaimsForEntity(t *testing.T) {
	t.Run("happy_path_returns_all_claims", func(t *testing.T) {
		s := newClaimsStore(t)
		seedClaims(t, s)
		claims, err := s.ClaimsForEntity(context.Background(), "e1")
		if err != nil {
			t.Fatalf("ClaimsForEntity: %v", err)
		}
		if len(claims) != 3 {
			t.Errorf("len = %d, want 3", len(claims))
		}
	})

	t.Run("unknown_entity_returns_empty_not_nil", func(t *testing.T) {
		s := newClaimsStore(t)
		seedClaims(t, s)
		claims, err := s.ClaimsForEntity(context.Background(), "nope")
		if err != nil {
			t.Fatalf("ClaimsForEntity: %v", err)
		}
		if claims == nil {
			t.Error("claims is nil; want non-nil empty slice for JSON round-trip")
		}
		if len(claims) != 0 {
			t.Errorf("len = %d, want 0", len(claims))
		}
	})
}

func TestListClaims(t *testing.T) {
	t.Run("no_filter_returns_all", func(t *testing.T) {
		s := newClaimsStore(t)
		seedClaims(t, s)
		claims, err := s.ListClaims(context.Background(), "", 0)
		if err != nil {
			t.Fatalf("ListClaims: %v", err)
		}
		if len(claims) != 3 {
			t.Errorf("len = %d, want 3", len(claims))
		}
	})

	t.Run("status_filter", func(t *testing.T) {
		s := newClaimsStore(t)
		seedClaims(t, s)
		claims, err := s.ListClaims(context.Background(), "verified", 0)
		if err != nil {
			t.Fatalf("ListClaims: %v", err)
		}
		if len(claims) != 2 {
			t.Errorf("len = %d, want 2", len(claims))
		}
		for _, c := range claims {
			if c.Status != "verified" {
				t.Errorf("status = %q, want verified", c.Status)
			}
		}
	})

	t.Run("limit_bound", func(t *testing.T) {
		s := newClaimsStore(t)
		seedClaims(t, s)
		claims, err := s.ListClaims(context.Background(), "", 1)
		if err != nil {
			t.Fatalf("ListClaims: %v", err)
		}
		if len(claims) != 1 {
			t.Errorf("len = %d, want 1", len(claims))
		}
	})

	t.Run("unknown_status_returns_empty", func(t *testing.T) {
		s := newClaimsStore(t)
		seedClaims(t, s)
		claims, err := s.ListClaims(context.Background(), "zzz", 0)
		if err != nil {
			t.Fatalf("ListClaims: %v", err)
		}
		if len(claims) != 0 {
			t.Errorf("len = %d, want 0", len(claims))
		}
	})
}
