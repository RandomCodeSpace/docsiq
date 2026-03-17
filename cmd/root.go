package cmd

import (
	"fmt"
	"os"

	"github.com/RandomCodeSpace/docsgraphcontext/internal/config"
	"github.com/spf13/cobra"
)

var (
	cfgFile string
	cfg     *config.Config
)

var rootCmd = &cobra.Command{
	Use:   "docsgraph",
	Short: "DocsGraphContext — Pure Go GraphRAG MCP server",
	Long: `DocsGraphContext ingests unstructured documents, builds a knowledge graph
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
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default ~/.docsgraph/config.yaml)")
}

func initConfig() {
	var err error
	cfg, err = config.Load(cfgFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		fmt.Fprintln(os.Stderr, "mkdir error:", err)
		os.Exit(1)
	}
}
