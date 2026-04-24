//go:build scale

package vectorindex

import (
	"fmt"
	"math/rand"
	"testing"
)

// normalizeVec returns v scaled to unit L2 norm (new slice). Lives in
// this scale-tagged file because TestHNSW_Recall10k is its only caller.
func normalizeVec(v []float32) []float32 {
	var s float32
	for _, x := range v {
		s += x * x
	}
	if s == 0 {
		return v
	}
	n := sqrt32(s)
	out := make([]float32, len(v))
	for i := range v {
		out[i] = v[i] / n
	}
	return out
}

// TestHNSW_Recall10k builds a 10k-vector index and verifies recall@10
// stays above 0.95 across 20 query probes. The workload is fully
// sequential — the race detector has nothing to catch here —
// so nightly invokes it WITHOUT -race. Concurrency correctness is
// covered by TestHNSW_ConcurrentAddSearch, which runs on every PR.
//
// Gated behind the `scale` build tag; the nightly workflow runs it via
// `-tags "sqlite_fts5 scale"`.
func TestHNSW_Recall10k(t *testing.T) {
	const (
		n   = 10_000
		dim = 384
		q   = 20 // number of query probes
		k   = 10
	)
	rng := rand.New(rand.NewSource(7))
	// Higher construction/search ef for a strong recall benchmark; the
	// default (16/200/50) hits ~0.85 on random vectors which is noisy.
	idx := NewHNSW(32, 400, 400)
	vecs := make(map[string][]float32, n)
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("v%d", i)
		v := normalizeVec(randomVec(rng, dim))
		vecs[id] = v
		if err := idx.Add(id, v); err != nil {
			t.Fatal(err)
		}
	}

	var totalRecall float64
	for qi := 0; qi < q; qi++ {
		qv := normalizeVec(randomVec(rng, dim))
		gold := bruteForceTopK(qv, vecs, k)
		hits, err := idx.Search(qv, k)
		if err != nil {
			t.Fatal(err)
		}
		goldSet := map[string]bool{}
		for _, id := range gold {
			goldSet[id] = true
		}
		matches := 0
		for _, h := range hits {
			if goldSet[h.ID] {
				matches++
			}
		}
		totalRecall += float64(matches) / float64(k)
	}
	recall := totalRecall / float64(q)
	t.Logf("HNSW recall@10 over %d queries (N=%d, dim=%d) = %.3f", q, n, dim, recall)
	if recall < 0.95 {
		t.Fatalf("recall@10 = %.3f, want >= 0.95", recall)
	}
}
