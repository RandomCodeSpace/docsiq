package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/amit/docsgraphcontext/internal/config"
	"github.com/amit/docsgraphcontext/internal/embedder"
	"github.com/amit/docsgraphcontext/internal/llm"
	"github.com/amit/docsgraphcontext/internal/pipeline"
	"github.com/amit/docsgraphcontext/internal/search"
	"github.com/amit/docsgraphcontext/internal/store"
)

type handlers struct {
	store    *store.Store
	provider llm.Provider
	embedder *embedder.Embedder
	cfg      *config.Config

	// Upload progress tracking
	uploadMu   sync.Mutex
	jobProgress []string
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func (h *handlers) getStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.store.GetStats(r.Context())
	if err != nil {
		writeError(w, 500, err.Error())
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
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, docs)
}

func (h *handlers) getDocumentVersions(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	doc, err := h.store.GetDocument(r.Context(), id)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	if doc == nil {
		writeError(w, 404, "document not found")
		return
	}
	versions, err := h.store.GetDocumentVersions(r.Context(), doc.CanonicalOrID())
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, versions)
}

func (h *handlers) getDocument(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	doc, err := h.store.GetDocument(r.Context(), id)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	if doc == nil {
		writeError(w, 404, "document not found")
		return
	}
	writeJSON(w, 200, doc)
}

type searchRequest struct {
	Query          string `json:"query"`
	Mode           string `json:"mode"`    // local | global
	TopK           int    `json:"top_k"`
	GraphDepth     int    `json:"graph_depth"`
	CommunityLevel int    `json:"community_level"`
}

func (h *handlers) search(w http.ResponseWriter, r *http.Request) {
	var req searchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "invalid JSON")
		return
	}
	if req.Query == "" {
		writeError(w, 400, "query required")
		return
	}
	if req.TopK <= 0 {
		req.TopK = 5
	}
	if req.GraphDepth <= 0 {
		req.GraphDepth = 2
	}

	ctx := r.Context()
	switch req.Mode {
	case "global":
		result, err := search.GlobalSearch(ctx, h.store, h.embedder, h.provider, req.Query, req.CommunityLevel)
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		writeJSON(w, 200, result)
	default: // local
		result, err := search.LocalSearch(ctx, h.store, h.embedder, req.Query, req.TopK, req.GraphDepth)
		if err != nil {
			writeError(w, 500, err.Error())
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
		writeError(w, 400, "entity parameter required")
		return
	}

	ctx := r.Context()
	entity, err := h.store.GetEntityByName(ctx, name)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	if entity == nil {
		writeJSON(w, 200, map[string]any{"nodes": []any{}, "edges": []any{}})
		return
	}

	rels, err := h.store.RelationshipsForEntity(ctx, entity.ID, depth)
	if err != nil {
		writeError(w, 500, err.Error())
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

	writeJSON(w, 200, map[string]any{"nodes": nodes, "edges": edges})
}

func (h *handlers) listEntities(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	typ := q.Get("type")
	limit := intQuery(q.Get("limit"), 20)
	offset := intQuery(q.Get("offset"), 0)

	entities, err := h.store.ListEntities(r.Context(), typ, limit, offset)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, entities)
}

func (h *handlers) listCommunities(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	level := intQuery(q.Get("level"), -1)

	communities, err := h.store.ListCommunities(r.Context(), level)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, communities)
}

func (h *handlers) getCommunity(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	comm, err := h.store.GetCommunity(r.Context(), id)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	if comm == nil {
		writeError(w, 404, "community not found")
		return
	}
	members, err := h.store.CommunityMembers(r.Context(), id)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"community": comm, "members": members})
}

func (h *handlers) upload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(128 << 20); err != nil {
		writeError(w, 400, "parse form: "+err.Error())
		return
	}
	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		writeError(w, 400, "no files provided")
		return
	}

	tmpDir, err := os.MkdirTemp("", "docsgraph-upload-*")
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}

	var paths []string
	for _, fh := range files {
		dst := filepath.Join(tmpDir, fh.Filename)
		f, err := fh.Open()
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		out, err := os.Create(dst)
		if err != nil {
			f.Close()
			writeError(w, 500, err.Error())
			return
		}
		_, err = io.Copy(out, f)
		f.Close()
		out.Close()
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		paths = append(paths, dst)
	}

	jobID := fmt.Sprintf("job-%d", len(h.jobProgress))
	h.uploadMu.Lock()
	h.jobProgress = append(h.jobProgress, fmt.Sprintf("queued: %d files", len(paths)))
	h.uploadMu.Unlock()

	// Run indexing in background
	go func() {
		defer os.RemoveAll(tmpDir)
		pl := pipeline.New(h.store, h.provider, h.cfg)
		for _, p := range paths {
			h.setProgress(jobID, fmt.Sprintf("indexing: %s", filepath.Base(p)))
			if err := pl.IndexPath(r.Context(), p, pipeline.IndexOptions{}); err != nil {
				h.setProgress(jobID, fmt.Sprintf("error: %v", err))
				return
			}
		}
		h.setProgress(jobID, "done")
	}()

	writeJSON(w, 202, map[string]string{"job_id": jobID, "status": "queued"})
}

func (h *handlers) setProgress(jobID, msg string) {
	h.uploadMu.Lock()
	defer h.uploadMu.Unlock()
	_ = jobID
	if len(h.jobProgress) > 0 {
		h.jobProgress[len(h.jobProgress)-1] = msg
	}
}

func (h *handlers) uploadProgress(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	h.uploadMu.Lock()
	progress := make([]string, len(h.jobProgress))
	copy(progress, h.jobProgress)
	h.uploadMu.Unlock()

	for _, p := range progress {
		fmt.Fprintf(w, "data: %s\n\n", p)
	}
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
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
