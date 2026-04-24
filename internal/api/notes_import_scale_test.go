//go:build scale

package api

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestImportTar_TotalBytesCap is a regression test for P0-3 kept behind
// the `scale` build tag. It allocates ~520 MB of tar payload; the
// dedicated nightly workflow runs it via `-tags "sqlite_fts5 scale"`.
// Default PR CI does not compile this file.
func TestImportTar_TotalBytesCap(t *testing.T) {
	h, slug, _ := setupNotesRouter(t)

	// Each entry is just under MaxNoteBytes (10 MB). Two 256 MB entries
	// would still fit under MaxImportTotalBytes (500 MB); we need > 500
	// MB total. Use 52 entries × 10 MB = 520 MB — exceeds the cap.
	// Build each entry's body once and reuse.
	perEntry := 10 * 1024 * 1024 // 10 MB, equal to MaxNoteBytes
	// Use slightly less to satisfy per-entry cap but still accumulate
	// fast.
	body := make([]byte, perEntry-1)
	for i := range body {
		body[i] = 'x'
	}
	entriesNeeded := int(MaxImportTotalBytes/int64(perEntry-1)) + 3
	entries := make([]tarEntry, entriesNeeded)
	for i := range entriesNeeded {
		entries[i] = tarEntry{
			name: fmt.Sprintf("big-%03d.md", i),
			body: body,
		}
	}
	tarBytes := newTarGz(t, entries)
	req := httptest.NewRequest(http.MethodPost,
		"/api/projects/"+slug+"/import", bytes.NewReader(tarBytes))
	req.Header.Set("Content-Type", "application/gzip")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413 for over-total-bytes tar, got %d body=%s",
			rec.Code, rec.Body.String())
	}
	if !strings.Contains(strings.ToLower(rec.Body.String()), "total") &&
		!strings.Contains(strings.ToLower(rec.Body.String()), "bytes") {
		t.Logf("body=%s", rec.Body.String())
	}
	_ = io.EOF
}
