package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/RandomCodeSpace/docscontext/internal/config"
	"github.com/RandomCodeSpace/docscontext/internal/embedder"
	"github.com/RandomCodeSpace/docscontext/internal/llm"
	"github.com/RandomCodeSpace/docscontext/internal/pipeline"
	"github.com/RandomCodeSpace/docscontext/internal/project"
	"github.com/RandomCodeSpace/docscontext/internal/search"
	"github.com/RandomCodeSpace/docscontext/internal/store"
)

type handlers struct {
	store    *store.Store
	provider llm.Provider
	embedder *embedder.Embedder
	cfg      *config.Config

	// Upload progress tracking
	uploadMu    sync.Mutex
	jobProgress map[string]string
	jobCounter  atomic.Int64
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, r *http.Request, status int, msg string, err error) {
	if status >= 500 && err != nil {
		slog.ErrorContext(r.Context(), "❌ handler error", "path", r.URL.Path, "err", err)
	}
	writeJSON(w, status, map[string]string{"error": msg})
}

// health is a trivially-always-200 liveness probe. No store/config
// dependency so it works even if the backend is degraded, and the
// auth middleware explicitly whitelists /health.
func (h *handlers) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

// projectsHandler is a thin read-only JSON shim around registry.List()
// so the Phase-4 UI can populate its project-selector dropdown.
type projectsHandler struct {
	registry *project.Registry
}

// listProjects returns registered projects as a JSON array. Falls back
// to [{"slug":"_default","name":"_default"}] when the registry is nil
// or empty so the UI always has a usable default selection.
func (p *projectsHandler) listProjects(w http.ResponseWriter, r *http.Request) {
	type projInfo struct {
		Slug string `json:"slug"`
		Name string `json:"name"`
	}
	out := []projInfo{}
	if p.registry != nil {
		projs, err := p.registry.List()
		if err == nil {
			for _, pr := range projs {
				out = append(out, projInfo{Slug: pr.Slug, Name: pr.Name})
			}
		}
	}
	if len(out) == 0 {
		out = append(out, projInfo{Slug: "_default", Name: "_default"})
	}
	writeJSON(w, 200, out)
}

func (h *handlers) getStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.store.GetStats(r.Context())
	if err != nil {
		writeError(w, r, 500, err.Error(), err)
		return
	}
	writeJSON(w, 200, stats)
}

func (h *handlers) listDocuments(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	docType := q.Get("doc_type")
	limit := intQuery(q.Get("limit"), 20)
	offset := intQuery(q.Get("offset"), 0)

	docs, err := h.store.ListDocuments(r.Context(), docType, limit, offset)
	if err != nil {
		writeError(w, r, 500, err.Error(), err)
		return
	}
	writeJSON(w, 200, docs)
}

func (h *handlers) getDocumentVersions(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	doc, err := h.store.GetDocument(r.Context(), id)
	if err != nil {
		writeError(w, r, 500, err.Error(), err)
		return
	}
	if doc == nil {
		writeError(w, r, 404, "document not found", nil)
		return
	}
	versions, err := h.store.GetDocumentVersions(r.Context(), doc.CanonicalOrID())
	if err != nil {
		writeError(w, r, 500, err.Error(), err)
		return
	}
	writeJSON(w, 200, versions)
}

func (h *handlers) getDocument(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	doc, err := h.store.GetDocument(r.Context(), id)
	if err != nil {
		writeError(w, r, 500, err.Error(), err)
		return
	}
	if doc == nil {
		writeError(w, r, 404, "document not found", nil)
		return
	}
	writeJSON(w, 200, doc)
}

type searchRequest struct {
	Query          string `json:"query"`
	Mode           string `json:"mode"` // local | global
	TopK           int    `json:"top_k"`
	GraphDepth     int    `json:"graph_depth"`
	CommunityLevel int    `json:"community_level"`
}

