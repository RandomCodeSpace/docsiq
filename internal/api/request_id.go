package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
)

// ctxRequestIDKey is the typed context key for the per-request ID. Typed
// struct keeps the key collision-free with other packages.
type ctxRequestIDKey struct{}

// ctxUserKey is the typed context key for the authenticated user ID.
// Reserved for future auth integration; today only the panic
// recoveryMiddleware reads it. Empty string means the request is
// unauthenticated (allowed on public routes). Block 3.7.
type ctxUserKey struct{}

// RequestIDFromContext returns the ID attached by loggingMiddleware. Empty
// string when called from a context that never hit the middleware (e.g.
// a direct handler call from a test that skips the router wrapper).
func RequestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ctxRequestIDKey{}).(string); ok {
		return v
	}
	return ""
}

// newRequestID returns a 16-char hex ID (8 random bytes). crypto/rand is
// used rather than math/rand so two concurrent servers sharing a log
// stream still see near-zero collision odds.
//
// An all-zero ID is returned on the (unreachable) error path so we never
// panic on a malformed OS RNG. Callers should treat empty-string as
// "missing" but "00000000…0000" as a valid-but-degraded ID.
func newRequestID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Caller still gets a syntactically-valid ID; the upstream log
		// stream keeps rolling.
		return "0000000000000000"
	}
	return hex.EncodeToString(b[:])
}
