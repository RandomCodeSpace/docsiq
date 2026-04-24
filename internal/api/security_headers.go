package api

import (
	"net/http"

	"github.com/RandomCodeSpace/docsiq/internal/config"
)

// Header values are deliberately strict for the air-gapped deployment
// posture: no CDN origins, no inline scripts, WASM allowed (shiki uses
// it for syntax highlighting), no iframing. Inline styles are permitted
// because Tailwind + shadcn/ui emit them.
const (
	contentSecurityPolicy = "default-src 'self'; " +
		"script-src 'self' 'wasm-unsafe-eval'; " +
		"style-src 'self' 'unsafe-inline'; " +
		"connect-src 'self'; " +
		"img-src 'self' data:; " +
		"font-src 'self'; " +
		"frame-ancestors 'none'; " +
		"base-uri 'self'"

	permissionsPolicy = "camera=(), microphone=(), geolocation=(), payment=(), usb=()"

	hstsValue = "max-age=31536000; includeSubDomains"
)

// securityHeadersMiddleware sets browser-side hardening headers on every
// response that actually carries a body (i.e. non-OPTIONS):
//   - Content-Security-Policy
//   - X-Content-Type-Options: nosniff
//   - Referrer-Policy: strict-origin-when-cross-origin
//   - Permissions-Policy (disables camera/mic/geo/payment/usb)
//   - Strict-Transport-Security (only when cfg.Server.HSTSEnabled=true)
//
// Intended to be wrapped as the OUTERMOST middleware so headers are
// emitted on panic recoveries and auth failures too.
func securityHeadersMiddleware(cfg *config.Config) func(http.Handler) http.Handler {
	hstsEnabled := cfg != nil && cfg.Server.HSTSEnabled
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}
			h := w.Header()
			h.Set("Content-Security-Policy", contentSecurityPolicy)
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
			h.Set("Permissions-Policy", permissionsPolicy)
			if hstsEnabled {
				h.Set("Strict-Transport-Security", hstsValue)
			}
			next.ServeHTTP(w, r)
		})
	}
}
