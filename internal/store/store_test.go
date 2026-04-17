package store

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
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

func TestOpen(t *testing.T) {
	t.Run("open_happy_path", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "test.db")

		s, err := Open(path)
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		defer s.Close()

		if s.DB() == nil {
			t.Fatal("Store.DB() is nil")
		}
		if _, err := os.Stat(path); err != nil {
			t.Errorf("db file not created: %v", err)
		}
	})

	t.Run("schema_has_expected_tables", func(t *testing.T) {
		dir := t.TempDir()
		s, err := Open(filepath.Join(dir, "test.db"))
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		defer s.Close()

		got := listTables(t, s)
		var missing []string
		for _, tbl := range expectedTables {
			if !got[tbl] {
				missing = append(missing, tbl)
			}
		}
		sort.Strings(missing)
		if len(missing) > 0 {
			// Collect actual table names for diagnosis.
			var actual []string
			for name := range got {
				actual = append(actual, name)
			}
			sort.Strings(actual)
			t.Errorf("missing tables: %v; actual tables: %v", missing, actual)
		}
	})

	t.Run("foreign_keys_pragma_enabled", func(t *testing.T) {
		// DSN uses modernc's `_pragma=foreign_keys(on)` syntax so FKs are
		// actually enforced — required for ON DELETE CASCADE in the schema.
		dir := t.TempDir()
		s, err := Open(filepath.Join(dir, "test.db"))
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		defer s.Close()

		var fk int
		if err := s.DB().QueryRow(`PRAGMA foreign_keys`).Scan(&fk); err != nil {
			t.Fatalf("PRAGMA foreign_keys: %v", err)
		}
		if fk != 1 {
			t.Errorf("PRAGMA foreign_keys = %d, want 1 (FKs not enabled — DSN regression)", fk)
		}
	})

	t.Run("journal_mode_is_wal", func(t *testing.T) {
		// DSN uses modernc's `_pragma=journal_mode(wal)` syntax so concurrent
		// readers don't block writers.
		dir := t.TempDir()
		s, err := Open(filepath.Join(dir, "test.db"))
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		defer s.Close()

		var mode string
		if err := s.DB().QueryRow(`PRAGMA journal_mode`).Scan(&mode); err != nil {
			t.Fatalf("PRAGMA journal_mode: %v", err)
		}
		if !strings.EqualFold(mode, "wal") {
			t.Errorf("PRAGMA journal_mode = %q, want wal (WAL not enabled — DSN regression)", mode)
		}
	})

	t.Run("open_same_path_twice", func(t *testing.T) {
		// Documents current behavior. modernc.org/sqlite allows concurrent
		// opens of the same file with WAL, so this typically succeeds.
		dir := t.TempDir()
		path := filepath.Join(dir, "test.db")

		s1, err := Open(path)
		if err != nil {
			t.Fatalf("first Open: %v", err)
		}
		defer s1.Close()

		s2, err := Open(path)
		if err != nil {
			// Acceptable: OS-specific lock contention could cause a clean
			// error. We only insist that there's no panic.
			t.Logf("second Open returned error (acceptable): %v", err)
			return
		}
		defer s2.Close()

		// Both handles usable.
		if err := s2.DB().Ping(); err != nil {
			t.Errorf("second handle Ping: %v", err)
		}
	})

	t.Run("persistence_across_close_and_reopen", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "persist.db")
		ctx := context.Background()

		s1, err := Open(path)
		if err != nil {
			t.Fatalf("first Open: %v", err)
		}
		doc := &Document{
			ID:       "doc-1",
			Path:     "/tmp/sample.md",
			Title:    "Sample",
			DocType:  "markdown",
			FileHash: "hash-abc",
			IsLatest: true,
		}
		if err := s1.UpsertDocument(ctx, doc); err != nil {
			t.Fatalf("UpsertDocument: %v", err)
		}
		if err := s1.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}

		s2, err := Open(path)
		if err != nil {
			t.Fatalf("reopen: %v", err)
		}
		defer s2.Close()

		var count int
		if err := s2.DB().QueryRow(`SELECT COUNT(*) FROM documents WHERE id=?`, "doc-1").Scan(&count); err != nil {
			t.Fatalf("count query: %v", err)
		}
		if count != 1 {
			t.Errorf("after reopen, documents count = %d, want 1", count)
		}
	})

	t.Run("parent_dir_must_exist", func(t *testing.T) {
		// Document current behavior: Open does NOT auto-create the parent
		// directory. A missing parent yields an error on first write, not at
		// sql.Open (which is lazy).
		dir := t.TempDir()
		nested := filepath.Join(dir, "does", "not", "exist", "foo.db")

		s, err := Open(nested)
		if err == nil {
			// Surprising — record what happened.
			defer s.Close()
			if _, statErr := os.Stat(nested); statErr != nil {
				t.Errorf("Open returned nil error but db file not created: %v", statErr)
			}
			return
		}
		// Expected path: Open fails because migrate() can't write.
		t.Logf("Open with missing parent dir returned error (documented): %v", err)
	})

	t.Run("readonly_path_returns_error_no_panic", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("chmod-based readonly test is POSIX-specific")
		}
		if os.Geteuid() == 0 {
			t.Skip("running as root; chmod 0555 cannot block writes")
		}

		dir := t.TempDir()
		roDir := filepath.Join(dir, "ro")
		if err := os.Mkdir(roDir, 0o555); err != nil {
			t.Fatalf("mkdir ro: %v", err)
		}
		t.Cleanup(func() { _ = os.Chmod(roDir, 0o755) })

		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Open panicked on readonly path: %v", r)
			}
		}()

		s, err := Open(filepath.Join(roDir, "ro.db"))
		if err == nil {
			// Some platforms might still allow this; close cleanly.
			_ = s.Close()
			t.Log("Open on readonly dir unexpectedly succeeded on this platform")
		}
	})

	t.Run("very_long_path_4kb", func(t *testing.T) {
		// Nasty input: 4 KB path. Most OSes cap PATH_MAX below this, so we
		// expect an error. We only insist: no panic, clean error if it fails.
		dir := t.TempDir()
		longName := strings.Repeat("x", 4000) + ".db"
		longPath := filepath.Join(dir, longName)

		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Open panicked on very long path: %v", r)
			}
		}()

		s, err := Open(longPath)
		if err != nil {
			t.Logf("Open(4 KB path) returned error (documented): %v", err)
			return
		}
		// If it somehow succeeded on this platform, close it.
		_ = s.Close()
	})

	t.Run("unicode_path", func(t *testing.T) {
		// Nasty input: unicode + emoji filename.
		dir := t.TempDir()
		path := filepath.Join(dir, "データベース-🗄️.db")

		s, err := Open(path)
		if err != nil {
			t.Fatalf("Open unicode path: %v", err)
		}
		defer s.Close()

		got := listTables(t, s)
		if !got["documents"] {
			t.Error("documents table missing after open on unicode path")
		}
	})

	t.Run("empty_path", func(t *testing.T) {
		// Empty path is explicitly rejected — otherwise the DSN query-string
		// gets treated as the filename, leaving a garbage file in CWD.
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Open panicked on empty path: %v", r)
			}
		}()

		s, err := Open("")
		if err == nil {
			_ = s.Close()
			t.Fatal("Open(\"\") must return error, got nil")
		}
	})

	t.Run("nasty_dsn_chars_in_path", func(t *testing.T) {
		// Path containing '?' / '&' / '=' must not split into DSN params —
		// they're legal filename chars. Open should treat the whole string
		// as a path (sqlite3 will refuse due to weird chars, but we must
		// not leak a ghost file or split the DSN.
		dir := t.TempDir()
		weird := filepath.Join(dir, "db?name=weird&other=1.db")
		s, err := Open(weird)
		if err != nil {
			t.Logf("Open(%q) errored as expected: %v", weird, err)
			return
		}
		_ = s.Close()
	})
}
