package api

import (
	"context"
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
	"sync"
	"sync/atomic"
	"time"

	"github.com/RandomCodeSpace/docsiq/internal/config"
	"github.com/RandomCodeSpace/docsiq/internal/embedder"
	"github.com/RandomCodeSpace/docsiq/internal/llm"
	"github.com/RandomCodeSpace/docsiq/internal/pipeline"
	"github.com/RandomCodeSpace/docsiq/internal/project"
	"github.com/RandomCodeSpace/docsiq/internal/search"
	"github.com/RandomCodeSpace/docsiq/internal/store"
	"github.com/RandomCodeSpace/docsiq/internal/vectorindex"
	"github.com/RandomCodeSpace/docsiq/internal/workq"
)

// handlers is the REST-side doc router state. Wave-2 drop: the
// long-lived *store.Store was removed — every method now resolves a
// per-project handle via h.stores.ForProject(ProjectFromContext(...)).
type handlers struct {
	stores   Storer
	provider llm.Provider
	embedder *embedder.Embedder
	cfg      *config.Config
	// vecIndexes is the per-project HNSW cache. Built lazily on first
	// local search (or eagerly at boot for registered projects). May
	// return nil for a slug with no embeddings; LocalSearch falls back
	// to brute-force in that case.
	vecIndexes *VectorIndexes
	// workq is the bounded worker pool for upload indexing jobs. When
	// nil (dev/test path), upload() falls back to a detached goroutine.
	workq *workq.Pool

	// Upload progress tracking. jobProgress is the legacy plain-string
	// channel — kept so existing tests / callers without a job channel
	// still see the most recent state. jobEvents holds the structured
	// per-phase event log used by the SSE handler.
	uploadMu    sync.Mutex
	jobProgress map[string]string
	jobEvents   map[string]*jobProgressLog
	jobCounter  atomic.Int64
}

// jobProgressLog accumulates structured upload events for one job.
// Append is O(1), the SSE handler walks the slice from a per-stream
// cursor. done is set when a terminal event lands; the handler uses
// it to flush and close the stream.
type jobProgressLog struct {
	mu     sync.Mutex
	events []uploadEvent
	done   bool
	cond   *sync.Cond
}

// uploadEvent is the JSON shape streamed over GET /api/upload/progress.
// Field order intentionally matches the wire format documented in
// docs/rest-api.md so the UI hook can rely on it.
type uploadEvent struct {
	JobID       string `json:"job_id"`
	File        string `json:"file,omitempty"`
	Phase       string `json:"phase"`
	ChunksDone  int    `json:"chunks_done,omitempty"`
	ChunksTotal int    `json:"chunks_total,omitempty"`
	Message     string `json:"message,omitempty"`
	Done        bool   `json:"done"`
	Error       string `json:"error,omitempty"`
}

// resolveStore is the single entry point for every doc handler. It
// pulls the slug off ctx and returns the matching per-project store.
// A missing/empty slug or open failure becomes a 500 — the project
// middleware has already ensured the slug is registered, so an error
// here is an infra problem (disk, permissions) not a user mistake.
func (h *handlers) resolveStore(w http.ResponseWriter, r *http.Request) (*store.Store, bool) {
	slug := ProjectFromContext(r.Context())
	if slug == "" {
		writeError(w, r, http.StatusInternalServerError, "project scope missing", nil)
		return nil, false
	}
	st, err := h.stores.ForProject(slug)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "open project store: "+err.Error(), err)
		return nil, false
	}
	return st, true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, r *http.Request, status int, msg string, err error) {
	if status >= 500 && err != nil {
		ContextLogger(r.Context()).Error("❌ handler error", "path", r.URL.Path, "err", err)
	}
	// NF-P1-3: docs/rest-api.md promises error bodies of shape
	// {"error":"...","request_id":"..."}. Echo the per-request ID into
	// the body so UI clients can correlate errors without scraping the
	// X-Request-ID response header.
	body := map[string]string{"error": msg}
	if reqID := RequestIDFromContext(r.Context()); reqID != "" {
		body["request_id"] = reqID
	}
	writeJSON(w, status, body)
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
	st, ok := h.resolveStore(w, r)
	if !ok {
		return
	}
	stats, err := st.GetStats(r.Context())
	if err != nil {
		writeError(w, r, 500, err.Error(), err)
		return
	}
	writeJSON(w, 200, stats)
}

