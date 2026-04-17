package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/RandomCodeSpace/docscontext/internal/config"
	"github.com/RandomCodeSpace/docscontext/internal/project"
)

func newTestRegistry(t *testing.T) *project.Registry {
	t.Helper()
	r, err := project.OpenRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("OpenRegistry: %v", err)
	}
	t.Cleanup(func() { _ = r.Close() })
	return r
}

func TestProjectMiddleware_DefaultFallback(t *testing.T) {
	cfg := &config.Config{DataDir: t.TempDir(), DefaultProject: config.DefaultProjectSlug}
	reg := newTestRegistry(t)

	var gotSlug string
	h := projectMiddleware(cfg, reg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSlug = ProjectFromContext(r.Context())
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if gotSlug != config.DefaultProjectSlug {
		t.Fatalf("slug = %q, want %q", gotSlug, config.DefaultProjectSlug)
	}
	// Auto-registered.
	if _, err := reg.Get(config.DefaultProjectSlug); err != nil {
		t.Fatalf("default slug not auto-registered: %v", err)
	}
}

func TestProjectMiddleware_QueryParam(t *testing.T) {
	cfg := &config.Config{DataDir: t.TempDir(), DefaultProject: config.DefaultProjectSlug}
	reg := newTestRegistry(t)
	if err := reg.Register(project.Project{Slug: "foo", Name: "foo", Remote: "r-foo"}); err != nil {
		t.Fatal(err)
	}

	var gotSlug string
	h := projectMiddleware(cfg, reg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSlug = ProjectFromContext(r.Context())
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/stats?project=foo", nil)
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if gotSlug != "foo" {
		t.Fatalf("slug = %q, want foo", gotSlug)
	}
}

func TestProjectMiddleware_XProjectHeader(t *testing.T) {
	cfg := &config.Config{DataDir: t.TempDir(), DefaultProject: config.DefaultProjectSlug}
	reg := newTestRegistry(t)
	if err := reg.Register(project.Project{Slug: "bar", Name: "bar", Remote: "r-bar"}); err != nil {
		t.Fatal(err)
	}

	var gotSlug string
	h := projectMiddleware(cfg, reg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSlug = ProjectFromContext(r.Context())
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	req.Header.Set("X-Project", "bar")
	h.ServeHTTP(rec, req)

	if gotSlug != "bar" {
		t.Fatalf("slug = %q, want bar", gotSlug)
	}
}

func TestProjectMiddleware_QueryBeatsHeader(t *testing.T) {
	cfg := &config.Config{DataDir: t.TempDir(), DefaultProject: config.DefaultProjectSlug}
	reg := newTestRegistry(t)
	if err := reg.Register(project.Project{Slug: "aa", Name: "aa", Remote: "ra"}); err != nil {
		t.Fatal(err)
	}
	if err := reg.Register(project.Project{Slug: "bb", Name: "bb", Remote: "rb"}); err != nil {
		t.Fatal(err)
	}

	var gotSlug string
	h := projectMiddleware(cfg, reg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSlug = ProjectFromContext(r.Context())
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/stats?project=aa", nil)
	req.Header.Set("X-Project", "bb")
	h.ServeHTTP(rec, req)

	if gotSlug != "aa" {
		t.Fatalf("slug = %q, want aa (query beats header)", gotSlug)
	}
}

func TestProjectMiddleware_InvalidSlug(t *testing.T) {
	cfg := &config.Config{DataDir: t.TempDir(), DefaultProject: config.DefaultProjectSlug}
	reg := newTestRegistry(t)
	h := projectMiddleware(cfg, reg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler must not run for invalid slug")
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/stats?project=NOT/VALID", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestProjectMiddleware_UnknownSlug(t *testing.T) {
	cfg := &config.Config{DataDir: t.TempDir(), DefaultProject: config.DefaultProjectSlug}
	reg := newTestRegistry(t)
	h := projectMiddleware(cfg, reg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler must not run for unknown non-default slug")
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/stats?project=unregistered", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestProjectFromContext_EmptyDefault(t *testing.T) {
	// A raw context (no middleware) returns empty string.
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	if got := ProjectFromContext(req.Context()); got != "" {
		t.Fatalf("ProjectFromContext = %q, want empty", got)
	}
}

func TestProjectMiddleware_NonAPIPassthrough(t *testing.T) {
	// Static UI and /health must bypass slug resolution entirely.
	cfg := &config.Config{DataDir: t.TempDir(), DefaultProject: config.DefaultProjectSlug}
	reg := newTestRegistry(t)

	called := false
	h := projectMiddleware(cfg, reg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		// No project context expected on pass-through.
		if got := ProjectFromContext(r.Context()); got != "" {
			t.Errorf("context slug = %q on non-api path, want empty", got)
		}
	}))

	for _, path := range []string{"/", "/index.html", "/health", "/assets/app.js"} {
		called = false
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path+"?project=NOT/VALID", nil)
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("path %s status = %d, want 200 (bypass)", path, rec.Code)
		}
		if !called {
			t.Errorf("path %s: handler not called", path)
		}
	}
}

func TestProjectMiddleware_NilRegistry(t *testing.T) {
	// Nil registry is tolerated — slug still lands on context.
	cfg := &config.Config{DataDir: t.TempDir(), DefaultProject: config.DefaultProjectSlug}
	var gotSlug string
	h := projectMiddleware(cfg, nil, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSlug = ProjectFromContext(r.Context())
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/stats?project=anything", nil)
	h.ServeHTTP(rec, req)
	if gotSlug != "anything" {
		t.Fatalf("slug = %q, want anything", gotSlug)
	}
}
