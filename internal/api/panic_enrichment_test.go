package api

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestRecoveryMiddleware_EnrichedLog verifies that a panic is logged
// with req_id, route, method, user, and a stack trace. Block 3.7.
func TestRecoveryMiddleware_EnrichedLog(t *testing.T) {
	// Capture slog output via a TextHandler into a buffer.
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	panicky := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	})
	handler := recoveryMiddleware(panicky)

	// Seed request with a request id (mimicking loggingMiddleware
	// having already run) and a user ctx value.
	req := httptest.NewRequest(http.MethodPost, "/api/documents/abc", nil)
	ctx := context.WithValue(req.Context(), ctxRequestIDKey{}, "rid-test-123")
	ctx = withUserForTest(ctx, "alice")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d; want 500", rec.Code)
	}

	logOutput := buf.String()
	for _, want := range []string{
		"panic recovered",
		"req_id=rid-test-123",
		"method=POST",
		"route=/api/documents/abc",
		"user=alice",
		"panic=boom",
	} {
		if !strings.Contains(logOutput, want) {
			t.Errorf("log missing %q\nlog: %s", want, logOutput)
		}
	}
	// A stack trace marker ("goroutine " or "runtime/panic.go") must
	// appear somewhere in the log. slog serializes newlines as \n
	// literals inside the stack attribute — either is acceptable.
	if !strings.Contains(logOutput, "goroutine") && !strings.Contains(logOutput, "runtime/panic") {
		t.Errorf("log missing stack trace marker\nlog: %s", logOutput)
	}
}

// TestRecoveryMiddleware_NoUserNoReqID verifies the middleware still
// recovers cleanly when neither ctxUserKey nor request id are set.
func TestRecoveryMiddleware_NoUserNoReqID(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	panicky := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom-bare")
	})
	handler := recoveryMiddleware(panicky)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d; want 500", rec.Code)
	}
	logOutput := buf.String()
	if strings.Contains(logOutput, "req_id=") {
		t.Errorf("unexpected req_id attr on unset ctx\nlog: %s", logOutput)
	}
	if strings.Contains(logOutput, "user=") {
		t.Errorf("unexpected user attr on unset ctx\nlog: %s", logOutput)
	}
}

// withUserForTest is a test-only helper that injects a user id into
// ctx using the same key the real auth middleware would use.
func withUserForTest(ctx context.Context, user string) context.Context {
	return context.WithValue(ctx, ctxUserKey{}, user)
}
