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
// Env vars (prefix DOCSIQ — legacy DOCSCONTEXT_* is still accepted as a
// deprecated alias and rewritten to DOCSIQ_* at Load() time):
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
	Host   string `mapstructure:"host"`
	Port   int    `mapstructure:"port"`
	APIKey string `mapstructure:"api_key"`
}

// migrateDeprecatedEnv scans os.Environ() for the legacy DOCSCONTEXT_ prefix
// and, for each such variable, mirrors it into the corresponding DOCSIQ_ name
// unless the DOCSIQ_ name is already set. A deprecation warning is emitted
// per variable plus one summary warning if any were found.
//
// Behavior decisions worth documenting:
//   - Empty-value deprecated var (DOCSCONTEXT_FOO=""): still warn and mirror
//     the empty string. The user still needs to migrate the variable name
//     even if it's empty today.
//   - Same value under both prefixes: DOCSIQ_ wins (no overwrite) but we
//     still warn about the deprecated one so the user knows to drop it.
//   - Oddly-named variables like DOCSCONTEXT__FOO (double underscore) are
//     mirrored verbatim to DOCSIQ__FOO; viper will simply ignore them if
//     they don't match any key. Harmless.
func migrateDeprecatedEnv() {
	const oldPrefix = "DOCSCONTEXT_"
	const newPrefix = "DOCSIQ_"

	var found int
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, oldPrefix) {
			continue
		}
		name, value, ok := strings.Cut(e, "=")
		if !ok {
			continue
		}
		newName := newPrefix + strings.TrimPrefix(name, oldPrefix)

		slog.Warn("⚠️ deprecated env var",
			"deprecated", name,
			"replacement", newName)
		found++

		// Only mirror if the new name is not already set (DOCSIQ wins).
		if _, set := os.LookupEnv(newName); set {
			continue
		}
		// Note: os.Setenv on empty-string values is a no-op on some platforms;
		// we accept that — a user with DOCSCONTEXT_FOO="" and no DOCSIQ_FOO
		// effectively means "unset", which is the intended outcome.
		_ = os.Setenv(newName, value)
	}

	if found > 0 {
		slog.Warn("⚠️ using deprecated DOCSCONTEXT_* env vars; support will be removed in v2.0")
	}
}

func Load(cfgFile string) (*Config, error) {
	// Step 1 — migrate deprecated env vars BEFORE viper reads env.
	// This is Option (c) from the Phase-0 design doc: instead of binding
	// each key under two prefixes, we rewrite the environment in-process
	// so viper only has to know about DOCSIQ_*.
	migrateDeprecatedEnv()

	v := viper.New()

	// Defaults
	home, _ := os.UserHomeDir()
	defaultDataDir := filepath.Join(home, ".docsiq", "data")
	legacyDataDir := filepath.Join(home, ".docscontext", "data")

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
	v.SetDefault("server.api_key", "")

	// Config file search paths. Order matters — viper uses the first hit.
	newCfgDir := filepath.Join(home, ".docsiq")              // NEW — wins
	oldCfgDirLower := filepath.Join(home, ".docscontext")    // legacy lower
	oldCfgDirMixed := filepath.Join(home, ".DocsContext")    // legacy mixed
	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
	} else {
		v.AddConfigPath(newCfgDir)
		v.AddConfigPath(oldCfgDirLower)
		v.AddConfigPath(oldCfgDirMixed)
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
				"searched_paths", []string{newCfgDir, oldCfgDirLower, oldCfgDirMixed, "."},
				"expected_names", "config.yaml or config.yml")
		} else {
			return nil, fmt.Errorf("reading config: %w", err)
		}
	} else {
		used := v.ConfigFileUsed()
		slog.Info("⚙️ loaded config file", "path", used)
		// If the winning config file sits under a legacy directory, nag the user.
		if cfgFile == "" {
			usedDir := filepath.Dir(used)
			if usedDir == oldCfgDirLower || usedDir == oldCfgDirMixed {
				slog.Warn("⚠️ deprecated config path",
					"path", used,
					"move_to", filepath.Join(newCfgDir, "config.yaml"))
			}
			// If a file is present in the new dir AND an old dir, warn that
			// the old one is shadowed.
			if usedDir == newCfgDir {
				for _, legacy := range []string{oldCfgDirLower, oldCfgDirMixed} {
					for _, ext := range []string{"yaml", "yml"} {
						shadowed := filepath.Join(legacy, "config."+ext)
						if _, err := os.Stat(shadowed); err == nil {
							slog.Warn("⚠️ shadowed legacy config file — remove to silence",
								"shadowed", shadowed,
								"winner", used)
						}
					}
				}
			}
		}
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

	// Data-dir migration nudge — do NOT auto-move. Only warn when the new
	// path is missing but the legacy path exists.
	if cfg.DataDir == defaultDataDir {
		if _, err := os.Stat(defaultDataDir); os.IsNotExist(err) {
			if _, err := os.Stat(legacyDataDir); err == nil {
				slog.Warn("⚠️ legacy data dir detected — please move it",
					"legacy", legacyDataDir,
					"move_to", defaultDataDir)
			}
		}
	}

	return &cfg, nil
}

func (c *Config) DBPath() string {
	return filepath.Join(c.DataDir, "DocsContext.db")
}
