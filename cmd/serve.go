package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/RandomCodeSpace/docscontext/internal/api"
	"github.com/RandomCodeSpace/docscontext/internal/embedder"
	"github.com/RandomCodeSpace/docscontext/internal/llm"
	"github.com/RandomCodeSpace/docscontext/internal/store"
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

		prov, err := llm.NewProvider(&cfg.LLM)
		if err != nil {
			return fmt.Errorf("llm provider: %w", err)
		}
		slog.Info("⚙️ LLM provider initialised", "provider", prov.Name(), "model", prov.ModelID())

		emb := embedder.New(prov, cfg.Indexing.BatchSize)
		router := api.NewRouter(st, prov, emb, cfg)

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

