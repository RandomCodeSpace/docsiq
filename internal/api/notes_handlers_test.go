package api

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/RandomCodeSpace/docsiq/internal/config"
	"github.com/RandomCodeSpace/docsiq/internal/project"
)

// setupNotesRouter spins up a full router + registry + one registered
// project. Returns the handler and the slug so individual tests can
// issue typed requests.
func setupNotesRouter(t *testing.T) (http.Handler, string, string) {
	t.Helper()
	dataDir := t.TempDir()
	cfg := &config.Config{DataDir: dataDir, DefaultProject: config.DefaultProjectSlug}
	reg, err := project.OpenRegistry(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reg.Close() })
	slug := "testproj"
	if err := reg.Register(project.Project{
		Slug: slug, Name: slug, Remote: "r-" + slug, CreatedAt: time.Now().Unix(),
	}); err != nil {
		t.Fatal(err)
	}
	h := NewRouter(nil, nil, cfg, reg)
	return h, slug, dataDir
}

func doN(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, r)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestNotesHandlers_PutGetRoundTrip(t *testing.T) {
	h, slug, _ := setupNotesRouter(t)
	body := `{"content":"hello world","author":"alice","tags":["a","b"]}`
	rec := doN(h, http.MethodPut, "/api/projects/"+slug+"/notes/arch/auth", body)
	if rec.Code != 200 {
		t.Fatalf("PUT status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = doN(h, http.MethodGet, "/api/projects/"+slug+"/notes/arch/auth", "")
	if rec.Code != 200 {
		t.Fatalf("GET status=%d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	json.Unmarshal(rec.Body.Bytes(), &resp)
	note := resp["note"].(map[string]any)
	if note["content"] != "hello world" {
		t.Errorf("content = %v", note["content"])
	}
}

func TestNotesHandlers_404UnknownProject(t *testing.T) {
	h, _, _ := setupNotesRouter(t)
	rec := doN(h, http.MethodGet, "/api/projects/ghost/notes/k", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("got %d, want 404; body=%s", rec.Code, rec.Body.String())
	}
}

func TestNotesHandlers_404MissingNote(t *testing.T) {
	h, slug, _ := setupNotesRouter(t)
	rec := doN(h, http.MethodGet, "/api/projects/"+slug+"/notes/does-not-exist", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("got %d, want 404", rec.Code)
	}
}

func TestNotesHandlers_TraversalRejected(t *testing.T) {
	h, slug, _ := setupNotesRouter(t)
	for _, bad := range []string{
		"/api/projects/" + slug + "/notes/../escape",
		"/api/projects/" + slug + "/notes/foo/../../etc",
	} {
		t.Run(bad, func(t *testing.T) {
			rec := doN(h, http.MethodGet, bad, "")
			if rec.Code == 200 {
				t.Errorf("traversal allowed: status=%d", rec.Code)
			}
		})
	}
}

func TestNotesHandlers_PutTraversalRejected(t *testing.T) {
	h, slug, _ := setupNotesRouter(t)
	body := `{"content":"x"}`
	// Go's http.ServeMux path-cleans `..` before dispatch, responding
	// with a 301/307 redirect to the cleaned path. Either outcome is
	// acceptable: the traversal never reaches the note handler, so no
	// escape happens. We only fail on a 2xx success.
	rec := doN(h, http.MethodPut, "/api/projects/"+slug+"/notes/../escape", body)
	if rec.Code >= 200 && rec.Code < 300 {
		t.Errorf("traversal succeeded: status=%d body=%s", rec.Code, rec.Body.String())
	}
	// Also assert: if we manually bypass the mux by passing a key with
	// a `..` segment embedded inside `{key...}` that the mux *doesn't*
	// normalize (it only normalizes leading `..`), the handler's own
	// ValidateKey should 403. Build such a path via RawPath.
}

func TestNotesHandlers_EmptyContentRejected(t *testing.T) {
	h, slug, _ := setupNotesRouter(t)
	rec := doN(h, http.MethodPut, "/api/projects/"+slug+"/notes/k", `{"content":""}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status=%d want 400", rec.Code)
	}
	rec = doN(h, http.MethodPut, "/api/projects/"+slug+"/notes/k", `{"content":"   \n\t  "}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("whitespace content should 400, got %d", rec.Code)
	}
}

func TestNotesHandlers_TagRoundTrip(t *testing.T) {
	h, slug, _ := setupNotesRouter(t)
	body := `{"content":"x","tags":["Security","Auth"]}`
	rec := doN(h, http.MethodPut, "/api/projects/"+slug+"/notes/k", body)
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	rec = doN(h, http.MethodGet, "/api/projects/"+slug+"/notes/k", "")
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	var resp map[string]any
	json.Unmarshal(rec.Body.Bytes(), &resp)
	note := resp["note"].(map[string]any)
	tags, _ := note["tags"].([]any)
	if len(tags) != 2 {
		t.Errorf("tags = %v", tags)
	}
}

func TestNotesHandlers_UnicodeContent(t *testing.T) {
	h, slug, _ := setupNotesRouter(t)
	body := `{"content":"日本語 emoji 🎉 test"}`
	rec := doN(h, http.MethodPut, "/api/projects/"+slug+"/notes/k", body)
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	rec = doN(h, http.MethodGet, "/api/projects/"+slug+"/notes/k", "")
	if !bytes.Contains(rec.Body.Bytes(), []byte("日本語 emoji 🎉")) {
		t.Errorf("unicode lost: %s", rec.Body.String())
	}
}

func TestNotesHandlers_DeleteRoundTrip(t *testing.T) {
	h, slug, _ := setupNotesRouter(t)
	doN(h, http.MethodPut, "/api/projects/"+slug+"/notes/k", `{"content":"x"}`)
	rec := doN(h, http.MethodDelete, "/api/projects/"+slug+"/notes/k", "")
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	rec = doN(h, http.MethodGet, "/api/projects/"+slug+"/notes/k", "")
	if rec.Code != 404 {
		t.Errorf("expected 404 after delete, got %d", rec.Code)
	}
}

func TestNotesHandlers_List(t *testing.T) {
	h, slug, _ := setupNotesRouter(t)
	for _, k := range []string{"a", "b", "folder/c"} {
		doN(h, http.MethodPut, "/api/projects/"+slug+"/notes/"+k, `{"content":"x"}`)
	}
	rec := doN(h, http.MethodGet, "/api/projects/"+slug+"/notes", "")
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	var r map[string]any
	json.Unmarshal(rec.Body.Bytes(), &r)
	keys, _ := r["keys"].([]any)
	if len(keys) != 3 {
		t.Errorf("keys = %v", keys)
	}
}

func TestNotesHandlers_Tree(t *testing.T) {
	h, slug, _ := setupNotesRouter(t)
	doN(h, http.MethodPut, "/api/projects/"+slug+"/notes/top", `{"content":"x"}`)
	doN(h, http.MethodPut, "/api/projects/"+slug+"/notes/sub/nested", `{"content":"x"}`)
	rec := doN(h, http.MethodGet, "/api/projects/"+slug+"/tree", "")
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "folder") {
		t.Errorf("tree missing folder: %s", rec.Body.String())
	}
}

func TestNotesHandlers_SearchFTS(t *testing.T) {
	h, slug, _ := setupNotesRouter(t)
	doN(h, http.MethodPut, "/api/projects/"+slug+"/notes/k1", `{"content":"oauth authentication strategy"}`)
	doN(h, http.MethodPut, "/api/projects/"+slug+"/notes/k2", `{"content":"unrelated content"}`)
	rec := doN(h, http.MethodGet, "/api/projects/"+slug+"/search?q=oauth", "")
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "k1") {
		t.Errorf("search hit missing: %s", rec.Body.String())
	}
}

func TestNotesHandlers_Graph(t *testing.T) {
	h, slug, _ := setupNotesRouter(t)
	doN(h, http.MethodPut, "/api/projects/"+slug+"/notes/a", `{"content":"link [[b]]"}`)
	doN(h, http.MethodPut, "/api/projects/"+slug+"/notes/b", `{"content":"nada"}`)
	rec := doN(h, http.MethodGet, "/api/projects/"+slug+"/graph", "")
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	var g map[string]any
	json.Unmarshal(rec.Body.Bytes(), &g)
	edges, _ := g["edges"].([]any)
	if len(edges) != 1 {
		t.Errorf("edges = %d, want 1", len(edges))
	}
}

func TestNotesHandlers_ExportImportRoundTrip(t *testing.T) {
	h, slug, _ := setupNotesRouter(t)
	doN(h, http.MethodPut, "/api/projects/"+slug+"/notes/a", `{"content":"first"}`)
	doN(h, http.MethodPut, "/api/projects/"+slug+"/notes/dir/b", `{"content":"second"}`)

	// Export
	rec := doN(h, http.MethodGet, "/api/projects/"+slug+"/export", "")
	if rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	archive := rec.Body.Bytes()

	// Set up a fresh project to import into.
	h2, slug2, _ := setupNotesRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/api/projects/"+slug2+"/import", bytes.NewReader(archive))
	req.Header.Set("Content-Type", "application/gzip")
	rec2 := httptest.NewRecorder()
	h2.ServeHTTP(rec2, req)
	if rec2.Code != 200 {
		t.Fatalf("import status=%d body=%s", rec2.Code, rec2.Body.String())
	}
	// Verify via GET
	got := doN(h2, http.MethodGet, "/api/projects/"+slug2+"/notes/a", "")
	if got.Code != 200 {
		t.Errorf("imported note a: %d body=%s", got.Code, got.Body.String())
	}
	got = doN(h2, http.MethodGet, "/api/projects/"+slug2+"/notes/dir/b", "")
	if got.Code != 200 {
		t.Errorf("imported note dir/b: %d", got.Code)
	}
}

func TestNotesHandlers_ImportTraversalRejected(t *testing.T) {
	h, slug, _ := setupNotesRouter(t)
	// Build a malicious tar with ".." in entry name.
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	data := []byte("pwned")
	tw.WriteHeader(&tar.Header{Name: "../../escape.md", Size: int64(len(data)), Mode: 0o644})
	tw.Write(data)
	tw.Close()
	gz.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/projects/"+slug+"/import", bytes.NewReader(buf.Bytes()))
	req.Header.Set("Content-Type", "application/gzip")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("status=%d want 403", rec.Code)
	}
}

func TestNotesHandlers_MaxSizeEnforced(t *testing.T) {
	h, slug, _ := setupNotesRouter(t)
	// Content larger than MaxNoteBytes (10 MB) — we construct JSON with
	// a ~11 MB content field.
	content := strings.Repeat("a", 11*1024*1024)
	body := fmt.Sprintf(`{"content":%q}`, content)
	rec := doN(h, http.MethodPut, "/api/projects/"+slug+"/notes/big", body)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status=%d want 413", rec.Code)
	}
}