func (h *handlers) listDocuments(w http.ResponseWriter, r *http.Request) {
	st, ok := h.resolveStore(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	docType := q.Get("doc_type")
	limit := intQuery(q.Get("limit"), 20)
	offset := intQuery(q.Get("offset"), 0)

	docs, err := st.ListDocuments(r.Context(), docType, limit, offset)
	if err != nil {
		writeError(w, r, 500, err.Error(), err)
		return
	}
	writeJSON(w, 200, docs)
}

func (h *handlers) getDocumentVersions(w http.ResponseWriter, r *http.Request) {
	st, ok := h.resolveStore(w, r)
	if !ok {
		return
	}
	id := r.PathValue("id")
	doc, err := st.GetDocument(r.Context(), id)
	if err != nil {
		writeError(w, r, 500, err.Error(), err)
		return
	}
	if doc == nil {
		writeError(w, r, 404, "document not found", nil)
		return
	}
	versions, err := st.GetDocumentVersions(r.Context(), doc.CanonicalOrID())
	if err != nil {
		writeError(w, r, 500, err.Error(), err)
		return
	}
	writeJSON(w, 200, versions)
}

func (h *handlers) getDocument(w http.ResponseWriter, r *http.Request) {
	st, ok := h.resolveStore(w, r)
	if !ok {
		return
	}
	id := r.PathValue("id")
	doc, err := st.GetDocument(r.Context(), id)
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

func (h *handlers) getDocumentChunks(w http.ResponseWriter, r *http.Request) {
	st, ok := h.resolveStore(w, r)
	if !ok {
		return
	}
	id := r.PathValue("id")
	doc, err := st.GetDocument(r.Context(), id)
	if err != nil {
		writeError(w, r, 500, err.Error(), err)
		return
	}
	if doc == nil {
		writeError(w, r, 404, "document not found", nil)
		return
	}
	chunks, err := st.ListChunksByDoc(r.Context(), id)
	if err != nil {
		writeError(w, r, 500, err.Error(), err)
		return
	}
	out := make([]map[string]any, 0, len(chunks))
	for _, c := range chunks {
		out = append(out, map[string]any{
			"id":          c.ID,
			"chunk_index": c.ChunkIndex,
			"content":     c.Content,
			"token_count": c.TokenCount,
		})
	}
	writeJSON(w, 200, out)
}

// deleteDocument hard-deletes a document and cascades cleanup of
// chunks, embeddings, claims, doc-scoped relationships, and any
// entities that no longer have a relationship or claim referencing
// them.
//
// Communities are deliberately NOT recomputed inline. Louvain on a
// graph of any meaningful size is too slow for an interactive DELETE,
// and stale community titles/summaries are tolerable until the next
// manual finalize. The vector index is invalidated so the next search
// rebuilds against the post-delete chunk set.
//
// Returns 204 on success, 404 if the id does not exist, 500 on
// transactional failure (the store rolls back so the graph stays
// consistent).
func (h *handlers) deleteDocument(w http.ResponseWriter, r *http.Request) {
	st, ok := h.resolveStore(w, r)
	if !ok {
		return
	}
	id := r.PathValue("id")
	doc, err := st.GetDocument(r.Context(), id)
	if err != nil {
		writeError(w, r, 500, err.Error(), err)
		return
	}
	if doc == nil {
		writeError(w, r, 404, "document not found", nil)
		return
	}

	affected, err := st.DeleteDocument(r.Context(), id)
	if err != nil {
		writeError(w, r, 500, fmt.Errorf("delete document: %w", err).Error(), err)
		return
	}
	if affected == 0 {
		// Race: the row vanished between our GetDocument and the
		// delete. Treat as 404 — the operation succeeded only if it
		// actually removed something.
		writeError(w, r, 404, "document not found", nil)
		return
	}

	// Drop the cached vector index for this project so the next local
	// search rebuilds against the post-delete chunk set. Safe on a nil
	// receiver.
	slug := ProjectFromContext(r.Context())
	h.vecIndexes.Invalidate(slug)

	slog.Info("🗑️  document deleted",
		"project", slug,
		"doc_id", id,
		"path", doc.Path,
	)

	w.WriteHeader(http.StatusNoContent)
}

func (h *handlers) entityGraph(w http.ResponseWriter, r *http.Request) {
	st, ok := h.resolveStore(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	limit := intQuery(q.Get("limit"), 500)
	typ := q.Get("type")

	entities, err := st.ListEntities(r.Context(), typ, limit, 0)
	if err != nil {
		writeError(w, r, 500, err.Error(), err)
		return
	}
	rels, err := st.AllRelationships(r.Context())
	if err != nil {
		writeError(w, r, 500, err.Error(), err)
		return
	}

	nodes := make([]map[string]any, 0, len(entities))
	keep := make(map[string]bool, len(entities))
	for _, e := range entities {
		keep[e.ID] = true
		nodes = append(nodes, map[string]any{
			"id":          e.ID,
			"label":       e.Name,
			"kind":        "entity",
			"type":        e.Type,
			"description": e.Description,
			"rank":        e.Rank,
			"community":   e.CommunityID,
		})
	}
	edges := make([]map[string]any, 0)
	for _, rel := range rels {
		if !keep[rel.SourceID] || !keep[rel.TargetID] {
			continue
		}
		edges = append(edges, map[string]any{
			"id":     rel.ID,
			"source": rel.SourceID,
			"target": rel.TargetID,
			"label":  rel.Predicate,
			"weight": rel.Weight,
		})
	}
	writeJSON(w, 200, map[string]any{"nodes": nodes, "edges": edges})
}

type searchRequest struct {
	Query          string `json:"query"`
	Mode           string `json:"mode"` // local | global
	TopK           int    `json:"top_k"`
	GraphDepth     int    `json:"graph_depth"`
	CommunityLevel int    `json:"community_level"`
}

func (h *handlers) search(w http.ResponseWriter, r *http.Request) {
	if h.provider == nil || h.embedder == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "LLM not configured; set llm.provider in config",
			"code":  "llm_disabled",
		})
		return
	}
	st, ok := h.resolveStore(w, r)
	if !ok {
		return
	}
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
	slug := ProjectFromContext(ctx)
	// Resolve per-project LLM provider. Falls back to h.provider (root
	// config) when no override is configured for the slug or when the
	// slug is empty.
	prov := h.provider
	if h.cfg != nil {
		if p, err := llm.ProviderForProject(h.cfg, slug); err == nil && p != nil {
			prov = p
		}
	}
	var idx vectorindex.Index
	if h.vecIndexes != nil {
		idx = h.vecIndexes.ForProject(slug, st)
	}
	switch req.Mode {
	case "global":
		result, err := search.GlobalSearch(ctx, st, h.embedder, prov, req.Query, req.CommunityLevel)
		if err != nil {
			writeError(w, r, 500, err.Error(), err)
			return
		}
		writeJSON(w, 200, result)
	default: // local
		result, err := search.LocalSearch(ctx, st, h.embedder, idx, req.Query, req.TopK, req.GraphDepth)
		if err != nil {
			writeError(w, r, 500, err.Error(), err)
			return
		}
		writeJSON(w, 200, result)
	}
}

