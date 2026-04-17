package notes

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestValidateKey(t *testing.T) {
	bad := []string{
		"",
		"..",
		"../etc",
		"foo/../bar",
		"/abs",
		"a\x00b",
		"a\\b",
		"./x",
		"x/",
		strings.Repeat("a", MaxKeyLen+1),
	}
	for _, k := range bad {
		t.Run("reject_"+sanitize(k), func(t *testing.T) {
			if err := ValidateKey(k); err == nil {
				t.Errorf("expected error for %q", k)
			}
		})
	}
	good := []string{
		"a",
		"foo/bar",
		"architecture/auth",
		"日本語",
		strings.Repeat("a", MaxKeyLen),
	}
	for _, k := range good {
		t.Run("accept_"+sanitize(k), func(t *testing.T) {
			if err := ValidateKey(k); err != nil {
				t.Errorf("unexpected error for %q: %v", k, err)
			}
		})
	}
}

func TestWriteReadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	n := &Note{
		Key:     "architecture/auth",
		Content: "Body line 1\nBody line 2\n",
		Author:  "alice",
		Tags:    []string{"security", "auth"},
	}
	if err := Write(dir, n); err != nil {
		t.Fatal(err)
	}
	got, err := Read(dir, "architecture/auth")
	if err != nil {
		t.Fatal(err)
	}
	if got.Content != n.Content {
		t.Errorf("content = %q, want %q", got.Content, n.Content)
	}
	if got.Author != "alice" {
		t.Errorf("author = %q", got.Author)
	}
	if len(got.Tags) != 2 {
		t.Errorf("tags = %v", got.Tags)
	}
}

func TestWrite_CreatesNestedDirs(t *testing.T) {
	dir := t.TempDir()
	n := &Note{Key: "a/b/c/d/deep", Content: "x"}
	if err := Write(dir, n); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "a", "b", "c", "d", "deep.md")); err != nil {
		t.Fatal(err)
	}
}

func TestWrite_Atomic_NoTempLeft(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 5; i++ {
		if err := Write(dir, &Note{Key: "k", Content: fmt.Sprintf("v%d", i)}); err != nil {
			t.Fatal(err)
		}
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".note-") {
			t.Errorf("temp file left behind: %s", e.Name())
		}
	}
}

func TestWrite_RejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	bad := []string{"../escape", "/abs", "foo/../../../etc/passwd"}
	for _, k := range bad {
		t.Run(sanitize(k), func(t *testing.T) {
			err := Write(dir, &Note{Key: k, Content: "x"})
			if err == nil || !errors.Is(err, ErrInvalidKey) {
				t.Errorf("expected ErrInvalidKey for %q, got %v", k, err)
			}
		})
	}
}

func TestRead_NotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := Read(dir, "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestDelete(t *testing.T) {
	dir := t.TempDir()
	if err := Write(dir, &Note{Key: "k", Content: "x"}); err != nil {
		t.Fatal(err)
	}
	if err := Delete(dir, "k"); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(dir, "k"); !errors.Is(err, ErrNotFound) {
		t.Errorf("post-delete read: got %v, want ErrNotFound", err)
	}
	if err := Delete(dir, "k"); !errors.Is(err, ErrNotFound) {
		t.Errorf("second delete: got %v, want ErrNotFound", err)
	}
}

func TestList_Empty(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "does-not-exist")
	notes, err := List(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 0 {
		t.Errorf("expected 0 notes, got %d", len(notes))
	}
}

func TestList_Sorted(t *testing.T) {
	dir := t.TempDir()
	for _, k := range []string{"zebra", "alpha", "mid/middle"} {
		if err := Write(dir, &Note{Key: k, Content: "x"}); err != nil {
			t.Fatal(err)
		}
	}
	notes, err := List(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"alpha", "mid/middle", "zebra"}
	if len(notes) != 3 {
		t.Fatalf("got %d notes", len(notes))
	}
	for i, w := range want {
		if notes[i].Key != w {
			t.Errorf("notes[%d].Key = %q, want %q", i, notes[i].Key, w)
		}
	}
}

func TestTree(t *testing.T) {
	dir := t.TempDir()
	for _, k := range []string{"a", "folder/b", "folder/c", "folder/sub/d"} {
		if err := Write(dir, &Note{Key: k, Content: "x"}); err != nil {
			t.Fatal(err)
		}
	}
	tree, err := Tree(dir)
	if err != nil {
		t.Fatal(err)
	}
	if tree.Type != "folder" {
		t.Errorf("root type = %q", tree.Type)
	}
	if len(tree.Children) < 2 {
		t.Fatalf("expected >= 2 children, got %d", len(tree.Children))
	}
}

func TestConcurrentWrites_LastWriterWins(t *testing.T) {
	// Document the behavior: without external locking, N goroutines
	// writing the same key each produce a valid file (no partial
	// writes); the winner is whichever rename executed last. We assert
	// that no corruption survives.
	dir := t.TempDir()
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = Write(dir, &Note{Key: "k", Content: fmt.Sprintf("v%d", i)})
		}(i)
	}
	wg.Wait()
	n, err := Read(dir, "k")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(n.Content, "v") {
		t.Errorf("corrupted content: %q", n.Content)
	}
}

func TestUnicodeKey(t *testing.T) {
	dir := t.TempDir()
	if err := Write(dir, &Note{Key: "日本語/ノート", Content: "こんにちは"}); err != nil {
		t.Fatal(err)
	}
	n, err := Read(dir, "日本語/ノート")
	if err != nil {
		t.Fatal(err)
	}
	if n.Content != "こんにちは" {
		t.Errorf("content = %q", n.Content)
	}
}

func TestScale_1000Notes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping 1000-note scale test in -short mode")
	}
	dir := t.TempDir()
	for i := 0; i < 1000; i++ {
		k := fmt.Sprintf("bucket%d/note%d", i%10, i)
		if err := Write(dir, &Note{Key: k, Content: "x"}); err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
	}
	keys, err := ListKeys(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1000 {
		t.Errorf("expected 1000, got %d", len(keys))
	}
}

func TestFrontmatterPreserved(t *testing.T) {
	dir := t.TempDir()
	n := &Note{
		Key:         "k",
		Content:     "body",
		Frontmatter: map[string]any{"custom": "value"},
	}
	if err := Write(dir, n); err != nil {
		t.Fatal(err)
	}
	got, err := Read(dir, "k")
	if err != nil {
		t.Fatal(err)
	}
	if got.Frontmatter["custom"] != "value" {
		t.Errorf("custom fm field lost: %v", got.Frontmatter)
	}
}

func TestBodyWithTripleDashes(t *testing.T) {
	dir := t.TempDir()
	body := "prefix\n---\ndivider\n---\nsuffix\n"
	n := &Note{Key: "k", Content: body, Author: "a"}
	if err := Write(dir, n); err != nil {
		t.Fatal(err)
	}
	got, err := Read(dir, "k")
	if err != nil {
		t.Fatal(err)
	}
	if got.Content != body {
		t.Errorf("body not preserved:\nwant %q\ngot  %q", body, got.Content)
	}
}

func sanitize(s string) string {
	r := strings.NewReplacer("/", "_", "\\", "_", "\x00", "0", "..", "dotdot", ".", "dot", " ", "sp")
	out := r.Replace(s)
	if len(out) > 30 {
		return out[:30]
	}
	if out == "" {
		return "empty"
	}
	return out
}
