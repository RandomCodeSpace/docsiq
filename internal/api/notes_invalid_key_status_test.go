package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestNotes_InvalidKey_Returns400 is a regression test for P1-3.
// notes.ErrInvalidKey is a client input validation error and must map
// to HTTP 400 Bad Request, not 403 Forbidden.
//
// We use a backslash-containing key because Go's ServeMux cleans
// "/../" path segments (issuing a 307 redirect) before our handler
// can see them. A backslash in the URL is preserved through routing
// and still trips notes.ValidateKey (which rejects backslashes).
func TestNotes_InvalidKey_Returns400(t *testing.T) {
	h, slug, _ := setupNotesRouter(t)

	invalidKey := `bad\path`
	escaped := strings.ReplaceAll(invalidKey, `\`, "%5C")

	cases := []struct {
		name, method, path, body string
	}{
		{"PUT_invalid", http.MethodPut,
			"/api/projects/" + slug + "/notes/" + escaped,
			`{"content":"x"}`},
		{"DELETE_invalid", http.MethodDelete,
			"/api/projects/" + slug + "/notes/" + escaped,
			""},
		{"GET_history_invalid", http.MethodGet,
			"/api/projects/" + slug + "/notes/" + escaped + "/history",
			""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var r *strings.Reader
			if tc.body != "" {
				r = strings.NewReader(tc.body)
			}
			var req *http.Request
			if r != nil {
				req = httptest.NewRequest(tc.method, tc.path, r)
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(tc.method, tc.path, nil)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code == http.StatusForbidden {
				t.Fatalf("%s %s: got 403 Forbidden; expected 400 Bad Request. body=%s",
					tc.method, tc.path, rec.Body.String())
			}
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("%s %s: got %d; expected 400. body=%s",
					tc.method, tc.path, rec.Code, rec.Body.String())
			}
		})
	}
}
