//go:build integration

package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/RandomCodeSpace/docsiq/internal/api/itest"
	"github.com/RandomCodeSpace/docsiq/internal/project"
)

// TestHooks_RegisteredRemoteReturnsContext registers a project with a
// known remote, POSTs the SessionStart hook body with that remote, and
// asserts 200 + a body containing both `project` and
// `additionalContext` fields.
func TestHooks_RegisteredRemoteReturnsContext(t *testing.T) {
	e := itest.New(t)

	remote := "git@github.com:org/repo.git"
	if err := e.Registry.Register(project.Project{
		Slug:      "org-repo",
		Name:      "org-repo",
		Remote:    remote,
		CreatedAt: time.Now().Unix(),
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	resp, body := e.POSTJSON(t, "/api/hook/SessionStart", map[string]any{
		"remote": remote,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", resp.StatusCode, string(body))
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, string(body))
	}
	if _, ok := payload["project"]; !ok {
		t.Errorf("response missing `project`: %s", string(body))
	}
	if _, ok := payload["additionalContext"]; !ok {
		t.Errorf("response missing `additionalContext`: %s", string(body))
	}
}

// TestHooks_UnknownRemoteReturns204 asserts an unregistered remote
// yields 204 No Content with an empty body.
func TestHooks_UnknownRemoteReturns204(t *testing.T) {
	e := itest.New(t)

	resp, body := e.POSTJSON(t, "/api/hook/SessionStart", map[string]any{
		"remote": "git@github.com:nobody/nope.git",
	})
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("want 204, got %d body=%s", resp.StatusCode, string(body))
	}
	if len(bytes.TrimSpace(body)) != 0 {
		t.Errorf("204 body should be empty, got: %q", string(body))
	}
}

// TestHooks_MalformedJSONReturns400 posts a non-JSON payload and
// asserts the handler rejects it with 400.
func TestHooks_MalformedJSONReturns400(t *testing.T) {
	e := itest.New(t)

	req, err := http.NewRequest(http.MethodPost,
		e.URL("/api/hook/SessionStart"),
		strings.NewReader("not-json"))
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+e.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp := e.Do(t, req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", resp.StatusCode)
	}
}
