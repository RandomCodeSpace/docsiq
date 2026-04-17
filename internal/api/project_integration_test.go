//go:build integration

package api_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/RandomCodeSpace/docsiq/internal/api/itest"
	"github.com/RandomCodeSpace/docsiq/internal/project"
)

// TestProject_IsolationEndToEnd writes a note under the same key in two
// separate projects and verifies each read returns the project's own
// content — proving per-project DB + notes dir isolation.
func TestProject_IsolationEndToEnd(t *testing.T) {
	e := itest.New(t)

	for _, slug := range []string{"proj-a", "proj-b"} {
		if err := e.Registry.Register(project.Project{
			Slug:      slug,
			Name:      slug,
			Remote:    "r-" + slug,
			CreatedAt: time.Now().Unix(),
		}); err != nil {
			t.Fatalf("register %s: %v", slug, err)
		}
	}

	if resp, body := e.PUTNoteBody(t, "proj-a", "x", "A-content", nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT proj-a/x: want 200, got %d body=%s", resp.StatusCode, string(body))
	}
	if resp, body := e.PUTNoteBody(t, "proj-b", "x", "B-content", nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT proj-b/x: want 200, got %d body=%s", resp.StatusCode, string(body))
	}

	readContent := func(slug string) string {
		t.Helper()
		resp, body := e.GET(t, "/api/projects/"+slug+"/notes/x")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET %s/x: want 200, got %d body=%s", slug, resp.StatusCode, string(body))
		}
		var payload struct {
			Note struct {
				Content string `json:"content"`
			} `json:"note"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("unmarshal %s/x: %v body=%s", slug, err, string(body))
		}
		return payload.Note.Content
	}

	aContent := readContent("proj-a")
	bContent := readContent("proj-b")

	if !strings.Contains(aContent, "A-content") {
		t.Errorf("proj-a content missing marker: %q", aContent)
	}
	if !strings.Contains(bContent, "B-content") {
		t.Errorf("proj-b content missing marker: %q", bContent)
	}
	if aContent == bContent {
		t.Fatalf("isolation broken — both projects returned identical content: %q", aContent)
	}
}

// TestProject_DefaultAutoRegisters asserts a fresh harness exposes
// the default project on GET /api/projects — the middleware
// auto-registers it on first request scoped to the default slug.
func TestProject_DefaultAutoRegisters(t *testing.T) {
	e := itest.New(t)

	// Trigger the middleware by hitting a default-scoped endpoint first.
	// /api/stats resolves the project via middleware and auto-registers
	// _default if missing.
	if resp, _ := e.GET(t, "/api/stats?project=_default"); resp.StatusCode >= 500 {
		t.Fatalf("prime /api/stats: 5xx %d", resp.StatusCode)
	}

	resp, body := e.GET(t, "/api/projects")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/projects: want 200, got %d body=%s", resp.StatusCode, string(body))
	}

	if !strings.Contains(string(body), "_default") {
		t.Errorf("GET /api/projects body does not mention _default: %s", string(body))
	}
}

// TestProject_UnknownReturns404 asserts a request against an
// unregistered project slug (that is not the default) is rejected
// with 404.
func TestProject_UnknownReturns404(t *testing.T) {
	e := itest.New(t)
	resp, _ := e.GET(t, "/api/projects/no-such/notes/foo")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("GET /api/projects/no-such/notes/foo: want 404, got %d", resp.StatusCode)
	}
}

// TestProject_EmptyProjectFallsBackToDefault asserts an API call that
// does not specify ?project= / path project falls through to the
// configured default slug. /api/stats has no {project} path segment,
// so it is the canonical probe.
func TestProject_EmptyProjectFallsBackToDefault(t *testing.T) {
	e := itest.New(t)
	// No ?project=, no X-Project — middleware must fall back to _default
	// and auto-register it rather than 404-ing.
	resp, body := e.GET(t, "/api/stats")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/stats (no project): want 200, got %d body=%s", resp.StatusCode, string(body))
	}
}
