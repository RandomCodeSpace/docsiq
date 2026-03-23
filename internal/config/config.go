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
	DataDir   string          `mapstructure:"data_dir"`
	LLM       LLMConfig       `mapstructure:"llm"`
	Indexing   IndexingConfig  `mapstructure:"indexing"`
	Community CommunityConfig `mapstructure:"community"`
	Server    ServerConfig    `mapstructure:"server"`
}

type LLMConfig struct {
	Provider string       `mapstructure:"provider"`
	Azure    AzureConfig  `mapstructure:"azure"`
	Ollama   OllamaConfig `mapstructure:"ollama"`
}

// AzureConfig supports shared defaults with per-service overrides.
// Top-level fields (endpoint, api_key, api_version) are shared defaults.
// Chat/Embed sub-configs override specific fields when set.
//
// Env vars (prefix DOCSCONTEXT):
//
//	DOCSCONTEXT_LLM_AZURE_ENDPOINT       — shared endpoint
//	DOCSCONTEXT_LLM_AZURE_API_KEY        — shared API key
//	DOCSCONTEXT_LLM_AZURE_API_VERSION    — shared API version
//	DOCSCONTEXT_LLM_AZURE_CHAT_ENDPOINT  — chat-specific endpoint
//	DOCSCONTEXT_LLM_AZURE_CHAT_API_KEY   — chat-specific API key
//	DOCSCONTEXT_LLM_AZURE_CHAT_MODEL     — chat model name
//	DOCSCONTEXT_LLM_AZURE_EMBED_ENDPOINT — embedding-specific endpoint
//	DOCSCONTEXT_LLM_AZURE_EMBED_API_KEY  — embedding-specific API key
//	DOCSCONTEXT_LLM_AZURE_EMBED_MODEL    — embedding model name
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

func (a *AzureConfig) ChatEndpoint() string   { return firstNonEmpty(a.Chat.Endpoint, a.Endpoint) }
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
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

func Load(cfgFile string) (*Config, error) {
	v := viper.New()

	// Defaults
	home, _ := os.UserHomeDir()
	defaultDataDir := filepath.Join(home, ".DocsContext", "data")
	defaultCfgDir := filepath.Join(home, ".DocsContext")

	// All keys must have SetDefault for env var binding to work.
	v.SetDefault("data_dir", defaultDataDir)

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

	// Config file
	defaultCfgDirLower := filepath.Join(home, ".docscontext")
	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
	} else {
		v.AddConfigPath(defaultCfgDirLower)
		v.AddConfigPath(defaultCfgDir)
		v.AddConfigPath(".")
		v.SetConfigName("config")
	}

	// Env overrides — every key with a SetDefault above is now reachable
	// as DOCSCONTEXT_<KEY_PATH> with dots replaced by underscores.
	v.SetEnvPrefix("DOCSCONTEXT")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			slog.Warn("⚠️ no config file found, using defaults",
				"searched_paths", []string{defaultCfgDirLower, defaultCfgDir, "."},
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

	slog.Info("⚙️ resolved LLM config", "provider", cfg.LLM.Provider)

	// Expand home dir
	if strings.HasPrefix(cfg.DataDir, "~/") {
		cfg.DataDir = filepath.Join(home, cfg.DataDir[2:])
	}

	return &cfg, nil
}

func (c *Config) DBPath() string {
	return filepath.Join(c.DataDir, "DocsContext.db")
}
