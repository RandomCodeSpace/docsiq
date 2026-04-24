package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/RandomCodeSpace/docsiq/internal/config"
	"github.com/RandomCodeSpace/docsiq/internal/obs"
	"github.com/spf13/cobra"
)

var (
	cfgFile   string
	cfg       *config.Config
	logLevel  string
	logFormat string
)

var rootCmd = &cobra.Command{
	Use:   "docsiq",
	Short: "docsiq — Pure Go GraphRAG MCP server",
	Long: `docsiq ingests unstructured documents, builds a knowledge graph
with community detection, and exposes an MCP server + embedded Web UI.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default ~/.docsiq/config.yaml)")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "Log level: debug, info, warn, error")
	rootCmd.PersistentFlags().StringVar(&logFormat, "log-format", "", "Log format: text|json (env DOCSIQ_LOG_FORMAT; default text)")
}

func initConfig() {
	// Set up structured logger. Level comes from --log-level only.
	var level slog.Level
	switch logLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	// Format resolution order (highest wins):
	//   1. --log-format flag
	//   2. DOCSIQ_LOG_FORMAT env var
	//   3. config file log.format
	//   4. default "text"
	// (3) requires config.Load() to have run; install a temporary
	// handler first so config-load errors land somewhere, then
	// upgrade once the final format is known.
	format := strings.ToLower(strings.TrimSpace(logFormat))
	if format == "" {
		format = strings.ToLower(strings.TrimSpace(os.Getenv("DOCSIQ_LOG_FORMAT")))
	}

	slog.SetDefault(slog.New(buildLogHandler(level, format)))

	var err error
	cfg, err = config.Load(cfgFile)
	if err != nil {
		slog.Error("❌ config error", "err", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		slog.Error("❌ failed to create data directory", "path", cfg.DataDir, "err", err)
		os.Exit(1)
	}

	// If neither flag nor env specified format, use the value from
	// the loaded config (falls back to default "text").
	if format == "" && cfg.Log.Format != "" {
		format = strings.ToLower(strings.TrimSpace(cfg.Log.Format))
		slog.SetDefault(slog.New(buildLogHandler(level, format)))
	}
}

// buildLogHandler assembles the slog handler chain. For "json" format
// we wrap the JSON handler in obs.NewProductionHandler to strip emoji
// prefixes from the message field (keeping them is harmless but noisy
// for log aggregators). "text" keeps emoji for human readability.
func buildLogHandler(level slog.Level, format string) slog.Handler {
	opts := &slog.HandlerOptions{Level: level}
	switch format {
	case "json":
		return obs.NewProductionHandler(slog.NewJSONHandler(os.Stderr, opts))
	default:
		return slog.NewTextHandler(os.Stderr, opts)
	}
}
