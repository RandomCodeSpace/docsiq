package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	DataDir        string          `mapstructure:"data_dir"`
	DefaultProject string          `mapstructure:"default_project"`
	LLM            LLMConfig       `mapstructure:"llm"`
	Indexing       IndexingConfig  `mapstructure:"indexing"`
	Community      CommunityConfig `mapstructure:"community"`
	Server         ServerConfig    `mapstructure:"server"`

	// Phase-5: per-project LLM overrides. Keyed by project slug.
	// When a slug is missing from the map, callers fall back to the
	// top-level LLM field. Not bound to env vars — configure via
	// YAML only (env flat-string rewriting doesn't nest well).
	LLMOverrides map[string]LLMConfig `mapstructure:"llm_overrides"`
}

// LLMConfigForProject returns the override for slug if present, otherwise
// the root LLM config. A missing or empty slug yields the root config.
// The Provider field is treated as the presence sentinel — a YAML block
// with no `provider:` key leaves Provider empty, so we treat that as
// "no override declared" and fall back to the root.
func (c *Config) LLMConfigForProject(slug string) LLMConfig {
	if slug == "" {
		return c.LLM
	}
	override, ok := c.LLMOverrides[slug]
	if !ok || override.Provider == "" {
		return c.LLM
	}
	return override
}

// DefaultProjectSlug is the slug used when no ?project= / X-Project value
// is supplied on a request. Locked by Phase-1 spec.
const DefaultProjectSlug = "_default"

type LLMConfig struct {
	Provider string       `mapstructure:"provider"`
	Azure    AzureConfig  `mapstructure:"azure"`
	Ollama   OllamaConfig `mapstructure:"ollama"`
	OpenAI   OpenAIConfig `mapstructure:"openai"`
}

// OpenAIConfig configures the direct OpenAI (api.openai.com) provider,
// as opposed to Azure OpenAI which lives on AzureConfig. Env vars:
//
//	DOCSIQ_LLM_OPENAI_API_KEY      — API key (required)
//	DOCSIQ_LLM_OPENAI_BASE_URL     — override for proxies / gateways
//	DOCSIQ_LLM_OPENAI_CHAT_MODEL   — chat model, default gpt-4o-mini
//	DOCSIQ_LLM_OPENAI_EMBED_MODEL  — embedding model, default
//	                                 text-embedding-3-small
//	DOCSIQ_LLM_OPENAI_ORGANIZATION — optional org header
type OpenAIConfig struct {
	APIKey       string `mapstructure:"api_key"`
	BaseURL      string `mapstructure:"base_url"`
	ChatModel    string `mapstructure:"chat_model"`
	EmbedModel   string `mapstructure:"embed_model"`
	Organization string `mapstructure:"organization"`
}

// AzureConfig supports shared defaults with per-service overrides.
// Top-level fields (endpoint, api_key, api_version) are shared defaults.
// Chat/Embed sub-configs override specific fields when set.
//
// Env vars (prefix DOCSIQ):
//
//	DOCSIQ_LLM_AZURE_ENDPOINT       — shared endpoint
//	DOCSIQ_LLM_AZURE_API_KEY        — shared API key
//	DOCSIQ_LLM_AZURE_API_VERSION    — shared API version
//	DOCSIQ_LLM_AZURE_CHAT_ENDPOINT  — chat-specific endpoint
//	DOCSIQ_LLM_AZURE_CHAT_API_KEY   — chat-specific API key
//	DOCSIQ_LLM_AZURE_CHAT_MODEL     — chat model name
//	DOCSIQ_LLM_AZURE_EMBED_ENDPOINT — embedding-specific endpoint
//	DOCSIQ_LLM_AZURE_EMBED_API_KEY  — embedding-specific API key
//	DOCSIQ_LLM_AZURE_EMBED_MODEL    — embedding model name
type AzureConfig struct {
	// Shared defaults — used when chat/embed-specific values are not set.
	Endpoint   string `mapstructure:"endpoint"`
	APIKey     string `mapstructure:"api_key"`
	APIVersion string `mapstructure:"api_version"`

	Chat  AzureServiceConfig `mapstructure:"chat"`
	Embed AzureServiceConfig `mapstructure:"embed"`
}

