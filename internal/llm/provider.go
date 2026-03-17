package llm

import (
	"context"
	"fmt"

	"github.com/RandomCodeSpace/docsgraphcontext/internal/config"
)

// Option configures LLM completion.
type Option func(*callOptions)

type callOptions struct {
	maxTokens   int
	temperature float64
	jsonMode    bool
}

func WithMaxTokens(n int) Option     { return func(o *callOptions) { o.maxTokens = n } }
func WithTemperature(t float64) Option { return func(o *callOptions) { o.temperature = t } }
func WithJSONMode() Option            { return func(o *callOptions) { o.jsonMode = true } }

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
	case "huggingface":
		return newHuggingFaceProvider(cfg)
	default:
		return nil, fmt.Errorf("unknown LLM provider: %s", cfg.Provider)
	}
}
