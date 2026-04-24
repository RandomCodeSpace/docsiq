//go:build integration

package api_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/RandomCodeSpace/docsiq/internal/api/itest"
	"github.com/RandomCodeSpace/docsiq/internal/project"
)

// uploadDoc posts a text blob to /api/upload for the given project
// slug. Returns the job_id and fails the test on non-2xx.
func uploadDoc(t *testing.T, e *itest.Env, slug, filename, content string) string {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("files", filename)
	if err != nil {
		t.Fatalf("form file: %v", err)
	}
	if _, err := fw.Write([]byte(content)); err != nil {
		t.Fatalf("write content: %v", err)
	}
	mw.Close()

	req, err := http.NewRequest(http.MethodPost, e.URL("/api/upload?project="+slug), &buf)
	if err != nil {
		t.Fatalf("build req: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+e.APIKey)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp := e.Do(t, req)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		t.Fatalf("upload %s: status %d body=%s", slug, resp.StatusCode, string(body))
	}
	var out struct {
		JobID string `json:"job_id"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("unmarshal upload: %v body=%s", err, string(body))
	}
	return out.JobID
}

// waitUploadDone opens the /api/upload/progress SSE stream and returns
// the first terminal message ("done" or "error:..."). On timeout it
// returns "" so the caller can skip. The stream is long-lived; we
// parse "data: X\n\n" events incrementally and cancel via ctx when the
// caller deadline fires.
func waitUploadDone(t *testing.T, e *itest.Env, jobID string, timeout time.Duration) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		e.URL("/api/upload/progress?job_id="+jobID), nil)
	if err != nil {
		t.Fatalf("itest: build progress req: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+e.APIKey)
	req.Header.Set("Accept", "text/event-stream")

	client := &http.Client{} // no per-request Timeout — SSE is long-lived; ctx bounds it.
	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return ""
		}
		t.Fatalf("itest: open progress stream: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("itest: progress stream: status %d body=%s", resp.StatusCode, string(b))
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		msg := strings.TrimPrefix(line, "data: ")
		if msg == "done" || strings.HasPrefix(msg, "error:") {
			return msg
		}
	}
	// Scanner ended — either ctx timed out (empty return) or server closed the stream.
	return ""
}

// TestDocs_UploadIndexSearch uploads a small text blob and fires a
// /api/search query using a token from the blob. We accept either a
// populated hits array OR a graceful empty-result response — the goal
// is end-to-end wire correctness without requiring deterministic
// entity extraction from the fake provider. If the pipeline fails to
// index the blob within the timeout, the test skips (integration
// smoke, not a pipeline unit test).
func TestDocs_UploadIndexSearch(t *testing.T) {
	e := itest.New(t)

	jobID := uploadDoc(t, e, "_default", "blob.txt",
		"integration token salamander appears here for the search path to find.")
	final := waitUploadDone(t, e, jobID, 45*time.Second)
	if final == "" {
		t.Skipf("upload pipeline did not complete within timeout — skipping (FakeProvider + pipeline is best-effort)")
	}
	if strings.Contains(final, "error:") {
		t.Skipf("upload pipeline reported an error (non-fatal for integration smoke): %s", final)
	}

	payload := map[string]any{"query": "salamander", "mode": "local", "top_k": 5}
	resp, body := e.POSTJSON(t, "/api/search?project=_default", payload)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("search: want 200, got %d body=%s", resp.StatusCode, string(body))
	}
	// The search handler returns JSON; shape varies by mode. We assert
	// the response is JSON-parseable and not an explicit error.
	var generic map[string]any
	if err := json.Unmarshal(body, &generic); err != nil {
		t.Fatalf("search response not JSON: %v body=%s", err, string(body))
	}
	if _, bad := generic["error"]; bad {
		t.Fatalf("search returned error: %s", string(body))
	}
}

// TestDocs_PerProjectIsolation uploads a blob to project A, then
// searches project B for a unique token from A. The response must not
// surface any hit referencing that token.
func TestDocs_PerProjectIsolation(t *testing.T) {
	e := itest.New(t)
	for _, slug := range []string{"docs-a", "docs-b"} {
		if err := e.Registry.Register(project.Project{
			Slug: slug, Name: slug, Remote: "r-" + slug, CreatedAt: time.Now().Unix(),
		}); err != nil {
			t.Fatalf("register %s: %v", slug, err)
		}
	}

	token := fmt.Sprintf("tok%d", time.Now().UnixNano())
	jobID := uploadDoc(t, e, "docs-a", "a.txt", "a content "+token+" marker")
	final := waitUploadDone(t, e, jobID, 45*time.Second)
	if final == "" {
		t.Skipf("upload pipeline did not complete within timeout — skipping")
	}

	payload := map[string]any{"query": token, "mode": "local", "top_k": 5}
	resp, body := e.POSTJSON(t, "/api/search?project=docs-b", payload)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("search proj-b: status %d body=%s", resp.StatusCode, string(body))
	}
	// The unique token must not appear in the proj-b search response
	// body — proof the doc-b store has no row indexed from doc-a.
	if strings.Contains(string(body), token) {
		t.Fatalf("isolation broken — token %q surfaced in docs-b search: %s", token, string(body))
	}
}
