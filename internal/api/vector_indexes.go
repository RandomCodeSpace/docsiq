package api

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/RandomCodeSpace/docsiq/internal/store"
	"github.com/RandomCodeSpace/docsiq/internal/vectorindex"
)

// VectorIndexes is a per-project cache of in-memory HNSW indexes. On
// first search against a slug the index is built from the store's
// (chunk, embedding) rows via vectorindex.BuildFromStore; subsequent
// searches reuse the cached index until Invalidate is called (e.g. at
// the end of an upload job).
//
// A nil receiver is safe: ForProject returns nil which LocalSearch
// treats as the brute-force fallback. This lets tests skip index
// construction entirely.
type VectorIndexes struct {
	mu      sync.Mutex
	indexes map[string]vectorindex.Index
}

// NewVectorIndexes constructs an empty cache.
func NewVectorIndexes() *VectorIndexes {
	return &VectorIndexes{indexes: map[string]vectorindex.Index{}}
}

// ForProject returns the cached index for slug, building one from st
// if no entry exists yet. A build error is logged and nil returned —
// LocalSearch falls back to brute-force, which is slow but correct.
func (v *VectorIndexes) ForProject(slug string, st *store.Store) vectorindex.Index {
	if v == nil {
		return nil
	}
	v.mu.Lock()
	if idx, ok := v.indexes[slug]; ok {
		v.mu.Unlock()
		return idx
	}
	v.mu.Unlock()

	buildCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	idx, err := vectorindex.BuildFromStore(buildCtx, st)
	if err != nil {
		slog.Warn("⚠️ vector index build failed; falling back to brute-force",
			"project", slug, "err", err)
		return nil
	}

	v.mu.Lock()
	defer v.mu.Unlock()
	// Re-check in case another goroutine won the race.
	if existing, ok := v.indexes[slug]; ok {
		return existing
	}
	v.indexes[slug] = idx
	return idx
}

// Set pre-populates the cache for slug. Used by cmd/serve to eagerly
// build indexes for every registered project at boot.
func (v *VectorIndexes) Set(slug string, idx vectorindex.Index) {
	if v == nil {
		return
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	v.indexes[slug] = idx
}

// Invalidate evicts slug from the cache so the next ForProject call
// rebuilds from the latest store state. Called by the upload handler
// after a successful index + finalize cycle.
func (v *VectorIndexes) Invalidate(slug string) {
	if v == nil {
		return
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	delete(v.indexes, slug)
}
