package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestWriteError_IncludesRequestID covers NF-P1-3: the JSON error body
// must include a `request_id` field matching X-Request-ID when the
// caller-supplied header is present. docs/rest-api.md has promised this
// shape since the first cut — the code only just caught up.
func TestWriteError_IncludesRequestID(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/not-real", nil)
	req = req.WithContext(context.WithValue(req.Context(), ctxRequestIDKey{}, "test-abc"))
	rec := httptest.NewRecorder()

	writeError(rec, req, http.StatusBadRequest, "boom", nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v (%s)", err, rec.Body.String())
	}
	if body["error"] != "boom" {
		t.Errorf("error=%q want %q", body["error"], "boom")
	}
	if body["request_id"] != "test-abc" {
		t.Errorf("request_id=%q want %q", body["request_id"], "test-abc")
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content-type=%q", ct)
	}
}

// TestWriteError_OmitsRequestIDWhenAbsent: if a handler is invoked
// outside the middleware chain (e.g. a unit test or a panic before the
// middleware ran), the JSON body must still be well-formed. The
// `request_id` key is optional — omitempty via the bare map is enough.
func TestWriteError_OmitsRequestIDWhenAbsent(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/no-id", nil)
	rec := httptest.NewRecorder()

	writeError(rec, req, http.StatusInternalServerError, "nope", nil)

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v (%s)", err, rec.Body.String())
	}
	if body["error"] != "nope" {
		t.Errorf("error=%q want nope", body["error"])
	}
	if _, ok := body["request_id"]; ok {
		t.Errorf("request_id should be omitted when ctx has no ID, got %q", body["request_id"])
	}
}

// TestWriteError_ThroughMiddlewareGeneratesID: when a handler is wired
// behind loggingMiddleware (no caller-supplied X-Request-ID), the
// middleware synthesises an ID, attaches it to ctx, and also writes
// the X-Request-ID response header. writeError must echo the same ID
// into the JSON body so the response header and body agree.
func TestWriteError_ThroughMiddlewareGeneratesID(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, r, http.StatusBadRequest, "bad", nil)
	})
	h := loggingMiddleware(recoveryMiddleware(inner))

	req := httptest.NewRequest(http.MethodGet, "/some-path", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	headerID := rec.Header().Get("X-Request-ID")
	if headerID == "" {
		t.Fatalf("X-Request-ID response header empty")
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v (%s)", err, rec.Body.String())
	}
	if body["request_id"] != headerID {
		t.Errorf("body.request_id=%q != X-Request-ID=%q", body["request_id"], headerID)
	}
	if body["error"] != "bad" {
		t.Errorf("error=%q want bad", body["error"])
	}
}
