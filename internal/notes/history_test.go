package notes

import (
	"os/exec"
	"strings"
	"sync"
	"testing"
)

// skipIfNoGit skips the test when git is not on PATH. Most CI images
// have git, but we still guard so local dev machines with a stripped
// PATH don't see spurious failures.
func skipIfNoGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
}

func mustWrite(t *testing.T, dir, key, content, author string) {
	t.Helper()
	n := &Note{Key: key, Content: content, Author: author}
	if err := Write(dir, n); err != nil {
		t.Fatalf("write %s: %v", key, err)
	}
}

func TestHistory_TwoWrites(t *testing.T) {
	skipIfNoGit(t)
	dir := t.TempDir()
	mustWrite(t, dir, "note-a", "first", "alice")
	mustWrite(t, dir, "note-a", "second", "alice")
	entries, err := History(dir, "note-a", 10)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("want 2 entries, got %d: %+v", len(entries), entries)
	}
	// Newest first — both have the same subject but distinct commits.
	if entries[0].Commit == entries[1].Commit {
		t.Fatalf("expected distinct commits, got %v", entries)
	}
	// Reverse-chronological: first entry is newer or equal.
	if entries[0].Time.Before(entries[1].Time) {
		t.Fatalf("expected newest-first ordering: %v", entries)
	}
	if entries[0].Message != "note: note-a" {
		t.Fatalf("unexpected subject %q", entries[0].Message)
	}
}

func TestHistory_WriteThenDelete(t *testing.T) {
	skipIfNoGit(t)
	dir := t.TempDir()
	mustWrite(t, dir, "ephemeral", "tmp", "bob")
	if err := Delete(dir, "ephemeral"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	entries, err := History(dir, "ephemeral", 10)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("want 2 entries, got %d: %+v", len(entries), entries)
	}
	if entries[0].Message != "remove: ephemeral" {
		t.Fatalf("want remove subject, got %q", entries[0].Message)
	}
	if entries[1].Message != "note: ephemeral" {
		t.Fatalf("want create subject, got %q", entries[1].Message)
	}
}

// TestHistory_NoGitBinary verifies the no-git fallback. We simulate the
// "no git" condition by setting PATH to an empty directory for the
// duration of the test so exec.LookPath fails.
func TestHistory_NoGitBinary(t *testing.T) {
	dir := t.TempDir()
	empty := t.TempDir()
	t.Setenv("PATH", empty)

	// Write should still succeed — the auto-commit warning is logged,
	// not returned.
	mustWrite(t, dir, "plain", "hello", "")

	entries, err := History(dir, "plain", 10)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("want empty slice, got %d", len(entries))
	}
	if entries == nil {
		t.Fatalf("want non-nil empty slice, got nil")
	}
}

func TestHistory_ConcurrentWrites(t *testing.T) {
	skipIfNoGit(t)
	dir := t.TempDir()

	var wg sync.WaitGroup
	const n = 10
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// Same key on purpose — race both writes through the
			// per-project mutex.
			if err := Write(dir, &Note{Key: "shared", Content: "v", Author: "x"}); err != nil {
				t.Errorf("write %d: %v", i, err)
			}
		}(i)
	}
	wg.Wait()

	entries, err := History(dir, "shared", 100)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(entries) != n {
		t.Fatalf("want %d entries, got %d", n, len(entries))
	}
	// All commits distinct.
	seen := map[string]bool{}
	for _, e := range entries {
		if seen[e.Commit] {
			t.Fatalf("duplicate commit %s", e.Commit)
		}
		seen[e.Commit] = true
	}
}

func TestHistory_UnicodeBody(t *testing.T) {
	skipIfNoGit(t)
	dir := t.TempDir()
	body := "日本語 — “smart quotes” — 🔥 emoji — тест"
	mustWrite(t, dir, "unicode", body, "山田 太郎")
	entries, err := History(dir, "unicode", 10)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(entries))
	}
	// Author is persisted on commit as the co-author trailer; the
	// commit AUTHOR (git `%an`) is still `docsiq`.
	if entries[0].Author != "docsiq" {
		t.Fatalf("commit author should be docsiq, got %q", entries[0].Author)
	}
}

