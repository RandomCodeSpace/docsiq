package vectorindex

import (
	"container/heap"
	"math"
	"math/rand"
	"sync"
	"time"
)

// HNSW is a small, pure-Go Hierarchical Navigable Small World index.
//
// It replaces the initial coder/hnsw wrapper, which shipped with broken
// top-k eviction (Max() on a min-heap returned data[last], evicting a near-
// arbitrary candidate instead of the worst). See:
// https://github.com/coder/hnsw/blob/v0.6.1/heap/heap.go#L85
//
// This implementation follows Malkov & Yashunin (arXiv:1603.09320) with the
// conventional simplifications used by many OSS ports (FAISS, hnswlib):
//
//   - Layer-0 is the full graph; higher layers are probabilistically
//     sampled copies with exponential decay (Ml = 1/ln(M)).
//   - Candidate/dynamic list uses a two-heap strategy: a max-heap of the
//     top-k found so far, and a min-heap of candidates to expand next.
//   - Cosine similarity is the default; vectors are not normalized in
//     place (caller should normalize if they care about strict cosine).
//
// Safe for concurrent use: all public methods take the RWMutex.
type HNSW struct {
	mu sync.RWMutex

	m        int // max neighbors per node per layer (layer > 0)
	m0       int // max neighbors per node on layer 0 (typically 2*m)
	efSearch int
	efConstr int
	ml       float64
	rng      *rand.Rand

	dims  int
	nodes map[string]*hnswNode
	entry *hnswNode
}

type hnswNode struct {
	id        string
	vec       []float32
	level     int
	neighbors [][]*hnswNode // per-layer adjacency (level+1 slices)
}

// NewDefaultHNSW is the conventional M=16, efSearch=50, efConstruction=200
// configuration.
func NewDefaultHNSW() *HNSW { return NewHNSW(16, 200, 50) }

// NewHNSW constructs an empty index with explicit parameters.
// m is the max neighbors per node (>=4, default 16 when <=0).
// efConstr is the ef parameter used during Add (default 200).
// efSearch is the ef parameter used at query time (default 50).
func NewHNSW(m, efConstr, efSearch int) *HNSW {
	if m <= 0 {
		m = 16
	}
	if efConstr <= 0 {
		efConstr = 200
	}
	if efSearch <= 0 {
		efSearch = 50
	}
	return &HNSW{
		m:        m,
		m0:       m * 2,
		efSearch: efSearch,
		efConstr: efConstr,
		ml:       1.0 / math.Log(float64(m)),
		rng:      rand.New(rand.NewSource(time.Now().UnixNano())),
		nodes:    map[string]*hnswNode{},
	}
}

// ─── distance ───────────────────────────────────────────────────────────────

// cosineDistance returns 1 - cosineSimilarity for non-zero vectors. Equal
// vectors return 0; orthogonal return 1; opposite return 2.
func cosineDistance(a, b []float32) float32 {
	var dot, na, nb float64
	for i := range a {
		da := float64(a[i])
		db := float64(b[i])
		dot += da * db
		na += da * da
		nb += db * db
	}
	if na == 0 || nb == 0 {
		return 1
	}
	return float32(1 - dot/(math.Sqrt(na)*math.Sqrt(nb)))
}

// ─── public api ─────────────────────────────────────────────────────────────

