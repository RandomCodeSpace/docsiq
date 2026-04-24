package search

import (
	"context"
	"sort"

	"github.com/RandomCodeSpace/docsiq/internal/embedder"
	"github.com/RandomCodeSpace/docsiq/internal/store"
	"github.com/RandomCodeSpace/docsiq/internal/vectorindex"
)

// ChunkResult is a search result from chunk/vector search.
type ChunkResult struct {
	Chunk     store.Chunk
	Score     float32
	EntityIDs []string
}

// LocalSearchResult combines vector + graph results.
type LocalSearchResult struct {
	Chunks   []ChunkResult
	Entities []*store.Entity
	Rels     []*store.Relationship
}

// LocalSearch performs vector similarity search + graph walk.
//
// If idx is non-nil and non-empty, the top-K chunk retrieval uses the HNSW
// index (O(log n)); otherwise LocalSearch falls back to the historical
// O(n) brute-force scan. This lets tests keep working without wiring an
// index, while production serve.go always passes one.
func LocalSearch(ctx context.Context, st *store.Store, emb *embedder.Embedder, idx vectorindex.Index, query string, topK, graphDepth int) (*LocalSearchResult, error) {
	// Embed query
	qVec, err := emb.EmbedOne(ctx, query)
	if err != nil {
		return nil, err
	}

	result := &LocalSearchResult{}

	if idx != nil && idx.Size() > 0 {
		hits, err := idx.Search(qVec, topK)
		if err != nil {
			return nil, err
		}
		for _, h := range hits {
			c, err := st.GetChunk(ctx, h.ID)
			if err != nil || c == nil {
				continue
			}
			result.Chunks = append(result.Chunks, ChunkResult{Chunk: *c, Score: h.Score})
		}
	} else {
		// Brute-force fallback — original behavior preserved for tests /
		// fresh installs where BuildFromStore hasn't been called yet.
		all, err := st.AllChunkEmbeddings(ctx, emb.ModelID())
		if err != nil {
			return nil, err
		}
		type scored struct {
			cwe   store.ChunkWithEmbedding
			score float32
		}
		scores := make([]scored, len(all))
		for i, cwe := range all {
			scores[i] = scored{cwe, store.CosineSimilarity(qVec, cwe.Vector)}
		}
		sort.Slice(scores, func(i, j int) bool { return scores[i].score > scores[j].score })
		if topK > len(scores) {
			topK = len(scores)
		}
		for _, s := range scores[:topK] {
			result.Chunks = append(result.Chunks, ChunkResult{
				Chunk: s.cwe.Chunk,
				Score: s.score,
			})
		}
	}

	seenEntities := map[string]bool{}

	// Graph walk: find entities related to top chunks via their doc
	if graphDepth > 0 && len(result.Chunks) > 0 {
		// Get entity names from top docs
		docIDs := map[string]bool{}
		for _, c := range result.Chunks {
			docIDs[c.Chunk.DocID] = true
		}

		// Scope entity fetch to the top-hit documents instead of a
		// full-table scan. Entities with no relationships to any top-hit
		// doc are out of local scope by definition.
		docIDList := make([]string, 0, len(docIDs))
		for id := range docIDs {
			docIDList = append(docIDList, id)
		}
		entities, err := st.EntitiesForDocs(ctx, docIDList)
		if err != nil {
			return nil, err
		}

		// Rank entities by vector similarity to query
		type entityScore struct {
			e     *store.Entity
			score float32
		}
		var eScores []entityScore
		for _, e := range entities {
			if e.Vector != nil {
				sc := store.CosineSimilarity(qVec, e.Vector)
				eScores = append(eScores, entityScore{e, sc})
			}
		}
		sort.Slice(eScores, func(i, j int) bool { return eScores[i].score > eScores[j].score })

		limit := topK * 2
		if limit > len(eScores) {
			limit = len(eScores)
		}
		for _, es := range eScores[:limit] {
			if seenEntities[es.e.ID] {
				continue
			}
			seenEntities[es.e.ID] = true
			result.Entities = append(result.Entities, es.e)

			// Walk relationships scoped to the top-hit doc set so the
			// graph expansion cannot leak edges from unrelated
			// documents into a scoped local-search result.
			rels, err := st.RelationshipsForEntityInDocs(ctx, es.e.ID, graphDepth, docIDList)
			if err != nil {
				continue
			}
			result.Rels = append(result.Rels, rels...)

			// Add neighbor entities
			for _, r := range rels {
				for _, nid := range []string{r.SourceID, r.TargetID} {
					if !seenEntities[nid] {
						seenEntities[nid] = true
						ne, err := st.GetEntity(ctx, nid)
						if err == nil && ne != nil {
							result.Entities = append(result.Entities, ne)
						}
					}
				}
			}
		}
	}

	return result, nil
}
