package notes

import (
	"errors"
	"sync/atomic"
	"testing"
)

// TestGitAvailable_CachesLookup is a regression test for P1-4.
// gitAvailable() must memoize the first result for the life of the
// process — calling it repeatedly must NOT re-run exec.LookPath on
// every invocation.
func TestGitAvailable_CachesLookup(t *testing.T) {
	var calls atomic.Int32
	// Inject a fake lookup and reset the cache so the next call
	// flows through our instrumented hook.
	origFn := gitLookupFn
	t.Cleanup(func() {
		gitLookupFn = origFn
		resetGitAvailableCache()
	})
	gitLookupFn = func() (string, error) {
		calls.Add(1)
		return "/fake/git", nil
	}
	resetGitAvailableCache()

	// First call — expect one lookup.
	if !gitAvailable() {
		t.Fatalf("gitAvailable returned false; want true")
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("first call: lookup invoked %d times; want 1", got)
	}

	// Ten more calls — lookup must stay at 1.
	for i := 0; i < 10; i++ {
		_ = gitAvailable()
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("after 10 more calls: lookup invoked %d times; want still 1", got)
	}
}

// TestGitAvailable_CacheWorksForMissingGit covers the "git not on
// PATH" branch — the negative result must also be cached.
func TestGitAvailable_CacheWorksForMissingGit(t *testing.T) {
	var calls atomic.Int32
	origFn := gitLookupFn
	t.Cleanup(func() {
		gitLookupFn = origFn
		resetGitAvailableCache()
	})
	gitLookupFn = func() (string, error) {
		calls.Add(1)
		return "", errors.New("exec: \"git\": executable file not found in $PATH")
	}
	resetGitAvailableCache()

	if gitAvailable() {
		t.Fatalf("gitAvailable returned true; want false when lookup errs")
	}
	for i := 0; i < 5; i++ {
		_ = gitAvailable()
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("lookup invoked %d times across 6 calls; want 1 (negative result must also cache)", got)
	}
}
