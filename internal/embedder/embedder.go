package embedder

import (
	"context"
	"fmt"
	"sync"

	"github.com/RandomCodeSpace/docsiq/internal/llm"
)

// Embedder batches text → []float32 vectors using an LLM provider.
type Embedder struct {
	provider    llm.Provider
	batchSize   int
	concurrency int // max concurrent batch requests
}

// New creates a new Embedder. If provider is nil (LLM disabled via
// provider=none), New returns nil. Callers must check for nil before use.
func New(provider llm.Provider, batchSize int) *Embedder {
	if provider == nil {
		return nil
	}
	if batchSize <= 0 {
		batchSize = 20
	}
	return &Embedder{provider: provider, batchSize: batchSize, concurrency: 4}
}

// ModelID returns the embedding model identifier.
func (e *Embedder) ModelID() string { return e.provider.ModelID() }

// EmbedTexts embeds a slice of texts using concurrent batches.
func (e *Embedder) EmbedTexts(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	// Split into batches
	type batchWork struct {
		idx   int
		texts []string
	}
	var batches []batchWork
	for i := 0; i < len(texts); i += e.batchSize {
		end := i + e.batchSize
		if end > len(texts) {
			end = len(texts)
		}
		batches = append(batches, batchWork{idx: i, texts: texts[i:end]})
	}

	// Result slot per batch
	results := make([][]float32, len(texts))
	errs := make([]error, len(batches))

	sem := make(chan struct{}, e.concurrency)
	var wg sync.WaitGroup

	for bi, b := range batches {
		wg.Add(1)
		sem <- struct{}{}
		go func(batchIdx int, work batchWork) {
			defer wg.Done()
			defer func() { <-sem }()

			vecs, err := e.provider.EmbedBatch(ctx, work.texts)
			if err != nil {
				errs[batchIdx] = fmt.Errorf("embed batch [%d:%d]: %w",
					work.idx, work.idx+len(work.texts), err)
				return
			}
			if len(vecs) != len(work.texts) {
				errs[batchIdx] = fmt.Errorf("embed batch [%d:%d]: expected %d vectors, got %d",
					work.idx, work.idx+len(work.texts), len(work.texts), len(vecs))
				return
			}
			for i, vec := range vecs {
				results[work.idx+i] = vec
			}
		}(bi, b)
	}
	wg.Wait()

	// Collect first error
	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}
	return results, nil
}

// EmbedOne embeds a single text.
func (e *Embedder) EmbedOne(ctx context.Context, text string) ([]float32, error) {
	return e.provider.Embed(ctx, text)
}

