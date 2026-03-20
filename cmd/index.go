package cmd

import (
	"fmt"
	"log/slog"

	"github.com/RandomCodeSpace/docscontext/internal/llm"
	"github.com/RandomCodeSpace/docscontext/internal/pipeline"
	"github.com/RandomCodeSpace/docscontext/internal/store"
	"github.com/spf13/cobra"
)

var (
	indexForce       bool
	indexWorkers     int
	indexBatch       int
	indexFinalize    bool
	indexVerbose     bool
	indexURL         string
	indexMaxPages    int
	indexMaxDepth    int
	indexSkipSitemap bool
)

var indexCmd = &cobra.Command{
	Use:   "index [path]",
	Short: "Index documents or a documentation website (Phases 1-2)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		st, err := store.Open(cfg.DBPath())
		if err != nil {
			return fmt.Errorf("open store: %w", err)
		}
		defer st.Close()

		if indexFinalize {
			prov, err := llm.NewProvider(&cfg.LLM)
			if err != nil {
				return fmt.Errorf("llm provider: %w", err)
			}
			pl := pipeline.New(st, prov, cfg)
			slog.Info("🧩 running Phase 3-4: community detection + summaries")
			if err := pl.Finalize(cmd.Context(), indexVerbose); err != nil {
				slog.Error("❌ finalization failed", "err", err)
				return err
			}
			slog.Info("✅ finalization complete")
			return nil
		}

		prov, err := llm.NewProvider(&cfg.LLM)
		if err != nil {
			return fmt.Errorf("llm provider: %w", err)
		}
		pl := pipeline.New(st, prov, cfg)
		opts := pipeline.IndexOptions{
			Force:       indexForce,
			Workers:     indexWorkers,
			Verbose:     indexVerbose,
			MaxPages:    indexMaxPages,
			MaxDepth:    indexMaxDepth,
			SkipSitemap: indexSkipSitemap,
		}

		if indexURL != "" {
			slog.Info("🌐 crawling documentation site", "url", indexURL, "workers", indexWorkers,
				"max_pages", indexMaxPages, "max_depth", indexMaxDepth)
			if err := pl.IndexURL(cmd.Context(), indexURL, opts); err != nil {
				slog.Error("❌ web indexing failed", "url", indexURL, "err", err)
				return err
			}
			slog.Info("✅ web indexing complete", "url", indexURL)
			return nil
		}

		if len(args) == 0 {
			return fmt.Errorf("path or --url required (or use --finalize)")
		}

		slog.Info("📄 indexing path", "path", args[0], "workers", indexWorkers, "force", indexForce)
		if err := pl.IndexPath(cmd.Context(), args[0], opts); err != nil {
			slog.Error("❌ indexing failed", "path", args[0], "err", err)
			return err
		}
		slog.Info("✅ indexing complete", "path", args[0])
		return nil
	},
}

func init() {
	rootCmd.AddCommand(indexCmd)
	indexCmd.Flags().BoolVar(&indexForce, "force", false, "Re-index even if file hash already exists")
	indexCmd.Flags().IntVar(&indexWorkers, "workers", 4, "Parallel workers for indexing")
	indexCmd.Flags().IntVar(&indexBatch, "batch-size", 20, "Embedding batch size")
	indexCmd.Flags().BoolVar(&indexFinalize, "finalize", false, "Run community detection + summaries (Phases 3-4)")
	indexCmd.Flags().BoolVar(&indexVerbose, "verbose", false, "Show per-file errors")
	indexCmd.Flags().StringVar(&indexURL, "url", "", "Root URL of a documentation site to crawl and index (MkDocs, Docusaurus, Sphinx)")
	indexCmd.Flags().IntVar(&indexMaxPages, "max-pages", 500, "Maximum pages to crawl (0 = unlimited)")
	indexCmd.Flags().IntVar(&indexMaxDepth, "max-depth", 0, "Maximum BFS link depth (0 = unlimited)")
	indexCmd.Flags().BoolVar(&indexSkipSitemap, "skip-sitemap", false, "Force BFS crawl even if sitemap.xml exists")
}

