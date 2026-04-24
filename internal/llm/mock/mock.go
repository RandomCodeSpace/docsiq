// Package mock provides a deterministic llm.Provider implementation for
// tests. It does NOT require any network, API key, or external process.
// Callers import it directly (no build tag) — the package lives under
// internal/ so it cannot leak into the public API surface.
package mock

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"
	"strings"

	"github.com/RandomCodeSpace/docsiq/internal/llm"
)

// DefaultDims is the default embedding dimensionality.
const DefaultDims = 128

// Provider is a deterministic, in-memory llm.Provider useful for unit
// and integration tests. It inspects the prompt for known substrings
// and returns canned, schema-valid JSON; embeddings are derived from a
// SHA-256 of the input so equal text yields equal vectors.
type Provider struct {
	Dims int
}

// Compile-time check that *Provider satisfies llm.Provider.
var _ llm.Provider = (*Provider)(nil)

// New returns a mock provider. Pass 0 for DefaultDims (128).
func New(dims int) *Provider {
	if dims <= 0 {
		dims = DefaultDims
	}
	return &Provider{Dims: dims}
}

func (p *Provider) Name() string    { return "mock" }
func (p *Provider) ModelID() string { return "mock-llm" }

// Complete returns a deterministic response chosen by prompt substring.
// Schema must match what internal/extractor and internal/community
// expect; see entityPrompt in internal/extractor/entities.go and
// communityPrompt in internal/community/summarizer.go.
func (p *Provider) Complete(ctx context.Context, prompt string, _ ...llm.Option) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	lower := strings.ToLower(prompt)

	switch {
	case strings.Contains(lower, "knowledge graph") && strings.Contains(lower, "entities"):
		// Entity + relationship extraction. The pipeline parses this
		// JSON via internal/extractor — schema must match exactly.
		// Stable entity names derived from prompt-hash so different
		// chunks yield different graphs; dedup then collapses across
		// the corpus.
		tag := hashTag(prompt, 2)
		return fmt.Sprintf(`{
  "entities": [
    {"name": "Entity_%s_A", "type": "Concept", "description": "deterministic mock entity A"},
    {"name": "Entity_%s_B", "type": "Concept", "description": "deterministic mock entity B"}
  ],
  "relationships": [
    {"source": "Entity_%s_A", "target": "Entity_%s_B", "predicate": "relates_to", "description": "mock edge", "weight": 1.0}
  ]
}`, tag, tag, tag, tag), nil

	case strings.Contains(lower, "claim"):
		tag := hashTag(prompt, 2)
		return fmt.Sprintf(`{
  "claims": [
    {"subject": "Entity_%s_A", "predicate": "is", "object": "mock claim", "description": "deterministic"}
  ]
}`, tag), nil

	case strings.Contains(lower, "community") || strings.Contains(lower, "summar"):
		// Must match parseCommunityReport which looks for "TITLE:" and "SUMMARY:" prefixes.
		return "TITLE: Mock community\nSUMMARY: A deterministic, test-only paragraph describing the community of entities in scope.", nil

	default:
		// Unknown prompt — return empty JSON so whatever caller gets
		// it can proceed without a parse error.
		return `{}`, nil
	}
}

// Embed returns a Dims-length vector derived from SHA-256(text). Equal
// text yields equal vectors.
func (p *Provider) Embed(ctx context.Context, text string) ([]float32, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return hashEmbedding(text, p.Dims), nil
}

func (p *Provider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		v, err := p.Embed(ctx, t)
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

// hashEmbedding derives a stable dims-length unit vector from SHA-256(text).
// Runs SHA-256 repeatedly with a counter suffix until dims float32s have
// been produced, then L2-normalises. O(dims) time, zero allocations in
// the hot path beyond the output slice.
func hashEmbedding(text string, dims int) []float32 {
	if dims <= 0 {
		dims = DefaultDims
	}
	out := make([]float32, dims)
	seed := []byte(text)
	var i int
	for counter := uint32(0); i < dims; counter++ {
		var ctrBuf [4]byte
		binary.LittleEndian.PutUint32(ctrBuf[:], counter)
		h := sha256.New()
		h.Write(seed)
		h.Write(ctrBuf[:])
		sum := h.Sum(nil)
		// Each sha256 gives 32 bytes → 8 float32s via uint32 LE.
		for j := 0; j < len(sum) && i < dims; j += 4 {
			u := binary.LittleEndian.Uint32(sum[j : j+4])
			// Map uint32 into (-1, 1).
			out[i] = float32(int32(u))/float32(math.MaxInt32) - 0
			i++
		}
	}
	// L2-normalise so cosine similarity stays well defined.
	var norm float64
	for _, v := range out {
		norm += float64(v) * float64(v)
	}
	if norm == 0 {
		out[0] = 1
		return out
	}
	inv := float32(1.0 / math.Sqrt(norm))
	for k := range out {
		out[k] *= inv
	}
	return out
}

// hashTag returns the first n hex chars of SHA-256(s) — used as a
// stable, short identifier in canned entity names.
func hashTag(s string, n int) string {
	sum := sha256.Sum256([]byte(s))
	const hex = "0123456789abcdef"
	out := make([]byte, n*2)
	for i := 0; i < n; i++ {
		out[2*i] = hex[sum[i]>>4]
		out[2*i+1] = hex[sum[i]&0x0f]
	}
	return string(out)
}
