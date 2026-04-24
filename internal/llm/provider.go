package llm

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/RandomCodeSpace/docsiq/internal/config"
	"github.com/tmc/langchaingo/embeddings"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/ollama"
	"github.com/tmc/langchaingo/llms/openai"
)

// Option configures LLM completion.
type Option func(*callOptions)

type callOptions struct {
	maxTokens   int
	temperature float64
	jsonMode    bool
}

func WithMaxTokens(n int) Option      { return func(o *callOptions) { o.maxTokens = n } }
func WithTemperature(t float64) Option { return func(o *callOptions) { o.temperature = t } }
func WithJSONMode() Option             { return func(o *callOptions) { o.jsonMode = true } }

func applyOptions(opts []Option) *callOptions {
	o := &callOptions{maxTokens: 2048, temperature: 0.0}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// Provider is the unified LLM interface.
type Provider interface {
	Complete(ctx context.Context, prompt string, opts ...Option) (string, error)
	Embed(ctx context.Context, text string) ([]float32, error)
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
	Name() string
	ModelID() string
	// BatchCeiling returns the maximum number of texts that can be
	// passed to EmbedBatch in a single call. Callers that need to
	// process larger inputs must slice to this ceiling. Zero means
	// "no declared ceiling" (rare — only for providers that don't
	// care). Block 3.4.
	BatchCeiling() int
}

// NewProvider constructs the configured provider.
// When cfg.Provider is "none", NewProvider returns (nil, nil) — the caller
// must treat a nil Provider as "LLM disabled" and guard accordingly.
func NewProvider(cfg *config.LLMConfig) (Provider, error) {
	switch cfg.Provider {
	case "none":
		return nil, nil
	case "azure":
		return newAzureProvider(cfg)
	case "ollama":
		return newOllamaProvider(cfg)
	case "openai":
		return newOpenAIProvider(cfg)
	default:
		return nil, fmt.Errorf("unknown LLM provider: %s (supported: azure, ollama, openai, none)", cfg.Provider)
	}
}

// ProviderForProject resolves the LLM provider for a given project slug,
// honoring cfg.LLMOverrides. An unknown / empty slug falls back to
// cfg.LLM gracefully. Providers are not cached — callers that issue
// many requests per second should memoize at the call site.
func ProviderForProject(cfg *config.Config, slug string) (Provider, error) {
	sub := cfg.LLMConfigForProject(slug)
	return NewProvider(&sub)
}

// lcProvider adapts langchaingo to our Provider interface.
type lcProvider struct {
	llm     llms.Model
	emb     embeddings.Embedder
	name    string
	modelID string

	// Block 3.5: pooled HTTP client shared with the langchaingo
	// sub-clients. Stored here so tests can assert on it and so
	// future work can swap it (e.g. for a tracing transport).
	httpClient *http.Client

	// Block 3.3: optional per-call timeout wrapped around ctx. Zero
	// means "no timeout" (caller's ctx is authoritative); positive
	// values trigger context.WithTimeout in Complete/Embed/EmbedBatch.
	callTimeout time.Duration

	// Block 3.4: provider-declared batch ceiling. EmbedBatch slices
	// input to this size; caller-visible chunking also uses this
	// value so the Embedder can construct correctly-sized jobs.
	batchCeiling int
}

func (p *lcProvider) Name() string       { return p.name }
func (p *lcProvider) ModelID() string    { return p.modelID }
func (p *lcProvider) BatchCeiling() int  { return p.batchCeiling }

// withCallTimeout returns a child ctx bounded by p.callTimeout when
// positive, plus its cancel. Zero/negative callTimeout returns the
// parent ctx unchanged and a no-op cancel — callers always defer
// cancel() without branching. Block 3.3.
func (p *lcProvider) withCallTimeout(parent context.Context) (context.Context, context.CancelFunc) {
	if p.callTimeout <= 0 {
		return parent, func() {}
	}
	return context.WithTimeout(parent, p.callTimeout)
}

func (p *lcProvider) Complete(ctx context.Context, prompt string, opts ...Option) (string, error) {
	ctx, cancel := p.withCallTimeout(ctx)
	defer cancel()
	o := applyOptions(opts)
	callOpts := []llms.CallOption{
		llms.WithMaxTokens(o.maxTokens),
		llms.WithTemperature(o.temperature),
	}
	if o.jsonMode {
		callOpts = append(callOpts, llms.WithJSONMode())
	}
	return llms.GenerateFromSinglePrompt(ctx, p.llm, prompt, callOpts...)
}

func (p *lcProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	ctx, cancel := p.withCallTimeout(ctx)
	defer cancel()
	return p.emb.EmbedQuery(ctx, text)
}

// EmbedBatch embeds texts in provider-sized chunks. Input is sliced to
// at-most p.batchCeiling per upstream request. Per-chunk results are
// pushed through a buffered channel — a slow consumer backpressures
// the producer once the buffer fills.
//
// The function assembles the final [][]float32 in input order. Errors
// from any chunk short-circuit the whole call via ctx cancellation.
//
// When batchCeiling <= 0 we fall back to a single upstream call — no
// chunking, no buffer. That path preserves behaviour for providers
// that have not declared a ceiling. Block 3.4.
func (p *lcProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	ctx, cancel := p.withCallTimeout(ctx)
	defer cancel()

	if len(texts) == 0 {
		return nil, nil
	}

	// No declared ceiling — single pass, preserve old behaviour.
	if p.batchCeiling <= 0 {
		return p.emb.EmbedDocuments(ctx, texts)
	}

	ceiling := p.batchCeiling
	if len(texts) <= ceiling {
		return p.emb.EmbedDocuments(ctx, texts)
	}

	// Chunk boundaries (start, end) pairs — deterministic order.
	type chunk struct {
		start, end int
	}
	var chunks []chunk
	for i := 0; i < len(texts); i += ceiling {
		end := i + ceiling
		if end > len(texts) {
			end = len(texts)
		}
		chunks = append(chunks, chunk{start: i, end: end})
	}

	type chunkResult struct {
		start int
		vecs  [][]float32
		err   error
	}
	// Buffer sized 2 chunks (equivalent to 2*ceiling vector slots). One
	// chunk completed, one en route; a slow consumer backpressures the
	// third. Concurrent multi-chunk dispatch is the Embedder's job.
	results := make(chan chunkResult, 2)

	// Producer: iterate chunks serially. Serial emission is intentional
	// — the buffer provides headroom for a single slow consumer step.
	go func() {
		defer close(results)
		for _, c := range chunks {
			slice := texts[c.start:c.end]
			vecs, err := p.emb.EmbedDocuments(ctx, slice)
			select {
			case results <- chunkResult{start: c.start, vecs: vecs, err: err}:
			case <-ctx.Done():
				return
			}
			if err != nil {
				return
			}
		}
	}()

	out := make([][]float32, len(texts))
	for r := range results {
		if r.err != nil {
			return nil, fmt.Errorf("embed batch [%d:%d]: %w", r.start, r.start+len(r.vecs), r.err)
		}
		for i, v := range r.vecs {
			out[r.start+i] = v
		}
	}

	// Defensive: every slot must be populated. If a chunk errored
	// between buffer push and loop drain we'd have returned above;
	// reaching here means every result arrived.
	return out, ctx.Err()
}

func newOllamaProvider(cfg *config.LLMConfig) (Provider, error) {
	// Block 3.5: one pooled *http.Client shared across chat + embed handles.
	httpClient := newHTTPClient()

	chatLLM, err := ollama.New(
		ollama.WithServerURL(cfg.Ollama.BaseURL),
		ollama.WithModel(cfg.Ollama.ChatModel),
		ollama.WithHTTPClient(httpClient),
	)
	if err != nil {
		return nil, fmt.Errorf("ollama chat LLM: %w", err)
	}
	embedLLM, err := ollama.New(
		ollama.WithServerURL(cfg.Ollama.BaseURL),
		ollama.WithModel(cfg.Ollama.EmbedModel),
		ollama.WithHTTPClient(httpClient),
	)
	if err != nil {
		return nil, fmt.Errorf("ollama embed LLM: %w", err)
	}
	emb, err := embeddings.NewEmbedder(embedLLM)
	if err != nil {
		return nil, fmt.Errorf("ollama embedder: %w", err)
	}
	return &lcProvider{
		llm:          chatLLM,
		emb:          emb,
		name:         "ollama",
		modelID:      cfg.Ollama.EmbedModel,
		httpClient:   httpClient,
		callTimeout:  cfg.CallTimeout,
		batchCeiling: 128,
	}, nil
}

func newAzureProvider(cfg *config.LLMConfig) (Provider, error) {
	az := &cfg.Azure

	// Block 3.5: one pooled *http.Client shared across chat + embed handles.
	httpClient := newHTTPClient()

	chatLLM, err := openai.New(
		openai.WithBaseURL(az.ChatEndpoint()),
		openai.WithToken(az.ChatAPIKey()),
		openai.WithAPIVersion(az.ChatAPIVersion()),
		openai.WithAPIType(openai.APITypeAzure),
		openai.WithModel(az.ChatModel()),
		openai.WithHTTPClient(httpClient),
	)
	if err != nil {
		return nil, fmt.Errorf("azure openai chat LLM: %w", err)
	}

	embedLLM, err := openai.New(
		openai.WithBaseURL(az.EmbedEndpoint()),
		openai.WithToken(az.EmbedAPIKey()),
		openai.WithAPIVersion(az.EmbedAPIVersion()),
		openai.WithAPIType(openai.APITypeAzure),
		openai.WithEmbeddingModel(az.EmbedModel()),
		openai.WithHTTPClient(httpClient),
	)
	if err != nil {
		return nil, fmt.Errorf("azure openai embed LLM: %w", err)
	}

	emb, err := embeddings.NewEmbedder(embedLLM)
	if err != nil {
		return nil, fmt.Errorf("azure openai embedder: %w", err)
	}
	return &lcProvider{
		llm:          chatLLM,
		emb:          emb,
		name:         "azure",
		modelID:      az.EmbedModel(),
		httpClient:   httpClient,
		callTimeout:  cfg.CallTimeout,
		batchCeiling: 16,
	}, nil
}
