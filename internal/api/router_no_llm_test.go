package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/RandomCodeSpace/docsiq/internal/config"
	"github.com/RandomCodeSpace/docsiq/internal/store"
)

// newNoLLMRouter builds a router with nil provider and nil embedder,
// simulating provider=none. Routes that don't need LLM must work; routes
// that do must return 503.
func newNoLLMRouter(t *testing.T) (http.Handler, *store.Store) {
	t.Helper()

	dir := t.TempDir()
	st, err := store.OpenForProject(dir, "_default")
	if err != nil {
		t.Fatalf("store.OpenForProject: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	cfg := &config.Config{}
	cfg.Server.Host = "127.0.0.1"
	cfg.Server.Port = 0
	cfg.DataDir = dir

	h := NewRouter(nil, nil, cfg, nil,
		WithProjectStores(testSingleStore(dir, st, "_default", "testproj")))
	if h == nil {
		t.Fatal("NewRouter returned nil handler")
	}
	return h, st
}

func TestRouterNoLLM(t *testing.T) {
	t.Run("health_returns_200", func(t *testing.T) {
		h, _ := newNoLLMRouter(t)
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("GET /health status = %d, want 200", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), `"status":"ok"`) {
			t.Errorf("GET /health body = %q, want status:ok", rec.Body.String())
		}
	})

	t.Run("notes_list_returns_200", func(t *testing.T) {
		h, _ := newNoLLMRouter(t)
		req := httptest.NewRequest(http.MethodGet, "/api/projects/_default/notes", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("GET /api/projects/_default/notes status = %d, want 200", rec.Code)
		}
	})

	t.Run("tree_returns_non_503", func(t *testing.T) {
		h, _ := newNoLLMRouter(t)
		req := httptest.NewRequest(http.MethodGet, "/api/projects/_default/tree", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		// tree is a pure store read — must not return 503 (LLM disabled)
		if rec.Code == http.StatusServiceUnavailable {
			t.Errorf("GET /api/projects/_default/tree returned 503 (LLM disabled), but it should work without LLM")
		}
	})

	t.Run("notes_search_returns_non_503", func(t *testing.T) {
		h, _ := newNoLLMRouter(t)
		req := httptest.NewRequest(http.MethodGet, "/api/projects/_default/search?q=test", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code == http.StatusServiceUnavailable {
			t.Errorf("GET /api/projects/_default/search returned 503 (LLM disabled), but notes-search should work without LLM")
		}
	})

	t.Run("search_returns_503_with_sentinel_body", func(t *testing.T) {
		h, _ := newNoLLMRouter(t)
		body := strings.NewReader(`{"query":"test","mode":"local"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/search", body)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("POST /api/search status = %d, want 503", rec.Code)
		}

		var resp map[string]string
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if resp["code"] != "llm_disabled" {
			t.Errorf("response code = %q, want llm_disabled", resp["code"])
		}
		if resp["error"] == "" {
			t.Error("response error field is empty")
		}
	})

	t.Run("upload_returns_503_with_sentinel_body", func(t *testing.T) {
		h, _ := newNoLLMRouter(t)
		req := httptest.NewRequest(http.MethodPost, "/api/upload", strings.NewReader(""))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("POST /api/upload status = %d, want 503", rec.Code)
		}

		var resp map[string]string
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if resp["code"] != "llm_disabled" {
			t.Errorf("response code = %q, want llm_disabled", resp["code"])
		}
	})

	t.Run("mcp_returns_503_with_sentinel_body", func(t *testing.T) {
		h, _ := newNoLLMRouter(t)
		req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader("{}"))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("POST /mcp status = %d, want 503", rec.Code)
		}

		var resp map[string]string
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if resp["code"] != "llm_disabled" {
			t.Errorf("response code = %q, want llm_disabled", resp["code"])
		}
	})

	t.Run("projects_list_returns_200", func(t *testing.T) {
		h, _ := newNoLLMRouter(t)
		req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("GET /api/projects status = %d, want 200", rec.Code)
		}
	})
}
