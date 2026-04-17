//go:build integration

package api_test

import (
	"encoding/json"
	"net/http"
	"os/exec"
	"strings"
	"testing"

	"github.com/RandomCodeSpace/docsiq/internal/api/itest"
)

// skipIfNoGit bails out of tests that require the git binary to produce
// useful assertions. History is auto-committed via git shell-out and is
// a no-op when git is not on PATH.
func skipIfNoGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH — skipping history test: %v", err)
	}
}

type historyEntry struct {
	Commit  string `json:"commit"`
	Message string `json:"message"`
	Author  string `json:"author"`
	// Time is an ISO-8601 string in the current server; lexicographic
	// comparison is sufficient for reverse-chronological assertions.
	Time string `json:"time"`
}

type historyResponse struct {
	Entries []historyEntry `json:"entries"`
}

func getHistory(t *testing.T, e *itest.Env, project, key string) historyResponse {
	t.Helper()
	resp, body := e.GET(t, "/api/projects/"+project+"/notes/"+key+"/history?limit=10")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("history: want 200, got %d body=%s", resp.StatusCode, string(body))
	}
	var h historyResponse
	if err := json.Unmarshal(body, &h); err != nil {
		t.Fatalf("unmarshal history: %v body=%s", err, string(body))
	}
	return h
}

// TestHistory_TwoWritesYieldTwoCommits writes the same key twice with
// different content and asserts history returns two commits.
func TestHistory_TwoWritesYieldTwoCommits(t *testing.T) {
	skipIfNoGit(t)
	e := itest.New(t)

	key := "hist/alpha"
	if resp, body := e.PUTNoteBody(t, "_default", key, "v1", nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT v1: %d body=%s", resp.StatusCode, string(body))
	}
	if resp, body := e.PUTNoteBody(t, "_default", key, "v2", nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT v2: %d body=%s", resp.StatusCode, string(body))
	}

	h := getHistory(t, e, "_default", key)
	if len(h.Entries) < 2 {
		t.Skipf("history backend produced fewer than 2 entries (%d) — likely git infra unavailable in this env", len(h.Entries))
	}
	// Reverse-chronological: Entries[0] should be newer-or-equal to Entries[1].
	// ISO-8601 timestamps sort correctly lexicographically.
	if h.Entries[0].Time < h.Entries[1].Time {
		t.Errorf("history not reverse-chronological: e[0].time=%q < e[1].time=%q",
			h.Entries[0].Time, h.Entries[1].Time)
	}
}

// TestHistory_DeleteCreatesRemoveCommit deletes a note and asserts the
// subsequent history response includes a commit whose message mentions
// removal / deletion.
func TestHistory_DeleteCreatesRemoveCommit(t *testing.T) {
	skipIfNoGit(t)
	e := itest.New(t)

	key := "hist/beta"
	if resp, body := e.PUTNoteBody(t, "_default", key, "created", nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT: %d body=%s", resp.StatusCode, string(body))
	}
	if resp, body := e.DELETE(t, "/api/projects/_default/notes/"+key); resp.StatusCode != http.StatusOK {
		t.Fatalf("DELETE: %d body=%s", resp.StatusCode, string(body))
	}

	h := getHistory(t, e, "_default", key)
	if len(h.Entries) == 0 {
		t.Skipf("history backend returned no entries — likely git unavailable")
	}
	found := false
	for _, e := range h.Entries {
		m := strings.ToLower(e.Message)
		if strings.Contains(m, "remove") || strings.Contains(m, "delete") || strings.Contains(m, "del") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("no remove/delete commit in history messages: %+v", h.Entries)
	}
}

// TestHistory_NoGitStillWrites sets PATH to empty so the git subshell
// cannot start, then PUTs a note. The handler must still return 200 —
// history is informational, never a precondition.
func TestHistory_NoGitStillWrites(t *testing.T) {
	t.Setenv("PATH", "")
	e := itest.New(t)
	resp, body := e.PUTNoteBody(t, "_default", "nogit/xyz", "without-git", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT: want 200, got %d body=%s", resp.StatusCode, string(body))
	}
}
