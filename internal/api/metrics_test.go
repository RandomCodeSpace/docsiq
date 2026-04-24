package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/RandomCodeSpace/docsiq/internal/config"
	"github.com/RandomCodeSpace/docsiq/internal/store"
)

// newMetricsRouter builds a minimal router so the /metrics endpoint can
// be reached via the full middleware chain (including the scrape
// bypasses in bearerAuthMiddleware).
func newMetricsRouter(t *testing.T) http.Handler {
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
	return NewRouter(nil, nil, cfg, nil,
		WithProjectStores(testSingleStore(dir, st, "_default", "testproj")))
}

func TestMetrics_EndpointReturns200(t *testing.T) {
	h := newMetricsRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want 200", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Content-Type=%q want text/plain*", ct)
	}
}

func TestMetrics_HTTPRequestsCounterIncrements(t *testing.T) {
	h := newMetricsRouter(t)

	// Warm up by hitting /healthz a few times so the HTTP counter
	// picks up samples for the GET /healthz route pattern.
	const warmups = 3
	for i := 0; i < warmups; i++ {
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
	}

	// Scrape.
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()

	if !strings.Contains(body, "docsiq_http_requests_total") {
		t.Fatalf("body missing docsiq_http_requests_total; body=\n%s", body)
	}
	if !strings.Contains(body, "docsiq_http_request_duration_seconds") {
		t.Errorf("body missing docsiq_http_request_duration_seconds")
	}
}

func TestMetrics_BuildInfoReflectsSetBuildInfo(t *testing.T) {
	SetBuildInfo("v9.9.9", "abc1234")

	h := newMetricsRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()

	if !strings.Contains(body, "docsiq_build_info") {
		t.Fatalf("missing docsiq_build_info; body=\n%s", body)
	}
	if !strings.Contains(body, `version="v9.9.9"`) {
		t.Errorf("expected version=v9.9.9 label in %q", body)
	}
	if !strings.Contains(body, `commit="abc1234"`) {
		t.Errorf("expected commit=abc1234 label in %q", body)
	}
}
