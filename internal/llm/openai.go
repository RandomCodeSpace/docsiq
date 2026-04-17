package llm

import (
	"fmt"

	"github.com/RandomCodeSpace/docsiq/internal/config"
	"github.com/tmc/langchaingo/embeddings"
	"github.com/tmc/langchaingo/llms/openai"
)

// Default model IDs — used when config leaves them blank. These live
// alongside the config defaults (which are set via Viper SetDefault) to
// keep the provider robust when callers construct an LLMConfig by hand
// (e.g. in tests) without going through config.Load.
const (
	defaultOpenAIBaseURL    = "https://api.openai.com/v1"
	defaultOpenAIChatModel  = "gpt-4o-mini"
	defaultOpenAIEmbedModel = "text-embedding-3-small"
)

// newOpenAIProvider constructs a Provider backed by the direct OpenAI
// API (i.e. api.openai.com, not Azure OpenAI). The BaseURL override
// exists so users can point at an OpenAI-compatible proxy (LiteLLM,
// vLLM, etc.) without Azure-style api-version negotiation.
//
// A missing API key is the most common config mistake, so it's called
// out with a dedicated error message rather than deferring to
// langchaingo's generic ErrMissingToken.
func newOpenAIProvider(cfg *config.LLMConfig) (Provider, error) {
	oc := &cfg.OpenAI
	if oc.APIKey == "" {
		return nil, fmt.Errorf("openai: API key is empty (set llm.openai.api_key or DOCSIQ_LLM_OPENAI_API_KEY)")
	}

	baseURL := oc.BaseURL
	if baseURL == "" {
		baseURL = defaultOpenAIBaseURL
	}
	chatModel := oc.ChatModel
	if chatModel == "" {
		chatModel = defaultOpenAIChatModel
	}
	embedModel := oc.EmbedModel
	if embedModel == "" {
		embedModel = defaultOpenAIEmbedModel
	}

	chatOpts := []openai.Option{
		openai.WithToken(oc.APIKey),
		openai.WithBaseURL(baseURL),
		openai.WithModel(chatModel),
	}
	if oc.Organization != "" {
		chatOpts = append(chatOpts, openai.WithOrganization(oc.Organization))
	}
	chatLLM, err := openai.New(chatOpts...)
	if err != nil {
		return nil, fmt.Errorf("openai chat LLM: %w", err)
	}

	embedOpts := []openai.Option{
		openai.WithToken(oc.APIKey),
		openai.WithBaseURL(baseURL),
		openai.WithEmbeddingModel(embedModel),
		// WithModel is still required by langchaingo's constructor
		// even for an embedding-only client — it refuses to build
		// without a chat model set. Reuse the same model here.
		openai.WithModel(chatModel),
	}
	if oc.Organization != "" {
		embedOpts = append(embedOpts, openai.WithOrganization(oc.Organization))
	}
	embedLLM, err := openai.New(embedOpts...)
	if err != nil {
		return nil, fmt.Errorf("openai embed LLM: %w", err)
	}
	emb, err := embeddings.NewEmbedder(embedLLM)
	if err != nil {
		return nil, fmt.Errorf("openai embedder: %w", err)
	}
	// ModelID surfaces the embedding model since that's what callers
	// care about for vector-dimension consistency — mirroring the
	// Azure provider's convention.
	return &lcProvider{
		llm:     chatLLM,
		emb:     emb,
		name:    "openai",
		modelID: embedModel,
	}, nil
}
