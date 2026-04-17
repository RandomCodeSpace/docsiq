package api

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/RandomCodeSpace/docsiq/internal/notes"
	"github.com/RandomCodeSpace/docsiq/internal/project"
	"github.com/RandomCodeSpace/docsiq/internal/store"
)

// MaxNoteBytes caps the size of a single note body. Decision: 10 MB.
// Requests larger than this get a 413. The ceiling keeps us well below
// any SQLite single-row limit while accommodating very long design docs.
const MaxNoteBytes = 10 * 1024 * 1024

// notesHandlers is the handler set that depends on the per-project
// store cache and the configured data dir. A separate struct (rather
// than adding fields to the main `handlers`) keeps Phase-2 surface
// isolated from the existing docs handlers.
type notesHandlers struct {
	cfg      interface{ NotesDir(string) string }
	stores   *projectStores
	registry *project.Registry // nil in tests → skip existence check
}

func newNotesHandlers(dataDir string, cfg interface{ NotesDir(string) string }, registry *project.Registry) *notesHandlers {
	return &notesHandlers{
		cfg:      cfg,
		stores:   newProjectStores(dataDir),
		registry: registry,
	}
}

// newNotesHandlersWithStores reuses an existing shared projectStores
// cache rather than allocating a private one. Wave-2 routers pass the
// same cache used by the doc handlers so both read/write through a
// single set of SQLite connections.
func newNotesHandlersWithStores(stores *projectStores, cfg interface{ NotesDir(string) string }, registry *project.Registry) *notesHandlers {
	return &notesHandlers{
		cfg:      cfg,
		stores:   stores,
		registry: registry,
	}
}

// writePayload is the JSON body for PUT /api/projects/{project}/notes/{key...}.
type writePayload struct {
	Content string   `json:"content"`
	Author  string   `json:"author"`
	Tags    []string `json:"tags"`
}

// extractKey pulls the `{key...}` wildcard suffix off a route that looks
// like `/api/projects/{project}/notes/<anything>`.
func extractKey(r *http.Request, prefix string) string {
	p := r.URL.Path
	idx := strings.Index(p, prefix)
	if idx < 0 {
		return ""
	}
	return p[idx+len(prefix):]
}

// errUnknownProject is returned by resolveProject when the slug from
// the URL path does not exist in the registry. Handlers translate this
// to a 404 response.
var errUnknownProject = errors.New("unknown project")

func (h *notesHandlers) resolveProject(r *http.Request) (slug, notesDir string, err error) {
	slug = r.PathValue("project")
	if slug == "" {
		// Fall back to middleware-resolved slug on contexts that don't
		// use the path value (shouldn't happen for these routes).
		slug = ProjectFromContext(r.Context())
	}
	if slug == "" {
		return "", "", fmt.Errorf("no project scope")
	}
	if !project.IsValidSlug(slug) {
		return "", "", fmt.Errorf("invalid project slug")
	}
	if h.registry != nil {
		if _, err := h.registry.Get(slug); err != nil {
			if errors.Is(err, project.ErrNotFound) {
				return "", "", errUnknownProject
			}
			return "", "", err
		}
	}
	notesDir = h.cfg.NotesDir(slug)
	return slug, notesDir, nil
}

func notesError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, errUnknownProject):
		writeError(w, r, http.StatusNotFound, "unknown project", err)
	case errors.Is(err, notes.ErrInvalidKey):
		writeError(w, r, http.StatusForbidden, "invalid key: "+err.Error(), err)
	case errors.Is(err, notes.ErrNotFound):
		writeError(w, r, http.StatusNotFound, "note not found", err)
	default:
		writeError(w, r, http.StatusInternalServerError, err.Error(), err)
	}
}

// projectErr is the first-class early-return helper: if resolveProject
// returned an error, translate unknown → 404 / invalid → 400 and return
// true so the handler can exit.
func projectErr(w http.ResponseWriter, r *http.Request, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, errUnknownProject) {
		writeError(w, r, http.StatusNotFound, "unknown project", err)
	} else {
		writeError(w, r, http.StatusBadRequest, err.Error(), err)
	}
	return true
}

// GET /api/projects/{project}/notes/{key...}
//
// This handler also serves the /history sub-resource — Go's ServeMux
// doesn't allow a literal segment after a `...` wildcard, so we
// dispatch on suffix here instead of registering a separate pattern.
func (h *notesHandlers) readNote(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, "/history") {
		h.noteHistory(w, r)
		return
	}
	slug, notesDir, err := h.resolveProject(r)
	if projectErr(w, r, err) {
		return
	}
	key := extractKey(r, "/notes/")
	n, err := notes.Read(notesDir, key)
	if err != nil {
		notesError(w, r, err)
		return
	}
	// Also attach outlinks so clients can render backlinks without a
	// second round-trip. Cheap for a single note.
	n.Frontmatter = nil // avoid duplicating fm; frontmatter already baked into round-trip on disk
	resp := map[string]any{
		"note":     n,
		"outlinks": notes.ExtractWikilinks([]byte(n.Content)),
		"project":  slug,
	}
	writeJSON(w, 200, resp)
}

