package llm

import (
	"context"
	"fmt"

	"github.com/RandomCodeSpace/docsgraphcontext/internal/config"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
)

type azureProvider struct {
	chatLLM    *openai.LLM
	embedLLM   *openai.LLM
	chatModel  string
	embedModel string
}

func newAzureProvider(cfg *config.LLMConfig) (Provider, error) {
	chatLLM, err := openai.New(
		openai.WithAPIType(openai.APITypeAzure),
		openai.WithBaseURL(cfg.Azure.Endpoint),
		openai.WithToken(cfg.Azure.APIKey),
		openai.WithAPIVersion(cfg.Azure.APIVersion),
		openai.WithModel(cfg.Azure.ChatModel),
	)
	if err != nil {
		return nil, fmt.Errorf("azure chat llm: %w", err)
	}
	embedLLM, err := openai.New(
		openai.WithAPIType(openai.APITypeAzure),
		openai.WithBaseURL(cfg.Azure.Endpoint),
		openai.WithToken(cfg.Azure.APIKey),
		openai.WithAPIVersion(cfg.Azure.APIVersion),
		openai.WithEmbeddingModel(cfg.Azure.EmbedModel),
	)
	if err != nil {
		return nil, fmt.Errorf("azure embed llm: %w", err)
	}
	return &azureProvider{
		chatLLM:    chatLLM,
		embedLLM:   embedLLM,
		chatModel:  cfg.Azure.ChatModel,
		embedModel: cfg.Azure.EmbedModel,
	}, nil
}

func (p *azureProvider) Name() string    { return "azure" }
func (p *azureProvider) ModelID() string { return p.chatModel }

func (p *azureProvider) Complete(ctx context.Context, prompt string, opts ...Option) (string, error) {
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
		return "", fmt.Errorf("azure complete: %w", err)
	}
	return resp, nil
}

func (p *azureProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	vecs, err := p.embedLLM.CreateEmbedding(ctx, []string{text})
	if err != nil {
		return nil, fmt.Errorf("azure embed: %w", err)
	}
	if len(vecs) == 0 {
		return nil, fmt.Errorf("azure embed: empty response")
	}
	return vecs[0], nil
}

func (p *azureProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	return p.embedLLM.CreateEmbedding(ctx, texts)
}
