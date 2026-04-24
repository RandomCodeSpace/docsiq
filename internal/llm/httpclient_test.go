package llm

import (
	"net/http"
	"testing"
	"time"
)

// TestNewHTTPClient_TransportSettings verifies the tuned transport
// settings required by Block 3.5.
func TestNewHTTPClient_TransportSettings(t *testing.T) {
	t.Parallel()
	c := newHTTPClient()
	if c == nil {
		t.Fatal("newHTTPClient returned nil")
	}
	tr, ok := c.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("Transport = %T; want *http.Transport", c.Transport)
	}
	if got, want := tr.MaxIdleConns, 100; got != want {
		t.Errorf("MaxIdleConns = %d; want %d", got, want)
	}
	if got, want := tr.MaxIdleConnsPerHost, 10; got != want {
		t.Errorf("MaxIdleConnsPerHost = %d; want %d", got, want)
	}
	if got, want := tr.IdleConnTimeout, 90*time.Second; got != want {
		t.Errorf("IdleConnTimeout = %v; want %v", got, want)
	}
	if got, want := tr.TLSHandshakeTimeout, 10*time.Second; got != want {
		t.Errorf("TLSHandshakeTimeout = %v; want %v", got, want)
	}
	if got, want := tr.ResponseHeaderTimeout, 60*time.Second; got != want {
		t.Errorf("ResponseHeaderTimeout = %v; want %v", got, want)
	}
}

// TestNewHTTPClient_NoClientTimeout asserts we do NOT set
// http.Client.Timeout — that would hard-cut the body mid-stream on
// large embedding responses. Per-call timeouts live on ctx instead
// (Task 3).
func TestNewHTTPClient_NoClientTimeout(t *testing.T) {
	t.Parallel()
	c := newHTTPClient()
	if c.Timeout != 0 {
		t.Fatalf("Client.Timeout = %v; want 0 (use ctx per-call)", c.Timeout)
	}
}
