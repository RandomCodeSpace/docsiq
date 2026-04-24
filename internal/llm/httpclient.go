// Package llm — HTTP client pooling per provider (Block 3.5).
//
// langchaingo constructs a fresh net/http.Transport inside each
// provider constructor by default. For a long-running server that
// calls the same provider on every request, that leaks connections:
// every call-site allocates its own idle-conn pool, TLS session
// cache, and DNS resolver bucket. Pooling one *http.Client per
// provider (constructed here) fixes the leak.
package llm

import (
	"net"
	"net/http"
	"time"
)

// newHTTPClient returns a *http.Client tuned for long-lived LLM
// provider traffic. The transport settings are spec-driven:
//   - MaxIdleConns=100          — plenty of headroom for bursty batching
//   - MaxIdleConnsPerHost=10    — matches langchaingo default fan-out
//   - IdleConnTimeout=90s       — trim idle conns before cloud LBs do
//   - TLSHandshakeTimeout=10s   — fail fast on broken TLS upstreams
//   - ResponseHeaderTimeout=60s — distinct from body-stream timeout;
//     bounds the silent-server failure mode
//
// Deliberately NOT set:
//   - Client.Timeout — would hard-cut streaming bodies; per-call
//     timeouts come from ctx (Task 3 / Block 3.3).
//   - DialContext timeout — Go's default (no timeout, relies on ctx)
//     is correct here; a fixed dial timeout fights ctx-driven shutdown.
func newHTTPClient() *http.Client {
	tr := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 60 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	return &http.Client{Transport: tr}
}
