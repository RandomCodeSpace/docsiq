//go:build sqlite_fts5

package store

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/RandomCodeSpace/docsiq/internal/notes"
)

// FuzzSearchTokenize asserts that Store.SearchNotes never panics and never
// returns a "malformed MATCH expression" error for any query string. Any
// input that trips the latter must be fixed by pre-sanitising inside
// SearchNotes — the HTTP boundary cannot be trusted to pre-filter FTS5
// control characters, and clients routinely pass the raw search box.
func FuzzSearchTokenize(f *testing.F) {
	// Seeds cover the FTS5 grammar corners that historically broke:
	// empty / whitespace, unbalanced quotes / parens, bare operators,
	// column-qualified terms, prefix wildcards, NULL bytes, Unicode, RTL.
	seeds := []string{
		"",
		"   ",
		"hello world",
		`"unbalanced`,
		"(lonely",
		"AND",
		"NOT title:foo",
		"foo*",
		"\x00\x00",
		strings.Repeat("a", 4096),
		"你好 世界",
		"مرحبا", // RTL
		"a OR (b AND \"c d\")",
		"col:tag1 tag2",
		"--comment",
		"/* comment */",
		"foo bar -",
		";",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	// One shared store per fuzz process. Re-opening per iteration would
	// dominate runtime and produce no new coverage — the surface under
	// test is the query path, not Open.
	dir := f.TempDir()
	s, err := OpenForProject(dir, "fuzzproj")
	if err != nil {
		f.Fatalf("OpenForProject: %v", err)
	}
	f.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()

	// Seed a handful of notes so MATCH has something non-empty to scan.
	// An empty FTS5 index short-circuits most of the query planner and
	// would hide grammar bugs.
	for i, body := range []string{"alpha beta", "gamma delta", "title something"} {
		key := "k" + strconv.Itoa(i)
		if err := s.IndexNote(ctx, &notes.Note{Key: key, Content: body, Tags: []string{"tag"}}); err != nil {
			f.Fatalf("seed IndexNote: %v", err)
		}
	}

	f.Fuzz(func(t *testing.T, query string) {
		// We don't care about the result — only that the call completes
		// without panic and without leaking a raw FTS5 syntax error.
		_, err := s.SearchNotes(ctx, query, 5)
		if err != nil && strings.Contains(err.Error(), "malformed MATCH expression") {
			t.Fatalf("unsanitised FTS5 grammar leaked for query %q: %v", query, err)
		}
		// Other errors (e.g. context cancelled) are acceptable during
		// fuzzing; they are not the class of bug this target is hunting.
	})
}
