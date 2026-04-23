package api

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestUploadMaxBytes verifies that requests whose body exceeds
// cfg.Server.MaxUploadBytes are rejected with 413 before the handler
// tries to parse the multipart form.
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
}
