//go:build integration

package itest

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sync/atomic"

	"github.com/RandomCodeSpace/docsiq/internal/llm"
)

// embedDim is the dimension of vectors produced by FakeProvider.Embed.
// 384 is a common sentence-embedding dimension (e.g. MiniLM) and matches
// what the rest of the pipeline treats as a reasonable default. Keep it
// in a const so tests that care about shape can reference it.
const embedDim = 384

// FakeProvider is a deterministic in-memory stub that satisfies
// llm.Provider. Same input → same output on every call. CallCount is an
// atomic counter incremented on every Complete/Embed/EmbedBatch call so
// tests can assert the provider was (or wasn't) hit.
type FakeProvider struct {
	// CallCount is the combined invocation count across all methods.
	// Read with CountCalls(); set via atomic.Add internally.
	CallCount atomic.Int64
}

// Compile-time interface check. Breaks the build if llm.Provider changes
// and FakeProvider falls out of sync.
var _ llm.Provider = (*FakeProvider)(nil)

// Name identifies this provider to the rest of the system.
func (p *FakeProvider) Name() string { return "fake" }

// ModelID returns a stable model identifier that callers can log.
func (p *FakeProvider) ModelID() string { return "fake-model-v1" }

// Complete returns a deterministic string derived from the prompt. The
// output embeds the sha256 prefix of the prompt so tests can assert the
// provider actually saw what they sent.
func (p *FakeProvider) Complete(ctx context.Context, prompt string, opts ...llm.Option) (string, error) {
	p.CallCount.Add(1)
	sum := sha256.Sum256([]byte(prompt))
	return fmt.Sprintf("fake-complete:%x", sum[:8]), nil
}

// Embed returns a deterministic []float32 of length embedDim derived from
// sha256(text). Same text always yields the same vector.
func (p *FakeProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	p.CallCount.Add(1)
	return deterministicVector(text), nil
}

// EmbedBatch embeds each text independently. Order-preserving.
func (p *FakeProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	p.CallCount.Add(1)
	out := make([][]float32, len(texts))
	for i, t := range texts {
		out[i] = deterministicVector(t)
	}
	return out, nil
}

// CountCalls returns the number of times any FakeProvider method was
// invoked since construction.
func (p *FakeProvider) CountCalls() int64 { return p.CallCount.Load() }

// deterministicVector derives a length-embedDim []float32 from sha256(text)
// by seeding a PCG-like rolling mix over the digest. The output is bounded
// to roughly [-1, 1] per component so callers that normalize (cosine) do
// not hit NaNs. The mapping is byte-exact deterministic across runs and
// architectures (explicit little-endian uint64 decoding).
func deterministicVector(text string) []float32 {
	sum := sha256.Sum256([]byte(text))
	// Use the first 32 bytes as four uint64 seeds and lcg-roll them to
	// fill embedDim components. 4 seeds × (embedDim/4) iterations per
	// seed is enough for a 384-dim vector (96 iterations per seed).
	vec := make([]float32, embedDim)
	seeds := [4]uint64{
		binary.LittleEndian.Uint64(sum[0:8]),
		binary.LittleEndian.Uint64(sum[8:16]),
		binary.LittleEndian.Uint64(sum[16:24]),
		binary.LittleEndian.Uint64(sum[24:32]),
	}
	for i := 0; i < embedDim; i++ {
		s := &seeds[i%4]
		// Numerical Recipes LCG constants — deterministic, no stdlib
		// rand to avoid any future behavioral drift.
		*s = (*s)*6364136223846793005 + 1442695040888963407
		// Map high 24 bits → [-1, 1].
		hi := uint32((*s) >> 40)
		vec[i] = float32(int32(hi)-(1<<23)) / float32(1<<23)
	}
	return vec
}
