package mcp

import (
	"net/http"
	"sync"

	"github.com/RandomCodeSpace/docscontext/internal/config"
	"github.com/RandomCodeSpace/docscontext/internal/embedder"
	"github.com/RandomCodeSpace/docscontext/internal/llm"
	"github.com/RandomCodeSpace/docscontext/internal/project"
	"github.com/RandomCodeSpace/docscontext/internal/store"
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
	registry   *project.Registry

	// Per-project note stores; lazy-opened, closed by Close().
	storesMu   sync.Mutex
	noteStores map[string]*store.Store
}

// New creates and registers all docs + notes MCP tools.
//
// Phase-2 signature change: takes *project.Registry so notes tools can
// resolve per-project DB handles. A nil registry is tolerated; notes
// tools that need one return a clear error at call time.
func New(st *store.Store, prov llm.Provider, emb *embedder.Embedder, cfg *config.Config, registry *project.Registry) *Server {
	s := &Server{
		store:      st,
		provider:   prov,
		embedder:   emb,
		cfg:        cfg,
		registry:   registry,
		noteStores: map[string]*store.Store{},
	}
	s.mcpServer = server.NewMCPServer(
		"DocsContext",
		"1.0.0",
		server.WithToolCapabilities(true),
	)
	registerTools(s)
	registerNotesTools(s)
	s.httpServer = server.NewStreamableHTTPServer(s.mcpServer)
	return s
}

// storeForProject opens (and caches) a per-project *store.Store.
func (s *Server) storeForProject(slug string) (*store.Store, error) {
	s.storesMu.Lock()
	defer s.storesMu.Unlock()
	if st, ok := s.noteStores[slug]; ok {
		return st, nil
	}
	st, err := store.OpenForProject(s.cfg.DataDir, slug)
	if err != nil {
		return nil, err
	}
	s.noteStores[slug] = st
	return st, nil
}

// Close releases all per-project note store handles.
func (s *Server) Close() error {
	s.storesMu.Lock()
	defer s.storesMu.Unlock()
	for k, st := range s.noteStores {
		_ = st.Close()
		delete(s.noteStores, k)
	}
	return nil
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



