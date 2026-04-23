package store

import (
	"context"
	"testing"
)

func TestEntitiesForDocs_ScopesByRelationshipDocID(t *testing.T) {
	t.Parallel()
	st := newTestStore(t)
	ctx := context.Background()

	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}

	// Insert documents first (FK enforcement is on).
	must(st.UpsertDocument(ctx, &Document{ID: "docA", Path: "/a", Title: "A", DocType: "txt", FileHash: "hashA"}))
	must(st.UpsertDocument(ctx, &Document{ID: "docB", Path: "/b", Title: "B", DocType: "txt", FileHash: "hashB"}))

	// Three entities; two relationships each scoped to a doc.
	must(st.UpsertEntity(ctx, &Entity{ID: "e1", Name: "Alpha"}))
	must(st.UpsertEntity(ctx, &Entity{ID: "e2", Name: "Beta"}))
	must(st.UpsertEntity(ctx, &Entity{ID: "e3", Name: "Gamma"}))
	must(st.InsertRelationship(ctx, &Relationship{ID: "r1", SourceID: "e1", TargetID: "e2", Predicate: "rel", DocID: "docA"}))
	must(st.InsertRelationship(ctx, &Relationship{ID: "r2", SourceID: "e3", TargetID: "e1", Predicate: "rel", DocID: "docB"}))

	got, err := st.EntitiesForDocs(ctx, []string{"docA"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("docA: want 2 entities (e1, e2); got %d", len(got))
	}

	// Empty input → empty slice, no error.
	empty, err := st.EntitiesForDocs(ctx, nil)
	if err != nil || len(empty) != 0 {
		t.Fatalf("empty input: want (0, nil); got (%d, %v)", len(empty), err)
	}
}

func TestEntitiesForDocs_HandlesLargeIDSets(t *testing.T) {
	t.Parallel()
	st := newTestStore(t)
	ctx := context.Background()

	ids := make([]string, 1500) // > SQLite's 999 default
	for i := range ids {
		ids[i] = "doc-xyz"
	}
	_, err := st.EntitiesForDocs(ctx, ids)
	if err != nil {
		t.Fatalf("chunking at >999 should not error: %v", err)
	}
}
