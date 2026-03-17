package cmd

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/RandomCodeSpace/docsgraphcontext/internal/api"
	"github.com/RandomCodeSpace/docsgraphcontext/internal/embedder"
	"github.com/RandomCodeSpace/docsgraphcontext/internal/llm"
	"github.com/RandomCodeSpace/docsgraphcontext/internal/store"
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
		// Override config with CLI flags
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

		prov, err := llm.NewProvider(&cfg.LLM)
		if err != nil {
			return fmt.Errorf("llm provider: %w", err)
		}

		emb := embedder.New(prov, cfg.Indexing.BatchSize)
		router := api.NewRouter(st, prov, emb, cfg)

		addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			return fmt.Errorf("listen: %w", err)
		}

		srv := &http.Server{Handler: router, ReadTimeout: 60 * time.Second, WriteTimeout: 120 * time.Second}

		fmt.Fprintf(os.Stderr, "DocsGraphContext server running on http://%s\n", addr)
		fmt.Fprintf(os.Stderr, "  Web UI:  http://%s/\n", addr)
		fmt.Fprintf(os.Stderr, "  MCP:     http://%s/mcp\n", addr)
		fmt.Fprintf(os.Stderr, "  API:     http://%s/api/\n", addr)
		fmt.Fprintf(os.Stderr, "  LLM:     %s (%s)\n", prov.Name(), prov.ModelID())

		// Graceful shutdown
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		go func() {
			if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
				fmt.Fprintln(os.Stderr, "server error:", err)
			}
		}()

		<-ctx.Done()
		fmt.Fprintln(os.Stderr, "\nShutting down...")
		shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutCtx)
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Flags().StringVar(&serveHost, "host", "", "Server host (overrides config)")
	serveCmd.Flags().IntVar(&servePort, "port", 0, "Server port (overrides config)")
}
