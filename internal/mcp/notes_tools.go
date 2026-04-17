package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/RandomCodeSpace/docsiq/internal/notes"
	"github.com/RandomCodeSpace/docsiq/internal/project"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// registerNotesTools adds the Phase-2 notes subsystem tools. All of
// these take a `project` slug argument; projects must exist in the
// registry (when one is configured) or the tool returns an error.
func registerNotesTools(s *Server) {
	// list_projects
	s.mcpServer.AddTool(mcpgo.NewTool("list_projects",
		mcpgo.WithDescription("List all projects registered in the notes registry"),
	), server.ToolHandlerFunc(func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		if s.registry == nil {
			return toolText(`{"projects":[]}`), nil
		}
		projs, err := s.registry.List()
		if err != nil {
			return toolError(err), nil
		}
		b, _ := json.Marshal(map[string]any{"projects": projs})
		return toolText(string(b)), nil
	}))

	// list_notes
	s.mcpServer.AddTool(mcpgo.NewTool("list_notes",
		mcpgo.WithDescription("List all note keys in a project"),
		mcpgo.WithString("project", mcpgo.Required(), mcpgo.Description("Project slug")),
	), server.ToolHandlerFunc(func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		args := req.GetArguments()
		slug := stringArg(args, "project", "")
		if err := s.validateProject(slug); err != nil {
			return toolError(err), nil
		}
		keys, err := notes.ListKeys(s.cfg.NotesDir(slug))
		if err != nil {
			return toolError(err), nil
		}
		if keys == nil {
			keys = []string{}
		}
		b, _ := json.Marshal(map[string]any{"project": slug, "keys": keys})
		return toolText(string(b)), nil
	}))

	// search_notes
	s.mcpServer.AddTool(mcpgo.NewTool("search_notes",
		mcpgo.WithDescription("Full-text search across notes in a project"),
		mcpgo.WithString("project", mcpgo.Required(), mcpgo.Description("Project slug")),
		mcpgo.WithString("query", mcpgo.Required(), mcpgo.Description("FTS5 query text")),
		mcpgo.WithNumber("limit", mcpgo.Description("Max hits (default 20)")),
	), server.ToolHandlerFunc(func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		args := req.GetArguments()
		slug := stringArg(args, "project", "")
		q := stringArg(args, "query", "")
		limit := intArg(args, "limit", 20)
		if q == "" {
			return toolError(fmt.Errorf("query required")), nil
		}
		if err := s.validateProject(slug); err != nil {
			return toolError(err), nil
		}
		st, err := s.storeForProject(slug)
		if err != nil {
			return toolError(err), nil
		}
		hits, err := st.SearchNotes(ctx, q, limit)
		if err != nil {
			return toolError(err), nil
		}
		b, _ := json.Marshal(map[string]any{"project": slug, "query": q, "hits": hits})
		return toolText(string(b)), nil
	}))

	// read_note
	s.mcpServer.AddTool(mcpgo.NewTool("read_note",
		mcpgo.WithDescription("Read a single note plus its wikilink outlinks"),
		mcpgo.WithString("project", mcpgo.Required(), mcpgo.Description("Project slug")),
		mcpgo.WithString("key", mcpgo.Required(), mcpgo.Description("Note key")),
	), server.ToolHandlerFunc(func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		args := req.GetArguments()
		slug := stringArg(args, "project", "")
		key := stringArg(args, "key", "")
		if err := s.validateProject(slug); err != nil {
			return toolError(err), nil
		}
		n, err := notes.Read(s.cfg.NotesDir(slug), key)
		if err != nil {
			return toolError(err), nil
		}
		resp := map[string]any{
			"note":     n,
			"outlinks": notes.ExtractWikilinks([]byte(n.Content)),
		}
		b, _ := json.Marshal(resp)
		return toolText(string(b)), nil
	}))

	// write_note
	s.mcpServer.AddTool(mcpgo.NewTool("write_note",
		mcpgo.WithDescription("Create or update a note; indexes into FTS5"),
		mcpgo.WithString("project", mcpgo.Required(), mcpgo.Description("Project slug")),
		mcpgo.WithString("key", mcpgo.Required(), mcpgo.Description("Note key (folders via '/')")),
		mcpgo.WithString("content", mcpgo.Required(), mcpgo.Description("Markdown body")),
		mcpgo.WithString("author", mcpgo.Description("Author name")),
		mcpgo.WithArray("tags", mcpgo.Description("Tag list"), mcpgo.Items(map[string]any{"type": "string"})),
	), server.ToolHandlerFunc(func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		args := req.GetArguments()
		slug := stringArg(args, "project", "")
		key := stringArg(args, "key", "")
		content := stringArg(args, "content", "")
		author := stringArg(args, "author", "")
		if err := s.validateProject(slug); err != nil {
			return toolError(err), nil
		}
		if err := notes.ValidateKey(key); err != nil {
			return toolError(err), nil
		}
		if content == "" {
			return toolError(fmt.Errorf("content required")), nil
		}
		n := &notes.Note{
			Key:     key,
			Content: content,
			Author:  author,
			Tags:    stringSliceArg(args, "tags"),
		}
		if err := notes.Write(s.cfg.NotesDir(slug), n); err != nil {
			return toolError(err), nil
		}
		st, err := s.storeForProject(slug)
		if err != nil {
			return toolError(err), nil
		}
		_ = st.IndexNote(ctx, n)
		b, _ := json.Marshal(n)
		return toolText(string(b)), nil
	}))

	// delete_note
	s.mcpServer.AddTool(mcpgo.NewTool("delete_note",
		mcpgo.WithDescription("Delete a note from disk and FTS5 index"),
		mcpgo.WithString("project", mcpgo.Required(), mcpgo.Description("Project slug")),
		mcpgo.WithString("key", mcpgo.Required(), mcpgo.Description("Note key")),
	), server.ToolHandlerFunc(func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		args := req.GetArguments()
		slug := stringArg(args, "project", "")
		key := stringArg(args, "key", "")
		if err := s.validateProject(slug); err != nil {
			return toolError(err), nil
		}
		if err := notes.ValidateKey(key); err != nil {
			return toolError(err), nil
		}
		if err := notes.Delete(s.cfg.NotesDir(slug), key); err != nil {
			return toolError(err), nil
		}
		if st, err := s.storeForProject(slug); err == nil {
			_ = st.DeleteNote(ctx, key)
		}
		return toolText(`{"ok":true}`), nil
	}))

	// get_notes_graph
	s.mcpServer.AddTool(mcpgo.NewTool("get_notes_graph",
		mcpgo.WithDescription("Return the wikilink-derived notes graph"),
		mcpgo.WithString("project", mcpgo.Required(), mcpgo.Description("Project slug")),
	), server.ToolHandlerFunc(func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		args := req.GetArguments()
		slug := stringArg(args, "project", "")
		if err := s.validateProject(slug); err != nil {
			return toolError(err), nil
		}
		g, err := notes.BuildGraph(s.cfg.NotesDir(slug))
		if err != nil {
			return toolError(err), nil
		}
		b, _ := json.Marshal(g)
		return toolText(string(b)), nil
	}))
}

// validateProject enforces slug-charset + registry existence. Mirrors
// the resolveProject behavior in the REST handlers so MCP and HTTP
// clients see the same errors for the same inputs.
func (s *Server) validateProject(slug string) error {
	if slug == "" {
		return fmt.Errorf("project required")
	}
	if !project.IsValidSlug(slug) {
		return fmt.Errorf("invalid project slug: %q", slug)
	}
	if s.registry == nil {
		return nil
	}
	if _, err := s.registry.Get(slug); err != nil {
		if errors.Is(err, project.ErrNotFound) {
			return fmt.Errorf("unknown project: %q", slug)
		}
		return err
	}
	return nil
}

// stringSliceArg pulls a []string from tool arguments, tolerating the
// two JSON shapes mcp-go may deliver: []any and []string.
func stringSliceArg(args map[string]any, key string) []string {
	v, ok := args[key]
	if !ok || v == nil {
		return nil
	}
	switch arr := v.(type) {
	case []string:
		return append([]string(nil), arr...)
	case []any:
		out := make([]string, 0, len(arr))
		for _, x := range arr {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}
