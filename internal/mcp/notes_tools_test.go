package mcp

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/RandomCodeSpace/docsiq/internal/config"
	"github.com/RandomCodeSpace/docsiq/internal/project"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

// buildTestServer builds a minimal *Server suitable for unit-testing the
// notes MCP tools. Docs-pipeline deps (LLM, embedder, store) are nil-
// wired — the notes tools never touch those fields.
func buildTestServer(t *testing.T) (*Server, string) {
	t.Helper()
	dataDir := t.TempDir()
	cfg := &config.Config{DataDir: dataDir}
	reg, err := project.OpenRegistry(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reg.Close() })
	slug := "proj1"
	if err := reg.Register(project.Project{
		Slug: slug, Name: slug, Remote: "r-" + slug, CreatedAt: time.Now().Unix(),
	}); err != nil {
		t.Fatal(err)
	}
	// Notes tools open per-project stores via s.stores.ForProject so
	// use a lazy-store test helper rooted at dataDir.
	ls := newLazyStorer(dataDir)
	t.Cleanup(ls.Close)
	s := New(ls, nil, nil, cfg, reg)
	t.Cleanup(func() { _ = s.Close() })
	return s, slug
}

// callTool invokes a registered tool's handler directly, bypassing the
// streamable HTTP transport (which requires a full session handshake).
// Returns the first Content element's text plus the IsError flag.
func callTool(t *testing.T, s *Server, name string, args map[string]any) (string, bool) {
	t.Helper()
	st := s.mcpServer.GetTool(name)
	if st == nil {
		t.Fatalf("tool %q not registered", name)
	}
	req := mcpgo.CallToolRequest{}
	req.Params.Name = name
	req.Params.Arguments = args
	res, err := st.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("tool %s: handler returned err: %v", name, err)
	}
	if res == nil || len(res.Content) == 0 {
		return "", res != nil && res.IsError
	}
	tc, ok := res.Content[0].(mcpgo.TextContent)
	if !ok {
		return "", res.IsError
	}
	return tc.Text, res.IsError
}

func TestMCP_ListProjects(t *testing.T) {
	s, slug := buildTestServer(t)
	out, isErr := callTool(t, s, "list_projects", map[string]any{})
	if isErr {
		t.Fatalf("error: %s", out)
	}
	if !strings.Contains(out, slug) {
		t.Errorf("listing missing slug %q: %s", slug, out)
	}
}

func TestMCP_ListNotes_Empty(t *testing.T) {
	s, slug := buildTestServer(t)
	out, isErr := callTool(t, s, "list_notes", map[string]any{"project": slug})
	if isErr {
		t.Fatalf("error: %s", out)
	}
	if !strings.Contains(out, `"keys":[]`) {
		t.Errorf("expected empty keys, got %s", out)
	}
}

func TestMCP_WriteRead(t *testing.T) {
	s, slug := buildTestServer(t)
	out, isErr := callTool(t, s, "write_note", map[string]any{
		"project": slug,
		"key":     "alpha",
		"content": "hello from mcp",
		"author":  "bob",
		"tags":    []any{"x", "y"},
	})
	if isErr {
		t.Fatalf("write error: %s", out)
	}
	out, isErr = callTool(t, s, "read_note", map[string]any{
		"project": slug,
		"key":     "alpha",
	})
	if isErr {
		t.Fatalf("read error: %s", out)
	}
	if !strings.Contains(out, "hello from mcp") {
		t.Errorf("content lost: %s", out)
	}
}

func TestMCP_SearchNotes(t *testing.T) {
	s, slug := buildTestServer(t)
	callTool(t, s, "write_note", map[string]any{"project": slug, "key": "k1", "content": "oauth authentication"})
	callTool(t, s, "write_note", map[string]any{"project": slug, "key": "k2", "content": "unrelated stuff"})
	out, isErr := callTool(t, s, "search_notes", map[string]any{
		"project": slug,
		"query":   "oauth",
	})
	if isErr {
		t.Fatalf("search error: %s", out)
	}
	if !strings.Contains(out, "k1") {
		t.Errorf("search missed k1: %s", out)
	}
}

