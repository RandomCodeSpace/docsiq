package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/RandomCodeSpace/docscontext/internal/api"
	"github.com/RandomCodeSpace/docscontext/internal/embedder"
	"github.com/RandomCodeSpace/docscontext/internal/llm"
	"github.com/RandomCodeSpace/docscontext/internal/project"
	"github.com/RandomCodeSpace/docscontext/internal/store"
	"github.com/RandomCodeSpace/docscontext/internal/vectorindex"
	"github.com/spf13/cobra"
)

var (
	serveHost string
	servePort int
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start MCP + Web UI server",
	RunE: func(cmd *cobra.Command, args []string) error {
		if serveHost != "" {
			cfg.Server.Host = serveHost
		}
		if servePort != 0 {
			cfg.Server.Port = servePort
		}

		st, err := store.Open(cfg.DBPath())
		if err != nil {
			return fmt.Errorf("open store: %w", err)
		}
		defer st.Close()
		slog.Info("📂 store opened", "path", cfg.DBPath())

		// Phase-1: open the per-project registry alongside the legacy
		// single-DB store. Handlers still use `st` until Phase-2 migrates
		// them to per-project stores; the registry exists today so the
		// project middleware can resolve ?project=/X-Project.
		projectsDir := filepath.Join(cfg.DataDir, "projects")
		if err := os.MkdirAll(projectsDir, 0o755); err != nil {
			return fmt.Errorf("mkdir projects dir: %w", err)
		}
		registry, err := project.OpenRegistry(cfg.DataDir)
		if err != nil {
			return fmt.Errorf("open registry: %w", err)
		}
		defer registry.Close()
		slog.Info("📒 project registry opened", "path", filepath.Join(cfg.DataDir, "registry.db"))

		prov, err := llm.NewProvider(&cfg.LLM)
		if err != nil {
			return fmt.Errorf("llm provider: %w", err)
		}
		slog.Info("⚙️ LLM provider initialised", "provider", prov.Name(), "model", prov.ModelID())

		emb := embedder.New(prov, cfg.Indexing.BatchSize)

		// Build in-memory HNSW vector index from the store's chunk
		// embeddings. Rebuilt on every boot; the store remains the
		// source of truth. Empty index (fresh install) is fine — search
		// handlers fall back to brute-force when nil/empty.
		buildCtx, cancelBuild := context.WithTimeout(context.Background(), 60*time.Second)
		vecIdx, err := vectorindex.BuildFromStore(buildCtx, st)
		cancelBuild()
		if err != nil {
			slog.Warn("⚠️ vector index build failed; using brute-force fallback", "err", err)
			vecIdx = nil
		}

		router := api.NewRouter(st, prov, emb, cfg, registry, api.WithVectorIndex(vecIdx))

		addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			return fmt.Errorf("listen: %w", err)
		}

		srv := &http.Server{Handler: router, ReadTimeout: 60 * time.Second, WriteTimeout: 120 * time.Second}

		slog.Info("🚀 server started",
			"addr", "http://"+addr,
			"ui", "http://"+addr+"/",
			"mcp", "http://"+addr+"/mcp",
			"api", "http://"+addr+"/api/",
		)

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		go func() {
			if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
				slog.Error("❌ server error", "err", err)
			}
		}()

		<-ctx.Done()
		slog.Info("🛑 shutting down...")
		shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutCtx); err != nil {
			slog.Error("❌ shutdown error", "err", err)
			return err
		}
		slog.Info("✅ shutdown complete")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Flags().StringVar(&serveHost, "host", "", "Server host (overrides config)")
	serveCmd.Flags().IntVar(&servePort, "port", 0, "Server port (overrides config)")
}