func (h *handlers) graphNeighborhood(w http.ResponseWriter, r *http.Request) {
	st, ok := h.resolveStore(w, r)
	if !ok {
		return
	}
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
	entity, err := st.GetEntityByName(ctx, name)
	if err != nil {
		writeError(w, r, 500, err.Error(), err)
		return
	}
	if entity == nil {
		writeJSON(w, 404, map[string]any{"error": "entity not found", "nodes": []any{}, "edges": []any{}})
		return
	}

	rels, err := st.RelationshipsForEntity(ctx, entity.ID, depth)
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
		e, err := st.GetEntity(ctx, nid)
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
	st, ok := h.resolveStore(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	typ := q.Get("type")
	limit := intQuery(q.Get("limit"), 20)
	offset := intQuery(q.Get("offset"), 0)

	entities, err := st.ListEntities(r.Context(), typ, limit, offset)
	if err != nil {
		writeError(w, r, 500, err.Error(), err)
		return
	}
	writeJSON(w, 200, entities)
}

func (h *handlers) listCommunities(w http.ResponseWriter, r *http.Request) {
	st, ok := h.resolveStore(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	level := intQuery(q.Get("level"), -1)

	communities, err := st.ListCommunities(r.Context(), level)
	if err != nil {
		writeError(w, r, 500, err.Error(), err)
		return
	}
	writeJSON(w, 200, communities)
}

func (h *handlers) getCommunity(w http.ResponseWriter, r *http.Request) {
	st, ok := h.resolveStore(w, r)
	if !ok {
		return
	}
	id := r.PathValue("id")
	comm, err := st.GetCommunity(r.Context(), id)
	if err != nil {
		writeError(w, r, 500, err.Error(), err)
		return
	}
	if comm == nil {
		writeError(w, r, 404, "community not found", nil)
		return
	}
	members, err := st.CommunityMembers(r.Context(), id)
	if err != nil {
		writeError(w, r, 500, err.Error(), err)
		return
	}
	writeJSON(w, 200, map[string]any{"community": comm, "members": members})
}

func (h *handlers) upload(w http.ResponseWriter, r *http.Request) {
	if h.provider == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "LLM not configured; set llm.provider in config",
			"code":  "llm_disabled",
		})
		return
	}
	st, ok := h.resolveStore(w, r)
	if !ok {
		return
	}
	slug := ProjectFromContext(r.Context())
	if !enforceUploadLimit(w, r, h.cfg.Server.MaxUploadBytes) {
		return
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		// MaxBytesReader translates overflow into an error here; the
		// response header is already 413 when that happens. For other
		// malformed-form errors we emit a 400.
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			// http.MaxBytesReader has already called w.WriteHeader(413)
			// internally; calling it again would emit "http: superfluous
			// response.WriteHeader call". Just return.
			return
		}
		writeError(w, r, 400, "parse form: "+err.Error(), nil)
		return
	}
	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		writeError(w, r, 400, "no files provided", nil)
		return
	}

	tmpDir, err := os.MkdirTemp("", "docsiq-upload-*")
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

	absTmp, err := filepath.Abs(tmpDir)
	if err != nil {
		writeError(w, r, 500, err.Error(), err)
		return
	}

	var paths []string
	for _, fh := range files {
		// Defense against multipart filename path traversal (P0-2). Strip
		// directory components first, reject degenerate names, then assert
		// absolute-path containment before creating the file.
		name := filepath.Base(fh.Filename)
		if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\") {
			ContextLogger(r.Context()).Warn("⚠️ upload: skipping invalid filename", "filename", fh.Filename)
			continue
		}
		dst := filepath.Join(tmpDir, name)
		absDst, err := filepath.Abs(dst)
		if err != nil {
			writeError(w, r, 500, err.Error(), err)
			return
		}
		if !strings.HasPrefix(absDst, absTmp+string(os.PathSeparator)) {
			ContextLogger(r.Context()).Warn("⚠️ upload: entry escapes tmp dir; skipping",
				"filename", fh.Filename, "resolved", absDst)
			continue
		}
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
	if len(paths) == 0 {
		writeError(w, r, 400, "no valid files provided", nil)
		return
	}

	jobID := fmt.Sprintf("job-%d", h.jobCounter.Add(1))
	slog.Info("📦 upload job queued", "job_id", jobID, "files", len(paths), "project", slug)

	h.setProgress(jobID, fmt.Sprintf("queued: %d files", len(paths)))
	h.appendEvent(jobID, uploadEvent{
		JobID:   jobID,
		Phase:   "queued",
		Message: fmt.Sprintf("queued: %d files", len(paths)),
	})

	job := func(ctx context.Context) {
		defer os.RemoveAll(tmpDir)
		pl := pipeline.New(st, h.provider, h.cfg)

		// Per-job pipeline progress channel. Buffered so a slow SSE
		// consumer never stalls indexing; the pipeline emit() helper is
		// also non-blocking so we are belt-and-braces here.
		progressCh := make(chan pipeline.ProgressEvent, 32)
		stopRelay := make(chan struct{})
		var relayDone sync.WaitGroup
		relayDone.Add(1)
		go func() {
			defer relayDone.Done()
			for {
				select {
				case evt, ok := <-progressCh:
					if !ok {
						return
					}
					errStr := ""
					if evt.Error != nil {
						errStr = evt.Error.Error()
					}
					h.appendEvent(jobID, uploadEvent{
						JobID:       jobID,
						File:        evt.File,
						Phase:       evt.Phase,
						ChunksDone:  evt.ChunksDone,
						ChunksTotal: evt.ChunksTotal,
						Message:     evt.Message,
						Done:        evt.Done,
						Error:       errStr,
					})
				case <-stopRelay:
					return
				}
			}
		}()

		for _, p := range paths {
			if ctx.Err() != nil {
				slog.Warn("🛑 upload indexing cancelled on shutdown", "job_id", jobID, "file", filepath.Base(p))
				h.setProgress(jobID, "cancelled")
				h.finishEvent(jobID, uploadEvent{
					JobID:   jobID,
					File:    filepath.Base(p),
					Phase:   "cancelled",
					Message: "cancelled on shutdown",
					Done:    true,
				})
				close(stopRelay)
				relayDone.Wait()
				return
			}
			slog.Info("📦 upload indexing file", "job_id", jobID, "file", filepath.Base(p))
			h.setProgress(jobID, fmt.Sprintf("indexing: %s", filepath.Base(p)))
			h.appendEvent(jobID, uploadEvent{
				JobID:   jobID,
				File:    filepath.Base(p),
				Phase:   "indexing",
				Message: "indexing " + filepath.Base(p),
			})
			if err := pl.IndexPath(ctx, p, pipeline.IndexOptions{Progress: progressCh}); err != nil {
				slog.Error("❌ upload indexing failed", "job_id", jobID, "file", filepath.Base(p), "err", err)
				h.setProgress(jobID, fmt.Sprintf("error: %v", err))
				h.finishEvent(jobID, uploadEvent{
					JobID:   jobID,
					File:    filepath.Base(p),
					Phase:   "error",
					Message: err.Error(),
					Done:    true,
					Error:   err.Error(),
				})
				close(stopRelay)
				relayDone.Wait()
				return
			}
		}
		h.setProgress(jobID, "finalizing")
		h.appendEvent(jobID, uploadEvent{
			JobID:   jobID,
			Phase:   "finalize",
			Message: "running community detection and summaries",
		})
		if err := pl.Finalize(ctx, false, true); err != nil {
			slog.Warn("⚠️ upload finalization failed", "job_id", jobID, "err", err)
			h.appendEvent(jobID, uploadEvent{
				JobID:   jobID,
				Phase:   "finalize",
				Message: "finalize warning: " + err.Error(),
			})
		}
		// Invalidate the vector index for this project so the next
		// search rebuild picks up the newly-indexed chunks.
		if h.vecIndexes != nil {
			h.vecIndexes.Invalidate(slug)
		}
		slog.Info("✅ upload job complete", "job_id", jobID, "files", len(paths), "project", slug)
		h.setProgress(jobID, "done")
		h.finishEvent(jobID, uploadEvent{
			JobID:   jobID,
			Phase:   "done",
			Message: fmt.Sprintf("indexed %d files", len(paths)),
			Done:    true,
		})
		close(stopRelay)
		relayDone.Wait()
	}

	if h.workq == nil {
		tmpDirOwned = true
		go job(context.Background()) // dev/test fallback
	} else {
		if err := h.workq.Submit(job); err != nil {
			if errors.Is(err, workq.ErrQueueFull) {
				h.setProgress(jobID, "rejected: indexing queue full")
				h.finishEvent(jobID, uploadEvent{
					JobID:   jobID,
					Phase:   "error",
					Message: "indexing queue full; retry later",
					Done:    true,
					Error:   "indexing queue full",
				})
				w.Header().Set("Retry-After", "30")
				writeError(w, r, http.StatusServiceUnavailable, "indexing queue full; retry later", nil)
				return
			}
			h.setProgress(jobID, "rejected: server unavailable")
			h.finishEvent(jobID, uploadEvent{
				JobID:   jobID,
				Phase:   "error",
				Message: "server shutting down",
				Done:    true,
				Error:   "server shutting down",
			})
			writeError(w, r, http.StatusServiceUnavailable, "server shutting down", err)
			return
		}
		tmpDirOwned = true
	}

	writeJSON(w, http.StatusAccepted, map[string]string{"job_id": jobID, "status": "accepted"})
}

