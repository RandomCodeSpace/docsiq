package api

import (
	"context"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/RandomCodeSpace/docscontext/internal/config"
	"github.com/RandomCodeSpace/docscontext/internal/store"
)

func TestRequestIDFromContext(t *testing.T) {
	t.Run("plain_ctx_returns_empty", func(t *testing.T) {
		if got := RequestIDFromContext(context.Background()); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})

	t.Run("ctx_with_value_returns_id", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), ctxRequestIDKey{}, "abc123")
		if got := RequestIDFromContext(ctx); got != "abc123" {
			t.Errorf("got %q, want abc123", got)
		}
	})
}

func TestNewRequestID(t *testing.T) {
	got := newRequestID()
	if len(got) != 16 {
		t.Errorf("len = %d, want 16 (8 bytes hex)", len(got))
	}
	if _, err := hex.DecodeString(got); err != nil {
		t.Errorf("not hex: %v", err)
	}
	// Two consecutive IDs should differ with astronomically high prob.
	if newRequestID() == got {
		t.Error("two consecutive IDs collided — RNG regression")
	}
}

// newRequestIDRouter builds a router and registers a test-only handler
// that echoes back RequestIDFromContext so we can verify the middleware
// wiring end-to-end without introducing a public production endpoint.
func newRequestIDRouter(t *testing.T, testHandler http.HandlerFunc) http.Handler {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "rid.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	cfg := &config.Config{}
	cfg.DataDir = dir

	// Wrap the testHandler in the same middleware chain the real router
	// uses so we exercise loggingMiddleware end-to-end.
	inner := http.HandlerFunc(testHandler)
	return loggingMiddleware(recoveryMiddleware(inner))
}

func TestRequestIDMiddleware(t *testing.T) {
	t.Run("generates_id_when_header_absent", func(t *testing.T) {
		var seenID string
		h := newRequestIDRouter(t, func(w http.ResponseWriter, r *http.Request) {
			seenID = RequestIDFromContext(r.Context())
			w.WriteHeader(http.StatusOK)
		})
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if seenID == "" {
			t.Fatal("handler saw empty request ID in ctx")
		}
		if len(seenID) != 16 {
			t.Errorf("ctx id len = %d, want 16", len(seenID))
		}
		resp := rec.Header().Get("X-Request-ID")
		if resp != seenID {
			t.Errorf("response header %q != ctx id %q", resp, seenID)
		}
	})

	t.Run("passes_through_client_supplied_id", func(t *testing.T) {
		var seenID string
		h := newRequestIDRouter(t, func(w http.ResponseWriter, r *http.Request) {
			seenID = RequestIDFromContext(r.Context())
			w.WriteHeader(http.StatusOK)
		})
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Request-ID", "caller-trace-42")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if seenID != "caller-trace-42" {
			t.Errorf("ctx id = %q, want caller-trace-42", seenID)
		}
		if got := rec.Header().Get("X-Request-ID"); got != "caller-trace-42" {
			t.Errorf("resp header = %q, want caller-trace-42", got)
		}
	})
}