// TestMCP_SearchNotes_EmptyQuery is a regression test for P1-5.
// search_notes must reject an empty query with a tool error rather
// than silently returning {"hits":[]}, matching the behavior of
// search_documents and local_search.
func TestMCP_SearchNotes_EmptyQuery(t *testing.T) {
	s, slug := buildTestServer(t)
	out, isErr := callTool(t, s, "search_notes", map[string]any{
		"project": slug,
		"query":   "",
	})
	if !isErr {
		t.Fatalf("expected tool error for empty query, got %s", out)
	}
	if !strings.Contains(strings.ToLower(out), "query") {
		t.Errorf("error text should mention 'query'; got %s", out)
	}
}

func TestMCP_DeleteNote(t *testing.T) {
	s, slug := buildTestServer(t)
	callTool(t, s, "write_note", map[string]any{"project": slug, "key": "k", "content": "x"})
	out, isErr := callTool(t, s, "delete_note", map[string]any{"project": slug, "key": "k"})
	if isErr {
		t.Fatalf("delete error: %s", out)
	}
	out, isErr = callTool(t, s, "read_note", map[string]any{"project": slug, "key": "k"})
	if !isErr {
		t.Errorf("expected read-after-delete to error, got %s", out)
	}
}

func TestMCP_GetNotesGraph(t *testing.T) {
	s, slug := buildTestServer(t)
	callTool(t, s, "write_note", map[string]any{"project": slug, "key": "a", "content": "[[b]]"})
	callTool(t, s, "write_note", map[string]any{"project": slug, "key": "b", "content": "nada"})
	out, isErr := callTool(t, s, "get_notes_graph", map[string]any{"project": slug})
	if isErr {
		t.Fatalf("graph error: %s", out)
	}
	if !strings.Contains(out, `"source":"a"`) {
		t.Errorf("edge a→b missing: %s", out)
	}
}

func TestMCP_MissingProject(t *testing.T) {
	s, _ := buildTestServer(t)
	out, isErr := callTool(t, s, "list_notes", map[string]any{"project": "ghost"})
	if !isErr {
		t.Errorf("expected error for missing project, got %s", out)
	}
}

func TestMCP_InvalidKey(t *testing.T) {
	s, slug := buildTestServer(t)
	out, isErr := callTool(t, s, "write_note", map[string]any{
		"project": slug,
		"key":     "../escape",
		"content": "x",
	})
	if !isErr {
		t.Errorf("expected error for traversal key, got %s", out)
	}
}

func TestMCP_EmptyProjectArg(t *testing.T) {
	s, _ := buildTestServer(t)
	out, isErr := callTool(t, s, "list_notes", map[string]any{})
	if !isErr {
		t.Errorf("expected error for missing project arg, got %s", out)
	}
}

func TestMCP_LongContent(t *testing.T) {
	s, slug := buildTestServer(t)
	long := strings.Repeat("x", 1_000_000)
	out, isErr := callTool(t, s, "write_note", map[string]any{
		"project": slug,
		"key":     "big",
		"content": long,
	})
	if isErr {
		t.Fatalf("long content rejected: %s", out)
	}
}

func TestMCP_TagEdgeCases(t *testing.T) {
	s, slug := buildTestServer(t)
	out, isErr := callTool(t, s, "write_note", map[string]any{
		"project": slug,
		"key":     "k",
		"content": "x",
		"tags":    []any{},
	})
	if isErr {
		t.Fatalf("empty tags errored: %s", out)
	}
	out, isErr = callTool(t, s, "write_note", map[string]any{
		"project": slug,
		"key":     "k2",
		"content": "x",
		"tags":    []any{"valid", 42, nil},
	})
	if isErr {
		t.Fatalf("mixed-type tags errored: %s", out)
	}
}

func TestMCP_ValidateProject(t *testing.T) {
	s, slug := buildTestServer(t)
	if err := s.validateProject(slug); err != nil {
		t.Errorf("valid slug errored: %v", err)
	}
	if err := s.validateProject(""); err == nil {
		t.Error("empty slug should error")
	}
	if err := s.validateProject("BAD SLUG"); err == nil {
		t.Error("invalid chars should error")
	}
	if err := s.validateProject("ghost"); err == nil {
		t.Error("unknown slug should error")
	}
}

func TestMCP_ReadMissingNote(t *testing.T) {
	s, slug := buildTestServer(t)
	out, isErr := callTool(t, s, "read_note", map[string]any{"project": slug, "key": "ghost"})
	if !isErr {
		t.Errorf("expected error for missing note, got %s", out)
	}
}
