package api

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"net/http"
	"net/http/httptest"
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