// Add inserts (or replaces) a vector under id.
func (h *HNSW) Add(id string, vec []float32) error {
	if len(vec) == 0 {
		return ErrEmptyQuery
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.dims == 0 {
		h.dims = len(vec)
	} else if len(vec) != h.dims {
		return ErrDimMismatch
	}

	if existing, ok := h.nodes[id]; ok {
		h.removeNode(existing)
	}

	level := h.randomLevel()
	n := &hnswNode{
		id:        id,
		vec:       append([]float32(nil), vec...),
		level:     level,
		neighbors: make([][]*hnswNode, level+1),
	}
	h.nodes[id] = n

	// First node becomes the entry point.
	if h.entry == nil {
		h.entry = n
		return nil
	}

	// Phase 1: greedy descent from entry to level+1 to find a start.
	ep := h.entry
	maxL := h.entry.level
	for l := maxL; l > level; l-- {
		ep = h.greedyStep(ep, n.vec, l)
	}

	// Phase 2: at each level from min(level,maxL) down to 0, search ef
	// candidates, select top m neighbors, link bi-directionally, then
	// prune each neighbor's link list.
	for l := min(level, maxL); l >= 0; l-- {
		candidates := h.searchLayer(ep, n.vec, h.efConstr, l)
		neighbors := h.selectNeighbors(candidates, h.maxConn(l))
		n.neighbors[l] = append(n.neighbors[l], neighbors...)
		for _, nb := range neighbors {
			nb.neighbors[l] = append(nb.neighbors[l], n)
			h.pruneNeighbors(nb, l)
		}
		if len(candidates) > 0 {
			ep = candidates[0].node
		}
	}

	if level > maxL {
		h.entry = n
	}
	return nil
}

// Remove drops id from the index. Missing ids are a no-op.
func (h *HNSW) Remove(id string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	n, ok := h.nodes[id]
	if !ok {
		return nil
	}
	h.removeNode(n)
	return nil
}

// Search returns the top-k nearest neighbors to query (by cosine similarity).
func (h *HNSW) Search(query []float32, k int) ([]Hit, error) {
	if len(query) == 0 {
		return nil, ErrEmptyQuery
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	if len(h.nodes) == 0 {
		return nil, nil
	}
	if h.dims != 0 && len(query) != h.dims {
		return nil, ErrDimMismatch
	}
	if k <= 0 {
		return nil, nil
	}

	ep := h.entry
	for l := h.entry.level; l >= 1; l-- {
		ep = h.greedyStep(ep, query, l)
	}
	ef := h.efSearch
	if k > ef {
		ef = k
	}
	cands := h.searchLayer(ep, query, ef, 0)
	if len(cands) > k {
		cands = cands[:k]
	}
	hits := make([]Hit, 0, len(cands))
	for _, c := range cands {
		hits = append(hits, Hit{ID: c.node.id, Score: 1 - c.dist})
	}
	return hits, nil
}

// Size returns the number of indexed vectors.
func (h *HNSW) Size() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.nodes)
}

// Dims returns the dimensionality of indexed vectors (0 before first Add).
func (h *HNSW) Dims() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.dims
}

// ─── internals ──────────────────────────────────────────────────────────────

func (h *HNSW) maxConn(layer int) int {
	if layer == 0 {
		return h.m0
	}
	return h.m
}

func (h *HNSW) randomLevel() int {
	r := h.rng.Float64()
	if r <= 0 {
		r = 1e-9
	}
	return int(-math.Log(r) * h.ml)
}

// greedyStep follows edges at the given layer toward query until no
// neighbor gets closer. Returns the local minimum on that layer.
func (h *HNSW) greedyStep(ep *hnswNode, query []float32, layer int) *hnswNode {
	current := ep
	currentDist := cosineDistance(current.vec, query)
	for {
		improved := false
		if layer >= len(current.neighbors) {
			return current
		}
		for _, nb := range current.neighbors[layer] {
			d := cosineDistance(nb.vec, query)
			if d < currentDist {
				current = nb
				currentDist = d
				improved = true
			}
		}
		if !improved {
			return current
		}
	}
}

