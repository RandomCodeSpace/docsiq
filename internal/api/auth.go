package api

import (
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
)

// bearerAuthMiddleware gates /api/* and /mcp on a shared bearer API key.
//
// Policy (locked by Phase-0 spec):
//   - Empty apiKey → middleware is a zero-overhead no-op (auth disabled).
//   - OPTIONS requests always bypass (CORS preflight).
//   - /health is always public (defense-in-depth even though the router
//     also registers it before the wrap).
//   - Anything outside /api/ and /mcp is public (UI, static assets).
//   - Scheme match is case-sensitive ("Bearer " only; "bearer"/"BEARER" reject).
//   - The raw Authorization header value is whitespace-trimmed before
//     prefix check; the key and token themselves are NOT trimmed.
//   - Token comparison uses crypto/subtle.ConstantTimeCompare.
//   - Never log the submitted token — only the failure reason.
func bearerAuthMiddleware(apiKey string, next http.Handler) http.Handler {
	if apiKey == "" {
		// Auth disabled — return the handler unwrapped for zero overhead.
		return next
	}

	keyBytes := []byte(apiKey)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// CORS preflight bypass.
		if r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		path := r.URL.Path

		// Observability + liveness probes are always public. Defense-in-
		// depth even though /health, /healthz, /readyz, /metrics, and
		// /api/version are registered on the mux directly.
		switch path {
		case "/health", "/healthz", "/readyz", "/metrics", "/api/version":
			next.ServeHTTP(w, r)
			return
		}

		// UI + static assets are public — only gate /api/* and /mcp.
		if !strings.HasPrefix(path, "/api/") && !strings.HasPrefix(path, "/mcp") {
			next.ServeHTTP(w, r)
			return
		}

		// /api/session is the auth boundary itself — always public.
		if path == "/api/session" {
			next.ServeHTTP(w, r)
			return
		}

		// Defense-in-depth: reject immediately if the server has no key
		// configured. This mirrors newSessionHandler's guard and keeps the
		// middleware correct under future refactors (rather than relying on
		// the no_token branch firing because keyBytes would also be empty).
		if apiKey == "" {
			slog.Warn("🔒 auth failure", "path", path, "remote_addr", r.RemoteAddr, "reason", "server_misconfigured")
			writeJSON401(w)
			return
		}

		token := extractToken(r)
		if token == "" {
			slog.Warn("🔒 auth failure",
				"path", path,
				"remote_addr", r.RemoteAddr,
				"reason", "no_token")
			writeJSON401(w)
			return
		}
		if subtle.ConstantTimeCompare([]byte(token), keyBytes) != 1 {
			slog.Warn("🔒 auth failure",
				"path", path,
				"remote_addr", r.RemoteAddr,
				"reason", "wrong_key")
			writeJSON401(w)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// writeJSON401 emits a fixed unauthorized JSON body. Kept separate from
// the generic writeJSON helper so the auth path never formats untrusted
// input into the response.
func writeJSON401(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
}

// extractToken returns the bearer token from either the Authorization
// header (preferred, for machine clients) or the session cookie (for
// browser clients after POST /api/session). Returns "" if neither.
func extractToken(r *http.Request) string {
	raw := strings.TrimSpace(r.Header.Get("Authorization"))
	const prefix = "Bearer "
	if strings.HasPrefix(raw, prefix) {
		return raw[len(prefix):]
	}
	if c, err := r.Cookie(sessionCookieName); err == nil {
		if v := strings.TrimSpace(c.Value); v != "" {
			return v
		}
	}
	return ""
}
