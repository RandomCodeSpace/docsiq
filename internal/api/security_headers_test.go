package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/RandomCodeSpace/docsiq/internal/config"
)

func TestSecurityHeaders_CSPOnEveryResponse(t *testing.T) {
	t.Parallel()
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	cfg := &config.Config{}
	h := securityHeadersMiddleware(cfg)(next)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	csp := rr.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Fatal("CSP header missing")
	}
	for _, want := range []string{
		"default-src 'self'",
		"script-src 'self' 'wasm-unsafe-eval'",
		"style-src 'self' 'unsafe-inline'",
		"connect-src 'self'",
		"img-src 'self' data:",
		"font-src 'self'",
		"frame-ancestors 'none'",
		"base-uri 'self'",
	} {
		if !strings.Contains(csp, want) {
			t.Errorf("CSP missing directive %q: got %q", want, csp)
		}
	}
}

func TestSecurityHeaders_SkipsOPTIONS(t *testing.T) {
	t.Parallel()
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	cfg := &config.Config{}
	h := securityHeadersMiddleware(cfg)(next)

	req := httptest.NewRequest(http.MethodOptions, "/api/ping", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Header().Get("Content-Security-Policy") != "" {
		t.Errorf("CSP should not be set on OPTIONS; got %q", rr.Header().Get("Content-Security-Policy"))
	}
	if rr.Code != http.StatusNoContent {
		t.Errorf("OPTIONS should pass through; got status %d", rr.Code)
	}
}

func TestSecurityHeaders_PreservesExistingHeaders(t *testing.T) {
	t.Parallel()
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom", "xyz")
		w.WriteHeader(http.StatusOK)
	})
	cfg := &config.Config{}
	h := securityHeadersMiddleware(cfg)(next)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Header().Get("X-Custom") != "xyz" {
		t.Errorf("downstream header clobbered")
	}
}

func TestSecurityHeaders_BaselineHeaders(t *testing.T) {
	t.Parallel()
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	cfg := &config.Config{}
	h := securityHeadersMiddleware(cfg)(next)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if got := rr.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options: want nosniff, got %q", got)
	}
	if got := rr.Header().Get("Referrer-Policy"); got != "strict-origin-when-cross-origin" {
		t.Errorf("Referrer-Policy: got %q", got)
	}
	perms := rr.Header().Get("Permissions-Policy")
	for _, want := range []string{"camera=()", "microphone=()", "geolocation=()", "payment=()", "usb=()"} {
		if !strings.Contains(perms, want) {
			t.Errorf("Permissions-Policy missing %q: got %q", want, perms)
		}
	}
	if rr.Header().Get("Strict-Transport-Security") != "" {
		t.Error("HSTS should not be set when HSTSEnabled=false")
	}
}

func TestSecurityHeaders_HSTSOnlyWhenEnabled(t *testing.T) {
	t.Parallel()
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	cfg := &config.Config{}
	cfg.Server.HSTSEnabled = true
	h := securityHeadersMiddleware(cfg)(next)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	hsts := rr.Header().Get("Strict-Transport-Security")
	if !strings.Contains(hsts, "max-age=31536000") {
		t.Errorf("HSTS missing max-age; got %q", hsts)
	}
}
