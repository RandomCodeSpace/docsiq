package notes

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// HistoryEntry is a single commit entry in a note's auto-commit log.
type HistoryEntry struct {
	Commit  string    `json:"commit"`
	Author  string    `json:"author"`
	Time    time.Time `json:"time"`
	Message string    `json:"message"`
}

// maxCommitMsgBytes caps the `<key>` / author value before it goes into
// `git commit -m`. Commit messages are normally tiny, but a note key
// could theoretically be up to MaxKeyLen (512) chars; a very long
// author / co-author trailer also gets truncated. 8 KB is a generous
// ceiling that still fits comfortably on any git invocation.
const maxCommitMsgBytes = 8 * 1024

// perProjectLocks guards `git add && git commit` pairs per notesDir so
// two concurrent writes to the same project can't interleave a stage
// from one note with a commit of another. The map itself is guarded by
// a single sync.Mutex — contention is low (one lock-lookup per write).
var (
	perProjectLocksMu sync.Mutex
	perProjectLocks   = map[string]*sync.Mutex{}
)

func lockFor(notesDir string) *sync.Mutex {
	abs, err := filepath.Abs(notesDir)
	if err != nil {
		abs = notesDir
	}
	perProjectLocksMu.Lock()
	defer perProjectLocksMu.Unlock()
	m, ok := perProjectLocks[abs]
	if !ok {
		m = &sync.Mutex{}
		perProjectLocks[abs] = m
	}
	return m
}

// gitLookupFn is overridable in tests. Production callers go through
// gitAvailable which memoizes the lookup via sync.OnceValue (P1-4).
var gitLookupFn = func() (string, error) { return exec.LookPath("git") }

// gitAvailable returns true if a `git` binary is resolvable on PATH.
// The result is memoized for process lifetime — git install state
// doesn't change during a running server and exec.LookPath is a full
// PATH scan (filesystem stats) that adds latency to every note write
// when cached uncondtionally.
//
// Reset by tests via resetGitAvailableCache().
var gitAvailable = sync.OnceValue(func() bool {
	_, err := gitLookupFn()
	return err == nil
})

// resetGitAvailableCache rebuilds the sync.OnceValue so tests can
// observe independent runs. Not exported — internal to the package.
func resetGitAvailableCache() {
	gitAvailable = sync.OnceValue(func() bool {
		_, err := gitLookupFn()
		return err == nil
	})
}

// initRepo initializes <notesDir>/.git on first use. Idempotent: if the
// directory already exists, this is a no-op. user.email/user.name are
// set LOCALLY (repo-scoped) so we never touch the global git config.
func initRepo(notesDir string) error {
	if _, err := os.Stat(filepath.Join(notesDir, ".git")); err == nil {
		return nil
	}
	if err := os.MkdirAll(notesDir, 0o755); err != nil {
		return fmt.Errorf("mkdir notes dir: %w", err)
	}
	if out, err := runGit(notesDir, "init"); err != nil {
		return fmt.Errorf("git init: %w (%s)", err, strings.TrimSpace(out))
	}
	if out, err := runGit(notesDir, "config", "user.email", "docsiq@local"); err != nil {
		return fmt.Errorf("git config email: %w (%s)", err, strings.TrimSpace(out))
	}
	if out, err := runGit(notesDir, "config", "user.name", "docsiq"); err != nil {
		return fmt.Errorf("git config name: %w (%s)", err, strings.TrimSpace(out))
	}
	return nil
}

// runGit invokes `git -C <notesDir> <args>` and returns combined output.
func runGit(notesDir string, args ...string) (string, error) {
	full := append([]string{"-C", notesDir}, args...)
	cmd := exec.Command("git", full...)
	// Detach from any inherited GIT_* env — the caller's config must
	// not influence this repo.
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_SYSTEM=/dev/null",
	)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	return buf.String(), err
}

// truncateForCommit caps a string to maxCommitMsgBytes (UTF-8-safe: cut
// at byte boundary, then trim trailing invalid UTF-8 fragment by
// walking back to a rune boundary).
func truncateForCommit(s string) string {
	if len(s) <= maxCommitMsgBytes {
		return s
	}
	cut := maxCommitMsgBytes
	// Walk back to a rune boundary.
	for cut > 0 && (s[cut]&0xC0) == 0x80 {
		cut--
	}
	return s[:cut] + "…"
}

// buildCommitMessage assembles the full message including the optional
// Co-Authored-By trailer.
func buildCommitMessage(subject, author string) string {
	subject = truncateForCommit(subject)
	if author == "" {
		return subject
	}
	author = truncateForCommit(author)
	// Conventional trailer format used by GitHub & friends.
	return subject + "\n\nCo-Authored-By: " + author + " <" + author + "@local>"
}

