package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/RandomCodeSpace/docsiq/internal/config"
	"github.com/RandomCodeSpace/docsiq/internal/store"
)

// buildDocsTestServer wires a real docs store into a *Server so the
// docs-pipeline MCP tools (stats, get_entity_claims, …) can be exercised.
// Provider/embedder are nil — none of the tested tools invoke the LLM.
func buildDocsTestServer(t *testing.T) (*Server, *store.Store) {
	t.Helper()
	dir := t.TempDir()
	st, err := store.OpenForProject(dir, "testproj")
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	cfg := &config.Config{DataDir: dir}
	s := New(newFakeStorer(st), nil, nil, cfg, nil)
	t.Cleanup(func() { _ = s.Close() })
	return s, st
}

func TestGetEntityClaimsTool(t *testing.T) {
	s, st := buildDocsTestServer(t)
	ctx := context.Background()
	if err := st.UpsertDocument(ctx, &store.Document{
		ID: "d1", Path: "/tmp/d1.md", DocType: "md", FileHash: "d1h", IsLatest: true,
	}); err != nil {
		t.Fatalf("UpsertDocument: %v", err)
	}
	if err := st.UpsertEntity(ctx, &store.Entity{ID: "e1", Name: "Alpha"}); err != nil {
		t.Fatalf("UpsertEntity: %v", err)
	}
	if err := st.BatchInsertClaims(ctx, []*store.Claim{
		{ID: "c1", EntityID: "e1", Claim: "does X", Status: "verified", DocID: "d1"},
		{ID: "c2", EntityID: "e1", Claim: "does Y", Status: "pending", DocID: "d1"},
	}); err != nil {
		t.Fatalf("BatchInsertClaims: %v", err)
	}

	t.Run("happy_path", func(t *testing.T) {
		text, isErr := callTool(t, s, "get_entity_claims", map[string]any{"entity_id": "e1"})
		if isErr {
			t.Fatalf("tool returned IsError: %s", text)
		}
		var claims []store.Claim
		if err := json.Unmarshal([]byte(text), &claims); err != nil {
			t.Fatalf("decode: %v (body=%q)", err, text)
		}
		if len(claims) != 2 {
			t.Errorf("len = %d, want 2", len(claims))
		}
	})

	t.Run("missing_entity_id_returns_tool_error", func(t *testing.T) {
		_, isErr := callTool(t, s, "get_entity_claims", map[string]any{})
		if !isErr {
			t.Error("expected IsError=true on missing entity_id")
		}
	})

	t.Run("unknown_entity_returns_empty_array", func(t *testing.T) {
		text, isErr := callTool(t, s, "get_entity_claims", map[string]any{"entity_id": "nope"})
		if isErr {
			t.Fatalf("tool returned IsError: %s", text)
		}
		if text != "[]" {
			t.Errorf("body = %q, want []", text)
		}
	})
}
