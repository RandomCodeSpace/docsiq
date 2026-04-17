package vectorindex

import (
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"testing"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

// randomVec returns a length-dim float32 slice with values in [-1,1), seeded
// from rng for deterministic tests.
func randomVec(rng *rand.Rand, dim int) []float32 {
	v := make([]float32, dim)
	for i := range v {
		v[i] = rng.Float32()*2 - 1
	}
	return v
}

// normalizeVec returns v scaled to unit L2 norm (new slice).
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

// cosineSim computes cosine similarity, used by the brute-force oracle.
func cosineSim(a, b []float32) float32 {
	var dot, na, nb float32
	for i := range a {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	if na == 0 || nb == 0 {
		return 0
	}
	// sqrt
	return dot / (sqrt32(na) * sqrt32(nb))
}

func sqrt32(x float32) float32 {
	// Newton — fine for tests.
	if x <= 0 {
		return 0
	}
	z := x
	for i := 0; i < 12; i++ {
		z = (z + x/z) / 2
	}
	return z
}

// bruteForceTopK returns the top-k ids sorted by descending cosine similarity.
func bruteForceTopK(query []float32, corpus map[string][]float32, k int) []string {
	type sc struct {
		id    string
		score float32
	}
	out := make([]sc, 0, len(corpus))
	for id, v := range corpus {
		out = append(out, sc{id, cosineSim(query, v)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].score > out[j].score })
	if k > len(out) {
		k = len(out)
	}
	ids := make([]string, 0, k)
	for _, s := range out[:k] {
		ids = append(ids, s.id)
	}
	return ids
}

// ─── tests ───────────────────────────────────────────────────────────────────

func TestHNSW_EmptyIndex(t *testing.T) {
	idx := NewDefaultHNSW()
	if idx.Size() != 0 {
		t.Fatalf("Size()=%d, want 0", idx.Size())
	}
	hits, err := idx.Search([]float32{1, 0, 0}, 5)
	if err != nil {
		t.Fatalf("Search on empty: %v", err)
	}
	if len(hits) != 0 {
		t.Fatalf("hits on empty index: %d", len(hits))
	}
}

func TestHNSW_AddSizeDims(t *testing.T) {
	idx := NewDefaultHNSW()
	if idx.Dims() != 0 {
		t.Fatalf("Dims()=%d, want 0", idx.Dims())
	}
	if err := idx.Add("a", []float32{1, 0, 0}); err != nil {
		t.Fatal(err)
	}
	if err := idx.Add("b", []float32{0, 1, 0}); err != nil {
		t.Fatal(err)
	}
	if idx.Size() != 2 {
		t.Fatalf("Size()=%d, want 2", idx.Size())
	}
	if idx.Dims() != 3 {
		t.Fatalf("Dims()=%d, want 3", idx.Dims())
	}
}

func TestHNSW_WrongDimension(t *testing.T) {
	idx := NewDefaultHNSW()
	if err := idx.Add("a", []float32{1, 0, 0}); err != nil {
		t.Fatal(err)
	}
	if err := idx.Add("b", []float32{1, 0}); err != ErrDimMismatch {
		t.Fatalf("Add wrong-dim: err=%v, want ErrDimMismatch", err)
	}
	if _, err := idx.Search([]float32{1, 0}, 1); err != ErrDimMismatch {
		t.Fatalf("Search wrong-dim: err=%v, want ErrDimMismatch", err)
	}
}

func TestHNSW_EmptyVectorErrors(t *testing.T) {
	idx := NewDefaultHNSW()
	if err := idx.Add("a", nil); err != ErrEmptyQuery {
		t.Fatalf("Add empty: %v", err)
	}
	if _, err := idx.Search(nil, 1); err != ErrEmptyQuery {
		t.Fatalf("Search empty: %v", err)
	}
}

func TestHNSW_ExactMatch_K1(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	// High EfSearch (equal to N) turns HNSW into an essentially-exact
	// lookup for the k=1-returns-itself invariant.
	idx := NewHNSW(16, 400, 400)
	vecs := map[string][]float32{}
	for i := 0; i < 100; i++ {
		id := fmt.Sprintf("v%d", i)
		v := randomVec(rng, 32)
		vecs[id] = v
		if err := idx.Add(id, v); err != nil {
			t.Fatal(err)
		}
	}
	// Query each inserted vector; top-1 must be itself.
	misses := 0
	for id, v := range vecs {
		hits, err := idx.Search(v, 1)
		if err != nil {
			t.Fatal(err)
		}
		if len(hits) != 1 || hits[0].ID != id {
			misses++
		}
	}
	// Accept up to 2 misses (recall >= 98%) — HNSW is an approximate
	// structure, not deterministic.
	if misses > 2 {
		t.Fatalf("k=1 self-lookup misses=%d/100 (want <=2)", misses)
	}
}

func TestHNSW_Remove(t *testing.T) {
	idx := NewDefaultHNSW()
	_ = idx.Add("a", []float32{1, 0, 0})
	_ = idx.Add("b", []float32{0, 1, 0})
	_ = idx.Add("c", []float32{0, 0, 1})
	if err := idx.Remove("b"); err != nil {
		t.Fatal(err)
	}
	if idx.Size() != 2 {
		t.Fatalf("Size after Remove = %d, want 2", idx.Size())
	}
	hits, err := idx.Search([]float32{0, 1, 0}, 3)
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range hits {
		if h.ID == "b" {
			t.Fatalf("removed id still returned: %+v", hits)
		}
	}
	// Remove-missing is a no-op.
	if err := idx.Remove("nonexistent"); err != nil {
		t.Fatalf("Remove missing: %v", err)
	}
}

func TestHNSW_Upsert(t *testing.T) {
	// Adding the same id twice should replace, not duplicate.
	idx := NewDefaultHNSW()
	_ = idx.Add("x", []float32{1, 0, 0})
	_ = idx.Add("x", []float32{0, 1, 0})
	if idx.Size() != 1 {
		t.Fatalf("Size after upsert = %d, want 1", idx.Size())
	}
}

func TestHNSW_Recall10k(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping 10k benchmark in -short")
	}
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

func TestHNSW_ConcurrentAddSearch(t *testing.T) {
	idx := NewDefaultHNSW()
	// Seed with a couple of vectors so Search has something to return.
	_ = idx.Add("seed", []float32{1, 0, 0, 0})

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// Writer goroutines
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(int64(w)))
			for i := 0; i < 500; i++ {
				select {
				case <-stop:
					return
				default:
				}
				id := fmt.Sprintf("w%d-%d", w, i)
				_ = idx.Add(id, randomVec(rng, 4))
			}
		}(w)
	}

	// Reader goroutines
	for r := 0; r < 4; r++ {
		wg.Add(1)
		go func(r int) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(int64(100 + r)))
			for i := 0; i < 500; i++ {
				select {
				case <-stop:
					return
				default:
				}
				_, _ = idx.Search(randomVec(rng, 4), 3)
			}
		}(r)
	}

	wg.Wait()
	close(stop)
	if idx.Size() < 1 {
		t.Fatalf("unexpected Size()=%d after concurrent Adds", idx.Size())
	}
}

