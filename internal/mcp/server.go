package mcp

import (
	"fmt"
	"net/http"

	"github.com/RandomCodeSpace/docsiq/internal/config"
	"github.com/RandomCodeSpace/docsiq/internal/embedder"
	"github.com/RandomCodeSpace/docsiq/internal/llm"
	"github.com/RandomCodeSpace/docsiq/internal/project"
	"github.com/RandomCodeSpace/docsiq/internal/store"
	"github.com/RandomCodeSpace/docsiq/internal/vectorindex"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Storer is the narrow contract the MCP server uses to obtain a
// per-project *store.Store on every tool invocation. It mirrors the
// REST-side api.Storer so the same cache can be injected into both.
type Storer interface {
	ForProject(slug string) (*store.Store, error)
}

// VectorIndexResolver returns the per-project HNSW index (or nil if
// unavailable / brute-force). Implementations must be safe for
// concurrent use. *api.VectorIndexes satisfies this — but the mcp
// package can't import api, so the resolver is declared as an
// interface here.
type VectorIndexResolver interface {
	ForProject(slug string, st *store.Store) vectorindex.Index
}

// Option configures an MCP Server at construction time.
type Option func(*Server)

// WithVectorIndexes wires a per-project HNSW index cache into the MCP
// search tools. Nil (the default) makes LocalSearch fall back to
// brute-force inside the search package.
func WithVectorIndexes(vi VectorIndexResolver) Option {
	return func(s *Server) { s.vecIndexes = vi }
}

// Server wraps the MCP server.
type Server struct {
	mcpServer  *server.MCPServer
	httpServer *server.StreamableHTTPServer
	stores     Storer
	provider   llm.Provider
	embedder   *embedder.Embedder
	cfg        *config.Config
	registry   *project.Registry
	vecIndexes VectorIndexResolver
}

// New creates and registers all docs + notes MCP tools.
//
// Wave-2 signature change: drops the long-lived *store.Store — the MCP
// layer now resolves a per-project store on every tool invocation via
// Storer.ForProject(slug). Passing nil for stores yields an error on
// any doc-tool call (notes tools still work if they only need cfg).
func New(stores Storer, prov llm.Provider, emb *embedder.Embedder, cfg *config.Config, registry *project.Registry, opts ...Option) *Server {
	s := &Server{
		stores:   stores,
		provider: prov,
		embedder: emb,
		cfg:      cfg,
		registry: registry,
	}
	for _, opt := range opts {
		opt(s)
	}
	s.mcpServer = server.NewMCPServer(
		"docsiq",
		"1.0.0",
		server.WithToolCapabilities(true),
	)
	registerTools(s)
	registerNotesTools(s)
	s.httpServer = server.NewStreamableHTTPServer(s.mcpServer)
	return s
}

// storeForProject resolves a per-project *store.Store. An empty slug
// resolves to config.DefaultProjectSlug per the Wave-2 policy: absent
// `project` argument on any doc MCP tool defaults to the root project.
func (s *Server) storeForProject(slug string) (*store.Store, error) {
	if slug == "" {
		slug = config.DefaultProjectSlug
	}
	if s.stores == nil {
		return nil, fmt.Errorf("mcp server: no store resolver configured")
	}
	return s.stores.ForProject(slug)
}

// projectArg is the standard "project" string argument used by every
// doc MCP tool. Empty → "_default" per Wave-2 policy.
func projectArg(args map[string]any) string {
	if v, ok := args["project"]; ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return config.DefaultProjectSlug
}

// Close is a no-op after Wave-2 — the per-project store cache is owned
// by the caller (typically cmd/serve), not by the MCP server. Retained
// for symmetry with the REST server shutdown path and to keep existing
// test teardown hooks (`_ = s.Close()`) compiling.
func (s *Server) Close() error { return nil }

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
