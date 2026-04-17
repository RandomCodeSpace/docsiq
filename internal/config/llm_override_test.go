package config

import (
	"testing"
)

func TestLLMConfigForProject(t *testing.T) {
	root := LLMConfig{Provider: "ollama"}
	override := LLMConfig{Provider: "azure"}

	t.Run("no_override_returns_root", func(t *testing.T) {
		cfg := &Config{LLM: root}
		got := cfg.LLMConfigForProject("some-project")
		if got.Provider != "ollama" {
			t.Errorf("provider = %q, want ollama", got.Provider)
		}
	})

	t.Run("override_is_used", func(t *testing.T) {
		cfg := &Config{
			LLM: root,
			LLMOverrides: map[string]LLMConfig{
				"special": override,
			},
		}
		got := cfg.LLMConfigForProject("special")
		if got.Provider != "azure" {
			t.Errorf("provider = %q, want azure", got.Provider)
		}
	})

	t.Run("unknown_slug_falls_back_to_root", func(t *testing.T) {
		cfg := &Config{
			LLM: root,
			LLMOverrides: map[string]LLMConfig{
				"special": override,
			},
		}
		got := cfg.LLMConfigForProject("not-in-map")
		if got.Provider != "ollama" {
			t.Errorf("provider = %q, want ollama", got.Provider)
		}
	})

	t.Run("empty_slug_returns_root", func(t *testing.T) {
		cfg := &Config{
			LLM: root,
			LLMOverrides: map[string]LLMConfig{
				"x": override,
			},
		}
		got := cfg.LLMConfigForProject("")
		if got.Provider != "ollama" {
			t.Errorf("provider = %q, want ollama", got.Provider)
		}
	})

	t.Run("override_with_empty_provider_is_ignored", func(t *testing.T) {
		cfg := &Config{
			LLM: root,
			LLMOverrides: map[string]LLMConfig{
				"bogus": {Provider: ""}, // present in map but no provider set
			},
		}
		got := cfg.LLMConfigForProject("bogus")
		if got.Provider != "ollama" {
			t.Errorf("provider = %q, want ollama (empty override should fall back)", got.Provider)
		}
	})
}
