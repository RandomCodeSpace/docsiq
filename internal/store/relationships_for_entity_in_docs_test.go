package store

import (
	"context"
	"testing"
)

// Fixture layout:
//
//   docA: e1 -[rA1]-> e2 -[rA2]-> e3
//   docB: e1 -[rB1]-> e4       (entity e1 is shared across both docs)
//
// A depth-2 BFS from e1 scoped to docA must reach e2 and e3 via edges rA1
// and rA2, and must NOT return rB1 (from the unrelated document).
func TestRelationshipsForEntityInDocs_OnlyReturnsEdgesFromScopedDocs(t *testing.T) {
	t.Parallel()
	st := newTestStore(t)
	ctx := context.Background()

	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}

	must(st.UpsertDocument(ctx, &Document{ID: "docA", Path: "/a", Title: "A", DocType: "txt", FileHash: "hashA"}))
	must(st.UpsertDocument(ctx, &Document{ID: "docB", Path: "/b", Title: "B", DocType: "txt", FileHash: "hashB"}))

	for _, id := range []string{"e1", "e2", "e3", "e4"} {
		must(st.UpsertEntity(ctx, &Entity{ID: id, Name: id}))
	}
	must(st.InsertRelationship(ctx, &Relationship{ID: "rA1", SourceID: "e1", TargetID: "e2", Predicate: "p", DocID: "docA"}))
	must(st.InsertRelationship(ctx, &Relationship{ID: "rA2", SourceID: "e2", TargetID: "e3", Predicate: "p", DocID: "docA"}))
	must(st.InsertRelationship(ctx, &Relationship{ID: "rB1", SourceID: "e1", TargetID: "e4", Predicate: "p", DocID: "docB"}))

	// Sanity: the unscoped walk must surface the out-of-scope edge.
	// (That is precisely the leak RAN-35 is closing.)
	all, err := st.RelationshipsForEntity(ctx, "e1", 2)
	if err != nil {
		t.Fatal(err)
	}
	var sawLeakUnscoped bool
	for _, r := range all {
		if r.ID == "rB1" {
			sawLeakUnscoped = true
			break
		}
	}
	if !sawLeakUnscoped {
		t.Fatalf("fixture sanity: unscoped walk did not include rB1 — test setup is wrong")
	}

	// Scoped walk must exclude rB1.
	got, err := st.RelationshipsForEntityInDocs(ctx, "e1", 2, []string{"docA"})
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]string{}
	for _, r := range got {
		ids[r.ID] = r.DocID
		if r.DocID != "docA" {
			t.Errorf("scoped walk returned edge %s from unrelated doc %q", r.ID, r.DocID)
		}
	}
	for _, want := range []string{"rA1", "rA2"} {
		if _, ok := ids[want]; !ok {
			t.Errorf("scoped walk: missing expected in-scope edge %s", want)
		}
	}
	if _, leaked := ids["rB1"]; leaked {
		t.Errorf("scoped walk leaked out-of-scope edge rB1 from docB")
	}
	if len(got) != 2 {
		t.Errorf("scoped walk: want exactly 2 in-scope edges, got %d (%v)", len(got), ids)
	}
}

func TestRelationshipsForEntityInDocs_EmptyDocsReturnsNil(t *testing.T) {
	t.Parallel()
	st := newTestStore(t)
	ctx := context.Background()

	got, err := st.RelationshipsForEntityInDocs(ctx, "anything", 2, nil)
	if err != nil {
		t.Fatalf("empty docIDs: want (nil, nil); got err=%v", err)
	}
	if len(got) != 0 {
		t.Fatalf("empty docIDs: want 0 relationships, got %d", len(got))
	}
}

// Depth must still bound traversal. With depth=1 we should see only the
// direct edge (rA1) out of e1, not rA2 which is one hop further out.
func TestRelationshipsForEntityInDocs_RespectsDepthLimit(t *testing.T) {
	t.Parallel()
	st := newTestStore(t)
	ctx := context.Background()

	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}

	must(st.UpsertDocument(ctx, &Document{ID: "docA", Path: "/a", Title: "A", DocType: "txt", FileHash: "hashA"}))
	for _, id := range []string{"e1", "e2", "e3"} {
		must(st.UpsertEntity(ctx, &Entity{ID: id, Name: id}))
	}
	must(st.InsertRelationship(ctx, &Relationship{ID: "rA1", SourceID: "e1", TargetID: "e2", Predicate: "p", DocID: "docA"}))
	must(st.InsertRelationship(ctx, &Relationship{ID: "rA2", SourceID: "e2", TargetID: "e3", Predicate: "p", DocID: "docA"}))

	got, err := st.RelationshipsForEntityInDocs(ctx, "e1", 1, []string{"docA"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "rA1" {
		t.Fatalf("depth=1 from e1: want [rA1]; got %v", got)
	}
}
