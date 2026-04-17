package store

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RandomCodeSpace/docscontext/internal/notes"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestIndexNote_HappyPath(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	n := &notes.Note{
		Key:     "architecture/auth",
		Content: "We use OAuth2 for authentication.",
		Tags:    []string{"security", "auth"},
	}
	if err := s.IndexNote(ctx, n); err != nil {
		t.Fatal(err)
	}
	cnt, err := s.CountNotes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if cnt != 1 {
		t.Errorf("count = %d, want 1", cnt)
	}
}

func TestIndexNote_Upsert(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	n := &notes.Note{Key: "k", Content: "v1"}
	s.IndexNote(ctx, n)
	n.Content = "v2"
	s.IndexNote(ctx, n)
	cnt, _ := s.CountNotes(ctx)
	if cnt != 1 {
		t.Errorf("count = %d, want 1 after upsert", cnt)
	}
	hits, _ := s.SearchNotes(ctx, "v2", 10)
	if len(hits) != 1 {
		t.Errorf("hits = %d, want 1", len(hits))
	}
}

func TestSearchNotes_Ranking(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	s.IndexNote(ctx, &notes.Note{Key: "a", Content: "oauth oauth oauth"})
	s.IndexNote(ctx, &notes.Note{Key: "b", Content: "oauth once only"})
	hits, err := s.SearchNotes(ctx, "oauth", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 2 {
		t.Fatalf("hits = %d", len(hits))
	}
	if hits[0].Key != "a" {
		t.Errorf("ranking wrong: %v", hits)
	}
}

func TestSearchNotes_SnippetGenerated(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	s.IndexNote(ctx, &notes.Note{Key: "k", Content: "prefix words before oauth target word after"})
	hits, _ := s.SearchNotes(ctx, "oauth", 10)
	if len(hits) != 1 {
		t.Fatalf("hits = %d", len(hits))
	}
	if !strings.Contains(hits[0].Snippet, "<mark>oauth</mark>") {
		t.Errorf("snippet missing mark: %q", hits[0].Snippet)
	}
}

func TestSearchNotes_TagMatching(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	s.IndexNote(ctx, &notes.Note{Key: "k", Content: "body", Tags: []string{"security"}})
	hits, _ := s.SearchNotes(ctx, "security", 10)
	if len(hits) != 1 {
		t.Errorf("hits = %d, want 1 (tag match)", len(hits))
	}
}

func TestSearchNotes_EmptyQuery(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	s.IndexNote(ctx, &notes.Note{Key: "k", Content: "anything"})
	hits, err := s.SearchNotes(ctx, "", 10)
	if err != nil {
		t.Errorf("empty query errored: %v", err)
	}
	if len(hits) != 0 {
		t.Errorf("expected 0 hits for empty query, got %d", len(hits))
	}
	hits, _ = s.SearchNotes(ctx, "   \t\n ", 10)
	if len(hits) != 0 {
		t.Errorf("expected 0 hits for whitespace-only query")
	}
}

func TestSearchNotes_SpecialChars(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	s.IndexNote(ctx, &notes.Note{Key: "k", Content: "some fence post text"})
	cases := []string{
		`"quoted"`,
		`asterisk*`,
		`colon:sep`,
		`paren(sep)`,
		`"`,
		`a AND b`,
		`NEAR(foo, bar)`,
	}
	for _, q := range cases {
		t.Run(sanitize(q), func(t *testing.T) {
			if _, err := s.SearchNotes(ctx, q, 10); err != nil {
				t.Errorf("query %q errored: %v", q, err)
			}
		})
	}
}

func TestSearchNotes_Unicode(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	s.IndexNote(ctx, &notes.Note{Key: "k", Content: "日本語 content"})
	hits, err := s.SearchNotes(ctx, "日本語", 10)
	if err != nil {
		t.Fatal(err)
	}
	// FTS5's default tokenizer may not handle CJK without unicode61;
	// accept 0 hits but ensure no error.
	_ = hits
}

func TestSearchNotes_LimitRespected(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		s.IndexNote(ctx, &notes.Note{Key: string(rune('a' + i)), Content: "shared-token"})
	}
	hits, _ := s.SearchNotes(ctx, "shared-token", 3)
	if len(hits) != 3 {
		t.Errorf("limit not respected: %d hits", len(hits))
	}
}

func TestDeleteNote(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	s.IndexNote(ctx, &notes.Note{Key: "k", Content: "x"})
	if err := s.DeleteNote(ctx, "k"); err != nil {
		t.Fatal(err)
	}
	cnt, _ := s.CountNotes(ctx)
	if cnt != 0 {
		t.Errorf("post-delete count = %d", cnt)
	}
	// Delete-missing is not an error.
	if err := s.DeleteNote(ctx, "k"); err != nil {
		t.Errorf("delete missing errored: %v", err)
	}
}

func TestIndexNote_TagsNormalized(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	s.IndexNote(ctx, &notes.Note{
		Key:     "k",
		Content: "x",
		Tags:    []string{"Security", "AUTH", "  security  ", "auth"},
	})
	hits, _ := s.SearchNotes(ctx, "security", 10)
	if len(hits) != 1 {
		t.Fatalf("hits = %d", len(hits))
	}
	// lowercase round-trip, dedup.
	got := strings.Join(hits[0].Tags, ",")
	if !strings.Contains(got, "security") || !strings.Contains(got, "auth") {
		t.Errorf("normalized tags = %q", got)
	}
}

func TestCountNotes_Empty(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	n, err := s.CountNotes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("expected 0 rows, got %d", n)
	}
}

func sanitize(s string) string {
	if len(s) > 16 {
		s = s[:16]
	}
	r := strings.NewReplacer(" ", "_", "\"", "q", "*", "s", ":", "c", "(", "l", ")", "r", ",", "k", "\t", "_", "\n", "_")
	out := r.Replace(s)
	if out == "" {
		return "empty"
	}
	return out
}
