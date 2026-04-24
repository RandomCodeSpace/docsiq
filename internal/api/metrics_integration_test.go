//go:build integration

package api_test

import (
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/RandomCodeSpace/docsiq/internal/api/itest"
)

// TestMetrics_EndpointReturns200PublicNoAuth asserts /metrics is
// reachable without an Authorization header — Prometheus scrapers must
// not be expected to send one.
func TestMetrics_EndpointReturns200PublicNoAuth(t *testing.T) {
	e := itest.New(t)
	req, err := http.NewRequest(http.MethodGet, e.URL("/metrics"), nil)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	resp := e.Do(t, req)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
}

// TestMetrics_IsPrometheusText asserts at least three output lines
// match the Prometheus text-format pattern `^docsiq_<metric> <value>`
// (with optional label braces before the value).
func TestMetrics_IsPrometheusText(t *testing.T) {
	e := itest.New(t)
	// Warm up at least one counter so multiple metric lines exist.
	resp, _ := e.GET(t, "/healthz")
	resp.Body.Close()

	req, _ := http.NewRequest(http.MethodGet, e.URL("/metrics"), nil)
	resp = e.Do(t, req)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	re := regexp.MustCompile(`(?m)^docsiq_\w+(?:\{[^}]*\})?\s+\S`)
	matches := re.FindAllString(string(body), -1)
	if len(matches) < 3 {
		t.Fatalf("expected >=3 docsiq_* metric lines, got %d. body=%s", len(matches), string(body))
	}
}

// TestMetrics_HTTPRequestsCounterIncrements fires N /healthz requests
// then scrapes /metrics and asserts the docsiq_http_requests_total
// counter for the matching route saw at least N increments.
func TestMetrics_HTTPRequestsCounterIncrements(t *testing.T) {
	e := itest.New(t)

	const n = 5
	for i := 0; i < n; i++ {
		req, _ := http.NewRequest(http.MethodGet, e.URL("/healthz"), nil)
		resp := e.Do(t, req)
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}

	req, _ := http.NewRequest(http.MethodGet, e.URL("/metrics"), nil)
	resp := e.Do(t, req)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	// Format: docsiq_http_requests_total{method="GET",route="GET /healthz",status="2xx"} 7
	re := regexp.MustCompile(`(?m)^docsiq_http_requests_total\{[^}]*route="GET /healthz"[^}]*\}\s+(\d+)`)
	matches := re.FindAllStringSubmatch(string(body), -1)
	if len(matches) == 0 {
		t.Fatalf("no docsiq_http_requests_total row for /healthz. body=\n%s", string(body))
	}
	total := 0
	for _, m := range matches {
		v, err := strconv.Atoi(m[1])
		if err != nil {
			t.Fatalf("parse counter %q: %v", m[1], err)
		}
		total += v
	}
	if total < n {
		t.Fatalf("/healthz counter %d < %d fired requests. matches=%v body_head=%s",
			total, n, matches, firstLines(string(body), 20))
	}
}

func firstLines(s string, n int) string {
	lines := strings.SplitN(s, "\n", n+1)
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}
