package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/RandomCodeSpace/docsiq/internal/config"
	"github.com/RandomCodeSpace/docsiq/internal/store"
)

// metricsTestLock serializes tests that mutate the package-level
// globalMetrics collector so parallel runs don't race on counts.
var metricsTestLock sync.Mutex

// resetMetrics wipes the global collector so a test starts from a
// deterministic state. Must hold metricsTestLock across the whole test
// body that inspects counters.
func resetMetrics() {
	globalMetrics.mu.Lock()
	defer globalMetrics.mu.Unlock()
	globalMetrics.requestsTotal = map[labelKey]uint64{}
	globalMetrics.requestDuration = map[histogramKey]*histogramCell{}
	globalMetrics.buildVersion = "dev"
	globalMetrics.buildCommit = "unknown"
}

func newMetricsRouter(t *testing.T) http.Handler {
	t.Helper()
	dir := t.TempDir()
	st, err := store.OpenForProject(dir, "testproj")
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	cfg := &config.Config{}
	cfg.DataDir = dir
	return NewRouter(nil, nil, cfg, nil,
		WithProjectStores(testSingleStore(dir, st, "_default", "testproj")))
}

func TestMetricsEndpoint(t *testing.T) {
	t.Run("exposes_expected_families", func(t *testing.T) {
		metricsTestLock.Lock()
		defer metricsTestLock.Unlock()
		resetMetrics()
		SetBuildInfo("v0.5.0", "abcdef1")

		h := newMetricsRouter(t)

		// Hit a couple of real endpoints so counters/histograms populate.
		for _, path := range []string{"/health", "/api/nonexistent", "/health"} {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
		}

		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		ct := rec.Header().Get("Content-Type")
		if !strings.HasPrefix(ct, "text/plain") {
			t.Errorf("Content-Type = %q, want text/plain*", ct)
		}
		body := rec.Body.String()

		// HELP/TYPE lines and each expected family must be present.
		wants := []string{
			"# HELP docsiq_build_info",
			"# TYPE docsiq_build_info gauge",
			`docsiq_build_info{version="v0.5.0",commit="abcdef1"} 1`,
			"# HELP docsiq_projects_total",
			"# TYPE docsiq_projects_total gauge",
			"docsiq_projects_total ",
			"# HELP docsiq_notes_total",
			"# HELP docsiq_requests_total",
			"# TYPE docsiq_requests_total counter",
			`docsiq_requests_total{method="GET",path="/health",status="200"} 2`,
			"# HELP docsiq_request_duration_seconds",
			"# TYPE docsiq_request_duration_seconds histogram",
			`docsiq_request_duration_seconds_bucket{method="GET",path="/health",le="0.005"}`,
			`docsiq_request_duration_seconds_bucket{method="GET",path="/health",le="+Inf"}`,
			`docsiq_request_duration_seconds_sum{method="GET",path="/health"}`,
			`docsiq_request_duration_seconds_count{method="GET",path="/health"} 2`,
		}
		for _, w := range wants {
			if !strings.Contains(body, w) {
				t.Errorf("missing %q in scrape body\n---\n%s\n---", w, body)
			}
		}
	})

	t.Run("format_parses_as_prometheus_text", func(t *testing.T) {
		metricsTestLock.Lock()
		defer metricsTestLock.Unlock()
		resetMetrics()

		h := newMetricsRouter(t)
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		req = httptest.NewRequest(http.MethodGet, "/metrics", nil)
		rec = httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		// Each non-comment, non-empty line must match the Prometheus
		// exposition grammar: `<name>{labels} <value>` or `<name> <value>`.
		// This is a structural sanity check, not a full parser.
		for line := range strings.SplitSeq(rec.Body.String(), "\n") {
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			// Every sample line must contain at least one space between
			// the series identifier and the value.
			if !strings.Contains(line, " ") {
				t.Errorf("invalid sample line: %q", line)
			}
			// Braces must balance (open count equals close count).
			if strings.Count(line, "{") != strings.Count(line, "}") {
				t.Errorf("unbalanced braces on line: %q", line)
			}
		}
	})

	t.Run("histogram_buckets_are_cumulative", func(t *testing.T) {
		metricsTestLock.Lock()
		defer metricsTestLock.Unlock()
		resetMetrics()

		// Record three synthetic requests at known durations.
		recordRequest("GET", "/synthetic", 200, 0.001) // <= 0.005
		recordRequest("GET", "/synthetic", 200, 0.5)   // <= 0.5
		recordRequest("GET", "/synthetic", 200, 10.0)  // <= 10

		h := newMetricsRouter(t)
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		body := rec.Body.String()
		// Cumulative: le=0.005 should be >= 1, le=0.5 should be >= 2,
		// le=+Inf should be 3.
		mustContain(t, body, `docsiq_request_duration_seconds_bucket{method="GET",path="/synthetic",le="0.005"} 1`)
		mustContain(t, body, `docsiq_request_duration_seconds_bucket{method="GET",path="/synthetic",le="0.5"} 2`)
		mustContain(t, body, `docsiq_request_duration_seconds_bucket{method="GET",path="/synthetic",le="+Inf"} 3`)
		mustContain(t, body, `docsiq_request_duration_seconds_count{method="GET",path="/synthetic"} 3`)
	})
}

func mustContain(t *testing.T, body, needle string) {
	t.Helper()
	if !strings.Contains(body, needle) {
		t.Errorf("missing %q in body", needle)
	}
}
