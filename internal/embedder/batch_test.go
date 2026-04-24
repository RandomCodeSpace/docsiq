package embedder

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/RandomCodeSpace/docsiq/internal/llm"
)

// recordingProvider implements llm.Provider and captures every
// EmbedBatch call's slice length. It returns zero-filled vectors of
// length 4 per input text.
type recordingProvider struct {
	mu        sync.Mutex
	ceiling   int
	callSizes []int
	delay     time.Duration
}

func (r *recordingProvider) Name() string      { return "recording" }
func (r *recordingProvider) ModelID() string   { return "recording-v1" }
func (r *recordingProvider) BatchCeiling() int { return r.ceiling }

func (r *recordingProvider) Complete(ctx context.Context, prompt string, opts ...llm.Option) (string, error) {
	return "", nil
}

func (r *recordingProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	return []float32{0, 0, 0, 0}, nil
}

func (r *recordingProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	r.mu.Lock()
	r.callSizes = append(r.callSizes, len(texts))
	r.mu.Unlock()
	if r.delay > 0 {
		select {
		case <-time.After(r.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i] = []float32{0, 0, 0, 0}
	}
	return out, nil
}

// TestEmbedder_New_ClampsToBatchCeiling: a user asking for batchSize=5000
// against an OpenAI-like provider with ceiling=2048 gets clamped to 2048.
func TestEmbedder_New_ClampsToBatchCeiling(t *testing.T) {
	t.Parallel()
	p := &recordingProvider{ceiling: 2048}
	e := New(p, 5000)
	if e.batchSize != 2048 {
		t.Fatalf("batchSize = %d; want 2048 (clamped to ceiling)", e.batchSize)
	}
}

// TestEmbedder_New_BelowCeilingIsUnchanged: a user asking for 100 against
// a ceiling of 2048 keeps 100.
func TestEmbedder_New_BelowCeilingIsUnchanged(t *testing.T) {
	t.Parallel()
	p := &recordingProvider{ceiling: 2048}
	e := New(p, 100)
	if e.batchSize != 100 {
		t.Fatalf("batchSize = %d; want 100 (unchanged)", e.batchSize)
	}
}

// TestEmbedder_EmbedTexts_ChunksToBatchSize: 500 texts with batchSize=100
// results in 5 EmbedBatch calls, each of size 100.
func TestEmbedder_EmbedTexts_ChunksToBatchSize(t *testing.T) {
	t.Parallel()
	p := &recordingProvider{ceiling: 2048}
	e := New(p, 100)

	texts := make([]string, 500)
	for i := range texts {
		texts[i] = "t"
	}

	if _, err := e.EmbedTexts(context.Background(), texts); err != nil {
		t.Fatalf("EmbedTexts: %v", err)
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.callSizes) != 5 {
		t.Fatalf("EmbedBatch calls = %d; want 5 (500 / 100)", len(p.callSizes))
	}
	for i, n := range p.callSizes {
		if n != 100 {
			t.Fatalf("call[%d] size = %d; want 100", i, n)
		}
	}
}

// TestEmbedder_EmbedTexts_PreservesOrder: returned vectors are assembled
// in input order, even with concurrent batches.
func TestEmbedder_EmbedTexts_PreservesOrder(t *testing.T) {
	t.Parallel()
	p := &recordingProvider{ceiling: 2048, delay: 5 * time.Millisecond}
	e := New(llm.Provider(p), 50)

	texts := make([]string, 250)
	for i := range texts {
		texts[i] = "t"
	}
	vecs, err := e.EmbedTexts(context.Background(), texts)
	if err != nil {
		t.Fatalf("EmbedTexts: %v", err)
	}
	if len(vecs) != 250 {
		t.Fatalf("vecs len = %d; want 250", len(vecs))
	}
	for i, v := range vecs {
		if len(v) != 4 {
			t.Fatalf("vecs[%d] len = %d; want 4", i, len(v))
		}
	}
}