func (h *handlers) setProgress(jobID, msg string) {
	h.uploadMu.Lock()
	defer h.uploadMu.Unlock()
	if h.jobProgress == nil {
		h.jobProgress = make(map[string]string)
	}
	h.jobProgress[jobID] = msg
}

// progressForJob returns the latest status for jobID. When jobID is
// empty it falls back to the Wave-A behavior (any job's latest
// message) so existing clients that don't supply ?job_id= still see
// something. (P1-1)
func (h *handlers) progressForJob(jobID string) (msg string, ok bool) {
	h.uploadMu.Lock()
	defer h.uploadMu.Unlock()
	if jobID != "" {
		m, found := h.jobProgress[jobID]
		return m, found
	}
	for _, v := range h.jobProgress {
		msg = v
		ok = true
	}
	return
}

// clearProgress removes jobID from the in-memory progress map. Called
// when the SSE stream observes a terminal status so the map does not
// grow unbounded over a server's lifetime. (P1-1)
func (h *handlers) clearProgress(jobID string) {
	if jobID == "" {
		return
	}
	h.uploadMu.Lock()
	defer h.uploadMu.Unlock()
	delete(h.jobProgress, jobID)
}

// appendEvent records a structured upload event for jobID. Wakes any
// SSE streamers blocked on the cond. Done flag must NOT be set here —
// terminal events go through finishEvent so the cleanup path stays
// in one place.
func (h *handlers) appendEvent(jobID string, evt uploadEvent) {
	if jobID == "" {
		return
	}
	log := h.getOrCreateEventLog(jobID)
	log.mu.Lock()
	log.events = append(log.events, evt)
	log.cond.Broadcast()
	log.mu.Unlock()
}

