package api

import (
	"net/http"
	"strings"

	"github.com/RandomCodeSpace/docsiq/internal/config"
)

// isUploadRoute reports whether r should be granted UploadTimeout.
// The carve-out covers two long-running paths:
//   - POST /api/upload           — multipart document upload
//   - POST /api/projects/{p}/import — tar / bulk notes import
//
// A /api/projects/* POST is only an upload if the trailing segment is
// /import — this avoids granting 10-minute timeouts to note-write POSTs
// on the same prefix. Block 3.2.
func isUploadRoute(r *http.Request) bool {
	if r.Method != http.MethodPost {
		return false
	}
	switch {
	case r.URL.Path == "/api/upload":
		return true
	case strings.HasPrefix(r.URL.Path, "/api/projects/") && strings.HasSuffix(r.URL.Path, "/import"):
		return true
	}
	return false
}

// isStreamingRoute reports whether r serves an SSE / long-poll stream
// that must not be wrapped by http.TimeoutHandler. TimeoutHandler
// buffers the response and does not propagate http.Flusher, so any
// stream wrapped by it stalls until the handler returns — at which
// point the client has already timed out reading the body.
//
// Streaming routes rely on ctx cancellation (client disconnect or
// server shutdown) for teardown rather than a per-request wall clock.
func isStreamingRoute(r *http.Request) bool {
	if r.Method != http.MethodGet {
		return false
	}
	switch r.URL.Path {
	case "/api/upload/progress", "/mcp":
		return true
	}
	return false
}

// requestTimeoutMiddleware wraps inner in http.TimeoutHandler with
// cfg.Server.RequestTimeout as the default bound, bumped to
// cfg.Server.UploadTimeout for upload routes.
//
// Zero timeout means "no cap" — useful for local dev. In that case
// inner is returned unchanged.
//
// Layering rationale (Block 3.2 comment): this middleware sits INSIDE
// securityHeadersMiddleware (so a 503 still carries CSP) and OUTSIDE
// loggingMiddleware (so the timeout is logged). See router.go.
func requestTimeoutMiddleware(cfg *config.Config) func(http.Handler) http.Handler {
	return func(inner http.Handler) http.Handler {
		reqTimeout := cfg.Server.RequestTimeout
		upTimeout := cfg.Server.UploadTimeout

		// Pre-build the two TimeoutHandler instances so each request
		// just dispatches to one — no per-request allocation.
		defaultTO := inner
		if reqTimeout > 0 {
			defaultTO = http.TimeoutHandler(inner, reqTimeout, "request timeout")
		}
		uploadTO := inner
		if upTimeout > 0 {
			uploadTO = http.TimeoutHandler(inner, upTimeout, "upload timeout")
		}

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isStreamingRoute(r) {
				inner.ServeHTTP(w, r)
				return
			}
			if isUploadRoute(r) {
				uploadTO.ServeHTTP(w, r)
				return
			}
			defaultTO.ServeHTTP(w, r)
		})
	}
}