func TestHNSW_SearchKLargerThanSize(t *testing.T) {
	idx := NewDefaultHNSW()
	_ = idx.Add("a", []float32{1, 0, 0})
	_ = idx.Add("b", []float32{0, 1, 0})
	hits, err := idx.Search([]float32{1, 0, 0}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 2 {
		t.Fatalf("len(hits)=%d, want 2", len(hits))
	}
}

func TestHNSW_ScoreIsCosineSimilarity(t *testing.T) {
	idx := NewDefaultHNSW()
	_ = idx.Add("same", []float32{1, 0, 0})
	hits, err := idx.Search([]float32{1, 0, 0}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 {
		t.Fatalf("no hits")
	}
	if hits[0].Score < 0.999 {
		t.Fatalf("identical-vector score = %f, want ~1.0", hits[0].Score)
	}
}

// BenchmarkHNSWSearch/Brute compares HNSW vs brute-force lookup on the same
// 10k-vector corpus. Invoke with `go test -run=^$ -bench=.` inside this
// package to read the latency delta.
func BenchmarkHNSWSearch(b *testing.B) {
	const n, dim, k = 10_000, 384, 10
	rng := rand.New(rand.NewSource(13))
	idx := NewDefaultHNSW()
	vecs := make(map[string][]float32, n)
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("v%d", i)
		v := randomVec(rng, dim)
		vecs[id] = v
		_ = idx.Add(id, v)
	}
	qv := randomVec(rng, dim)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = idx.Search(qv, k)
	}
}

func BenchmarkBruteForce(b *testing.B) {
	const n, dim, k = 10_000, 384, 10
	rng := rand.New(rand.NewSource(13))
	vecs := make(map[string][]float32, n)
	for i := 0; i < n; i++ {
		vecs[fmt.Sprintf("v%d", i)] = randomVec(rng, dim)
	}
	qv := randomVec(rng, dim)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = bruteForceTopK(qv, vecs, k)
	}
}
