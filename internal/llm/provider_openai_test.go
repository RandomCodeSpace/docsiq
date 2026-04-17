package llm

import (
	"strings"
	"testing"

	"github.com/RandomCodeSpace/docsiq/internal/config"
)

func TestOpenAIProvider_MissingAPIKeyRejected(t *testing.T) {
	cfg := &config.LLMConfig{
		Provider: "openai",
		OpenAI: config.OpenAIConfig{
			APIKey: "",
		},
	}
	_, err := NewProvider(cfg)
	if err == nil {
		t.Fatalf("expected error for empty API key")
	}
	if !strings.Contains(err.Error(), "API key is empty") {
		t.Fatalf("error message should mention empty API key, got %q", err.Error())
	}
}

func TestOpenAIProvider_HappyPathConstruct(t *testing.T) {
	cfg := &config.LLMConfig{
		Provider: "openai",
		OpenAI: config.OpenAIConfig{
			APIKey:     "sk-test-dummy",
			BaseURL:    "https://api.openai.com/v1",
			ChatModel:  "gpt-4o-mini",
			EmbedModel: "text-embedding-3-small",
		},
	}
	p, err := NewProvider(cfg)
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	if p.Name() != "openai" {
		t.Fatalf("want name openai, got %q", p.Name())
	}
	if p.ModelID() != "text-embedding-3-small" {
		t.Fatalf("want embed model id, got %q", p.ModelID())
	}
}

func TestOpenAIProvider_ModelOverride(t *testing.T) {
	cfg := &config.LLMConfig{
		Provider: "openai",
		OpenAI: config.OpenAIConfig{
			APIKey:     "sk-test",
			ChatModel:  "gpt-4o",
			EmbedModel: "text-embedding-3-large",
		},
	}
	p, err := NewProvider(cfg)
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	// ModelID reflects the embedding model (vector-dim consistency).
	if p.ModelID() != "text-embedding-3-large" {
		t.Fatalf("want overridden embed model, got %q", p.ModelID())
	}
}

func TestOpenAIProvider_BaseURLOverride(t *testing.T) {
	// A non-default BaseURL (proxy / gateway) must not cause
	// construction to fail — we can't reach langchaingo's internal
	// client fields to assert the URL, so the assertion is simply
	// "no error" + "provider Name() is openai".
	cfg := &config.LLMConfig{
		Provider: "openai",
		OpenAI: config.OpenAIConfig{
			APIKey:  "sk-test",
			BaseURL: "https://proxy.example.com/v1",
		},
	}
	p, err := NewProvider(cfg)
	if err != nil {
		t.Fatalf("construct with base-url override: %v", err)
	}
	if p.Name() != "openai" {
		t.Fatalf("want name openai, got %q", p.Name())
	}
}

func TestOpenAIProvider_DefaultsWhenFieldsEmpty(t *testing.T) {
	// Only APIKey set — everything else should fall back to
	// hard-coded defaults inside newOpenAIProvider.
	cfg := &config.LLMConfig{
		Provider: "openai",
		OpenAI: config.OpenAIConfig{
			APIKey: "sk-test",
		},
	}
	p, err := NewProvider(cfg)
	if err != nil {
		t.Fatalf("construct with defaults: %v", err)
	}
	if p.Name() != "openai" {
		t.Fatalf("want openai, got %q", p.Name())
	}
	if p.ModelID() != defaultOpenAIEmbedModel {
		t.Fatalf("want default embed model %q, got %q", defaultOpenAIEmbedModel, p.ModelID())
	}
}

func TestOpenAIProvider_OrganizationOptional(t *testing.T) {
	cfg := &config.LLMConfig{
		Provider: "openai",
		OpenAI: config.OpenAIConfig{
			APIKey:       "sk-test",
			Organization: "org-12345",
		},
	}
	p, err := NewProvider(cfg)
	if err != nil {
		t.Fatalf("construct with org: %v", err)
	}
	if p.Name() != "openai" {
		t.Fatalf("want openai, got %q", p.Name())
	}
}

func TestOpenAIProvider_UnknownProviderError(t *testing.T) {
	// Unchanged behavior: non-openai/azure/ollama provider is rejected.
	cfg := &config.LLMConfig{Provider: "bogus"}
	_, err := NewProvider(cfg)
	if err == nil {
		t.Fatalf("expected error for bogus provider")
	}
	if !strings.Contains(err.Error(), "openai") {
		t.Fatalf("error should list openai as supported, got %q", err.Error())
	}
}
