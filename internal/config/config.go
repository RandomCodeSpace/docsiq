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
	DataDir   string        `mapstructure:"data_dir"`
	LLM       LLMConfig     `mapstructure:"llm"`
	Indexing  IndexingConfig `mapstructure:"indexing"`
	Community CommunityConfig `mapstructure:"community"`
	Server    ServerConfig  `mapstructure:"server"`
}

type LLMConfig struct {
	Provider    string             `mapstructure:"provider"`
	Azure       AzureConfig        `mapstructure:"azure"`
	Ollama      OllamaConfig       `mapstructure:"ollama"`
	HuggingFace HuggingFaceConfig  `mapstructure:"huggingface"`
}

type AzureConfig struct {
	Endpoint   string `mapstructure:"endpoint"`
	APIKey     string `mapstructure:"api_key"`
	APIVersion string `mapstructure:"api_version"`
	ChatModel  string `mapstructure:"chat_model"`
	EmbedModel string `mapstructure:"embed_model"`
}

type OllamaConfig struct {
	BaseURL    string `mapstructure:"base_url"`
	ChatModel  string `mapstructure:"chat_model"`
	EmbedModel string `mapstructure:"embed_model"`
}

type HuggingFaceConfig struct {
	BaseURL    string `mapstructure:"base_url"`
	APIKey     string `mapstructure:"api_key"`
	ChatModel  string `mapstructure:"chat_model"`
	EmbedModel string `mapstructure:"embed_model"`
}

type IndexingConfig struct {
	ChunkSize    int  `mapstructure:"chunk_size"`
	ChunkOverlap int  `mapstructure:"chunk_overlap"`
	BatchSize    int  `mapstructure:"batch_size"`
	Workers      int  `mapstructure:"workers"`
	ExtractGraph bool `mapstructure:"extract_graph"`
	ExtractClaims bool `mapstructure:"extract_claims"`
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

	v.SetDefault("data_dir", defaultDataDir)
	v.SetDefault("llm.provider", "ollama")
	v.SetDefault("llm.ollama.base_url", "http://localhost:11434")
	v.SetDefault("llm.ollama.chat_model", "llama3.2")
	v.SetDefault("llm.ollama.embed_model", "nomic-embed-text")
	v.SetDefault("llm.azure.api_version", "2024-02-01")
	v.SetDefault("llm.azure.chat_model", "gpt-4o")
	v.SetDefault("llm.azure.embed_model", "text-embedding-3-small")
	v.SetDefault("llm.huggingface.base_url", "http://localhost:8000")
	v.SetDefault("llm.huggingface.chat_model", "mistralai/Mistral-7B-Instruct-v0.3")
	v.SetDefault("llm.huggingface.embed_model", "sentence-transformers/all-MiniLM-L6-v2")
	v.SetDefault("indexing.chunk_size", 512)
	v.SetDefault("indexing.chunk_overlap", 50)
	v.SetDefault("indexing.batch_size", 20)
	v.SetDefault("indexing.workers", 4)
	v.SetDefault("indexing.extract_graph", true)
	v.SetDefault("indexing.extract_claims", true)
	v.SetDefault("community.min_community_size", 3)
	v.SetDefault("community.max_levels", 3)
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

	// Env overrides
	v.SetEnvPrefix("DocsContext")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			slog.Warn("no config file found, using defaults",
				"searched_paths", []string{defaultCfgDirLower, defaultCfgDir, "."},
				"expected_names", "config.yaml or config.yml")
		} else {
			return nil, fmt.Errorf("reading config: %w", err)
		}
	} else {
		slog.Info("loaded config file", "path", v.ConfigFileUsed())
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	slog.Info("resolved LLM config", "provider", cfg.LLM.Provider)

	// Expand home dir
	if strings.HasPrefix(cfg.DataDir, "~/") {
		cfg.DataDir = filepath.Join(home, cfg.DataDir[2:])
	}

	return &cfg, nil
}

func (c *Config) DBPath() string {
	return filepath.Join(c.DataDir, "DocsContext.db")
}

