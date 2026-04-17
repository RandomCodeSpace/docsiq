//go:build integration

package api_test

import (
	"net/http"
	"testing"
	"time"

	"go.uber.org/goleak"

	"github.com/RandomCodeSpace/docsiq/internal/api/itest"
)

// TestShutdown_NoGoroutineLeaks fires a handful of requests through the
// harness, then relies on t.Cleanup (LIFO: server close → stores close
// → registry close) plus goleak.VerifyNone to assert no stray
// goroutine survived teardown.
//
// Ignored patterns:
//   - stdlib net/http keep-alive readLoop/writeLoop goroutines parked
//     on the test client's persistent connections. We also explicitly
//     call CloseIdleConnections before VerifyNone runs.
//   - database/sql.connectionOpener — the sqlite driver's opener
//     goroutine blocks in a select on a channel that is not drained
//     synchronously by *DB.Close(); go-sqlite3's docs acknowledge a
//     brief post-Close window. Tolerated as a stdlib artifact.
//
// goleak.IgnoreCurrent() baselines out any stdlib goroutines that
// predate test start (Go runtime housekeeping, test framework, etc.).
func TestShutdown_NoGoroutineLeaks(t *testing.T) {
	defer goleak.VerifyNone(t,
		goleak.IgnoreCurrent(),
		goleak.IgnoreAnyFunction("net/http.(*persistConn).readLoop"),
		goleak.IgnoreAnyFunction("net/http.(*persistConn).writeLoop"),
		goleak.IgnoreAnyFunction("database/sql.(*DB).connectionOpener"),
		goleak.IgnoreAnyFunction("internal/poll.runtime_pollWait"),
	)

	e := itest.New(t)
	// Close idle client connections once the test body is done so the
	// readLoop/writeLoop goroutines have a chance to drain before the
	// goleak verifier runs.
	t.Cleanup(func() {
		// Let the transport release its persistent conns.
		type idleCloser interface{ CloseIdleConnections() }
		if ic, ok := any(e.Client.Transport).(idleCloser); ok {
			ic.CloseIdleConnections()
		}
		if ic, ok := any(http.DefaultTransport).(idleCloser); ok {
			ic.CloseIdleConnections()
		}
		// Give the readLoop goroutines a tick to return.
		time.Sleep(50 * time.Millisecond)
	})

	// A mix of endpoints to exercise the full middleware stack.
	resp, _ := e.GET(t, "/health")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/health: %d", resp.StatusCode)
	}
	resp, _ = e.GET(t, "/metrics")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/metrics: %d", resp.StatusCode)
	}
	if resp, body := e.PUTNoteBody(t, "_default", "shutdown/probe", "hi", nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT: %d body=%s", resp.StatusCode, string(body))
	}
	resp, _ = e.GET(t, "/api/projects/_default/notes/shutdown/probe")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET note: %d", resp.StatusCode)
	}
}
