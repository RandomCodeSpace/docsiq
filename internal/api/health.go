package api

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/RandomCodeSpace/docsiq/internal/llm"
)

// readyCheckTTL is how long the /readyz handler caches its aggregated
// verdict. Ten seconds is short enough to notice a real outage quickly
// and long enough to absorb a chatty Prometheus + Kubernetes probe loop
// without hammering SQLite or the LLM endpoint.
const readyCheckTTL = 10 * time.Second

// readyCheckTimeout bounds each individual probe. The SQLite ping is a
// microsecond-scale PRAGMA query; the LLM ping is network-bound, so we
// allow more headroom but still fail fast when the provider is wedged.
const (
	sqliteCheckTimeout = 500 * time.Millisecond
	llmCheckTimeout    = 2 * time.Second
)

// healthPinger is the narrow interface readyz needs for SQLite
// reachability: a bounded Ping call that reports the first hard error.
type healthPinger interface {
	Ping(ctx context.Context) error
}

// llmPinger is the narrow interface readyz needs for LLM reachability.
// Implementations may issue a tiny Complete call or a model-list GET.
type llmPinger interface {
	Ping(ctx context.Context) error
}

// checkStatus holds per-component readiness.
type checkStatus struct {
	Status string `json:"status"` // "ok" | "error" | "skipped"
	Err    string `json:"err,omitempty"`
}

type readyzBody struct {
	Status string                 `json:"status"` // "ready" | "not_ready"
	Checks map[string]checkStatus `json:"checks"`
}

// healthzHandler implements GET /healthz. Liveness: always 200 as long
// as the goroutine scheduler can run this function. No dependencies.
func healthzHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
}

// readyzHandler implements GET /readyz. Readiness: aggregates a SQLite
// ping and an LLM reach check with a 10-second in-memory cache. When
// llmp is nil (provider=none), the LLM check is reported as "skipped"
// and does NOT fail readiness.
//
// Pass the default project's store adapter and an llmPinger wrapper
// around the configured provider (or nil). See router.go for wiring.
func readyzHandler(sq healthPinger, llmp llmPinger) http.Handler {
	return readyzHandlerForTest(sq, llmp, readyCheckTTL)
}

// readyzHandlerForTest is the injectable-TTL variant used only by tests.
// Production code must use readyzHandler.
func readyzHandlerForTest(sq healthPinger, llmp llmPinger, ttl time.Duration) http.Handler {
	rc := &readyzCache{ttl: ttl}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body, code := rc.check(r.Context(), sq, llmp)
		writeJSON(w, code, body)
	})
}

// readyzCache is the TTL-cached readiness result. Uses a single mutex
// because the hot path is cheap; lock contention only matters when the
// TTL expires and many requests arrive before the first unlock.
type readyzCache struct {
	ttl time.Duration

	mu     sync.Mutex
	expiry time.Time
	body   readyzBody
	code   int
}

func (c *readyzCache) check(ctx context.Context, sq healthPinger, llmp llmPinger) (readyzBody, int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.expiry.IsZero() && time.Now().Before(c.expiry) {
		return c.body, c.code
	}

	// Decouple probe lifetime from the caller's request ctx: if the
	// probing client (Kubernetes, Prometheus, curl) disconnects or its
	// own deadline expires, an in-flight Ping would otherwise return
	// context.Canceled / DeadlineExceeded and poison the 10-second
	// cache for every subsequent caller. WithoutCancel preserves any
	// values on ctx (e.g. req_id, for logging adapters inside Ping
	// implementations) while detaching its cancellation.
	probeCtx := context.WithoutCancel(ctx)

	body := readyzBody{
		Status: "ready",
		Checks: map[string]checkStatus{},
	}
	code := http.StatusOK

	// SQLite probe — mandatory. Failure fails readiness.
	{
		sqCtx, cancel := context.WithTimeout(probeCtx, sqliteCheckTimeout)
		err := sq.Ping(sqCtx)
		cancel()
		if err != nil {
			body.Checks["sqlite"] = checkStatus{Status: "error", Err: err.Error()}
			body.Status = "not_ready"
			code = http.StatusServiceUnavailable
		} else {
			body.Checks["sqlite"] = checkStatus{Status: "ok"}
		}
	}

	// LLM probe — optional. Nil provider (provider=none) reports skipped.
	switch {
	case llmp == nil:
		body.Checks["llm"] = checkStatus{Status: "skipped", Err: "provider=none"}
	default:
		llmCtx, cancel := context.WithTimeout(probeCtx, llmCheckTimeout)
		err := llmp.Ping(llmCtx)
		cancel()
		if err != nil {
			body.Checks["llm"] = checkStatus{Status: "error", Err: err.Error()}
			body.Status = "not_ready"
			code = http.StatusServiceUnavailable
		} else {
			body.Checks["llm"] = checkStatus{Status: "ok"}
		}
	}

	c.body = body
	c.code = code
	c.expiry = time.Now().Add(c.ttl)
	return body, code
}

// sqlDBPinger adapts a *sql.DB for the healthPinger interface. SQLite's
// driver treats PingContext as a cheap no-op when the handle is healthy.
type sqlDBPinger struct{ db *sql.DB }

func (p sqlDBPinger) Ping(ctx context.Context) error {
	if p.db == nil {
		return errors.New("nil sql.DB")
	}
	return p.db.PingContext(ctx)
}

// providerPinger adapts an llm.Provider for the llmPinger interface. It
// issues a tiny Complete call with a 1-token cap; providers that cannot
// produce tokens (e.g. misconfigured) fail the ping.
type providerPinger struct{ prov llm.Provider }

func (p providerPinger) Ping(ctx context.Context) error {
	if p.prov == nil {
		return errors.New("nil provider")
	}
	// A minimal generation request — 1 token, temp 0, no JSON mode.
	// Providers that stream will still return promptly when maxTokens=1.
	_, err := p.prov.Complete(ctx, "ping", llm.WithMaxTokens(1), llm.WithTemperature(0))
	return err
}

// healthPingerFuncForRouter is the non-test counterpart of
// healthPingerFunc; lives in the prod file so the wiring in router.go
// does not have to depend on a test-only adapter.
type healthPingerFuncForRouter func(ctx context.Context) error

func (f healthPingerFuncForRouter) Ping(ctx context.Context) error { return f(ctx) }
