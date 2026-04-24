package api

import (
	"net/http"

	"github.com/RandomCodeSpace/docsiq/internal/buildinfo"
)

// versionHandler serves GET /api/version. Returns buildinfo.Info as
// JSON, including the direct-dependency map. Public endpoint — no
// secrets are exposed; commit hash and Go version are considered
// non-sensitive for a self-hosted MCP server.
func versionHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		info := buildinfo.Resolve(true)
		writeJSON(w, http.StatusOK, info)
	})
}
