package mcp

import (
	"net/http"

	"github.com/RandomCodeSpace/docsgraphcontext/internal/config"
	"github.com/RandomCodeSpace/docsgraphcontext/internal/embedder"
	"github.com/RandomCodeSpace/docsgraphcontext/internal/llm"
	"github.com/RandomCodeSpace/docsgraphcontext/internal/store"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Server wraps the MCP server.
type Server struct {
	mcpServer  *server.MCPServer
	httpServer *server.StreamableHTTPServer
	store      *store.Store
	provider   llm.Provider
	embedder   *embedder.Embedder
	cfg        *config.Config
}

// New creates and registers all 12 MCP tools.
func New(st *store.Store, prov llm.Provider, emb *embedder.Embedder, cfg *config.Config) *Server {
	s := &Server{
		store:    st,
		provider: prov,
		embedder: emb,
		cfg:      cfg,
	}
	s.mcpServer = server.NewMCPServer(
		"docsgraphcontext",
		"1.0.0",
		server.WithToolCapabilities(true),
	)
	registerTools(s)
	s.httpServer = server.NewStreamableHTTPServer(s.mcpServer)
	return s
}

// Handler returns an http.Handler for the Streamable HTTP MCP transport.
func (s *Server) Handler() http.Handler {
	return s.httpServer
}

func toolError(err error) *mcpgo.CallToolResult {
	return &mcpgo.CallToolResult{
		IsError: true,
		Content: []mcpgo.Content{mcpgo.NewTextContent(err.Error())},
	}
}

func toolText(text string) *mcpgo.CallToolResult {
	return mcpgo.NewToolResultText(text)
}

func intArg(args map[string]any, key string, def int) int {
	if v, ok := args[key]; ok {
		switch n := v.(type) {
		case float64:
			return int(n)
		case int:
			return n
		}
	}
	return def
}

func stringArg(args map[string]any, key string, def string) string {
	if v, ok := args[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return def
}
