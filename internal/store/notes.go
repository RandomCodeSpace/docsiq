package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/RandomCodeSpace/docsiq/internal/notes"
)

// NoteHit is a single result from SearchNotes.
type NoteHit struct {
	Key     string   `json:"key"`
	Title   string   `json:"title"`
	Snippet string   `json:"snippet"`
	Tags    []string `json:"tags,omitempty"`
	Rank    float64  `json:"rank"`
}

// IndexNote upserts a note into notes_fts. DELETE-by-key + INSERT is used
// instead of `INSERT ... ON CONFLICT` because the contentless FTS5 table
// has no declared primary key.
//
// Tags are lowercased on index (decision: lowercase normalization so
// searches are case-insensitive). The title is derived from the last
// path segment of the key.
func (s *Store) IndexNote(ctx context.Context, n *notes.Note) error {
	if n == nil {
		return fmt.Errorf("index note: nil note")
	}
	if err := notes.ValidateKey(n.Key); err != nil {
		return err
	}
	title := n.Key
	if i := strings.LastIndex(n.Key, "/"); i >= 0 {
		title = n.Key[i+1:]
	}
	tagsStr := strings.Join(normalizeTags(n.Tags), " ")

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM notes_fts WHERE key = ?`, n.Key); err != nil {
		return fmt.Errorf("notes_fts delete: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO notes_fts(key, title, content, tags) VALUES (?, ?, ?, ?)`,
		n.Key, title, n.Content, tagsStr,
	); err != nil {
		return fmt.Errorf("notes_fts insert: %w", err)
	}
	return tx.Commit()
}

// DeleteNote removes a key from notes_fts. Not an error if the key is
// absent.
func (s *Store) DeleteNote(ctx context.Context, key string) error {
	if key == "" {
		return fmt.Errorf("delete note: empty key")
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM notes_fts WHERE key = ?`, key)
	return err
}

// CountNotes returns the row count of notes_fts.
func (s *Store) CountNotes(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM notes_fts`).Scan(&n)
	return n, err
}

// SearchNotes runs a FTS5 MATCH query ranked by bm25() and returns up to
// `limit` hits with a highlighted snippet. `limit` <= 0 defaults to 20.
//
// Empty / whitespace-only queries return an empty slice (no hits) rather
// than an error — callers can keep their code simple.
//
// Special chars in `query` are escaped by wrapping each whitespace-
// delimited token in double quotes (FTS5 string literal syntax). This
// lets callers pass arbitrary user text including `:`, `*`, `"`, etc.
// without triggering a "malformed MATCH expression" error.
func (s *Store) SearchNotes(ctx context.Context, query string, limit int) ([]NoteHit, error) {
	if limit <= 0 {
		limit = 20
	}
	q := strings.TrimSpace(query)
	if q == "" {
		return []NoteHit{}, nil
	}
	matchExpr := ftsEscape(q)

	rows, err := s.db.QueryContext(ctx, `
        SELECT key, title, snippet(notes_fts, 2, '<mark>', '</mark>', '…', 10), tags, bm25(notes_fts)
        FROM notes_fts
        WHERE notes_fts MATCH ?
        ORDER BY bm25(notes_fts)
        LIMIT ?`, matchExpr, limit)
	if err != nil {
		return nil, fmt.Errorf("notes search: %w", err)
	}
	defer rows.Close()

	var out []NoteHit
	for rows.Next() {
		var h NoteHit
		var tagsStr string
		if err := rows.Scan(&h.Key, &h.Title, &h.Snippet, &tagsStr, &h.Rank); err != nil {
			return nil, err
		}
		if tagsStr != "" {
			h.Tags = strings.Fields(tagsStr)
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// normalizeTags lowercases, trims, and de-dupes tags.
func normalizeTags(tags []string) []string {
	if len(tags) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(tags))
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		t = strings.ToLower(strings.TrimSpace(t))
		if t == "" {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	return out
}

// ftsEscape wraps each whitespace-delimited token in double quotes so
// user-supplied punctuation does not confuse FTS5's MATCH parser. Inner
// double quotes are doubled per SQLite's string-literal escape rules.
func ftsEscape(q string) string {
	tokens := strings.Fields(q)
	escaped := make([]string, 0, len(tokens))
	for _, t := range tokens {
		t = strings.ReplaceAll(t, `"`, `""`)
		escaped = append(escaped, `"`+t+`"`)
	}
	return strings.Join(escaped, " ")
}