// searchLayer is the classical HNSW search that maintains a dynamic "ef"
// candidate list. Returns up to ef candidates sorted by ascending distance.
func (h *HNSW) searchLayer(ep *hnswNode, query []float32, ef, layer int) []scoredNode {
	visited := map[string]struct{}{ep.id: {}}
	epDist := cosineDistance(ep.vec, query)

	// Candidate min-heap (closest to query first).
	cand := &minHeap{{node: ep, dist: epDist}}
	// Result max-heap (furthest-kept first, so we know what to evict).
	results := &maxHeap{{node: ep, dist: epDist}}
	heap.Init(cand)
	heap.Init(results)

	for cand.Len() > 0 {
		c := heap.Pop(cand).(scoredNode)
		// Termination: closest candidate is worse than the worst kept.
		if results.Len() >= ef && c.dist > (*results)[0].dist {
			break
		}
		if layer >= len(c.node.neighbors) {
			continue
		}
		for _, nb := range c.node.neighbors[layer] {
			if _, ok := visited[nb.id]; ok {
				continue
			}
			visited[nb.id] = struct{}{}
			d := cosineDistance(nb.vec, query)
			if results.Len() < ef || d < (*results)[0].dist {
				heap.Push(cand, scoredNode{nb, d})
				heap.Push(results, scoredNode{nb, d})
				if results.Len() > ef {
					heap.Pop(results)
				}
			}
		}
	}

	out := make([]scoredNode, results.Len())
	// Extract in ascending distance (pop all, reverse).
	for i := len(out) - 1; i >= 0; i-- {
		out[i] = heap.Pop(results).(scoredNode)
	}
	return out
}

// selectNeighbors keeps the m closest from an already-sorted list. (We use
// the simple variant — the Malkov "heuristic" version gives marginal gains
// on random data and complicates the code.)
func (h *HNSW) selectNeighbors(cands []scoredNode, m int) []*hnswNode {
	if len(cands) > m {
		cands = cands[:m]
	}
	out := make([]*hnswNode, len(cands))
	for i, c := range cands {
		out[i] = c.node
	}
	return out
}

// pruneNeighbors trims node's layer-l neighbor list back to maxConn by
// keeping the closest ones.
func (h *HNSW) pruneNeighbors(node *hnswNode, layer int) {
	max := h.maxConn(layer)
	if layer >= len(node.neighbors) || len(node.neighbors[layer]) <= max {
		return
	}
	scored := make([]scoredNode, len(node.neighbors[layer]))
	for i, nb := range node.neighbors[layer] {
		scored[i] = scoredNode{nb, cosineDistance(node.vec, nb.vec)}
	}
	// Partial sort via heap.
	h2 := maxHeap(append([]scoredNode(nil), scored...))
	heap.Init(&h2)
	for h2.Len() > max {
		heap.Pop(&h2)
	}
	kept := make([]*hnswNode, h2.Len())
	for i := range kept {
		kept[i] = h2[i].node
	}
	node.neighbors[layer] = kept
}

// removeNode detaches n from its neighbors at every layer and deletes it
// from the node map. If n was the entry point, picks another survivor.
func (h *HNSW) removeNode(n *hnswNode) {
	for l, nbs := range n.neighbors {
		for _, nb := range nbs {
			if l < len(nb.neighbors) {
				nb.neighbors[l] = removeRef(nb.neighbors[l], n)
			}
		}
	}
	delete(h.nodes, n.id)
	if h.entry == n {
		h.entry = nil
		// Promote any remaining node with the highest level.
		for _, other := range h.nodes {
			if h.entry == nil || other.level > h.entry.level {
				h.entry = other
			}
		}
	}
}

func removeRef(slice []*hnswNode, target *hnswNode) []*hnswNode {
	for i, v := range slice {
		if v == target {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}

// ─── heap primitives ────────────────────────────────────────────────────────

type scoredNode struct {
	node *hnswNode
	dist float32
}

// minHeap orders by ascending distance (closest on top).
type minHeap []scoredNode

func (h minHeap) Len() int            { return len(h) }
func (h minHeap) Less(i, j int) bool  { return h[i].dist < h[j].dist }
func (h minHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *minHeap) Push(x interface{}) { *h = append(*h, x.(scoredNode)) }
func (h *minHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

// maxHeap orders by descending distance (furthest on top).
type maxHeap []scoredNode

func (h maxHeap) Len() int            { return len(h) }
func (h maxHeap) Less(i, j int) bool  { return h[i].dist > h[j].dist }
func (h maxHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *maxHeap) Push(x interface{}) { *h = append(*h, x.(scoredNode)) }
func (h *maxHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