type AzureServiceConfig struct {
	Endpoint   string `mapstructure:"endpoint"`
	APIKey     string `mapstructure:"api_key"`
	APIVersion string `mapstructure:"api_version"`
	Model      string `mapstructure:"model"`
}

// Resolved accessors — per-service value with shared fallback.

func (a *AzureConfig) ChatEndpoint() string    { return firstNonEmpty(a.Chat.Endpoint, a.Endpoint) }
func (a *AzureConfig) ChatAPIKey() string      { return firstNonEmpty(a.Chat.APIKey, a.APIKey) }
func (a *AzureConfig) ChatAPIVersion() string  { return firstNonEmpty(a.Chat.APIVersion, a.APIVersion) }
func (a *AzureConfig) ChatModel() string       { return a.Chat.Model }
func (a *AzureConfig) EmbedEndpoint() string   { return firstNonEmpty(a.Embed.Endpoint, a.Endpoint) }
func (a *AzureConfig) EmbedAPIKey() string     { return firstNonEmpty(a.Embed.APIKey, a.APIKey) }
func (a *AzureConfig) EmbedAPIVersion() string { return firstNonEmpty(a.Embed.APIVersion, a.APIVersion) }
func (a *AzureConfig) EmbedModel() string      { return a.Embed.Model }

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

type OllamaConfig struct {
	BaseURL    string `mapstructure:"base_url"`
	ChatModel  string `mapstructure:"chat_model"`
	EmbedModel string `mapstructure:"embed_model"`
}

type IndexingConfig struct {
	ChunkSize     int  `mapstructure:"chunk_size"`
	ChunkOverlap  int  `mapstructure:"chunk_overlap"`
	BatchSize     int  `mapstructure:"batch_size"`
	Workers       int  `mapstructure:"workers"`
	ExtractGraph  bool `mapstructure:"extract_graph"`
	ExtractClaims bool `mapstructure:"extract_claims"`
	MaxGleanings  int  `mapstructure:"max_gleanings"`
}

type CommunityConfig struct {
	MinCommunitySize int `mapstructure:"min_community_size"`
	MaxLevels        int `mapstructure:"max_levels"`
}

type ServerConfig struct {
	Host   string `mapstructure:"host"`
	Port   int    `mapstructure:"port"`
	APIKey string `mapstructure:"api_key"`
}