func (h *handlers) search(w http.ResponseWriter, r *http.Request) {
	var req searchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, 400, "invalid JSON", nil)
		return
	}
	if req.Query == "" {
		writeError(w, r, 400, "query required", nil)
		return
	}
	if req.TopK <= 0 {
		req.TopK = 5
	}
	if req.GraphDepth <= 0 {
		req.GraphDepth = 2
	}

	slog.InfoContext(r.Context(), "🔍 search request", "mode", req.Mode, "query", req.Query, "top_k", req.TopK)

	ctx := r.Context()
	switch req.Mode {
	case "global":
		result, err := search.GlobalSearch(ctx, h.store, h.embedder, h.provider, req.Query, req.CommunityLevel)
		if err != nil {
			writeError(w, r, 500, err.Error(), err)
			return
		}
		writeJSON(w, 200, result)
	default: // local
		result, err := search.LocalSearch(ctx, h.store, h.embedder, req.Query, req.TopK, req.GraphDepth)
		if err != nil {
			writeError(w, r, 500, err.Error(), err)
			return
		}
		writeJSON(w, 200, result)
	}
}

func (h *handlers) graphNeighborhood(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	name := q.Get("entity")
	depth := intQuery(q.Get("depth"), 2)
	maxNodes := intQuery(q.Get("max_nodes"), 50)

	if name == "" {
		writeError(w, r, 400, "entity parameter required", nil)
		return
	}

	slog.DebugContext(r.Context(), "🔗 graph neighborhood request", "entity", name, "depth", depth)

	ctx := r.Context()
	entity, err := h.store.GetEntityByName(ctx, name)
	if err != nil {
		writeError(w, r, 500, err.Error(), err)
		return
	}
	if entity == nil {
		writeJSON(w, 404, map[string]any{"error": "entity not found", "nodes": []any{}, "edges": []any{}})
		return
	}

	rels, err := h.store.RelationshipsForEntity(ctx, entity.ID, depth)
	if err != nil {
		writeError(w, r, 500, err.Error(), err)
		return
	}

	nodeIDs := map[string]bool{entity.ID: true}
	for _, r2 := range rels {
		nodeIDs[r2.SourceID] = true
		nodeIDs[r2.TargetID] = true
	}

	var nodes []map[string]any
	count := 0
	for nid := range nodeIDs {
		if count >= maxNodes {
			break
		}
		e, err := h.store.GetEntity(ctx, nid)
		if err != nil || e == nil {
			continue
		}
		nodes = append(nodes, map[string]any{
			"id": e.ID, "label": e.Name, "type": e.Type,
			"description": e.Description, "rank": e.Rank,
		})
		count++
	}

	var edges []map[string]any
	for _, r2 := range rels {
		edges = append(edges, map[string]any{
			"id": r2.ID, "from": r2.SourceID, "to": r2.TargetID,
			"label": r2.Predicate, "weight": r2.Weight,
		})
	}

	slog.DebugContext(r.Context(), "🔗 graph neighborhood result", "entity", name, "nodes", len(nodes), "edges", len(edges))
	writeJSON(w, 200, map[string]any{"nodes": nodes, "edges": edges})
}

func (h *handlers) listEntities(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	typ := q.Get("type")
	limit := intQuery(q.Get("limit"), 20)
	offset := intQuery(q.Get("offset"), 0)

	entities, err := h.store.ListEntities(r.Context(), typ, limit, offset)
	if err != nil {
		writeError(w, r, 500, err.Error(), err)
		return
	}
	writeJSON(w, 200, entities)
}

func (h *handlers) listCommunities(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	level := intQuery(q.Get("level"), -1)

	communities, err := h.store.ListCommunities(r.Context(), level)
	if err != nil {
		writeError(w, r, 500, err.Error(), err)
		return
	}
	writeJSON(w, 200, communities)
}

