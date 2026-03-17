package search

import (
	"context"
	"sort"

	"github.com/RandomCodeSpace/docsgraphcontext/internal/embedder"
	"github.com/RandomCodeSpace/docsgraphcontext/internal/store"
)

// ChunkResult is a search result from chunk/vector search.
type ChunkResult struct {
	Chunk      store.Chunk
	Score      float32
	EntityIDs  []string
}

// LocalSearchResult combines vector + graph results.
type LocalSearchResult struct {
	Chunks     []ChunkResult
	Entities   []*store.Entity
	Rels       []*store.Relationship
}

// LocalSearch performs vector similarity search + graph walk.
func LocalSearch(ctx context.Context, st *store.Store, emb *embedder.Embedder, query string, topK, graphDepth int) (*LocalSearchResult, error) {
	// Embed query
	qVec, err := emb.EmbedOne(ctx, query)
	if err != nil {
		return nil, err
	}

	// Load all chunk embeddings
	all, err := st.AllChunkEmbeddings(ctx, emb.ModelID())
	if err != nil {
		return nil, err
	}

	// Score and rank
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

	result := &LocalSearchResult{}
	seenEntities := map[string]bool{}

	for _, s := range scores[:topK] {
		result.Chunks = append(result.Chunks, ChunkResult{
			Chunk: s.cwe.Chunk,
			Score: s.score,
		})
	}

	// Graph walk: find entities related to top chunks via their doc
	if graphDepth > 0 && len(result.Chunks) > 0 {
		// Get entity names from top docs
		docIDs := map[string]bool{}
		for _, c := range result.Chunks {
			docIDs[c.Chunk.DocID] = true
		}

		entities, err := st.AllEntities(ctx)
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

			// Walk relationships
			rels, err := st.RelationshipsForEntity(ctx, es.e.ID, graphDepth)
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
