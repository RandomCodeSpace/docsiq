package api

import (
	"context"
	"log/slog"
)

// ContextLogger returns slog.Default() enriched with the per-request ID
// when the context carries one. Handler trees that funnel log calls
// through this helper get free request-level log correlation; downstream
// code that needs the ID for metric labels should still read it via
// RequestIDFromContext(ctx) directly.
//
// Callers that don't need the enrichment can keep using slog.Default()
// — the helper is additive, not mandatory.
func ContextLogger(ctx context.Context) *slog.Logger {
	if id := RequestIDFromContext(ctx); id != "" {
		return slog.Default().With("req_id", id)
	}
	return slog.Default()
}
