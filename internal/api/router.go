package api

import (
	"bytes"
	"context"
	"html"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/RandomCodeSpace/docsiq/internal/config"
	"github.com/RandomCodeSpace/docsiq/internal/embedder"
	"github.com/RandomCodeSpace/docsiq/internal/llm"
	"github.com/RandomCodeSpace/docsiq/internal/mcp"
	"github.com/RandomCodeSpace/docsiq/internal/project"
	"github.com/RandomCodeSpace/docsiq/ui"
)

// RouterOption configures NewRouter. Zero-or-more options are appended to the
// existing positional arguments without breaking any existing call site.
type RouterOption func(*routerOptions)

type routerOptions struct {
	vecIndexes *VectorIndexes
	stores     *projectStores
}

// WithVectorIndexes wires a per-project HNSW index cache into the
// search handlers and MCP server. Nil (default) makes LocalSearch fall
// back to brute-force per request.
func WithVectorIndexes(vi *VectorIndexes) RouterOption {
	return func(o *routerOptions) { o.vecIndexes = vi }
}

// WithProjectStores lets callers inject a pre-built ProjectStores
// cache so they can close it at shutdown. Nil (default) causes
// NewRouter to allocate its own — fine for tests, but real servers
// should supply one for controlled teardown.
func WithProjectStores(p *ProjectStores) RouterOption {
	return func(o *routerOptions) {
		if p != nil {
			o.stores = p.inner()
		}
	}
}

// NewRouter builds the single http.ServeMux with all routes.
//
// Wave-2 signature change: the long-lived *store.Store positional
// argument is gone. Handlers resolve per-project stores via a shared
// Storer (the projectStores cache). Callers that want lifecycle control
// over that cache can inject it with WithProjectStores; otherwise one
// is created internally (leaked for process lifetime — fine for tests).
func NewRouter(prov llm.Provider, emb *embedder.Embedder, cfg *config.Config, registry *project.Registry, opts ...RouterOption) http.Handler {
	ro := &routerOptions{}
	for _, opt := range opts {
		opt(ro)
	}
	stores := ro.stores
	if stores == nil {
		stores = newProjectStores(cfg.DataDir)
	}

	mcpServer := mcp.New(stores, prov, emb, cfg, registry, mcp.WithVectorIndexes(ro.vecIndexes))
	h := &handlers{
		stores:     stores,
		provider:   prov,
		embedder:   emb,
		cfg:        cfg,
		vecIndexes: ro.vecIndexes,
	}
	nh := newNotesHandlersWithStores(stores, cfg, registry)
	ph := &projectsHandler{registry: registry}

	mux := http.NewServeMux()

	// Public liveness probe — registered on the mux itself. The auth
	// middleware also explicitly bypasses /health as defense-in-depth.
	mux.HandleFunc("GET /health", h.health)

	// Prometheus scrape endpoint — public, NOT gated by auth or project
	// middleware (auth/project explicitly bypass /metrics below).
	// TODO(docsiq): P2-2 consider optional scrape token via cfg.Server.MetricsKey
	mux.Handle("GET /metrics", metricsHandler(registry, stores, cfg))

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
	mux.HandleFunc("GET /api/entities/{id}/claims", h.claimsForEntity)
	mux.HandleFunc("GET /api/claims", h.listClaims)
	mux.HandleFunc("POST /api/upload", h.upload)
	mux.HandleFunc("GET /api/upload/progress", h.uploadProgress)

	// REST API — project registry (Phase-4). Thin shim for UI dropdown.
	mux.HandleFunc("GET /api/projects", ph.listProjects)

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
	mux.Handle("/", spaHandler(ui.Assets, cfg))

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

func spaHandler(assets fs.FS, cfg *config.Config) http.Handler {
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

		if cleanPath != "index.html" && strings.Contains(path.Base(cleanPath), ".") {
			fileServer.ServeHTTP(w, r)
			return
		}

		content, err := fs.ReadFile(assets, "index.html")
		if err != nil {
			http.Error(w, "index.html not found", http.StatusInternalServerError)
			return
		}

		if cfg.Server.APIKey != "" {
			content = bytes.Replace(
				content,
				[]byte("</head>"),
				[]byte(`<meta name="docsiq-api-key" content="`+html.EscapeString(cfg.Server.APIKey)+`"></head>`),
				1,
			)
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

// loggingMiddleware logs method, path, status code, and duration for every
// request, assigns a request ID (X-Request-ID passthrough or new hex), and
// feeds the Prometheus collector.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Request ID: header pass-through, otherwise generate fresh 16-hex
		// (8 random bytes). Put on ctx + echo back as response header.
		rid := strings.TrimSpace(r.Header.Get("X-Request-ID"))
		if rid == "" {
			rid = newRequestID()
		}
		ctx := context.WithValue(r.Context(), ctxRequestIDKey{}, rid)
		r = r.WithContext(ctx)
		w.Header().Set("X-Request-ID", rid)

		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		duration := time.Since(start)

		// /metrics itself is noisy and self-referential — skip recording it
		// as an observed request so a tight Prometheus scrape loop doesn't
		// dominate the time series.
		if r.URL.Path != "/metrics" {
			recordRequest(r.Method, r.URL.Path, rw.status, duration.Seconds())
		}

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
			"request_id", rid,
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