func Load(cfgFile string) (*Config, error) {
	v := viper.New()

	// Defaults
	home, _ := os.UserHomeDir()
	defaultDataDir := filepath.Join(home, ".docsiq", "data")

	// All keys must have SetDefault for env var binding to work.
	v.SetDefault("data_dir", defaultDataDir)
	v.SetDefault("default_project", DefaultProjectSlug)

	// LLM — provider
	v.SetDefault("llm.provider", "ollama")

	// LLM — Ollama
	v.SetDefault("llm.ollama.base_url", "http://localhost:11434")
	v.SetDefault("llm.ollama.chat_model", "llama3.2")
	v.SetDefault("llm.ollama.embed_model", "nomic-embed-text")

	// LLM — Azure shared defaults
	v.SetDefault("llm.azure.endpoint", "")
	v.SetDefault("llm.azure.api_key", "")
	v.SetDefault("llm.azure.api_version", "2024-08-01")

	// LLM — Azure chat
	v.SetDefault("llm.azure.chat.endpoint", "")
	v.SetDefault("llm.azure.chat.api_key", "")
	v.SetDefault("llm.azure.chat.api_version", "")
	v.SetDefault("llm.azure.chat.model", "gpt-4o")

	// LLM — Azure embed
	v.SetDefault("llm.azure.embed.endpoint", "")
	v.SetDefault("llm.azure.embed.api_key", "")
	v.SetDefault("llm.azure.embed.api_version", "")
	v.SetDefault("llm.azure.embed.model", "text-embedding-3-small")

	// LLM — OpenAI (direct, as opposed to Azure OpenAI)
	v.SetDefault("llm.openai.api_key", "")
	v.SetDefault("llm.openai.base_url", "https://api.openai.com/v1")
	v.SetDefault("llm.openai.chat_model", "gpt-4o-mini")
	v.SetDefault("llm.openai.embed_model", "text-embedding-3-small")
	v.SetDefault("llm.openai.organization", "")

	// Indexing
	v.SetDefault("indexing.chunk_size", 512)
	v.SetDefault("indexing.chunk_overlap", 50)
	v.SetDefault("indexing.batch_size", 20)
	v.SetDefault("indexing.workers", 4)
	v.SetDefault("indexing.extract_graph", true)
	v.SetDefault("indexing.extract_claims", true)
	v.SetDefault("indexing.max_gleanings", 1)

	// Community
	v.SetDefault("community.min_community_size", 2)
	v.SetDefault("community.max_levels", 3)

	// Server
	v.SetDefault("server.host", "127.0.0.1")
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.api_key", "")

	// Config file search paths. Only ~/.docsiq and CWD are consulted.
	newCfgDir := filepath.Join(home, ".docsiq")
	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
	} else {
		v.AddConfigPath(newCfgDir)
		v.AddConfigPath(".")
		v.SetConfigName("config")
	}

	// Env overrides — every key with a SetDefault above is now reachable
	// as DOCSIQ_<KEY_PATH> with dots replaced by underscores.
	v.SetEnvPrefix("DOCSIQ")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Explicit alias: DOCSIQ_API_KEY is a convenience shortcut for
	// DOCSIQ_SERVER_API_KEY (the auth-middleware API key). Bind both so
	// either form populates server.api_key. BindEnv names are matched
	// verbatim (not prefixed), so we list the full env var name.
	_ = v.BindEnv("server.api_key", "DOCSIQ_SERVER_API_KEY", "DOCSIQ_API_KEY")

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			slog.Warn("⚠️ no config file found, using defaults",
				"searched_paths", []string{newCfgDir, "."},
				"expected_names", "config.yaml or config.yml")
		} else {
			return nil, fmt.Errorf("reading config: %w", err)
		}
	} else {
		slog.Info("⚙️ loaded config file", "path", v.ConfigFileUsed())
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	// "none" is an explicit opt-out of LLM; treat it as valid and log clearly.
	if cfg.LLM.Provider == "none" {
		slog.Info("⚙️ resolved LLM config", "provider", "none", "llm_disabled", true)
	} else {
		slog.Info("⚙️ resolved LLM config", "provider", cfg.LLM.Provider)
	}

	// Expand home dir
	if strings.HasPrefix(cfg.DataDir, "~/") {
		cfg.DataDir = filepath.Join(home, cfg.DataDir[2:])
	}

	return &cfg, nil
}

// ProjectDBPath returns the per-project SQLite path for the given slug:
// $DATA_DIR/projects/<slug>/docsiq.db. Does NOT validate the slug —
// callers should use project.IsValidSlug or store.OpenForProject (which
// performs the check) when the slug came from untrusted input.
func (c *Config) ProjectDBPath(slug string) string {
	return filepath.Join(c.DataDir, "projects", slug, "docsiq.db")
}

// NotesDir returns the per-project notes directory:
// $DATA_DIR/projects/<slug>/notes. Callers that actually intend to read
// or write notes should use os.MkdirAll on this path first — the config
// helper does not touch the filesystem.
func (c *Config) NotesDir(slug string) string {
	return filepath.Join(c.DataDir, "projects", slug, "notes")
}