// finishEvent appends a terminal event and marks the log done.
// SSE streamers wake, flush the final event, and disconnect.
func (h *handlers) finishEvent(jobID string, evt uploadEvent) {
	if jobID == "" {
		return
	}
	log := h.getOrCreateEventLog(jobID)
	log.mu.Lock()
	evt.Done = true
	log.events = append(log.events, evt)
	log.done = true
	log.cond.Broadcast()
	log.mu.Unlock()
}

// getOrCreateEventLog returns the per-job structured log, allocating
// on first access. Holds uploadMu only briefly.
func (h *handlers) getOrCreateEventLog(jobID string) *jobProgressLog {
	h.uploadMu.Lock()
	defer h.uploadMu.Unlock()
	if h.jobEvents == nil {
		h.jobEvents = make(map[string]*jobProgressLog)
	}
	log, ok := h.jobEvents[jobID]
	if !ok {
		log = &jobProgressLog{}
		log.cond = sync.NewCond(&log.mu)
		h.jobEvents[jobID] = log
	}
	return log
}

// clearEventLog drops the per-job structured log. Called after the SSE
// stream observes a terminal state so the map does not grow unbounded.
func (h *handlers) clearEventLog(jobID string) {
	if jobID == "" {
		return
	}
	h.uploadMu.Lock()
	defer h.uploadMu.Unlock()
	delete(h.jobEvents, jobID)
}

