package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/RandomCodeSpace/docscontext/internal/config"
	"github.com/spf13/cobra"
)

var (
	cfgFile   string
	cfg       *config.Config
	logLevel  string
	logFormat string
)

var rootCmd = &cobra.Command{
	Use:   "DocsContext",
	Short: "DocsContext — Pure Go GraphRAG MCP server",
	Long: `DocsContext ingests unstructured documents, builds a knowledge graph
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
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default ~/.docscontext/config.yaml or ~/.DocsContext/config.yaml)")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "Log level: debug, info, warn, error")
	rootCmd.PersistentFlags().StringVar(&logFormat, "log-format", "", "Log format: text|json (env DOCSIQ_LOG_FORMAT; default text)")
}

func initConfig() {
	// Set up structured logger
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
	// Log format: --log-format wins, else DOCSIQ_LOG_FORMAT, else "text".
	format := strings.ToLower(strings.TrimSpace(logFormat))
	if format == "" {
		format = strings.ToLower(strings.TrimSpace(os.Getenv("DOCSIQ_LOG_FORMAT")))
	}
	handlerOpts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	switch format {
	case "json":
		handler = slog.NewJSONHandler(os.Stderr, handlerOpts)
	default:
		handler = slog.NewTextHandler(os.Stderr, handlerOpts)
	}
	slog.SetDefault(slog.New(handler))

	var err error
	cfg, err = config.Load(cfgFile)
	if err != nil {
		slog.Error("❌ config error", "err", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		slog.Error("❌ failed to create data directory", "path", cfg.DataDir, "err", err)
		os.Exit(1)
	}
}