func TestHistory_LongCommitMessage(t *testing.T) {
	skipIfNoGit(t)
	dir := t.TempDir()
	// A key at MaxKeyLen would produce a long subject; instead we
	// exercise the truncateForCommit helper on a synthesized oversize
	// author string through buildCommitMessage and verify no panic /
	// correct length bound.
	huge := strings.Repeat("a", maxCommitMsgBytes+100)
	msg := buildCommitMessage("note: x", huge)
	if !strings.Contains(msg, "Co-Authored-By:") {
		t.Fatalf("trailer missing")
	}
	// buildCommitMessage should have truncated the author. The author
	// appears twice in the trailer (name + email), so the upper bound
	// is ~2*maxCommitMsgBytes + small literal overhead.
	if len(msg) > 2*maxCommitMsgBytes+256 {
		t.Fatalf("message not truncated: %d bytes", len(msg))
	}
	// And the truncate marker is present.
	if !strings.Contains(msg, "…") {
		t.Fatalf("truncation marker missing")
	}

	// Also end-to-end: write with a very long author and confirm the
	// commit lands.
	mustWrite(t, dir, "longauth", "body", huge)
	entries, err := History(dir, "longauth", 10)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(entries))
	}
}

func TestHistory_NestedKey(t *testing.T) {
	skipIfNoGit(t)
	dir := t.TempDir()
	mustWrite(t, dir, "arch/auth/sso", "nested body", "alice")
	mustWrite(t, dir, "arch/auth/sso", "v2", "alice")
	entries, err := History(dir, "arch/auth/sso", 10)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("want 2 entries, got %d", len(entries))
	}
}

func TestHistory_LimitZeroMeansAll(t *testing.T) {
	skipIfNoGit(t)
	dir := t.TempDir()
	for i := 0; i < 5; i++ {
		mustWrite(t, dir, "k", "v", "a")
	}
	entries, err := History(dir, "k", 0)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(entries) != 5 {
		t.Fatalf("want 5, got %d", len(entries))
	}
}

func TestHistory_LimitCapsResults(t *testing.T) {
	skipIfNoGit(t)
	dir := t.TempDir()
	for i := 0; i < 5; i++ {
		mustWrite(t, dir, "k", "v", "a")
	}
	entries, err := History(dir, "k", 2)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("want 2 (limited), got %d", len(entries))
	}
}

func TestHistory_UnknownKeyReturnsEmpty(t *testing.T) {
	skipIfNoGit(t)
	dir := t.TempDir()
	// No writes at all — .git doesn't exist yet.
	entries, err := History(dir, "does-not-exist", 10)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("want empty, got %d", len(entries))
	}
}

func TestHistory_InvalidKeyRejected(t *testing.T) {
	// No git calls expected — ValidateKey runs first.
	dir := t.TempDir()
	if _, err := History(dir, "../escape", 10); err == nil {
		t.Fatalf("expected invalid-key error")
	}
}

func TestHistory_CoAuthorTrailer(t *testing.T) {
	skipIfNoGit(t)
	dir := t.TempDir()
	mustWrite(t, dir, "trailer", "body", "Alice Example")
	// Reach past the high-level History() API into git directly to
	// inspect the full commit body — `%s` only gives us the subject.
	out, err := runGit(dir, "log", "-1", "--pretty=format:%B", "--", "trailer.md")
	if err != nil {
		t.Fatalf("raw log: %v (%s)", err, out)
	}
	if !strings.Contains(out, "Co-Authored-By: Alice Example") {
		t.Fatalf("missing co-author trailer in:\n%s", out)
	}
}
