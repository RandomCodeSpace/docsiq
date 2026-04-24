package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestVersionHandler_ReturnsJSON(t *testing.T) {
	t.Parallel()
	h := versionHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/version", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want 200", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type=%q want application/json*", ct)
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body not JSON: %v — raw=%q", err, rec.Body.String())
	}

	for _, key := range []string{"version", "commit", "build_date", "go_version"} {
		if _, ok := body[key]; !ok {
			t.Errorf("response missing %q field; got keys=%v", key, mapKeys(body))
		}
	}
}

func TestVersionHandler_RejectsNonGET(t *testing.T) {
	t.Parallel()
	h := versionHandler()
	req := httptest.NewRequest(http.MethodPost, "/api/version", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /api/version status=%d want 405", rec.Code)
	}
	if got := rec.Header().Get("Allow"); got != "GET" {
		t.Errorf("Allow=%q want GET", got)
	}
}

func mapKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