func (h *handlers) uploadProgress(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	// Filter by job_id when present. Without one, fall back to the
	// legacy plain-text relay so older clients still see something.
	jobID := r.URL.Query().Get("job_id")
	if jobID == "" {
		h.uploadProgressLegacy(w, r, flusher)
		return
	}

	log := h.getOrCreateEventLog(jobID)
	ctx := r.Context()
	cursor := 0

	// Wake the cond from a watcher goroutine when the client cancels;
	// otherwise sync.Cond.Wait would block forever.
	doneCh := make(chan struct{})
	defer close(doneCh)
	go func() {
		select {
		case <-ctx.Done():
			log.mu.Lock()
			log.cond.Broadcast()
			log.mu.Unlock()
		case <-doneCh:
		}
	}()

	enc := json.NewEncoder(w)
	for {
		log.mu.Lock()
		for cursor >= len(log.events) && !log.done && ctx.Err() == nil {
			log.cond.Wait()
		}
		// Snapshot the events we have not yet flushed.
		batch := append([]uploadEvent(nil), log.events[cursor:]...)
		cursor = len(log.events)
		terminal := log.done
		log.mu.Unlock()

		if ctx.Err() != nil {
			return
		}

		for _, evt := range batch {
			if _, err := w.Write([]byte("data: ")); err != nil {
				return
			}
			if err := enc.Encode(evt); err != nil {
				return
			}
			// json.Encoder appends a newline; SSE needs a blank line.
			if _, err := w.Write([]byte("\n")); err != nil {
				return
			}
			flusher.Flush()
		}

		if terminal {
			h.clearEventLog(jobID)
			return
		}
	}
}

