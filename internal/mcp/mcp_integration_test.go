//go:build integration

package mcp_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/RandomCodeSpace/docsiq/internal/api/itest"
)

// mcpCall drives the Streamable HTTP transport through the full
// handshake → tools/call round-trip and returns the parsed reply. We
// negotiate a session once per call because each test builds a fresh
// harness; this keeps the helper stateless from the caller's POV.
func mcpCall(t *testing.T, e *itest.Env, tool string, args map[string]any) map[string]any {
	t.Helper()

	post := func(sessionID string, payload map[string]any) (*http.Response, []byte) {
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		req, err := http.NewRequest(http.MethodPost, e.URL("/mcp"), bytes.NewReader(raw))
		if err != nil {
			t.Fatalf("build: %v", err)
		}
		req.Header.Set("Authorization", "Bearer "+e.APIKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		if sessionID != "" {
			req.Header.Set("Mcp-Session-Id", sessionID)
		}
		resp := e.Do(t, req)
		defer resp.Body.Close()
		data, _ := io.ReadAll(resp.Body)
		return resp, data
	}

	// 1) initialize
	initResp, initBody := post("", map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "itest", "version": "0.0.1"},
		},
	})
	if initResp.StatusCode >= 300 {
		t.Fatalf("initialize: status %d body=%s", initResp.StatusCode, string(initBody))
	}
	sessionID := initResp.Header.Get("Mcp-Session-Id")
	if sessionID == "" {
		// Some mcp-go versions lowercase the header — retry a lookup.
		for k, v := range initResp.Header {
			if strings.EqualFold(k, "Mcp-Session-Id") && len(v) > 0 {
				sessionID = v[0]
				break
			}
		}
	}

	// 2) notifications/initialized — fire-and-forget.
	_, _ = post(sessionID, map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	})

	// 3) tools/call
	callResp, callBody := post(sessionID, map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      tool,
			"arguments": args,
		},
	})
	if callResp.StatusCode >= 300 {
		t.Fatalf("tools/call %s: status %d body=%s", tool, callResp.StatusCode, string(callBody))
	}
	// Response may be an SSE event stream or a plain JSON document —
	// parse whichever came back.
	body := string(callBody)
	if idx := strings.Index(body, "data:"); idx >= 0 {
		// Take the last data: line's payload.
		for _, line := range strings.Split(body, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "data:") {
				body = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			}
		}
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatalf("parse tools/call reply: %v body=%s", err, string(callBody))
	}
	if errField, ok := out["error"]; ok && errField != nil {
		t.Fatalf("tools/call %s returned error: %v", tool, errField)
	}
	return out
}

// TestMCP_WriteAndSearchNoteRoundTrip proves the MCP JSON-RPC transport
// can drive the write_note → search_notes loop end-to-end.
func TestMCP_WriteAndSearchNoteRoundTrip(t *testing.T) {
	e := itest.New(t)
	// Prime the default project via a REST call.
	if resp, _ := e.GET(t, "/api/stats?project=_default"); resp.StatusCode >= 500 {
		t.Fatalf("prime stats: %d", resp.StatusCode)
	}

	writeReply := mcpCall(t, e, "write_note", map[string]any{
		"project": "_default",
		"key":     "mcp/hello",
		"content": "mcp round trip beacon",
	})
	if writeReply["result"] == nil {
		t.Fatalf("write_note missing result: %v", writeReply)
	}

	searchReply := mcpCall(t, e, "search_notes", map[string]any{
		"project": "_default",
		"query":   "beacon",
	})
	if searchReply["result"] == nil {
		t.Fatalf("search_notes missing result: %v", searchReply)
	}
	// Walk the result.content[*].text blob for the key we just wrote.
	blob, _ := json.Marshal(searchReply["result"])
	if !strings.Contains(string(blob), "mcp/hello") {
		t.Fatalf("search_notes did not surface the written note: %s", string(blob))
	}
}

// TestMCP_ListProjectsReturnsDefault asserts list_projects includes the
// default slug once it has been auto-registered.
func TestMCP_ListProjectsReturnsDefault(t *testing.T) {
	e := itest.New(t)
	// Prime the default project via a REST call so the registry row exists.
	if resp, _ := e.GET(t, "/api/stats?project=_default"); resp.StatusCode >= 500 {
		t.Fatalf("prime stats: %d", resp.StatusCode)
	}

	reply := mcpCall(t, e, "list_projects", map[string]any{})
	blob, _ := json.Marshal(reply["result"])
	if !strings.Contains(string(blob), "_default") {
		t.Fatalf("list_projects did not include _default: %s", string(blob))
	}
}

// TestMCP_StatsToolWorks asserts the `stats` docs tool returns a
// non-error, non-empty payload.
func TestMCP_StatsToolWorks(t *testing.T) {
	e := itest.New(t)
	// Prime the default project.
	if resp, _ := e.GET(t, "/api/stats?project=_default"); resp.StatusCode >= 500 {
		t.Fatalf("prime stats: %d", resp.StatusCode)
	}

	reply := mcpCall(t, e, "stats", map[string]any{"project": "_default"})
	res, ok := reply["result"]
	if !ok || res == nil {
		t.Fatalf("stats missing result: %v", reply)
	}
	blob, _ := json.Marshal(res)
	if len(blob) < 3 {
		t.Fatalf("stats result suspiciously empty: %s", string(blob))
	}
}
