package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/RandomCodeSpace/docsiq/internal/project"
)

// hookBody is the JSON body accepted by POST /api/hook/SessionStart.
//
// remote      — the git remote URL (e.g. "git@github.com:owner/repo.git").
//
//	Required. Mirrors kgraph's body field of the same name.
//
// cwd         — optional working directory the client was invoked in.
//
//	Currently only forwarded for parity; docsiq does not consult
//	the filesystem during hook resolution.
//
// sessionID  — optional Claude Code session identifier. Stored in the
//
//	response context for future correlation (not exposed
//	to clients today).
type hookBody struct {
	Remote    string `json:"remote"`
	CWD       string `json:"cwd,omitempty"`
	SessionID string `json:"session_id,omitempty"`
}

// hookResponse is the 200-OK payload for a resolved project.
type hookResponse struct {
	Project           string `json:"project"`
	AdditionalContext string `json:"additionalContext"`
}

// maxHookBody caps the POST body at 64 KiB — more than enough for a
// well-formed hook payload but small enough to bound a malicious caller.
const maxHookBody = 64 * 1024

// registerHookRoutes wires the SessionStart hook endpoint onto mux. The
// handler is closure-scoped to the registry so it can perform the
// remote → slug → project lookup without a package-level global.
func registerHookRoutes(mux *http.ServeMux, registry *project.Registry) {
	mux.HandleFunc("POST /api/hook/SessionStart", func(w http.ResponseWriter, r *http.Request) {
		handleSessionStart(w, r, registry)
	})
}

// handleSessionStart is the POST /api/hook/SessionStart handler.
//
// Behavior (ported from kgraph/src/api/hooks.ts):
//   - Malformed JSON, empty body, or missing `remote` → 400.
//   - Unknown remote (not registered) → 204 No Content.
//   - Registered remote → 200 {project, additionalContext}.
//
// Remote normalization runs via project.Slug(). A registry miss on the
// slug is NOT an error — the hook should stay silent so the agent picks
// up no extra context.
func handleSessionStart(w http.ResponseWriter, r *http.Request, registry *project.Registry) {
	raw, err := io.ReadAll(io.LimitReader(r.Body, maxHookBody+1))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "failed to read body", err)
		return
	}
	if len(raw) == 0 {
		writeError(w, r, http.StatusBadRequest, "empty body", nil)
		return
	}
	if len(raw) > maxHookBody {
		writeError(w, r, http.StatusBadRequest, "body too large", nil)
		return
	}

	var body hookBody
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields() // reject obvious injection attempts
	// We intentionally keep DisallowUnknownFields enabled after some
	// deliberation: the hook body is a small, fixed shape; quietly
	// ignoring unknown fields here hides client bugs.
	if err := dec.Decode(&body); err != nil {
		// Fall back to a permissive decode so forward-compatible fields
		// (e.g. new kgraph hook_event_name style extras) still work.
		if strings.Contains(err.Error(), "unknown field") {
			if err2 := json.Unmarshal(raw, &body); err2 != nil {
				writeError(w, r, http.StatusBadRequest, "invalid JSON", err2)
				return
			}
		} else {
			writeError(w, r, http.StatusBadRequest, "invalid JSON", err)
			return
		}
	}

	if strings.TrimSpace(body.Remote) == "" {
		writeError(w, r, http.StatusBadRequest, "remote is required", nil)
		return
	}

	// A nil registry mirrors the tolerated-nil-registry pattern used
	// elsewhere (see NewRouter). Without a registry there is nothing to
	// resolve, so behave like "unknown remote".
	if registry == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	slug, err := project.Slug(body.Remote)
	if err != nil {
		// Garbage remote — treat as unresolvable, not a protocol error.
		w.WriteHeader(http.StatusNoContent)
		return
	}

	p, err := registry.GetByRemote(body.Remote)
	if err != nil {
		if errors.Is(err, project.ErrNotFound) {
			// Some clients auto-register by slug, so try a slug lookup too.
			p2, err2 := registry.Get(slug)
			if err2 != nil {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			p = p2
		} else {
			writeError(w, r, http.StatusInternalServerError, "registry lookup failed", err)
			return
		}
	}

	writeJSON(w, http.StatusOK, hookResponse{
		Project: p.Slug,
		AdditionalContext: "docsiq active (project \"" + p.Slug + "\"). " +
			"Use MCP tools: search_documents, local_search, global_search, " +
			"list_notes, search_notes, read_note, write_note, get_notes_graph.",
	})
}
