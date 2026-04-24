package api

import (
	"net/http"

	"github.com/RandomCodeSpace/docsiq/internal/config"
)

// contentSecurityPolicy is deliberately strict for the air-gapped
// deployment posture: no CDN origins, no inline scripts, WASM allowed
// (shiki uses it for syntax highlighting), no iframing. Inline styles
// are permitted because Tailwind + shadcn/ui emit them.
const contentSecurityPolicy = "default-src 'self'; " +
	"script-src 'self' 'wasm-unsafe-eval'; " +
	"style-src 'self' 'unsafe-inline'; " +
	"connect-src 'self'; " +
	"img-src 'self' data:; " +
	"font-src 'self'; " +
	"frame-ancestors 'none'; " +
	"base-uri 'self'"

// securityHeadersMiddleware sets browser-side hardening headers on every
// response that actually carries a body (i.e. non-OPTIONS). Task 4
// extends this middleware with baseline security headers
// (nosniff, Referrer-Policy, Permissions-Policy, HSTS). Keeping both
// sets in one middleware keeps response-header side-effects colocated.
func securityHeadersMiddleware(_ *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}
			w.Header().Set("Content-Security-Policy", contentSecurityPolicy)
			next.ServeHTTP(w, r)
		})
	}
}
