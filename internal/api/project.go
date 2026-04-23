package api

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/RandomCodeSpace/docsiq/internal/config"
	"github.com/RandomCodeSpace/docsiq/internal/project"
)

// ctxProjectKey is the typed context key under which the resolved project
// slug lives. Using a named struct type (not a string) keeps the key
// collision-free across packages.
type ctxProjectKey struct{}

// ProjectFromContext returns the project slug stored on the request
// context. If no middleware ran (ctx is plain), returns the empty string.
// Handlers should treat empty as "fall back to cfg.DefaultProject" for
// defensive robustness.
func ProjectFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ctxProjectKey{}).(string); ok {
		return v
	}
	return ""
}

// projectMiddleware resolves the per-request project scope.
//
// Resolution precedence:
//  1. ?project=<slug> query parameter
//  2. X-Project: <slug> request header
//  3. cfg.DefaultProject
//
// Behavior:
//   - Invalid slug (charset violation) → 400 "invalid project slug".
//   - Unknown slug that matches cfg.DefaultProject → auto-register on the
//     fly with remote="_default" so first-run users get a working scope
//     without running `docsiq init`.
//   - Unknown slug that does NOT match cfg.DefaultProject → 404.
//
// Phase-1 explicit scope limit: existing handlers keep using the single
// *store.Store passed to NewRouter. This middleware only attaches the
// slug to the context — Phase-2 notes handlers are the first consumers.
func projectMiddleware(cfg *config.Config, registry *project.Registry, next http.Handler) http.Handler {
	// Defensive fallback: if a caller passed an empty default, don't wedge
	// the whole server — substitute the locked constant.
	defaultSlug := cfg.DefaultProject
	if defaultSlug == "" {
		defaultSlug = config.DefaultProjectSlug
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only scope /api/* and /mcp. Static UI assets, "/" and /health
		// do not need a project context and must not 400/404 on an
		// invalid slug that was never intended for them.
		path := r.URL.Path
		if !strings.HasPrefix(path, "/api/") && !strings.HasPrefix(path, "/mcp") {
			next.ServeHTTP(w, r)
			return
		}

		slug := r.URL.Query().Get("project")
		if slug == "" {
			slug = r.Header.Get("X-Project")
		}
		if slug == "" {
			slug = defaultSlug
		}

		if !project.IsValidSlug(slug) {
			http.Error(w, "invalid project slug", http.StatusBadRequest)
			return
		}
		// IsValidSlug already rejects anything path-dangerous (enforces
		// `^[a-z0-9_-]+$`); filepath.IsLocal is a CodeQL-recognised
		// path-injection sanitiser — belt-and-braces for static analysis.
		if !filepath.IsLocal(slug) {
			http.Error(w, "invalid project slug", http.StatusBadRequest)
			return
		}

		// Lookup. The registry may be nil in tests that bypass serveCmd —
		// treat nil as "skip registration, just attach the slug."
		if registry != nil {
			_, err := registry.Get(slug)
			switch err {
			case nil:
				// registered — proceed
			case project.ErrNotFound:
				if slug == defaultSlug {
					if regErr := registry.Register(project.Project{
						Slug:      slug,
						Name:      slug,
						Remote:    "_default",
						CreatedAt: time.Now().Unix(),
					}); regErr != nil && regErr != project.ErrDuplicateRemote {
						// A concurrent request may have registered the default
						// slug under us — that's fine. Any other error is
						// logged and we still proceed (defensive: a read-only
						// registry shouldn't 500 every request).
						slog.Warn("⚠️ default project auto-register failed",
							"slug", slug, "err", regErr)
					}
					// Also ensure the per-project dir exists so a note/write
					// handler in Phase 2 doesn't trip on a missing path.
					projectDir := filepath.Join(cfg.DataDir, "projects", slug)
					if mkErr := os.MkdirAll(projectDir, 0o755); mkErr != nil {
						slog.Warn("⚠️ default project dir mkdir failed",
							"path", projectDir, "err", mkErr)
					}
				} else {
					http.Error(w, "unknown project: "+slug, http.StatusNotFound)
					return
				}
			default:
				slog.Error("❌ project registry lookup failed", "slug", slug, "err", err)
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
		}

		ctx := context.WithValue(r.Context(), ctxProjectKey{}, slug)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
