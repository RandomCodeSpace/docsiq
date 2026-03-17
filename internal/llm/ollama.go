package llm

import (
	"context"
	"fmt"

	"github.com/RandomCodeSpace/docsgraphcontext/internal/config"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/ollama"
)

type ollamaProvider struct {
	chatLLM    *ollama.LLM
	embedLLM   *ollama.LLM
	chatModel  string
	embedModel string
}

func newOllamaProvider(cfg *config.LLMConfig) (Provider, error) {
	chatLLM, err := ollama.New(
		ollama.WithServerURL(cfg.Ollama.BaseURL),
		ollama.WithModel(cfg.Ollama.ChatModel),
	)
	if err != nil {
		return nil, fmt.Errorf("ollama chat: %w", err)
	}
	embedLLM, err := ollama.New(
		ollama.WithServerURL(cfg.Ollama.BaseURL),
		ollama.WithModel(cfg.Ollama.EmbedModel),
	)
	if err != nil {
		return nil, fmt.Errorf("ollama embed: %w", err)
	}
	return &ollamaProvider{
		chatLLM:    chatLLM,
		embedLLM:   embedLLM,
		chatModel:  cfg.Ollama.ChatModel,
		embedModel: cfg.Ollama.EmbedModel,
	}, nil
}

func (p *ollamaProvider) Name() string    { return "ollama" }
func (p *ollamaProvider) ModelID() string { return p.chatModel }

func (p *ollamaProvider) Complete(ctx context.Context, prompt string, opts ...Option) (string, error) {
	o := applyOptions(opts)
	callOpts := []llms.CallOption{
		llms.WithMaxTokens(o.maxTokens),
		llms.WithTemperature(o.temperature),
	}
	if o.jsonMode {
		callOpts = append(callOpts, llms.WithJSONMode())
	}
	resp, err := llms.GenerateFromSinglePrompt(ctx, p.chatLLM, prompt, callOpts...)
	if err != nil {
		return "", fmt.Errorf("ollama complete: %w", err)
	}
	return resp, nil
}

func (p *ollamaProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	vecs, err := p.embedLLM.CreateEmbedding(ctx, []string{text})
	if err != nil {
		return nil, fmt.Errorf("ollama embed: %w", err)
	}
	if len(vecs) == 0 {
		return nil, fmt.Errorf("ollama embed: empty response")
	}
	return vecs[0], nil
}

func (p *ollamaProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	return p.embedLLM.CreateEmbedding(ctx, texts)
}
