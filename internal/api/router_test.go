package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/RandomCodeSpace/docsiq/internal/config"
	"github.com/RandomCodeSpace/docsiq/internal/store"
)

// newTestRouter builds a router with a real store and config and nil
// provider/embedder. mcp.New and NewRouter don't deref prov/emb during
// construction, so nil is safe — but endpoints that invoke the LLM would
// panic. We only hit /, /api/* paths that don't need the provider.
func newTestRouter(t *testing.T) (http.Handler, *store.Store) {
	t.Helper()

	dir := t.TempDir()
	st, err := store.OpenForProject(dir, "testproj")
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	cfg := &config.Config{}
	cfg.Server.Host = "127.0.0.1"
	cfg.Server.Port = 0
	cfg.DataDir = dir

	// Phase-1: NewRouter takes a registry. nil is tolerated — the project
	// middleware falls back to the default slug and skips auto-register.
	h := NewRouter(st, nil, nil, cfg, nil)
	if h == nil {
		t.Fatal("NewRouter returned nil handler")
	}
	return h, st
}

func TestNewRouter(t *testing.T) {
	t.Run("constructs_non_nil_handler", func(t *testing.T) {
		h, _ := newTestRouter(t)
		if h == nil {
			t.Fatal("handler is nil")
		}
	})

	t.Run("root_returns_html_200", func(t *testing.T) {
		h, _ := newTestRouter(t)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", rec.Code)
		}
		ct := rec.Header().Get("Content-Type")
		if !strings.Contains(strings.ToLower(ct), "text/html") {
			t.Errorf("Content-Type = %q, want text/html*", ct)
		}
		if rec.Body.Len() == 0 {
			t.Error("empty body for GET /")
		}
	})

	t.Run("unknown_api_path_returns_404", func(t *testing.T) {
		h, _ := newTestRouter(t)

		req := httptest.NewRequest(http.MethodGet, "/api/nonexistent", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("GET /api/nonexistent status = %d, want 404", rec.Code)
		}
	})

	t.Run("panic_recovery_middleware_returns_500", func(t *testing.T) {
		// Wrap a handler that always panics with the exported recovery
		// middleware to assert it converts panics into 500s. No production
		// code is modified; this exercises recoveryMiddleware directly.
		panicker := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("boom")
		})
		h := recoveryMiddleware(panicker)

		req := httptest.NewRequest(http.MethodGet, "/whatever", nil)
		rec := httptest.NewRecorder()

		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("recoveryMiddleware leaked panic: %v", r)
			}
		}()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", rec.Code)
		}
	})

	t.Run("builds_with_api_key_set", func(t *testing.T) {
		// Full-stack integration: NewRouter must build successfully when
		// cfg.Server.APIKey is non-empty, and a protected route must 401
		// when no Authorization header is supplied.
		dir := t.TempDir()
		st, err := store.OpenForProject(dir, "testproj")
		if err != nil {
			t.Fatalf("store.Open: %v", err)
		}
		t.Cleanup(func() { _ = st.Close() })

		cfg := &config.Config{}
		cfg.Server.Host = "127.0.0.1"
		cfg.Server.Port = 0
		cfg.Server.APIKey = "test-secret"
		cfg.DataDir = dir

		h := NewRouter(st, nil, nil, cfg, nil)
		if h == nil {
			t.Fatal("NewRouter returned nil with APIKey set")
		}

		req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("GET /api/stats with no auth status=%d want 401", rec.Code)
		}

		// /health remains public even with APIKey set.
		req2 := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec2 := httptest.NewRecorder()
		h.ServeHTTP(rec2, req2)
		if rec2.Code != http.StatusOK {
			t.Errorf("GET /health (no auth) status=%d want 200", rec2.Code)
		}
		if !strings.Contains(rec2.Body.String(), `"status":"ok"`) {
			t.Errorf("GET /health body=%q missing status:ok", rec2.Body.String())
		}

		// With the correct key, /api/stats passes auth (the handler
		// itself may still 500 because we have no real data — that's
		// fine; we're only asserting the middleware gate lets us past
		// the 401.)
		req3 := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
		req3.Header.Set("Authorization", "Bearer test-secret")
		rec3 := httptest.NewRecorder()
		h.ServeHTTP(rec3, req3)
		if rec3.Code == http.StatusUnauthorized {
			t.Errorf("GET /api/stats with correct auth 401'd")
		}
	})

	t.Run("nasty_inputs_do_not_panic", func(t *testing.T) {
		// Nasty inputs: null byte, unicode, very long path, path-traversal
		// attempt. None of these should panic; all should yield a clean
		// HTTP response (4xx or 200 via SPA fallback).
		h, _ := newTestRouter(t)

		cases := []string{
			"/api/" + strings.Repeat("a", 8192),   // 8 KB path
			"/api/こんにちは/世界",                      // unicode
			"/api/\x00nullbyte",                   // null byte
			"/../../../../../etc/passwd",          // path traversal
			"/" + strings.Repeat("deep/", 200),    // deep nesting
		}

		for _, target := range cases {
			t.Run(strings.ReplaceAll(target, "\x00", "_NUL_"), func(t *testing.T) {
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("panic on %q: %v", target, r)
					}
				}()

				req, err := http.NewRequest(http.MethodGet, "http://test"+target, nil)
				if err != nil {
					// httptest/NewRequest panics on bad URLs; net/http.NewRequest
					// returns an error. Treat a parse failure as acceptable.
					t.Logf("URL not parseable (acceptable): %v", err)
					return
				}
				rec := httptest.NewRecorder()
				h.ServeHTTP(rec, req)

				if rec.Code < 200 || rec.Code >= 600 {
					t.Errorf("bogus status %d for %q", rec.Code, target)
				}
			})
		}
	})
}
