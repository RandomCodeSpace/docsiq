package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RandomCodeSpace/docsiq/internal/config"
	"github.com/RandomCodeSpace/docsiq/internal/store"
)

// TestUpload_TraversalFilename is a regression test for P0-2.
//
// Go's mime/multipart applies filepath.Base to filenames surfaced via
// FileHeader.Filename, but ".." survives that stripping — so a crafted
// multipart body with `filename=".."` lands in FileHeader.Filename as
// the literal "..", and filepath.Join(tmpDir, "..") resolves to the
// parent of tmpDir. Before the fix, os.Create(parent) would either
// corrupt/replace that directory or (common case) return an error
// that surfaces as a 500 — both are bugs.
//
// After the fix, ".." (and other degenerate names) must be skipped,
// and if no valid entries remain the handler returns 400, NOT 500.
func TestUpload_TraversalFilename(t *testing.T) {
	// Isolate os.MkdirTemp under a sandbox we control.
	sandboxRoot := t.TempDir()
	t.Setenv("TMPDIR", sandboxRoot)

	dataDir := filepath.Join(sandboxRoot, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("mkdir dataDir: %v", err)
	}
	st, err := store.OpenForProject(dataDir, "testproj")
	if err != nil {
		t.Fatalf("OpenForProject: %v", err)
	}
	defer st.Close()

	cfg := &config.Config{}
	cfg.DataDir = dataDir

	h := &handlers{
		stores:   testSingleStore(dataDir, st, "testproj"),
		provider: nil,
		cfg:      cfg,
	}

	// Craft a multipart body by hand. Using multipart.Writer.CreateFormFile
	// would call its own sanitization; the raw body mirrors what an
	// attacker-controlled client would send on the wire.
	boundary := "TEST_BOUNDARY_P0_2"

	var body bytes.Buffer
	// filename=".." survives mime/multipart's internal filepath.Base.
	// Without the P0-2 fix, Join(tmpDir, "..") → parent dir and
	// os.Create(parent) fails → 500 bubbles up.
	body.WriteString("--" + boundary + "\r\n")
	body.WriteString("Content-Disposition: form-data; name=\"files\"; filename=\"..\"\r\n")
	body.WriteString("Content-Type: application/octet-stream\r\n\r\n")
	body.WriteString("dots-payload")
	body.WriteString("\r\n--" + boundary + "--\r\n")

	req := httptest.NewRequest(http.MethodPost, "/api/upload", &body)
	req.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
	ctx := context.WithValue(req.Context(), ctxProjectKey{}, "testproj")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	h.upload(w, req)

	// Before the fix: os.Create(tmpDir/..) fails → writeError 500.
	// After the fix: ".." is skipped; zero valid files → 400.
	// Either way, never a 2xx (the request had no valid payload), and
	// never a 5xx (traversal names are a client error, not an infra one).
	if w.Code == http.StatusInternalServerError {
		t.Fatalf("upload returned 500 on traversal filename — fix missing. body=%s",
			w.Body.String())
	}
	if w.Code >= 200 && w.Code < 300 {
		t.Fatalf("upload accepted a traversal-only body as 2xx: %d %s",
			w.Code, w.Body.String())
	}

	// Sanity: no stray files at sandboxRoot beyond the docsiq-upload-*
	// tmp dirs and the data/ subtree we created.
	entries, _ := os.ReadDir(sandboxRoot)
	for _, e := range entries {
		name := e.Name()
		if name == "data" || strings.HasPrefix(name, "docsiq-upload-") {
			continue
		}
		t.Errorf("unexpected entry at sandbox root: %s (possible traversal)", name)
	}
	_ = filepath.Join // avoid unused import
}
