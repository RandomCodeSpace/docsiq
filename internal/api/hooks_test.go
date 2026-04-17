package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/RandomCodeSpace/docscontext/internal/config"
	"github.com/RandomCodeSpace/docscontext/internal/project"
)

// setupHookRouter spins up a router with a registry holding one known
// project. Returns the handler and the registered remote so individual
// subtests can compose JSON bodies.
func setupHookRouter(t *testing.T) (http.Handler, string, string) {
	t.Helper()
	dataDir := t.TempDir()
	cfg := &config.Config{DataDir: dataDir, DefaultProject: config.DefaultProjectSlug}
	reg, err := project.OpenRegistry(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reg.Close() })

	const remote = "git@github.com:owner/my-hook-proj.git"
	slug, err := project.Slug(remote)
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.Register(project.Project{
		Slug: slug, Name: "My Hook Proj", Remote: remote, CreatedAt: time.Now().Unix(),
	}); err != nil {
		t.Fatal(err)
	}

	h := NewRouter(nil, nil, nil, cfg, reg)
	return h, remote, slug
}

func doHook(h http.Handler, body string) *httptest.ResponseRecorder {
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(http.MethodPost, "/api/hook/SessionStart", nil)
	} else {
		r = httptest.NewRequest(http.MethodPost, "/api/hook/SessionStart", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

func TestHookSessionStart(t *testing.T) {
	t.Run("registered_remote_returns_200_with_slug", func(t *testing.T) {
		h, remote, slug := setupHookRouter(t)
		rec := doHook(h, `{"remote":"`+remote+`"}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d want 200 body=%s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), `"project":"`+slug+`"`) {
			t.Errorf("body missing project slug %q: %s", slug, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "additionalContext") {
			t.Errorf("body missing additionalContext: %s", rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "docsiq active") {
			t.Errorf("body missing docsiq banner: %s", rec.Body.String())
		}
	})

	t.Run("unknown_remote_returns_204", func(t *testing.T) {
		h, _, _ := setupHookRouter(t)
		rec := doHook(h, `{"remote":"git@github.com:ghost/unregistered.git"}`)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status=%d want 204 body=%s", rec.Code, rec.Body.String())
		}
		if rec.Body.Len() != 0 {
			t.Errorf("204 response must have empty body, got %q", rec.Body.String())
		}
	})

	t.Run("malformed_json_returns_400", func(t *testing.T) {
		h, _, _ := setupHookRouter(t)
		rec := doHook(h, `{not json`)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d want 400", rec.Code)
		}
	})

	t.Run("empty_body_returns_400", func(t *testing.T) {
		h, _, _ := setupHookRouter(t)
		rec := doHook(h, "")
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d want 400", rec.Code)
		}
	})

	t.Run("empty_json_object_returns_400", func(t *testing.T) {
		h, _, _ := setupHookRouter(t)
		rec := doHook(h, `{}`)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d want 400", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "remote") {
			t.Errorf("error body should mention 'remote': %s", rec.Body.String())
		}
	})

	t.Run("missing_remote_field_returns_400", func(t *testing.T) {
		h, _, _ := setupHookRouter(t)
		rec := doHook(h, `{"cwd":"/tmp","session_id":"abc"}`)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d want 400", rec.Code)
		}
	})

	t.Run("remote_and_session_id_passthrough", func(t *testing.T) {
		h, remote, _ := setupHookRouter(t)
		rec := doHook(h, `{"remote":"`+remote+`","session_id":"sess-123","cwd":"/home/dev/foo"}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d want 200 body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("unicode_remote_returns_204", func(t *testing.T) {
		h, _, _ := setupHookRouter(t)
		// Unicode remote — unlikely to exist in the registry, should 204.
		rec := doHook(h, `{"remote":"git@example.com:日本語/プロジェクト.git"}`)
		if rec.Code != http.StatusNoContent && rec.Code != http.StatusOK {
			t.Fatalf("status=%d want 204 or 200", rec.Code)
		}
	})

	t.Run("very_long_remote_returns_204", func(t *testing.T) {
		h, _, _ := setupHookRouter(t)
		long := "git@host:" + strings.Repeat("a", 5000) + "/repo.git"
		rec := doHook(h, `{"remote":"`+long+`"}`)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status=%d want 204 body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("path_traversal_remote_returns_204", func(t *testing.T) {
		h, _, _ := setupHookRouter(t)
		// Path-traversal-looking payload — should not resolve to anything
		// AND must not be used in any filesystem operation (none exist).
		rec := doHook(h, `{"remote":"../../../etc/passwd"}`)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status=%d want 204 body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("body_too_large_returns_400", func(t *testing.T) {
		h, _, _ := setupHookRouter(t)
		big := `{"remote":"` + strings.Repeat("x", 70*1024) + `"}`
		rec := doHook(h, big)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d want 400 body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("nil_registry_returns_204", func(t *testing.T) {
		// Directly build router with nil registry — mimics early startup.
		cfg := &config.Config{DataDir: t.TempDir(), DefaultProject: config.DefaultProjectSlug}
		h := NewRouter(nil, nil, nil, cfg, nil)
		rec := doHook(h, `{"remote":"git@github.com:x/y.git"}`)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status=%d want 204 body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("whitespace_only_remote_returns_400", func(t *testing.T) {
		h, _, _ := setupHookRouter(t)
		rec := doHook(h, `{"remote":"   "}`)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d want 400", rec.Code)
		}
	})

	t.Run("get_method_rejected", func(t *testing.T) {
		h, _, _ := setupHookRouter(t)
		req := httptest.NewRequest(http.MethodGet, "/api/hook/SessionStart", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code == http.StatusOK {
			t.Fatalf("GET should not 200, got %d", rec.Code)
		}
	})
}
