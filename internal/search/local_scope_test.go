package search

import (
	"context"
	"testing"

	"github.com/RandomCodeSpace/docsiq/internal/store"
)

// TestLocalSearch_GraphExpansionScopedToTopHitDocs is the RAN-35 regression
// guard. LocalSearch used to re-expand through every relationship a seed
// entity touched, regardless of doc_id, so a scoped query could surface
// unrelated-doc edges into the result set. After the fix, the graph walk
// must stay inside the top-hit doc set.
//
// Fixture:
//
//	d-alpha: chunk "alpha" + entity "alpha" with one edge alpha -> beta
//	         (doc_id=d-alpha)
//	d-delta: chunk "delta" + entity "alpha" shares a second edge
//	         alpha -> gamma (doc_id=d-delta)
//
// A query for "almost alpha" tops out on chunk "c-alpha" (d-alpha). With
// graphDepth=1 the result must include the d-alpha edge and must NOT
// include the d-delta edge, even though the seed entity "alpha" has a
// relationship row in d-delta.
func TestLocalSearch_GraphExpansionScopedToTopHitDocs(t *testing.T) {
	st, emb, _ := seedCorpus(t)
	ctx := context.Background()

	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}

	// Seed entities with vectors close to query so they rank in the
	// top-K entity set. "alpha" shares an ID so scoped and unscoped
	// edges collide on a single seed.
	entAlpha := &store.Entity{ID: "ent-alpha", Name: "alpha", Vector: []float32{1, 0, 0, 0}}
	entBeta := &store.Entity{ID: "ent-beta", Name: "beta", Vector: []float32{0, 1, 0, 0}}
	entGamma := &store.Entity{ID: "ent-gamma", Name: "gamma", Vector: []float32{0, 0, 1, 0}}
	must(st.UpsertEntity(ctx, entAlpha))
	must(st.UpsertEntity(ctx, entBeta))
	must(st.UpsertEntity(ctx, entGamma))

	// In-scope edge: alpha -> beta in d-alpha (the top-hit doc).
	must(st.InsertRelationship(ctx, &store.Relationship{
		ID: "rel-in-scope", SourceID: "ent-alpha", TargetID: "ent-beta",
		Predicate: "knows", DocID: "d-alpha",
	}))
	// Out-of-scope edge: alpha -> gamma in d-delta (unrelated doc).
	must(st.InsertRelationship(ctx, &store.Relationship{
		ID: "rel-out-of-scope", SourceID: "ent-alpha", TargetID: "ent-gamma",
		Predicate: "knows", DocID: "d-delta",
	}))

	res, err := LocalSearch(ctx, st, emb, nil, "almost alpha", 1, 1)
	if err != nil {
		t.Fatalf("LocalSearch: %v", err)
	}

	// Expect the top chunk to belong to d-alpha.
	if len(res.Chunks) == 0 || res.Chunks[0].Chunk.DocID != "d-alpha" {
		t.Fatalf("top chunk: want doc d-alpha; got %+v", res.Chunks)
	}

	// Every returned relationship must belong to a top-hit doc.
	topHitDocs := map[string]bool{}
	for _, c := range res.Chunks {
		topHitDocs[c.Chunk.DocID] = true
	}
	for _, r := range res.Rels {
		if !topHitDocs[r.DocID] {
			t.Errorf("relationship %s leaked from unrelated doc %q (top-hit docs: %v)",
				r.ID, r.DocID, topHitDocs)
		}
		if r.ID == "rel-out-of-scope" {
			t.Errorf("scoped local search returned out-of-scope edge %s (doc=%s)", r.ID, r.DocID)
		}
	}

	// Sanity: the in-scope edge should actually be there — otherwise
	// the negative assertion above is vacuous.
	var sawInScope bool
	for _, r := range res.Rels {
		if r.ID == "rel-in-scope" {
			sawInScope = true
			break
		}
	}
	if !sawInScope {
		t.Errorf("scoped local search did not return the in-scope edge rel-in-scope; rels=%v", relIDs(res.Rels))
	}
}

// relIDs is a tiny helper so assertion failures above print readable ids
// instead of a slice of pointers.
func relIDs(rs []*store.Relationship) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.ID
	}
	return out
}