func TestNotesHandlers_InvalidProjectSlug(t *testing.T) {
	h, _, _ := setupNotesRouter(t)
	rec := doN(h, http.MethodGet, "/api/projects/BAD.SLUG/notes/k", "")
	// Middleware short-circuits with 400 before reaching the handler
	// when the slug from ?project= is invalid; but in our PATH-based
	// scheme, the middleware sees no ?project and defaults to _default.
	// Handler then validates the path slug itself: 400 on invalid chars.
	if rec.Code != http.StatusBadRequest && rec.Code != http.StatusNotFound {
		t.Errorf("status=%d want 400 or 404", rec.Code)
	}
}

func TestNotesHandlers_WikilinkOutlinksInResponse(t *testing.T) {
	h, slug, _ := setupNotesRouter(t)
	doN(h, http.MethodPut, "/api/projects/"+slug+"/notes/a", `{"content":"see [[b]] and [[c]]"}`)
	rec := doN(h, http.MethodGet, "/api/projects/"+slug+"/notes/a", "")
	var resp map[string]any
	json.Unmarshal(rec.Body.Bytes(), &resp)
	outlinks, _ := resp["outlinks"].([]any)
	if len(outlinks) != 2 {
		t.Errorf("outlinks = %v", outlinks)
	}
}

func TestNotesHandlers_SearchEmptyQuery(t *testing.T) {
	// NF-P1-1: empty / whitespace-only q must match the MCP contract and
	// be rejected with 400 "query required" rather than silently returning
	// an empty hit list (which misled clients into thinking the corpus was
	// empty). Covers bare `q=`, missing `q` entirely, and whitespace.
	h, slug, _ := setupNotesRouter(t)
	for _, path := range []string{
		"/api/projects/" + slug + "/search?q=",
		"/api/projects/" + slug + "/search",
		"/api/projects/" + slug + "/search?q=%20%20",
	} {
		rec := doN(h, http.MethodGet, path, "")
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status=%d want 400; body=%s", path, rec.Code, rec.Body.String())
		}
		var body map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		if body["error"] != "query required" {
			t.Errorf("%s: body=%v want error=\"query required\"", path, body)
		}
	}
}

func TestNotesHandlers_PutInvalidJSON(t *testing.T) {
	h, slug, _ := setupNotesRouter(t)
	rec := doN(h, http.MethodPut, "/api/projects/"+slug+"/notes/k", "{not json")
	if rec.Code != 400 {
		t.Errorf("status=%d want 400", rec.Code)
	}
}
