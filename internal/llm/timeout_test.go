package llm

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/tmc/langchaingo/embeddings"
	"github.com/tmc/langchaingo/llms"
)

// stubModel implements llms.Model by blocking forever on GenerateContent
// until the context is cancelled. It proves the provider honours ctx
// deadlines rather than swallowing them.
type stubModel struct{}

func (stubModel) Call(ctx context.Context, prompt string, opts ...llms.CallOption) (string, error) {
	return (stubModel{}).generate(ctx)
}

func (stubModel) GenerateContent(ctx context.Context, msgs []llms.MessageContent, opts ...llms.CallOption) (*llms.ContentResponse, error) {
	if _, err := (stubModel{}).generate(ctx); err != nil {
		return nil, err
	}
	return &llms.ContentResponse{}, nil
}

func (stubModel) generate(ctx context.Context) (string, error) {
	<-ctx.Done()
	return "", ctx.Err()
}

// stubEmbedder blocks on EmbedDocuments / EmbedQuery until ctx done.
type stubEmbedder struct{}

func (stubEmbedder) EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func (stubEmbedder) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

var _ embeddings.Embedder = stubEmbedder{}

func TestLcProvider_Complete_HonoursCallTimeout(t *testing.T) {
	t.Parallel()
	p := &lcProvider{
		llm:         stubModel{},
		emb:         stubEmbedder{},
		name:        "stub",
		modelID:     "stub",
		callTimeout: 50 * time.Millisecond,
	}
	start := time.Now()
	_, err := p.Complete(context.Background(), "hello")
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("Complete: want non-nil error on timeout, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Complete error: want context.DeadlineExceeded, got %v", err)
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("Complete returned after %v; callTimeout=50ms — deadline not propagated", elapsed)
	}
}

func TestLcProvider_Embed_HonoursCallTimeout(t *testing.T) {
	t.Parallel()
	p := &lcProvider{
		llm:         stubModel{},
		emb:         stubEmbedder{},
		callTimeout: 50 * time.Millisecond,
	}
	start := time.Now()
	_, err := p.Embed(context.Background(), "hello")
	elapsed := time.Since(start)
	if err == nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Embed error: want DeadlineExceeded, got %v", err)
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("Embed elapsed = %v; want < 500ms", elapsed)
	}
}

func TestLcProvider_ZeroCallTimeout_LeavesParentCtxAuthoritative(t *testing.T) {
	t.Parallel()
	p := &lcProvider{
		llm:         stubModel{},
		emb:         stubEmbedder{},
		callTimeout: 0, // disabled — parent ctx wins
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := p.Complete(ctx, "hello")
	if err == nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Complete error with parent deadline: want DeadlineExceeded, got %v", err)
	}
}

// chunkCountingEmbedder counts how many times EmbedDocuments is called
// and with what sizes. Used to verify provider-level chunking.
type chunkCountingEmbedder struct {
	mu        sync.Mutex
	callSizes []int
}

func (c *chunkCountingEmbedder) EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error) {
	c.mu.Lock()
	c.callSizes = append(c.callSizes, len(texts))
	c.mu.Unlock()
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i] = []float32{float32(len(c.callSizes)), float32(i)}
	}
	return out, nil
}

func (c *chunkCountingEmbedder) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
	return []float32{0}, nil
}

func TestLcProvider_EmbedBatch_ChunksToCeiling(t *testing.T) {
	t.Parallel()
	ce := &chunkCountingEmbedder{}
	p := &lcProvider{
		llm:          stubModel{},
		emb:          ce,
		batchCeiling: 16, // Azure-sized
	}

	texts := make([]string, 50)
	for i := range texts {
		texts[i] = "t"
	}

	vecs, err := p.EmbedBatch(context.Background(), texts)
	if err != nil {
		t.Fatalf("EmbedBatch: %v", err)
	}
	if len(vecs) != 50 {
		t.Fatalf("vecs len = %d; want 50", len(vecs))
	}

	ce.mu.Lock()
	defer ce.mu.Unlock()
	// 50 / 16 = 3 full chunks of 16 + 1 tail of 2 → 4 calls.
	if len(ce.callSizes) != 4 {
		t.Fatalf("chunk calls = %d; want 4", len(ce.callSizes))
	}
	if ce.callSizes[0] != 16 || ce.callSizes[1] != 16 || ce.callSizes[2] != 16 || ce.callSizes[3] != 2 {
		t.Fatalf("chunk sizes = %v; want [16 16 16 2]", ce.callSizes)
	}
}
