package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/RandomCodeSpace/docsgraphcontext/internal/config"
)

// huggingFaceProvider calls a local TGI (Text Generation Inference) endpoint.
type huggingFaceProvider struct {
	baseURL    string
	apiKey     string
	chatModel  string
	embedModel string
	client     *http.Client
}

func newHuggingFaceProvider(cfg *config.LLMConfig) (Provider, error) {
	return &huggingFaceProvider{
		baseURL:    cfg.HuggingFace.BaseURL,
		apiKey:     cfg.HuggingFace.APIKey,
		chatModel:  cfg.HuggingFace.ChatModel,
		embedModel: cfg.HuggingFace.EmbedModel,
		client:     &http.Client{},
	}, nil
}

func (p *huggingFaceProvider) Name() string    { return "huggingface" }
func (p *huggingFaceProvider) ModelID() string { return p.chatModel }

func (p *huggingFaceProvider) Complete(ctx context.Context, prompt string, opts ...Option) (string, error) {
	o := applyOptions(opts)
	payload := map[string]any{
		"inputs": prompt,
		"parameters": map[string]any{
			"max_new_tokens": o.maxTokens,
			"temperature":    o.temperature,
			"return_full_text": false,
		},
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/generate", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("huggingface complete: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("huggingface complete HTTP %d: %s", resp.StatusCode, b)
	}
	var result struct {
		GeneratedText string `json:"generated_text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result.GeneratedText, nil
}

func (p *huggingFaceProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	vecs, err := p.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(vecs) == 0 {
		return nil, fmt.Errorf("huggingface embed: empty response")
	}
	return vecs[0], nil
}

func (p *huggingFaceProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	payload := map[string]any{"inputs": texts}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/embed", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("huggingface embed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("huggingface embed HTTP %d: %s", resp.StatusCode, b)
	}
	var result [][]float32
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}
