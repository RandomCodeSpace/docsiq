//go:build integration
// +build integration

package mcp_test

import (
	"context"
	"testing"
	"time"

	"github.com/RandomCodeSpace/docsiq/internal/config"
	"github.com/RandomCodeSpace/docsiq/internal/llm"
	"github.com/RandomCodeSpace/docsiq/internal/project"
)

// TestGlobalSearch_PerProjectLLMOverride verifies that
// llm.ProviderForProject — the same resolver wired into the
// global_search MCP tool — returns a *different* provider for a slug
// that has an LLMOverrides entry versus the root LLM config, and falls
// back to the root provider for a slug with no override.
//
// We can't drive the full global_search handler end-to-end in a unit
// test (it requires a populated community table and a live LLM to call
// prov.Complete), so we assert on the selection layer that the handler
// delegates to. The MCP tool handler now calls exactly this function,
// so a passing test here == the right provider reaches search.GlobalSearch.
func TestGlobalSearch_PerProjectLLMOverride(t *testing.T) {
	_ = context.Background()

	cfg := &config.Config{
		DataDir:        t.TempDir(),
		DefaultProject: config.DefaultProjectSlug,
		LLM: config.LLMConfig{
			Provider: "openai",
			OpenAI: config.OpenAIConfig{
				APIKey:     "root-key",
				BaseURL:    "http://127.0.0.1:1",
				ChatModel:  "root-chat",
				EmbedModel: "root-embed",
			},
		},
		LLMOverrides: map[string]config.LLMConfig{
			"override-proj": {
				Provider: "openai",
				OpenAI: config.OpenAIConfig{
					APIKey:     "override-key",
					BaseURL:    "http://127.0.0.1:2",
					ChatModel:  "override-chat",
					EmbedModel: "override-embed",
				},
			},
		},
	}

	// Register two projects in the registry so either slug is valid.
	reg, err := project.OpenRegistry(cfg.DataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer reg.Close()
	for _, slug := range []string{"override-proj", "plain-proj"} {
		if err := reg.Register(project.Project{
			Slug:      slug,
			Name:      slug,
			Remote:    "r-" + slug,
			CreatedAt: time.Now().Unix(),
		}); err != nil {
			t.Fatalf("register %s: %v", slug, err)
		}
	}

	// Project WITH an override: resolved provider uses the override cfg.
	provOverride, err := llm.ProviderForProject(cfg, "override-proj")
	if err != nil {
		t.Fatalf("ProviderForProject(override): %v", err)
	}
	if got := provOverride.ModelID(); got != "override-embed" {
		t.Errorf("override slug: ModelID = %q, want override-embed", got)
	}

	// Project WITHOUT an override: falls back to root LLM config.
	provRoot, err := llm.ProviderForProject(cfg, "plain-proj")
	if err != nil {
		t.Fatalf("ProviderForProject(plain): %v", err)
	}
	if got := provRoot.ModelID(); got != "root-embed" {
		t.Errorf("plain slug: ModelID = %q, want root-embed", got)
	}

	// Divergence check — this is the guarantee the MCP tool depends on.
	if provOverride.ModelID() == provRoot.ModelID() {
		t.Fatalf("per-project override did not yield a distinct provider: both report %q", provOverride.ModelID())
	}

	// Empty slug should resolve to root (DefaultProject) — the MCP
	// handler stringArg default matches this path.
	provEmpty, err := llm.ProviderForProject(cfg, "")
	if err != nil {
		t.Fatalf("ProviderForProject(empty): %v", err)
	}
	if got := provEmpty.ModelID(); got != "root-embed" {
		t.Errorf("empty slug: ModelID = %q, want root-embed", got)
	}
}
