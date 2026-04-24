package api

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// captureLogs swaps the default slog handler for a JSON-to-buffer one
// for the duration of the test, then restores the previous default.
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	h := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	prev := slog.Default()
	slog.SetDefault(slog.New(h))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

func TestLoggingMiddleware_EmitsStructuredAccessLog(t *testing.T) {
	// NOT parallel — mutates global slog.
	buf := captureLogs(t)

	h := loggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("hello world"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	req.Header.Set("Authorization", "Bearer dev")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) == 0 {
		t.Fatal("no log lines emitted")
	}
	var last map[string]any
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &last); err != nil {
		t.Fatalf("last log line not JSON: %v — raw=%q", err, lines[len(lines)-1])
	}

	want := map[string]any{
		"msg":    "http",
		"method": "GET",
		"path":   "/api/stats",
		"status": float64(200),
		"auth":   "bearer",
	}
	for k, v := range want {
		if got := last[k]; got != v {
			t.Errorf("log[%s]=%v want %v", k, got, v)
		}
	}
	if _, ok := last["req_id"].(string); !ok {
		t.Errorf("req_id missing or not string: %v", last["req_id"])
	}
	if b, ok := last["bytes_out"].(float64); !ok || b != 11 {
		t.Errorf("bytes_out=%v want 11", last["bytes_out"])
	}
}

func TestLoggingMiddleware_PanicStillLogsAccessEntry(t *testing.T) {
	// NOT parallel — mutates global slog.
	buf := captureLogs(t)

	// Chain: loggingMiddleware wraps a handler that panics. Without
	// recoveryMiddleware here the panic propagates — but the deferred
	// access log must still have fired.
	h := loggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))

	req := httptest.NewRequest(http.MethodGet, "/panic-path", nil)
	rec := httptest.NewRecorder()

	func() {
		defer func() { _ = recover() }() // swallow in test
		h.ServeHTTP(rec, req)
	}()

	if buf.Len() == 0 {
		t.Fatal("access log not emitted through panic path")
	}
	if !strings.Contains(buf.String(), `"panic":"boom"`) {
		t.Errorf("log should mention panic=boom; got: %s", buf.String())
	}
	if !strings.Contains(buf.String(), `"level":"ERROR"`) {
		t.Errorf("panic log should be ERROR level; got: %s", buf.String())
	}
}

func TestLoggingMiddleware_ReqIDPassThrough(t *testing.T) {
	t.Parallel()
	h := loggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := RequestIDFromContext(r.Context()); got != "caller-id-abc" {
			t.Errorf("ctx req_id=%q want caller-id-abc", got)
		}
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "caller-id-abc")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Request-ID"); got != "caller-id-abc" {
		t.Errorf("echoed X-Request-ID=%q", got)
	}
}

func TestLoggingMiddleware_AnonCookieBearerClassification(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		setup func(r *http.Request)
		want  string
	}{
		{name: "anon_no_auth", setup: func(r *http.Request) {}, want: "anon"},
		{name: "bearer_header", setup: func(r *http.Request) {
			r.Header.Set("Authorization", "Bearer k")
		}, want: "bearer"},
		{name: "session_cookie", setup: func(r *http.Request) {
			r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "cookie-token"})
		}, want: "cookie"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			tc.setup(req)
			if got := classifyAuth(req); got != tc.want {
				t.Errorf("classifyAuth=%q want %q", got, tc.want)
			}
		})
	}
}
