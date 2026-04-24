package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/RandomCodeSpace/docsiq/internal/config"
)

// TestRequestTimeoutMiddleware_FiresOnSlowHandler: a handler that
// sleeps past the request timeout returns 503 Service Unavailable.
func TestRequestTimeoutMiddleware_FiresOnSlowHandler(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}
	cfg.Server.RequestTimeout = 50 * time.Millisecond
	cfg.Server.UploadTimeout = 1 * time.Second

	slow := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		_, _ = w.Write([]byte("too late"))
	})
	handler := requestTimeoutMiddleware(cfg)(slow)

	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d; want 503", rec.Code)
	}
	body, _ := io.ReadAll(rec.Body)
	if !strings.Contains(string(body), "request timeout") {
		t.Fatalf("body = %q; want substring 'request timeout'", body)
	}
}

// TestRequestTimeoutMiddleware_UploadRouteGetsExtendedTimeout: an upload
// request that completes within UploadTimeout (but exceeds
// RequestTimeout) succeeds.
func TestRequestTimeoutMiddleware_UploadRouteGetsExtendedTimeout(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}
	cfg.Server.RequestTimeout = 50 * time.Millisecond
	cfg.Server.UploadTimeout = 500 * time.Millisecond

	slow := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	handler := requestTimeoutMiddleware(cfg)(slow)

	req := httptest.NewRequest(http.MethodPost, "/api/upload", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200 (upload route under UploadTimeout)", rec.Code)
	}
}

// TestRequestTimeoutMiddleware_FastHandlerUnaffected: a handler that
// responds well within the timeout is passed through unchanged.
func TestRequestTimeoutMiddleware_FastHandlerUnaffected(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}
	cfg.Server.RequestTimeout = 100 * time.Millisecond
	cfg.Server.UploadTimeout = 1 * time.Second

	fast := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Test", "ok")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	handler := requestTimeoutMiddleware(cfg)(fast)

	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200", rec.Code)
	}
	if got := rec.Header().Get("X-Test"); got != "ok" {
		t.Fatalf("X-Test = %q; want ok", got)
	}
}

// TestIsUploadRoute_Classification: rules for which routes receive the
// upload timeout.
func TestIsUploadRoute_Classification(t *testing.T) {
	t.Parallel()
	cases := []struct {
		method, path string
		want         bool
	}{
		{http.MethodPost, "/api/upload", true},
		{http.MethodGet, "/api/upload", false}, // GET → request timeout
		{http.MethodPost, "/api/projects/foo/import", true},
		{http.MethodPost, "/api/projects/foo/notes", false},
		{http.MethodPost, "/api/projects/foo", false},
		{http.MethodPost, "/api/stats", false},
	}
	for _, c := range cases {
		c := c
		t.Run(c.method+" "+c.path, func(t *testing.T) {
			req := httptest.NewRequest(c.method, c.path, nil)
			got := isUploadRoute(req)
			if got != c.want {
				t.Fatalf("isUploadRoute(%s %s) = %v; want %v", c.method, c.path, got, c.want)
			}
		})
	}
}