func (h *handlers) getCommunity(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	comm, err := h.store.GetCommunity(r.Context(), id)
	if err != nil {
		writeError(w, r, 500, err.Error(), err)
		return
	}
	if comm == nil {
		writeError(w, r, 404, "community not found", nil)
		return
	}
	members, err := h.store.CommunityMembers(r.Context(), id)
	if err != nil {
		writeError(w, r, 500, err.Error(), err)
		return
	}
	writeJSON(w, 200, map[string]any{"community": comm, "members": members})
}

func (h *handlers) upload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(128 << 20); err != nil {
		writeError(w, r, 400, "parse form: "+err.Error(), nil)
		return
	}
	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		writeError(w, r, 400, "no files provided", nil)
		return
	}

	tmpDir, err := os.MkdirTemp("", "DocsContext-upload-*")
	if err != nil {
		writeError(w, r, 500, err.Error(), err)
		return
	}
	// Clean up tmpDir if we return early before the background goroutine takes ownership.
	tmpDirOwned := false
	defer func() {
		if !tmpDirOwned {
			os.RemoveAll(tmpDir)
		}
	}()

	var paths []string
	for _, fh := range files {
		dst := filepath.Join(tmpDir, fh.Filename)
		f, err := fh.Open()
		if err != nil {
			writeError(w, r, 500, err.Error(), err)
			return
		}
		out, err := os.Create(dst)
		if err != nil {
			f.Close()
			writeError(w, r, 500, err.Error(), err)
			return
		}
		_, err = io.Copy(out, f)
		f.Close()
		out.Close()
		if err != nil {
			writeError(w, r, 500, err.Error(), err)
			return
		}
		paths = append(paths, dst)
	}

	jobID := fmt.Sprintf("job-%d", h.jobCounter.Add(1))
	slog.Info("📦 upload job queued", "job_id", jobID, "files", len(paths))

	h.setProgress(jobID, fmt.Sprintf("queued: %d files", len(paths)))

	// Use a detached context so the background goroutine is not cancelled
	// when the HTTP response is sent.
	bgCtx := context.Background()
	tmpDirOwned = true

	go func() {
		defer os.RemoveAll(tmpDir)
		pl := pipeline.New(h.store, h.provider, h.cfg)
		for _, p := range paths {
			slog.Info("📦 upload indexing file", "job_id", jobID, "file", filepath.Base(p))
			h.setProgress(jobID, fmt.Sprintf("indexing: %s", filepath.Base(p)))
			if err := pl.IndexPath(bgCtx, p, pipeline.IndexOptions{}); err != nil {
				slog.Error("❌ upload indexing failed", "job_id", jobID, "file", filepath.Base(p), "err", err)
				h.setProgress(jobID, fmt.Sprintf("error: %v", err))
				return
			}
		}
		h.setProgress(jobID, "finalizing")
		if err := pl.Finalize(bgCtx, false, true); err != nil {
			slog.Warn("⚠️ upload finalization failed", "job_id", jobID, "err", err)
		}
		slog.Info("✅ upload job complete", "job_id", jobID, "files", len(paths))
		h.setProgress(jobID, "done")
	}()

	writeJSON(w, 202, map[string]string{"job_id": jobID, "status": "queued"})
}

func (h *handlers) setProgress(jobID, msg string) {
	h.uploadMu.Lock()
	defer h.uploadMu.Unlock()
	if h.jobProgress == nil {
		h.jobProgress = make(map[string]string)
	}
	h.jobProgress[jobID] = msg
}

func (h *handlers) uploadProgress(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	ctx := r.Context()
	lastMsg := ""
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.uploadMu.Lock()
			// Send the latest message from any job.
			var msg string
			for _, v := range h.jobProgress {
				msg = v
			}
			h.uploadMu.Unlock()

			if msg != "" && msg != lastMsg {
				fmt.Fprintf(w, "data: %s\n\n", msg)
				flusher.Flush()
				lastMsg = msg
				if msg == "done" || strings.HasPrefix(msg, "error:") {
					return
				}
			}
		}
	}
}

func intQuery(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}


