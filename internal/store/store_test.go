package store

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// expectedTables lists the tables every freshly-opened store must have.
// NOTE: the schema names the community-entity join table `community_members`
// (not `community_reports`). There is no `community_reports` table today.
var expectedTables = []string{
	"documents",
	"chunks",
	"embeddings",
	"entities",
	"relationships",
	"claims",
	"communities",
	"community_members",
}

func listTables(t *testing.T, s *Store) map[string]bool {
	t.Helper()
	rows, err := s.DB().Query(`SELECT name FROM sqlite_master WHERE type='table'`)
	if err != nil {
		t.Fatalf("query sqlite_master: %v", err)
	}
	defer rows.Close()

	got := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got[name] = true
	}
	return got
}

func TestOpenForProject(t *testing.T) {
	t.Run("happy_path_creates_dir_and_db", func(t *testing.T) {
		dir := t.TempDir()
		s, err := OpenForProject(dir, "my-project")
		if err != nil {
			t.Fatalf("OpenForProject: %v", err)
		}
		defer s.Close()

		want := filepath.Join(dir, "projects", "my-project", "docsiq.db")
		if _, err := os.Stat(want); err != nil {
			t.Errorf("db file not created at %s: %v", want, err)
		}
		got := listTables(t, s)
		if !got["documents"] {
			t.Error("documents table missing after OpenForProject")
		}
	})

	t.Run("schema_has_expected_tables", func(t *testing.T) {
		dir := t.TempDir()
		s, err := OpenForProject(dir, "testproj")
		if err != nil {
			t.Fatalf("OpenForProject: %v", err)
		}
		defer s.Close()

		got := listTables(t, s)
		var missing []string
		for _, want := range expectedTables {
			if !got[want] {
				missing = append(missing, want)
			}
		}
		sort.Strings(missing)
		if len(missing) > 0 {
			t.Errorf("missing tables after migrate: %v", missing)
		}
	})

	t.Run("foreign_keys_pragma_enabled", func(t *testing.T) {
		dir := t.TempDir()
		s, err := OpenForProject(dir, "testproj")
		if err != nil {
			t.Fatalf("OpenForProject: %v", err)
		}
		defer s.Close()
		var on int
		if err := s.DB().QueryRow(`PRAGMA foreign_keys`).Scan(&on); err != nil {
			t.Fatalf("PRAGMA foreign_keys: %v", err)
		}
		if on != 1 {
			t.Errorf("foreign_keys = %d, want 1", on)
		}
	})

	t.Run("journal_mode_is_wal", func(t *testing.T) {
		dir := t.TempDir()
		s, err := OpenForProject(dir, "testproj")
		if err != nil {
			t.Fatalf("OpenForProject: %v", err)
		}
		defer s.Close()
		var mode string
		if err := s.DB().QueryRow(`PRAGMA journal_mode`).Scan(&mode); err != nil {
			t.Fatalf("PRAGMA journal_mode: %v", err)
		}
		if mode != "wal" {
			t.Errorf("journal_mode = %q, want %q", mode, "wal")
		}
	})

	t.Run("persistence_across_close_and_reopen", func(t *testing.T) {
		dir := t.TempDir()
		ctx := context.Background()

		s1, err := OpenForProject(dir, "testproj")
		if err != nil {
			t.Fatalf("open 1: %v", err)
		}
		doc := &Document{ID: "persist-1", Path: "/a.md", Title: "A", DocType: "md", FileHash: "h1", IsLatest: true}
		if err := s1.UpsertDocument(ctx, doc); err != nil {
			t.Fatalf("UpsertDocument: %v", err)
		}
		if err := s1.Close(); err != nil {
			t.Fatalf("close: %v", err)
		}

		s2, err := OpenForProject(dir, "testproj")
		if err != nil {
			t.Fatalf("open 2: %v", err)
		}
		defer s2.Close()
		var count int
		if err := s2.DB().QueryRow(`SELECT COUNT(*) FROM documents WHERE id=?`, "persist-1").Scan(&count); err != nil {
			t.Fatalf("count: %v", err)
		}
		if count != 1 {
			t.Errorf("doc did not persist (count=%d)", count)
		}
	})

	t.Run("nested_slug_creation", func(t *testing.T) {
		// Non-existent parent data dir — OpenForProject must mkdir -p.
		parent := t.TempDir()
		dataDir := filepath.Join(parent, "deep", "data")
		s, err := OpenForProject(dataDir, "proj")
		if err != nil {
			t.Fatalf("OpenForProject nested: %v", err)
		}
		defer s.Close()
		if _, err := os.Stat(filepath.Join(dataDir, "projects", "proj")); err != nil {
			t.Errorf("nested project dir not created: %v", err)
		}
	})

	t.Run("empty_slug_rejected", func(t *testing.T) {
		if _, err := OpenForProject(t.TempDir(), ""); err == nil {
			t.Fatal("OpenForProject empty slug = nil, want error")
		}
	})

	t.Run("invalid_slug_chars_rejected", func(t *testing.T) {
		cases := []string{
			"UPPER",
			"has space",
			"has/slash",
			"has\\back",
			"dots.not.allowed",
			"..",
			"../escape",
			"nul\x00",
		}
		for _, slug := range cases {
			if _, err := OpenForProject(t.TempDir(), slug); err == nil {
				t.Errorf("OpenForProject(%q) = nil, want error", slug)
			}
		}
	})

	t.Run("empty_data_dir_rejected", func(t *testing.T) {
		if _, err := OpenForProject("", "ok"); err == nil {
			t.Fatal("OpenForProject empty dataDir = nil, want error")
		}
	})

	t.Run("two_projects_isolated", func(t *testing.T) {
		dir := t.TempDir()
		ctx := context.Background()

		a, err := OpenForProject(dir, "alpha")
		if err != nil {
			t.Fatalf("open alpha: %v", err)
		}
		defer a.Close()
		b, err := OpenForProject(dir, "beta")
		if err != nil {
			t.Fatalf("open beta: %v", err)
		}
		defer b.Close()

		doc := &Document{ID: "d-only-in-alpha", Path: "/x.md", Title: "A", DocType: "md", FileHash: "h-a", IsLatest: true}
		if err := a.UpsertDocument(ctx, doc); err != nil {
			t.Fatalf("UpsertDocument alpha: %v", err)
		}

		// Beta must not see alpha's document.
		var count int
		if err := b.DB().QueryRow(`SELECT COUNT(*) FROM documents WHERE id=?`, "d-only-in-alpha").Scan(&count); err != nil {
			t.Fatalf("count in beta: %v", err)
		}
		if count != 0 {
			t.Errorf("beta sees alpha's doc (count=%d); isolation broken", count)
		}

		// Alpha still has it.
		if err := a.DB().QueryRow(`SELECT COUNT(*) FROM documents WHERE id=?`, "d-only-in-alpha").Scan(&count); err != nil {
			t.Fatalf("count in alpha: %v", err)
		}
		if count != 1 {
			t.Errorf("alpha lost its doc (count=%d)", count)
		}
	})
}