// PUT /api/projects/{project}/notes/{key...}
func (h *notesHandlers) writeNote(w http.ResponseWriter, r *http.Request) {
	slug, notesDir, err := h.resolveProject(r)
	if projectErr(w, r, err) {
		return
	}
	key := extractKey(r, "/notes/")
	if err := notes.ValidateKey(key); err != nil {
		writeError(w, r, http.StatusForbidden, "invalid key: "+err.Error(), err)
		return
	}
	if r.ContentLength > 0 && r.ContentLength > MaxNoteBytes {
		writeError(w, r, http.StatusRequestEntityTooLarge,
			fmt.Sprintf("note exceeds %d bytes", MaxNoteBytes), nil)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, MaxNoteBytes+1))
	if err != nil {
		writeError(w, r, 400, "read body: "+err.Error(), err)
		return
	}
	if len(body) > MaxNoteBytes {
		writeError(w, r, http.StatusRequestEntityTooLarge,
			fmt.Sprintf("note exceeds %d bytes", MaxNoteBytes), nil)
		return
	}

	var p writePayload
	if err := json.Unmarshal(body, &p); err != nil {
		writeError(w, r, 400, "invalid json: "+err.Error(), err)
		return
	}
	// Decision: empty content on PUT is rejected with 400. kgraph accepts
	// empty content, but docsiq treats a notes API without content as a
	// user error — use DELETE instead to remove a note.
	if strings.TrimSpace(p.Content) == "" {
		writeError(w, r, http.StatusBadRequest, "content must not be empty (use DELETE to remove)", nil)
		return
	}

	n := &notes.Note{
		Key:     key,
		Content: p.Content,
		Author:  p.Author,
		Tags:    p.Tags,
	}
	if err := notes.Write(notesDir, n); err != nil {
		notesError(w, r, err)
		return
	}
	st, err := h.stores.Get(slug)
	if err != nil {
		notesError(w, r, err)
		return
	}
	if err := st.IndexNote(r.Context(), n); err != nil {
		slog.WarnContext(r.Context(), "⚠️ notes FTS index failed", "slug", slug, "key", key, "err", err)
	}
	writeJSON(w, 200, n)
}

// DELETE /api/projects/{project}/notes/{key...}
func (h *notesHandlers) deleteNote(w http.ResponseWriter, r *http.Request) {
	slug, notesDir, err := h.resolveProject(r)
	if projectErr(w, r, err) {
		return
	}
	key := extractKey(r, "/notes/")
	if err := notes.ValidateKey(key); err != nil {
		writeError(w, r, http.StatusForbidden, "invalid key: "+err.Error(), err)
		return
	}
	if err := notes.Delete(notesDir, key); err != nil {
		notesError(w, r, err)
		return
	}
	if st, err := h.stores.Get(slug); err == nil {
		_ = st.DeleteNote(r.Context(), key)
	}
	writeJSON(w, 200, map[string]any{"ok": true, "key": key})
}

// GET /api/projects/{project}/notes/{key...}/history?limit=<n>
//
// Returns the auto-commit history for a single note, newest first.
// `limit` is optional and capped to 500. If git is unavailable or the
// project has never been written to, this returns `{"entries": []}`
// with HTTP 200 — history is informational, not a precondition.
func (h *notesHandlers) noteHistory(w http.ResponseWriter, r *http.Request) {
	_, notesDir, err := h.resolveProject(r)
	if projectErr(w, r, err) {
		return
	}
	// Strip the trailing `/history` from the path wildcard before
	// treating the rest as a note key.
	raw := extractKey(r, "/notes/")
	key := strings.TrimSuffix(raw, "/history")
	if err := notes.ValidateKey(key); err != nil {
		writeError(w, r, http.StatusForbidden, "invalid key: "+err.Error(), err)
		return
	}
	limit := 50
	if ls := r.URL.Query().Get("limit"); ls != "" {
		if n, err := strconv.Atoi(ls); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}
	entries, err := notes.History(notesDir, key, limit)
	if err != nil {
		notesError(w, r, err)
		return
	}
	if entries == nil {
		entries = []notes.HistoryEntry{}
	}
	writeJSON(w, 200, map[string]any{"entries": entries})
}

// GET /api/projects/{project}/notes
func (h *notesHandlers) listNotes(w http.ResponseWriter, r *http.Request) {
	_, notesDir, err := h.resolveProject(r)
	if projectErr(w, r, err) {
		return
	}
	keys, err := notes.ListKeys(notesDir)
	if err != nil {
		notesError(w, r, err)
		return
	}
	if keys == nil {
		keys = []string{}
	}
	writeJSON(w, 200, map[string]any{"keys": keys})
}

// GET /api/projects/{project}/tree
func (h *notesHandlers) tree(w http.ResponseWriter, r *http.Request) {
	_, notesDir, err := h.resolveProject(r)
	if projectErr(w, r, err) {
		return
	}
	t, err := notes.Tree(notesDir)
	if err != nil {
		notesError(w, r, err)
		return
	}
	writeJSON(w, 200, t)
}

