package api

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestContextLogger_AddsReqIDFromContext(t *testing.T) {
	// Cannot t.Parallel() because we mutate slog.Default.
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	ctx := context.WithValue(context.Background(), ctxRequestIDKey{}, "abc123")
	ContextLogger(ctx).Info("hello", "k", "v")

	out := buf.String()
	if !strings.Contains(out, "req_id=abc123") {
		t.Fatalf("expected req_id=abc123 in log output; got %q", out)
	}
	if !strings.Contains(out, "k=v") {
		t.Fatalf("expected k=v to survive; got %q", out)
	}
}

func TestContextLogger_NoReqIDWhenMissing(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	ContextLogger(context.Background()).Info("hello")

	out := buf.String()
	if strings.Contains(out, "req_id=") {
		t.Fatalf("req_id should be absent when context has no ID; got %q", out)
	}
}
