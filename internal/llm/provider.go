package llm

import (
	"context"
	"fmt"

	"github.com/RandomCodeSpace/docscontext/internal/config"
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
}

// NewProvider constructs the configured provider.
func NewProvider(cfg *config.LLMConfig) (Provider, error) {
	switch cfg.Provider {
	case "azure":
		return newAzureProvider(cfg)
	case "ollama":
		return newOllamaProvider(cfg)
	case "openai":
		return newOpenAIProvider(cfg)
	default:
		return nil, fmt.Errorf("unknown LLM provider: %s (supported: azure, ollama, openai)", cfg.Provider)
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
}

func (p *lcProvider) Name() string    { return p.name }
func (p *lcProvider) ModelID() string { return p.modelID }

func (p *lcProvider) Complete(ctx context.Context, prompt string, opts ...Option) (string, error) {
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
	return p.emb.EmbedQuery(ctx, text)
}

func (p *lcProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	return p.emb.EmbedDocuments(ctx, texts)
}

func newOllamaProvider(cfg *config.LLMConfig) (Provider, error) {
	chatLLM, err := ollama.New(
		ollama.WithServerURL(cfg.Ollama.BaseURL),
		ollama.WithModel(cfg.Ollama.ChatModel),
	)
	if err != nil {
		return nil, fmt.Errorf("ollama chat LLM: %w", err)
	}
	embedLLM, err := ollama.New(
		ollama.WithServerURL(cfg.Ollama.BaseURL),
		ollama.WithModel(cfg.Ollama.EmbedModel),
	)
	if err != nil {
		return nil, fmt.Errorf("ollama embed LLM: %w", err)
	}
	emb, err := embeddings.NewEmbedder(embedLLM)
	if err != nil {
		return nil, fmt.Errorf("ollama embedder: %w", err)
	}
	return &lcProvider{llm: chatLLM, emb: emb, name: "ollama", modelID: cfg.Ollama.EmbedModel}, nil
}

func newAzureProvider(cfg *config.LLMConfig) (Provider, error) {
	az := &cfg.Azure

	chatLLM, err := openai.New(
		openai.WithBaseURL(az.ChatEndpoint()),
		openai.WithToken(az.ChatAPIKey()),
		openai.WithAPIVersion(az.ChatAPIVersion()),
		openai.WithAPIType(openai.APITypeAzure),
		openai.WithModel(az.ChatModel()),
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
	)
	if err != nil {
		return nil, fmt.Errorf("azure openai embed LLM: %w", err)
	}

	emb, err := embeddings.NewEmbedder(embedLLM)
	if err != nil {
		return nil, fmt.Errorf("azure openai embedder: %w", err)
	}
	return &lcProvider{llm: chatLLM, emb: emb, name: "azure", modelID: az.EmbedModel()}, nil
}
