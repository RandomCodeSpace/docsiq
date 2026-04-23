package api

import (
	"bytes"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestUploadMaxBytes verifies that requests whose body exceeds
// cfg.Server.MaxUploadBytes are rejected with 413 before the handler
// tries to parse the multipart form. This exercises the fast-path
// (Content-Length declared and already over the limit).
func TestUploadMaxBytes(t *testing.T) {
	t.Parallel()
	const limit int64 = 1024 // 1 KiB for the test

	// Build a multipart body larger than the limit.
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, err := mw.CreateFormFile("files", "big.txt")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := io.Copy(part, strings.NewReader(strings.Repeat("x", int(limit)*2))); err != nil {
		t.Fatalf("copy: %v", err)
	}
	_ = mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/upload", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rr := httptest.NewRecorder()

	// enforceUploadLimit is the unit-testable shim applied inside upload().
	// It wraps r.Body with http.MaxBytesReader and returns a 413 on overflow.
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !enforceUploadLimit(w, r, limit) {
			return
		}
		if err := r.ParseMultipartForm(32 << 10); err != nil {
			// MaxBytesReader converts overflow into a ParseMultipartForm error
			// AFTER the header has been written by http.MaxBytesReader. We
			// still exit here; the header is already 413 in that case.
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %q", got)
	}
	if !strings.Contains(rr.Body.String(), "exceeds maximum upload size") {
		t.Fatalf("expected JSON error to mention upload size, got: %s", rr.Body.String())
	}
}

// TestUploadMaxBytes_UnknownContentLength covers the slow-path where the
// Content-Length is unknown (e.g. chunked transfer encoding). In that case
// the fast-path cannot reject up-front; enforcement happens when
// ParseMultipartForm reads through the MaxBytesReader wrapper and that
// returns a *http.MaxBytesError.
//
// Note: the real net/http server calls an internal requestTooLarge() hook
// on its own response writer that commits a 413, so production callers
// can "just return" on *MaxBytesError without a WriteHeader of their
// own. httptest.ResponseRecorder does not implement that hook, so this
// test verifies the downstream signal (the error type is what production
// code matches on) by asserting the *MaxBytesError surfaces and that
// writeTooLarge, given the limit, produces the expected 413 JSON.
func TestUploadMaxBytes_UnknownContentLength(t *testing.T) {
	t.Parallel()
	const limit int64 = 1024 // 1 KiB for the test

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, err := mw.CreateFormFile("files", "big.txt")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := io.Copy(part, strings.NewReader(strings.Repeat("x", int(limit)*2))); err != nil {
		t.Fatalf("copy: %v", err)
	}
	_ = mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/upload", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.ContentLength = -1 // force slow path: unknown Content-Length
	rr := httptest.NewRecorder()

	// This inner handler mirrors the prod upload() flow. In a real
	// http.Server the MaxBytesReader's requestTooLarge() hook commits
	// 413 automatically; in httptest we explicitly writeTooLarge()
	// after matching *MaxBytesError, which exercises the same helper
	// used by the fast-path.
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !enforceUploadLimit(w, r, limit) {
			return
		}
		if err := r.ParseMultipartForm(32 << 10); err != nil {
			var mbe *http.MaxBytesError
			if errors.As(err, &mbe) {
				writeTooLarge(w, mbe.Limit)
				return
			}
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %q", got)
	}
	if !strings.Contains(rr.Body.String(), "exceeds maximum upload size") {
		t.Fatalf("expected JSON error to mention upload size, got: %s", rr.Body.String())
	}
}