// GET /api/projects/{project}/search?q=...&limit=...
func (h *notesHandlers) searchNotes(w http.ResponseWriter, r *http.Request) {
	slug, _, err := h.resolveProject(r)
	if projectErr(w, r, err) {
		return
	}
	q := r.URL.Query().Get("q")
	limit := 20
	if ls := r.URL.Query().Get("limit"); ls != "" {
		if n, err := strconv.Atoi(ls); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}
	st, err := h.stores.Get(slug)
	if err != nil {
		notesError(w, r, err)
		return
	}
	hits, err := st.SearchNotes(r.Context(), q, limit)
	if err != nil {
		notesError(w, r, err)
		return
	}
	if hits == nil {
		hits = []store.NoteHit{}
	}
	writeJSON(w, 200, map[string]any{"query": q, "hits": hits})
}

// GET /api/projects/{project}/graph
func (h *notesHandlers) graph(w http.ResponseWriter, r *http.Request) {
	_, notesDir, err := h.resolveProject(r)
	if projectErr(w, r, err) {
		return
	}
	g, err := notes.BuildGraph(notesDir)
	if err != nil {
		notesError(w, r, err)
		return
	}
	writeJSON(w, 200, g)
}

// GET /api/projects/{project}/export → tar.gz of notes/
func (h *notesHandlers) export(w http.ResponseWriter, r *http.Request) {
	slug, notesDir, err := h.resolveProject(r)
	if projectErr(w, r, err) {
		return
	}
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-notes.tar.gz"`, slug))

	gz := gzip.NewWriter(w)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	info, err := os.Stat(notesDir)
	if err != nil || !info.IsDir() {
		// Empty export — emit a valid (empty) tar.gz.
		return
	}
	err = filepath.Walk(notesDir, func(path string, fi os.FileInfo, werr error) error {
		if werr != nil {
			return werr
		}
		if fi.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".md") {
			return nil
		}
		rel, err := filepath.Rel(notesDir, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		hdr := &tar.Header{
			Name:    filepath.ToSlash(rel),
			Mode:    0o644,
			Size:    int64(len(data)),
			ModTime: fi.ModTime(),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		_, err = tw.Write(data)
		return err
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "❌ export walk failed", "slug", slug, "err", err)
	}
}

// POST /api/projects/{project}/import → multipart tar.gz upload
func (h *notesHandlers) importTar(w http.ResponseWriter, r *http.Request) {
	slug, notesDir, err := h.resolveProject(r)
	if projectErr(w, r, err) {
		return
	}
	var reader io.Reader
	ct := r.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "multipart/") {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			writeError(w, r, 400, "parse multipart: "+err.Error(), err)
			return
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			writeError(w, r, 400, "missing 'file' field", err)
			return
		}
		defer file.Close()
		reader = file
	} else {
		reader = r.Body
	}

	gz, err := gzip.NewReader(reader)
	if err != nil {
		writeError(w, r, 400, "not a gzip stream: "+err.Error(), err)
		return
	}
	defer gz.Close()
	tr := tar.NewReader(gz)

	if err := os.MkdirAll(notesDir, 0o755); err != nil {
		writeError(w, r, 500, "mkdir notes: "+err.Error(), err)
		return
	}

	imported := 0
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			writeError(w, r, 400, "tar read: "+err.Error(), err)
			return
		}
		name := filepath.ToSlash(filepath.Clean(hdr.Name))
		// Reject traversal / absolute paths in archive entries.
		if strings.HasPrefix(name, "/") || strings.Contains(name, "..") {
			writeError(w, r, http.StatusForbidden,
				"tar entry traversal: "+hdr.Name, nil)
			return
		}
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		dest := filepath.Join(notesDir, filepath.FromSlash(name))
		// Double-check containment (defense in depth).
		absDest, _ := filepath.Abs(dest)
		absBase, _ := filepath.Abs(notesDir)
		if !strings.HasPrefix(absDest, absBase+string(os.PathSeparator)) {
			writeError(w, r, http.StatusForbidden, "tar entry escapes notes dir", nil)
			return
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			writeError(w, r, 500, "mkdir: "+err.Error(), err)
			return
		}
		data, err := io.ReadAll(io.LimitReader(tr, MaxNoteBytes+1))
		if err != nil {
			writeError(w, r, 400, "read entry: "+err.Error(), err)
			return
		}
		if len(data) > MaxNoteBytes {
			writeError(w, r, http.StatusRequestEntityTooLarge,
				"entry too large: "+hdr.Name, nil)
			return
		}
		if err := os.WriteFile(dest, data, 0o644); err != nil {
			writeError(w, r, 500, "write: "+err.Error(), err)
			return
		}
		imported++

		// Best-effort index.
		key := strings.TrimSuffix(name, ".md")
		if n, rerr := notes.Read(notesDir, key); rerr == nil {
			if st, serr := h.stores.Get(slug); serr == nil {
				_ = st.IndexNote(r.Context(), n)
			}
		}
	}
	writeJSON(w, 200, map[string]any{"ok": true, "imported": imported})
}