// autoCommit stages and commits the change for a single note. Failures
// are logged as WARN; the caller never observes an error.
//
// deleted=true means the file no longer exists — we use `git add -A`
// scoped to the file path (or `git rm`) to record the deletion. For
// creates/updates, `git add <file>` is sufficient.
func autoCommit(notesDir, key, author, subject string, deleted bool) {
	if notesDir == "" || key == "" {
		return
	}
	if !gitAvailable() {
		slog.Warn("note history: git not available", "notesDir", notesDir)
		return
	}

	// NOTE: the per-project lock is acquired by the CALLER (Write /
	// Delete) around the entire filesystem-write + commit sequence so
	// two concurrent writes to the same key can't clobber each other's
	// bytes before either commit runs. autoCommit assumes it holds the
	// lock already.

	if err := initRepo(notesDir); err != nil {
		slog.Warn("note history: git commit failed", "err", err)
		return
	}

	// Relative path of the note file from notesDir — `git -C` makes this
	// relative too. We always work with forward slashes inside git args.
	rel := filepath.ToSlash(filepath.FromSlash(key) + ".md")

	if deleted {
		// `git add -A -- <rel>` records removals as well as
		// modifications, which is what we want when the file was just
		// unlinked on disk.
		if out, err := runGit(notesDir, "add", "-A", "--", rel); err != nil {
			slog.Warn("note history: git commit failed",
				"err", fmt.Errorf("git add: %w (%s)", err, strings.TrimSpace(out)))
			return
		}
	} else {
		if out, err := runGit(notesDir, "add", "--", rel); err != nil {
			slog.Warn("note history: git commit failed",
				"err", fmt.Errorf("git add: %w (%s)", err, strings.TrimSpace(out)))
			return
		}
	}

	msg := buildCommitMessage(subject, author)
	// --allow-empty is intentionally NOT set: a Write() that doesn't
	// change the serialized bytes (same content + same UpdatedAt to
	// nanosecond precision) produces no diff and therefore no commit,
	// which is the desired behavior. `git commit` exits non-zero on
	// "nothing to commit" — treat that as a silent no-op rather than
	// a warning.
	if out, err := runGit(notesDir, "commit",
		"--no-gpg-sign",
		"-m", msg,
	); err != nil {
		if strings.Contains(out, "nothing to commit") ||
			strings.Contains(out, "no changes added") {
			return
		}
		slog.Warn("note history: git commit failed",
			"err", fmt.Errorf("git commit: %w (%s)", err, strings.TrimSpace(out)))
		return
	}
}

// History returns recent commits touching the given note key, newest
// first. If git is not installed or the repo has no history for the
// file, returns an empty slice with nil error — the endpoint should not
// 500 just because a project has never been committed to.
//
// limit <= 0 is treated as "no cap".
func History(notesDir, key string, limit int) ([]HistoryEntry, error) {
	if err := ValidateKey(key); err != nil {
		return nil, err
	}
	if !gitAvailable() {
		return []HistoryEntry{}, nil
	}
	if _, err := os.Stat(filepath.Join(notesDir, ".git")); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []HistoryEntry{}, nil
		}
		return nil, fmt.Errorf("stat .git: %w", err)
	}

	rel := filepath.ToSlash(filepath.FromSlash(key) + ".md")

	// %H=sha, %an=author name, %at=author unix epoch, %s=subject.
	// Null-byte separator keeps fields unambiguous even with whitespace
	// in author / subject.
	args := []string{
		"log",
		"--pretty=format:%H%x00%an%x00%at%x00%s",
	}
	if limit > 0 {
		args = append(args, "-n", strconv.Itoa(limit))
	}
	args = append(args, "--", rel)

	out, err := runGit(notesDir, args...)
	if err != nil {
		// Empty history on a brand-new repo returns exit 128 ("does
		// not have any commits yet"). Treat that as "no entries".
		if strings.Contains(out, "does not have any commits") ||
			strings.Contains(out, "unknown revision") {
			return []HistoryEntry{}, nil
		}
		return nil, fmt.Errorf("git log: %w (%s)", err, strings.TrimSpace(out))
	}
	out = strings.TrimRight(out, "\n")
	if out == "" {
		return []HistoryEntry{}, nil
	}

	lines := strings.Split(out, "\n")
	entries := make([]HistoryEntry, 0, len(lines))
	for _, line := range lines {
		parts := strings.SplitN(line, "\x00", 4)
		if len(parts) != 4 {
			continue
		}
		ts, _ := strconv.ParseInt(parts[2], 10, 64)
		entries = append(entries, HistoryEntry{
			Commit:  parts[0],
			Author:  parts[1],
			Time:    time.Unix(ts, 0).UTC(),
			Message: parts[3],
		})
	}
	return entries, nil
}
