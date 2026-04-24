package api

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newTarGz builds an in-memory tar.gz with the given entries. Each
// entry is a .md note under "k<i>.md".
func newTarGz(t *testing.T, entries []tarEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		hdr := &tar.Header{
			Name:     e.name,
			Mode:     0o644,
			Size:     int64(len(e.body)),
			Typeflag: tar.TypeReg,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(e.body); err != nil {
			t.Fatal(err)
		}
	}
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

type tarEntry struct {
	name string
	body []byte
}

// TestImportTar_EntryCountCap is a regression test for P0-3.
// A tar with more than MaxImportEntries .md files must be rejected
// with 413 before exhausting resources.
func TestImportTar_EntryCountCap(t *testing.T) {
	h, slug, _ := setupNotesRouter(t)

	// Slightly above the cap; tiny payloads so we're testing entry
	// count, not byte totals.
	n := MaxImportEntries + 5
	entries := make([]tarEntry, n)
	for i := range n {
		entries[i] = tarEntry{
			name: fmt.Sprintf("note-%06d.md", i),
			body: []byte("x"),
		}
	}
	body := newTarGz(t, entries)

	req := httptest.NewRequest(http.MethodPost,
		"/api/projects/"+slug+"/import", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/gzip")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413 for over-entry-cap tar, got %d body=%s",
			rec.Code, rec.Body.String())
	}
}

// TestImportTar_TotalBytesCap is a regression test for P0-3.
// A tar whose total uncompressed bytes across entries exceed
// MaxImportTotalBytes must be rejected with 413.
func TestImportTar_TotalBytesCap(t *testing.T) {
	if testing.Short() {
		// TODO(#62): large-tar import test skipped under -short; tracked in flake-register.
		t.Skip("skipping large-tar test in -short mode")
	}
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
