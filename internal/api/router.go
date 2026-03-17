package api

import (
	"net/http"

	"github.com/RandomCodeSpace/docsgraphcontext/internal/config"
	"github.com/RandomCodeSpace/docsgraphcontext/internal/embedder"
	"github.com/RandomCodeSpace/docsgraphcontext/internal/llm"
	"github.com/RandomCodeSpace/docsgraphcontext/internal/mcp"
	"github.com/RandomCodeSpace/docsgraphcontext/internal/store"
	"github.com/RandomCodeSpace/docsgraphcontext/ui"
)

// NewRouter builds the single http.ServeMux with all routes.
func NewRouter(st *store.Store, prov llm.Provider, emb *embedder.Embedder, cfg *config.Config) http.Handler {
	mcpServer := mcp.New(st, prov, emb, cfg)
	h := &handlers{store: st, provider: prov, embedder: emb, cfg: cfg}

	mux := http.NewServeMux()

	// MCP Streamable HTTP transport (POST /mcp, GET /mcp for SSE stream)
	mux.Handle("/mcp", mcpServer.Handler())

	// REST API
	mux.HandleFunc("GET /api/stats", h.getStats)
	mux.HandleFunc("GET /api/documents", h.listDocuments)
	mux.HandleFunc("GET /api/documents/{id}", h.getDocument)
	mux.HandleFunc("GET /api/documents/{id}/versions", h.getDocumentVersions)
	mux.HandleFunc("POST /api/search", h.search)
	mux.HandleFunc("GET /api/graph/neighborhood", h.graphNeighborhood)
	mux.HandleFunc("GET /api/entities", h.listEntities)
	mux.HandleFunc("GET /api/communities", h.listCommunities)
	mux.HandleFunc("GET /api/communities/{id}", h.getCommunity)
	mux.HandleFunc("POST /api/upload", h.upload)
	mux.HandleFunc("GET /api/upload/progress", h.uploadProgress)

	// Embedded UI
	mux.Handle("/", http.FileServer(http.FS(ui.Assets)))

	return mux
}