// uploadProgressLegacy preserves the Wave-A plain-text behaviour for
// callers that connect without a job_id query parameter.
func (h *handlers) uploadProgressLegacy(w http.ResponseWriter, r *http.Request, flusher http.Flusher) {
	ctx := r.Context()
	lastMsg := ""
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			msg, _ := h.progressForJob("")
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

// writeTooLarge emits a 413 JSON error describing the configured limit.
// Callers must ensure w.WriteHeader has not already been committed.
func writeTooLarge(w http.ResponseWriter, limit int64) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusRequestEntityTooLarge)
	_, _ = fmt.Fprintf(w, `{"error":"request body exceeds maximum upload size of %d bytes"}`, limit)
}

// enforceUploadLimit checks Content-Length against limit and, if the
// declared size is within bounds, wraps r.Body with http.MaxBytesReader
// so that any overflow during parsing is caught. Returns false and writes
// a 413 JSON response when the request is known to exceed the limit;
// the caller must return immediately in that case.
func enforceUploadLimit(w http.ResponseWriter, r *http.Request, limit int64) bool {
	if limit <= 0 {
		return true // unlimited (opt-in via 0 or negative)
	}
	// Fast path: Content-Length is declared and already exceeds the limit.
	if r.ContentLength > limit {
		ContextLogger(r.Context()).Warn("⚠️ upload: rejected oversize request", "content_length", r.ContentLength, "limit", limit)
		writeTooLarge(w, limit)
		return false
	}
	// Slow path: wrap body so overflow is caught during parsing.
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	return true
}
