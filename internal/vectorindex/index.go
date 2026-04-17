// Package vectorindex provides an in-memory HNSW vector index for fast
// approximate nearest-neighbor search over chunk/entity embeddings.
//
// It replaces the previous O(n) brute-force scan (store.CosineSimilarity over
// every chunk) with a logarithmic HNSW lookup, while keeping the SQLite store
// as the single source of truth: the index is rebuilt from BLOBs on boot via
// BuildFromStore. Concurrent reads + writes are safe (sync.RWMutex).
package vectorindex

import "errors"

// Hit is a single search result.
type Hit struct {
	ID    string
	Score float32 // cosine similarity in [-1, 1]; 1.0 == identical
}

// Index is the vector-index interface. Implementations must be safe for
// concurrent use by multiple goroutines.
type Index interface {
	// Add inserts (or replaces) a vector under id. vec must match the
	// dimensionality of previously-added vectors; if not, an error is
	// returned (first Add establishes the dimension).
	Add(id string, vec []float32) error
	// Remove drops id from the index. Missing ids are a no-op.
	Remove(id string) error
	// Search returns the top-k most similar vectors to query, ordered by
	// descending cosine similarity.
	Search(query []float32, k int) ([]Hit, error)
	// Size returns the number of vectors currently in the index.
	Size() int
	// Dims returns the vector dimension (0 before first Add).
	Dims() int
}

// ErrDimMismatch is returned when a vector's dimension doesn't match the
// index's established dimension.
var ErrDimMismatch = errors.New("vectorindex: vector dimension mismatch")

// ErrEmptyQuery is returned when Search is called with an empty query vector.
var ErrEmptyQuery = errors.New("vectorindex: empty query vector")
