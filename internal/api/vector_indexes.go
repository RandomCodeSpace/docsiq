package api

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

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
	// sf deduplicates concurrent build-from-store calls for the same
	// slug. Without it two first-search goroutines for the same project
	// both read the cache miss, both launch BuildFromStore against the
	// single-writer SQLite store, and block each other for up to 60s
	// each (P0-4).
	sf singleflight.Group
	// build is the function used to build an index from a store.
	// Overridable in tests so we can assert "called exactly once" per
	// slug under concurrent ForProject callers.
	build func(ctx context.Context, st *store.Store) (vectorindex.Index, error)
}

// NewVectorIndexes constructs an empty cache.
func NewVectorIndexes() *VectorIndexes {
	return &VectorIndexes{
		indexes: map[string]vectorindex.Index{},
		build:   vectorindex.BuildFromStore,
	}
}

// ForProject returns the cached index for slug, building one from st
// if no entry exists yet. A build error is logged and nil returned —
// LocalSearch falls back to brute-force, which is slow but correct.
//
// Concurrent first-touch callers for the same slug are coalesced via
// singleflight — only one BuildFromStore runs; others wait on the same
// result.
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

	result, _, _ := v.sf.Do(slug, func() (any, error) {
		// Re-check under the lock in case another goroutine populated
		// the cache between our miss and entering Do.
		v.mu.Lock()
		if idx, ok := v.indexes[slug]; ok {
			v.mu.Unlock()
			return idx, nil
		}
		v.mu.Unlock()

		buildCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		build := v.build
		if build == nil {
			build = vectorindex.BuildFromStore
		}
		idx, err := build(buildCtx, st)
		if err != nil {
			slog.Warn("⚠️ vector index build failed; falling back to brute-force",
				"project", slug, "err", err)
			return nil, err
		}

		v.mu.Lock()
		v.indexes[slug] = idx
		v.mu.Unlock()
		return idx, nil
	})
	if result == nil {
		return nil
	}
	idx, _ := result.(vectorindex.Index)
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
	// Also forget any in-flight single-flight entry so the next
	// ForProject after Invalidate re-runs the build.
	v.sf.Forget(slug)
}
