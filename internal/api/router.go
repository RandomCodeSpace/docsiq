package api

import (
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/RandomCodeSpace/docscontext/internal/config"
	"github.com/RandomCodeSpace/docscontext/internal/embedder"
	"github.com/RandomCodeSpace/docscontext/internal/llm"
	"github.com/RandomCodeSpace/docscontext/internal/mcp"
	"github.com/RandomCodeSpace/docscontext/internal/project"
	"github.com/RandomCodeSpace/docscontext/internal/store"
	"github.com/RandomCodeSpace/docscontext/ui"
)

// NewRouter builds the single http.ServeMux with all routes.
//
// Phase-1 signature change: takes a *project.Registry so the project
// middleware can resolve ?project= / X-Project on every /api/* and /mcp
// request. Passing a nil registry is tolerated (the middleware still
// attaches the default slug to the context) so tests that only exercise
// docs handlers don't need to spin up a real registry.
func NewRouter(st *store.Store, prov llm.Provider, emb *embedder.Embedder, cfg *config.Config, registry *project.Registry) http.Handler {
	mcpServer := mcp.New(st, prov, emb, cfg, registry)
	h := &handlers{store: st, provider: prov, embedder: emb, cfg: cfg}
	nh := newNotesHandlers(cfg.DataDir, cfg, registry)

	mux := http.NewServeMux()

	// Public liveness probe — registered on the mux itself. The auth
	// middleware also explicitly bypasses /health as defense-in-depth.
	mux.HandleFunc("GET /health", h.health)

	// MCP Streamable HTTP transport (POST /mcp, GET /mcp for SSE stream)
	mux.Handle("/mcp", mcpServer.Handler())

	// REST API — docs pipeline (Phase-0)
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

	// REST API — notes (Phase-2). Every endpoint takes a project slug
	// in the path. The project middleware still runs and resolves
	// ?project= / X-Project; these handlers prefer the path value but
	// fall back to ProjectFromContext when it is empty.
	mux.HandleFunc("GET /api/projects/{project}/notes", nh.listNotes)
	mux.HandleFunc("GET /api/projects/{project}/notes/{key...}", nh.readNote)
	mux.HandleFunc("PUT /api/projects/{project}/notes/{key...}", nh.writeNote)
	mux.HandleFunc("DELETE /api/projects/{project}/notes/{key...}", nh.deleteNote)
	mux.HandleFunc("GET /api/projects/{project}/tree", nh.tree)
	mux.HandleFunc("GET /api/projects/{project}/search", nh.searchNotes)
	mux.HandleFunc("GET /api/projects/{project}/graph", nh.graph)
	mux.HandleFunc("GET /api/projects/{project}/export", nh.export)
	mux.HandleFunc("POST /api/projects/{project}/import", nh.importTar)

	// REST API — hooks (Phase-3). SessionStart is the only handler; it
	// resolves a git remote to a registered project slug and returns an
	// "additionalContext" blob the AI client can inject into its prompt.
	registerHookRoutes(mux, registry)

	// Embedded UI
	mux.Handle("/", spaHandler(ui.Assets))

	// Middleware ordering (outermost → innermost):
	//   logging → recovery → auth → project → mux
	// project scope sits BELOW auth (an unauthenticated caller never
	// reaches the registry) and ABOVE the mux (so handlers and the MCP
	// server see the resolved slug via ProjectFromContext).
	return loggingMiddleware(
		recoveryMiddleware(
			bearerAuthMiddleware(cfg.Server.APIKey,
				projectMiddleware(cfg, registry, mux))))
}

func spaHandler(assets fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(assets))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/mcp") {
			http.NotFound(w, r)
			return
		}

		cleanPath := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if cleanPath == "." || cleanPath == "" {
			cleanPath = "index.html"
		}

		if strings.Contains(path.Base(cleanPath), ".") {
			fileServer.ServeHTTP(w, r)
			return
		}

		content, err := fs.ReadFile(assets, "index.html")
		if err != nil {
			http.Error(w, "index.html not found", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(content)
	})
}

// recoveryMiddleware catches panics in handlers and returns a 500 response.
func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("❌ panic recovered", "path", r.URL.Path, "panic", rec)
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// loggingMiddleware logs method, path, status code, and duration for every request.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		duration := time.Since(start)

		level := slog.LevelInfo
		if rw.status >= 500 {
			level = slog.LevelError
		} else if rw.status >= 400 {
			level = slog.LevelWarn
		}

		slog.Log(r.Context(), level, "http",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.status,
			"duration_ms", duration.Milliseconds(),
		)
	})
}

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}
