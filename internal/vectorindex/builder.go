package vectorindex

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/RandomCodeSpace/docsiq/internal/store"
)

// BuildFromStore scans every (chunk, embedding) row in st and loads it into a
// fresh HNSW index. The embedding model is auto-detected from the first row;
// if multiple models coexist, only the dominant one is indexed (warning
// logged).
//
// Returns an empty index (Size()==0) when there are no embeddings yet — this
// is the common case on a fresh install and not an error.
func BuildFromStore(ctx context.Context, st *store.Store) (Index, error) {
	if st == nil {
		return nil, fmt.Errorf("vectorindex: nil store")
	}
	start := time.Now()

	// Discover which embedding model(s) are in the store. We rebuild the
	// index for the dominant model — callers using multiple models should
	// keep separate Index instances per model.
	models, err := st.DistinctEmbeddingModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("list embedding models: %w", err)
	}
	idx := NewDefaultHNSW()
	if len(models) == 0 {
		slog.Info("🧭 vector index: no embeddings yet, starting empty")
		return idx, nil
	}
	if len(models) > 1 {
		slog.Warn("⚠️ multiple embedding models in store; indexing only the first",
			"chosen", models[0], "all", models)
	}

	cwes, err := st.AllChunkEmbeddings(ctx, models[0])
	if err != nil {
		return nil, fmt.Errorf("load embeddings: %w", err)
	}
	for _, cwe := range cwes {
		if len(cwe.Vector) == 0 {
			continue
		}
		if err := idx.Add(cwe.Chunk.ID, cwe.Vector); err != nil {
			return nil, fmt.Errorf("index %s: %w", cwe.Chunk.ID, err)
		}
	}
	slog.Info("🧭 vector index built",
		"model", models[0],
		"vectors", idx.Size(),
		"dims", idx.Dims(),
		"took", time.Since(start).String())
	return idx, nil
}
