package api

import (
	"net/http"
)

// claimsForEntity returns all claims attached to the requested entity ID.
// Unknown entity → 200 with empty array; the client can treat that the
// same way whether the entity truly has no claims or was never indexed.
func (h *handlers) claimsForEntity(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, r, 400, "id required", nil)
		return
	}
	claims, err := h.store.ClaimsForEntity(r.Context(), id)
	if err != nil {
		writeError(w, r, 500, err.Error(), err)
		return
	}
	writeJSON(w, 200, claims)
}

// listClaims returns claims with optional ?status= and ?limit= filters.
func (h *handlers) listClaims(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	status := q.Get("status")
	limit := intQuery(q.Get("limit"), 100)
	claims, err := h.store.ListClaims(r.Context(), status, limit)
	if err != nil {
		writeError(w, r, 500, err.Error(), err)
		return
	}
	writeJSON(w, 200, claims)
}
