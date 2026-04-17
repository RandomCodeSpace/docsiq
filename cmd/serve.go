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

		// Wave 2: no more root store. Every handler and MCP tool
		// resolves a per-project *store.Store through the shared
		// projectStores cache. The default slug is opened eagerly so
		// vector-index pre-warming and sqlite-vec loading have a
		// concrete DB to work against.
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

		// Ensure the default project is registered so doc handlers
		// targeted at _default don't 404 on a fresh install. Duplicate
		// registrations are a no-op.
		defaultSlug := cfg.DefaultProject
		if defaultSlug == "" {
			defaultSlug = "_default"
		}
		if _, getErr := registry.Get(defaultSlug); getErr != nil {
			_ = registry.Register(project.Project{
				Slug:      defaultSlug,
				Name:      defaultSlug,
				Remote:    "_default",
				CreatedAt: time.Now().Unix(),
			})
			_ = os.MkdirAll(filepath.Join(cfg.DataDir, "projects", defaultSlug), 0o755)
		}

		stores := api.NewProjectStores(cfg.DataDir)
		defer func() {
			if err := stores.Close(); err != nil {
				slog.Warn("⚠️ project stores shutdown error", "err", err)
			}
		}()

		// Eagerly open the default store so sqlite-vec extension
		// loading has a concrete DB handle to target. Other projects
		// open lazily when they first receive a request.
		defaultStore, err := stores.ForProject(defaultSlug)
		if err != nil {
			return fmt.Errorf("open default store: %w", err)
		}
		slog.Info("📂 default store opened", "path", cfg.ProjectDBPath(defaultSlug))

		// Attempt to load the embedded sqlite-vec extension. On any
		// failure (placeholder build, unsupported GOOS/GOARCH, dlopen
		// refused) we log WARN and continue — the HNSW in-memory index
		// and/or brute-force fallback still answer vector queries.
		if soPath, extErr := sqlitevec.Extract(cfg.DataDir); extErr != nil {
			slog.Warn("⚠️ sqlite-vec unavailable; using HNSW / brute-force fallback", "err", extErr)
			setVecMode(vecModeNone, extErr.Error())
		} else if loadErr := sqlitevec.LoadInto(defaultStore.DB(), soPath); loadErr != nil {
			slog.Warn("⚠️ sqlite-vec load failed; using HNSW / brute-force fallback", "err", loadErr, "path", soPath)
			setVecMode(vecModeNone, loadErr.Error())
		} else {
			slog.Info("🧮 sqlite-vec loaded", "path", soPath)
			setVecMode(vecModeSqliteVec, soPath)
		}

		prov, err := llm.NewProvider(&cfg.LLM)
		if err != nil {
			return fmt.Errorf("llm provider: %w", err)
		}
		slog.Info("⚙️ LLM provider initialised", "provider", prov.Name(), "model", prov.ModelID())

		emb := embedder.New(prov, cfg.Indexing.BatchSize)

		// Per-project vector index cache. Eagerly build one index per
		// registered project so the first search doesn't pay the build
		// cost. Unknown projects (registered later at runtime) build
		// lazily inside api.VectorIndexes.ForProject.
		vecIndexes := api.NewVectorIndexes()
		projs, listErr := registry.List()
		if listErr != nil {
			slog.Warn("⚠️ registry list failed; skipping eager index build", "err", listErr)
		} else {
			for _, p := range projs {
				st, err := stores.ForProject(p.Slug)
				if err != nil {
					slog.Warn("⚠️ skipping index for project", "slug", p.Slug, "err", err)
					continue
				}
				buildCtx, cancelBuild := context.WithTimeout(context.Background(), 60*time.Second)
				idx, err := vectorindex.BuildFromStore(buildCtx, st)
				cancelBuild()
				if err != nil {
					slog.Warn("⚠️ vector index build failed", "slug", p.Slug, "err", err)
					continue
				}
				vecIndexes.Set(p.Slug, idx)
				slog.Info("🧭 vector index ready", "slug", p.Slug, "size", idx.Size())
			}
		}

		router := api.NewRouter(prov, emb, cfg, registry,
			api.WithProjectStores(stores),
			api.WithVectorIndexes(vecIndexes),
		)

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
