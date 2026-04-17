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

	"github.com/RandomCodeSpace/docsiq/internal/api"
	"github.com/RandomCodeSpace/docsiq/internal/embedder"
	"github.com/RandomCodeSpace/docsiq/internal/llm"
	"github.com/RandomCodeSpace/docsiq/internal/project"
	"github.com/RandomCodeSpace/docsiq/internal/sqlitevec"
	"github.com/RandomCodeSpace/docsiq/internal/store"
	"github.com/RandomCodeSpace/docsiq/internal/vectorindex"
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

		// Wave 1: the root store now lives at the _default project's
		// per-project DB path ($DATA_DIR/projects/_default/docsiq.db) so
		// Wave 2 only has to migrate handlers off the `st` reference —
		// the file itself is already in the right place.
		// TODO(wave-2): migrate doc handlers to per-project stores via
		// the registry + project middleware, then drop the root `st`.
		rootSlug := cfg.DefaultProject
		if rootSlug == "" {
			rootSlug = "_default"
		}
		st, err := store.OpenForProject(cfg.DataDir, rootSlug)
		if err != nil {
			return fmt.Errorf("open store: %w", err)
		}
		defer st.Close()
		rootDBPath := cfg.ProjectDBPath(rootSlug)
		slog.Info("📂 store opened", "path", rootDBPath)

		// Attempt to load the embedded sqlite-vec extension. On any
		// failure (placeholder build, unsupported GOOS/GOARCH, dlopen
		// refused) we log WARN and continue — the HNSW in-memory index
		// and/or brute-force fallback still answer vector queries.
		if soPath, extErr := sqlitevec.Extract(cfg.DataDir); extErr != nil {
			slog.Warn("⚠️ sqlite-vec unavailable; using HNSW / brute-force fallback", "err", extErr)
			setVecMode(vecModeNone, extErr.Error())
		} else if loadErr := sqlitevec.LoadInto(st.DB(), soPath); loadErr != nil {
			slog.Warn("⚠️ sqlite-vec load failed; using HNSW / brute-force fallback", "err", loadErr, "path", soPath)
			setVecMode(vecModeNone, loadErr.Error())
		} else {
			slog.Info("🧮 sqlite-vec loaded", "path", soPath)
			setVecMode(vecModeSqliteVec, soPath)
		}

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

