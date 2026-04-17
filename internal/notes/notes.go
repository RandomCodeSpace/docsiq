package notes

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// MaxKeyLen caps note keys. Keeps paths well under filesystem limits even
// with the `.md` suffix and nested dirs. Phase-2 decision.
const MaxKeyLen = 512

// ErrNotFound is returned when a note key does not exist on disk.
var ErrNotFound = errors.New("note not found")

// ErrInvalidKey is returned when a key contains path-traversal
// components, absolute path prefixes, null bytes, or exceeds MaxKeyLen.
var ErrInvalidKey = errors.New("invalid note key")

// Note is the in-memory representation of a Markdown note.
//
// Frontmatter is the raw YAML map; Author/Tags are convenience copies
// hoisted from common keys. Timestamps are derived from filesystem
// mtime on Read and from time.Now() on Write.
type Note struct {
	Key         string         `json:"key"`
	Content     string         `json:"content"`
	Author      string         `json:"author,omitempty"`
	Tags        []string       `json:"tags,omitempty"`
	Frontmatter map[string]any `json:"frontmatter,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// TreeNode is the recursive folder/file tree returned by Tree().
type TreeNode struct {
	Name     string      `json:"name"`
	Path     string      `json:"path"`
	Type     string      `json:"type"` // "folder" | "note"
	Children []*TreeNode `json:"children,omitempty"`
}

// ValidateKey enforces the invariants documented on ErrInvalidKey.
// Public so handlers can short-circuit on bad input before touching the
// filesystem.
func ValidateKey(key string) error {
	if key == "" {
		return fmt.Errorf("%w: empty", ErrInvalidKey)
	}
	if len(key) > MaxKeyLen {
		return fmt.Errorf("%w: length %d exceeds %d", ErrInvalidKey, len(key), MaxKeyLen)
	}
	if strings.ContainsRune(key, 0) {
		return fmt.Errorf("%w: null byte", ErrInvalidKey)
	}
	if strings.Contains(key, "\\") {
		return fmt.Errorf("%w: backslash", ErrInvalidKey)
	}
	if strings.HasPrefix(key, "/") {
		return fmt.Errorf("%w: absolute path", ErrInvalidKey)
	}
	// Reject any `..` segment (covers ".." alone, "../x", "x/..", "x/../y").
	for seg := range strings.SplitSeq(key, "/") {
		if seg == ".." {
			return fmt.Errorf("%w: parent dir segment", ErrInvalidKey)
		}
		if seg == "." {
			return fmt.Errorf("%w: current dir segment", ErrInvalidKey)
		}
		if seg == "" {
			return fmt.Errorf("%w: empty segment", ErrInvalidKey)
		}
	}
	return nil
}

// resolvePath maps a key → absolute on-disk .md path rooted at notesDir.
// Also re-verifies the resolved path stays inside notesDir as a
// defense-in-depth check against any edge case that slipped past
// ValidateKey.
func resolvePath(notesDir, key string) (string, error) {
	if err := ValidateKey(key); err != nil {
		return "", err
	}
	cleanBase, err := filepath.Abs(notesDir)
	if err != nil {
		return "", fmt.Errorf("resolve notes dir: %w", err)
	}
	full := filepath.Join(cleanBase, filepath.FromSlash(key)+".md")
	cleanFull, err := filepath.Abs(full)
	if err != nil {
		return "", fmt.Errorf("resolve note path: %w", err)
	}
	// On Unix filepath.Abs + Join already produces a clean path; still
	// assert containment so a symlink or exotic key can't escape.
	if !strings.HasPrefix(cleanFull, cleanBase+string(os.PathSeparator)) && cleanFull != cleanBase {
		return "", fmt.Errorf("%w: escapes notes dir", ErrInvalidKey)
	}
	return cleanFull, nil
}

// Read loads the note at key from notesDir. Returns ErrNotFound if the
// `.md` file does not exist.
func Read(notesDir, key string) (*Note, error) {
	path, err := resolvePath(notesDir, key)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("read note %q: %w", key, err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat note %q: %w", key, err)
	}

	fm, body, err := ParseFrontmatter(raw)
	if err != nil {
		return nil, fmt.Errorf("parse note %q: %w", key, err)
	}
	n := &Note{
		Key:         key,
		Content:     string(body),
		Frontmatter: fm,
		UpdatedAt:   fi.ModTime(),
		CreatedAt:   fi.ModTime(),
	}
	if v, ok := fm["author"].(string); ok {
		n.Author = v
	}
	if v, ok := fm["created_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			n.CreatedAt = t
		}
	}
	if v, ok := fm["updated_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			n.UpdatedAt = t
		}
	}
	if tags, ok := fm["tags"]; ok {
		n.Tags = coerceStringSlice(tags)
	}
	return n, nil
}

// Write persists n to disk atomically (temp file + rename). Parent
// directories are created as needed. The frontmatter embedded in the
// file is built from n.Frontmatter, then overlaid with Author, Tags,
// CreatedAt, UpdatedAt so the struct fields win over stale map entries.
//
// Acquires the per-project mutex for the whole write+auto-commit
// sequence so concurrent writes to the same notesDir are serialized —
// without this, write N's bytes could land on disk before write N-1's
// commit runs, causing the earlier commit to record the later content.
func Write(notesDir string, n *Note) error {
	if n == nil {
		return fmt.Errorf("write note: nil note")
	}
	path, err := resolvePath(notesDir, n.Key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir note parent: %w", err)
	}

	mu := lockFor(notesDir)
	mu.Lock()
	defer mu.Unlock()

	now := time.Now().UTC()
	if n.CreatedAt.IsZero() {
		n.CreatedAt = now
	}
	n.UpdatedAt = now

	fm := map[string]any{}
	for k, v := range n.Frontmatter {
		fm[k] = v
	}
	if n.Author != "" {
		fm["author"] = n.Author
	}
	if len(n.Tags) > 0 {
		fm["tags"] = append([]string(nil), n.Tags...)
	}
	fm["created_at"] = n.CreatedAt.Format(time.RFC3339)
	// RFC3339Nano on updated_at — avoids identical file content when a
	// note is rewritten within the same wall-clock second, which would
	// otherwise cause git auto-commit to see no diff and drop the
	// entry from `git log -- <path>`.
	fm["updated_at"] = n.UpdatedAt.Format(time.RFC3339Nano)

	out, err := EncodeFrontmatter(fm, []byte(n.Content))
	if err != nil {
		return err
	}

	// Atomic write: temp file in the target dir, then rename.
	tmp, err := os.CreateTemp(filepath.Dir(path), ".note-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpName := tmp.Name()
	_, writeErr := tmp.Write(out)
	syncErr := tmp.Sync()
	closeErr := tmp.Close()
	if writeErr != nil || syncErr != nil || closeErr != nil {
		_ = os.Remove(tmpName)
		if writeErr != nil {
			return fmt.Errorf("write temp: %w", writeErr)
		}
		if syncErr != nil {
			return fmt.Errorf("sync temp: %w", syncErr)
		}
		return fmt.Errorf("close temp: %w", closeErr)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("rename note: %w", err)
	}

	// Best-effort git auto-commit — never fails the write.
	autoCommit(notesDir, n.Key, n.Author, fmt.Sprintf("note: %s", n.Key), false)
	return nil
}

// Delete removes the note file. Returns ErrNotFound if missing.
// Serialized via the per-project mutex (see Write).
func Delete(notesDir, key string) error {
	path, err := resolvePath(notesDir, key)
	if err != nil {
		return err
	}
	mu := lockFor(notesDir)
	mu.Lock()
	defer mu.Unlock()
	if err := os.Remove(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return ErrNotFound
		}
		return fmt.Errorf("delete note: %w", err)
	}
	// Best-effort git auto-commit on delete.
	autoCommit(notesDir, key, "", fmt.Sprintf("remove: %s", key), true)
	return nil
}

// List returns every note found under notesDir, sorted by key. Missing
// notesDir is treated as "no notes" (empty slice, nil error) — handlers
// should not 500 just because nobody has written a note yet.
func List(notesDir string) ([]*Note, error) {
	keys, err := listKeys(notesDir)
	if err != nil {
		return nil, err
	}
	out := make([]*Note, 0, len(keys))
	for _, k := range keys {
		n, err := Read(notesDir, k)
		if err != nil {
			// A file vanished between walk and read — skip.
			if errors.Is(err, ErrNotFound) {
				continue
			}
			return nil, err
		}
		out = append(out, n)
	}
	return out, nil
}

// ListKeys is the cheap variant of List: it walks the tree once and
// returns keys only. Useful for the lean `GET /api/projects/.../notes`
// endpoint where frontmatter isn't needed.
func ListKeys(notesDir string) ([]string, error) {
	return listKeys(notesDir)
}

func listKeys(notesDir string) ([]string, error) {
	info, err := os.Stat(notesDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("stat notes dir: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("notes dir %q is not a directory", notesDir)
	}

	var keys []string
	err = filepath.WalkDir(notesDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".md") {
			return nil
		}
		rel, err := filepath.Rel(notesDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		rel = strings.TrimSuffix(rel, ".md")
		keys = append(keys, rel)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(keys)
	return keys, nil
}

// Tree returns the recursive folder/note tree rooted at notesDir. Used
// by the `/api/projects/{p}/tree` endpoint.
func Tree(notesDir string) (*TreeNode, error) {
	root := &TreeNode{Name: "", Path: "", Type: "folder"}
	info, err := os.Stat(notesDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return root, nil
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("notes dir %q is not a directory", notesDir)
	}
	if err := buildTree(notesDir, "", root); err != nil {
		return nil, err
	}
	return root, nil
}

func buildTree(notesDir, relDir string, parent *TreeNode) error {
	absDir := filepath.Join(notesDir, filepath.FromSlash(relDir))
	entries, err := os.ReadDir(absDir)
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		rel := name
		if relDir != "" {
			rel = relDir + "/" + name
		}
		if e.IsDir() {
			node := &TreeNode{Name: name, Path: rel, Type: "folder"}
			if err := buildTree(notesDir, rel, node); err != nil {
				return err
			}
			parent.Children = append(parent.Children, node)
			continue
		}
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		key := strings.TrimSuffix(rel, ".md")
		parent.Children = append(parent.Children, &TreeNode{
			Name: strings.TrimSuffix(name, ".md"),
			Path: key,
			Type: "note",
		})
	}
	return nil
}

// coerceStringSlice turns an arbitrary yaml-decoded value into []string.
// Accepts []any, []string, or a single string; anything else → nil.
func coerceStringSlice(v any) []string {
	switch t := v.(type) {
	case []string:
		return append([]string(nil), t...)
	case []any:
		out := make([]string, 0, len(t))
		for _, x := range t {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case string:
		if t == "" {
			return nil
		}
		return []string{t}
	}
	return nil
}
