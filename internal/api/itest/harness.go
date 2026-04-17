//go:build integration

// Package itest provides the shared integration-test harness for the REST
// API surface. It stands up a real httptest.Server wrapping api.NewRouter,
// a per-project store cache rooted in a tempdir, a project.Registry, and
// a deterministic FakeProvider so integration tests can exercise the full
// request path without depending on Azure/Ollama.
//
// Build tag: `integration`. All files in this package require
// `-tags "sqlite_fts5 integration"`.
package itest

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"go.uber.org/goleak"

	"github.com/RandomCodeSpace/docsiq/internal/api"
	"github.com/RandomCodeSpace/docsiq/internal/config"
	"github.com/RandomCodeSpace/docsiq/internal/embedder"
	"github.com/RandomCodeSpace/docsiq/internal/project"
)

// VerifyNoLeaks is a thin wrapper around goleak.VerifyNone that C-wave
// tests can call in TestMain to assert the harness (and the code under
// test) left no stray goroutines. Exposed here so goleak stays a direct
// dependency of this module.
func VerifyNoLeaks(t *testing.T) {
	t.Helper()
	goleak.VerifyNone(t)
}

// Env is the integration-test fixture. Construct with New(t); all
// resources are registered with t.Cleanup in LIFO order, so a failing
// test never leaks a server, DB handle, tempdir, or registry.
type Env struct {
	Server   *httptest.Server
	DataDir  string
	Cfg      *config.Config
	Registry *project.Registry
	Stores   *api.ProjectStores
	Provider *FakeProvider
	Embedder *embedder.Embedder
	APIKey   string
	Client   *http.Client
}

// New constructs a fully-wired Env. Layout:
//
//   - DataDir is a t.TempDir() rooted at the runner's temp area.
//   - A fresh project.Registry lives at DataDir/registry.db.
//   - A ProjectStores cache is seeded into NewRouter via WithProjectStores;
//     first access to a slug opens (and caches) DataDir/projects/<slug>/docsiq.db.
//   - The bearer token is 16 random hex bytes (32 chars) set on both
//     cfg.Server.APIKey and Env.APIKey.
//   - A FakeProvider supplies deterministic Complete/Embed. Embedder uses
//     batch size 8.
//
// Cleanup order (LIFO): httptest server close → stores close → registry close.
// The tempdir is reaped automatically by testing.T.
func New(t *testing.T) *Env {
	t.Helper()

	dataDir := t.TempDir()

	// 32-char hex bearer token.
	var keyBytes [16]byte
	if _, err := rand.Read(keyBytes[:]); err != nil {
		t.Fatalf("itest: generate api key: %v", err)
	}
	apiKey := hex.EncodeToString(keyBytes[:])

	cfg := &config.Config{
		DataDir:        dataDir,
		DefaultProject: config.DefaultProjectSlug,
		Server: config.ServerConfig{
			APIKey: apiKey,
		},
	}

	registry, err := project.OpenRegistry(dataDir)
	if err != nil {
		t.Fatalf("itest: open registry: %v", err)
	}
	t.Cleanup(func() { _ = registry.Close() })

	stores := api.NewProjectStores(dataDir)
	t.Cleanup(func() { _ = stores.Close() })

	provider := &FakeProvider{}
	emb := embedder.New(provider, 8)

	handler := api.NewRouter(provider, emb, cfg, registry,
		api.WithProjectStores(stores),
	)

	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)

	return &Env{
		Server:   ts,
		DataDir:  dataDir,
		Cfg:      cfg,
		Registry: registry,
		Stores:   stores,
		Provider: provider,
		Embedder: emb,
		APIKey:   apiKey,
		Client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// URL returns the test server's base URL joined with path.
// Path should start with "/".
func (e *Env) URL(path string) string {
	return e.Server.URL + path
}

// DB returns the on-disk path for the _default project's store DB.
// Tests that want to assert on raw DB state can open this read-only.
func (e *Env) DB() string {
	return filepath.Join(e.DataDir, "projects", config.DefaultProjectSlug, "docsiq.db")
}

// authReq builds an http.Request with the bearer token and, when body is
// non-nil, a JSON Content-Type. Callers are responsible for closing the
// response body.
func (e *Env) authReq(t *testing.T, method, path string, body []byte) *http.Request {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, e.URL(path), rdr)
	if err != nil {
		t.Fatalf("itest: build %s %s: %v", method, path, err)
	}
	req.Header.Set("Authorization", "Bearer "+e.APIKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

// Do executes req via e.Client and fails the test on transport error.
// The returned response body is NOT drained — callers read and close it.
func (e *Env) Do(t *testing.T, req *http.Request) *http.Response {
	t.Helper()
	resp, err := e.Client.Do(req)
	if err != nil {
		t.Fatalf("itest: %s %s: %v", req.Method, req.URL.Path, err)
	}
	return resp
}

// GET issues GET path with the bearer token and returns both the response
// and the fully-read body. The response Body is closed before return so
// callers don't have to.
func (e *Env) GET(t *testing.T, path string) (*http.Response, []byte) {
	t.Helper()
	req := e.authReq(t, http.MethodGet, path, nil)
	resp := e.Do(t, req)
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("itest: read GET %s body: %v", path, err)
	}
	return resp, data
}

// PUTNote writes a note to the given project. The returned *http.Response
// has its Body intact and un-drained — callers must close it. Use PUTNoteBody
// if you want the body slurped for you.
func (e *Env) PUTNote(t *testing.T, proj, key, content string, tags []string) *http.Response {
	t.Helper()
	if tags == nil {
		tags = []string{}
	}
	payload := map[string]any{
		"content": content,
		"tags":    tags,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("itest: marshal note payload: %v", err)
	}
	path := fmt.Sprintf("/api/projects/%s/notes/%s", proj, key)
	req := e.authReq(t, http.MethodPut, path, raw)
	return e.Do(t, req)
}

// PUTNoteBody is a convenience wrapper over PUTNote that drains and closes
// the response body, returning it alongside the response.
func (e *Env) PUTNoteBody(t *testing.T, proj, key, content string, tags []string) (*http.Response, []byte) {
	t.Helper()
	resp := e.PUTNote(t, proj, key, content, tags)
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("itest: read PUT body: %v", err)
	}
	return resp, data
}

// POSTJSON posts the given JSON-marshalled payload to path and returns the
// response plus a fully-read body (response is closed).
func (e *Env) POSTJSON(t *testing.T, path string, payload any) (*http.Response, []byte) {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("itest: marshal payload: %v", err)
	}
	req := e.authReq(t, http.MethodPost, path, raw)
	resp := e.Do(t, req)
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("itest: read POST %s body: %v", path, err)
	}
	return resp, data
}

// DELETE issues DELETE path with the bearer token and returns the response
// with its body drained.
func (e *Env) DELETE(t *testing.T, path string) (*http.Response, []byte) {
	t.Helper()
	req := e.authReq(t, http.MethodDelete, path, nil)
	resp := e.Do(t, req)
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("itest: read DELETE %s body: %v", path, err)
	}
	return resp, data
}
