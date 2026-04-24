//go:build sqlite_fts5

package mcp

import (
	"encoding/json"
	"strings"
	"testing"
)

// FuzzMCPToolArgs asserts that the argument-coercion helpers (stringArg,
// intArg, and the `project` shortcut projectArg) never panic on any JSON
// payload an MCP client might send. We fuzz a JSON blob, unmarshal it
// into map[string]any (the exact type the real handlers receive via
// mcpgo.CallToolRequest.GetArguments()), and poke each helper with the
// known keys plus a couple of keys that intentionally don't exist.
func FuzzMCPToolArgs(f *testing.F) {
	// Seeds cover the shapes that flow through the real tool registrations
	// in tools.go: strings, numbers (float64 after JSON round-trip),
	// booleans, nulls, nested objects, and arrays. Malformed JSON is fed
	// via the "ignore" branch — the unmarshal error is expected and
	// skipped so it does not count as a fuzzer-discovered crash.
	seeds := []string{
		`{}`,
		`{"query":"hello"}`,
		`{"query":""}`,
		`{"top_k":5}`,
		`{"top_k":5.5}`,
		`{"top_k":-1}`,
		`{"top_k":"not a number"}`,
		`{"project":null}`,
		`{"project":true}`,
		`{"project":["nested","array"]}`,
		`{"project":{"nested":"object"}}`,
		`{"entity_name":"foo","depth":2}`,
		`{"community_level":0}`,
		`{"` + strings.Repeat("a", 1024) + `":"long-key"}`,
		`not json at all`,
		``,
	}
	for _, s := range seeds {
		f.Add(s)
	}

	// All known argument keys used across internal/mcp/tools.go and
	// notes_tools.go. Exhaustive is cheap; if a new tool adds a new
	// key this list lags but the fuzz target still covers the helpers.
	keys := []string{
		"query", "top_k", "doc_type", "project",
		"community_level", "entity_name", "depth",
		"from", "to", "predicate",
		"note_key", "content", "tags", "limit",
		"max_nodes", "graph_depth", "doc_id", "type",
		"nonexistent_key_for_default_path",
	}

	f.Fuzz(func(t *testing.T, raw string) {
		var args map[string]any
		if err := json.Unmarshal([]byte(raw), &args); err != nil {
			// Not valid JSON — not our target. MCP transport layer
			// already rejects these before they reach tool handlers.
			t.Skip()
		}
		if args == nil {
			// JSON "null" at the top level — nothing to coerce.
			return
		}

		for _, k := range keys {
			_ = stringArg(args, k, "default")
			_ = intArg(args, k, 0)
		}
		// projectArg lives in server.go and wraps stringArg for "project".
		_ = projectArg(args)
	})
}
