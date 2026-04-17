package project

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// ErrNotFound is returned when a registry lookup has no matching row.
var ErrNotFound = errors.New("project not found")

// ErrDuplicateRemote is returned when Register would violate the UNIQUE
// constraint on projects.remote (two projects cannot share a remote).
var ErrDuplicateRemote = errors.New("remote already registered to another project")

// Registry wraps a tiny SQLite database at $DATA_DIR/registry.db that
// stores the slug → project mapping. It uses the same DSN-pragma pattern
// as internal/store so WAL and foreign keys are actually enforced under
// the mattn/go-sqlite3 driver.
type Registry struct {
	db *sql.DB
}

// registrySchema defines the single `projects` table. Kept inline so the
// registry package has no runtime coupling to internal/store.
const registrySchema = `
CREATE TABLE IF NOT EXISTS projects (
    slug       TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    remote     TEXT NOT NULL UNIQUE,
    created_at INTEGER NOT NULL
);
`

// OpenRegistry opens (or creates) the registry DB at $DATA_DIR/registry.db.
// Creates dataDir (0o755) if missing. Fails if dataDir is empty.
func OpenRegistry(dataDir string) (*Registry, error) {
	if strings.TrimSpace(dataDir) == "" {
		return nil, fmt.Errorf("open registry: data dir is empty")
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("open registry: mkdir %s: %w", dataDir, err)
	}

	path := filepath.Join(dataDir, "registry.db")
	// Pragma syntax matches mattn/go-sqlite3 — the `?_foreign_keys=on`
	// and `?_journal_mode=WAL` shorthand form. Getting this wrong leaves
	// FKs disabled in registry.db.
	dsn := path + "?_journal_mode=WAL&_foreign_keys=on"
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("open registry: %w", err)
	}
	db.SetMaxOpenConns(1) // SQLite WAL — 1 writer, many readers.

	if _, err := db.Exec(registrySchema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("open registry: migrate: %w", err)
	}
	return &Registry{db: db}, nil
}

// Close releases the underlying database handle.
func (r *Registry) Close() error {
	if r == nil || r.db == nil {
		return nil
	}
	return r.db.Close()
}

// DB exposes the raw *sql.DB handle for callers that need it (e.g. tests
// or future admin tooling). Returns nil on a zero Registry.
func (r *Registry) DB() *sql.DB {
	if r == nil {
		return nil
	}
	return r.db
}

// Register inserts a new project. The slug must pass IsValidSlug. The
// remote must be non-empty; duplicates (by remote) return ErrDuplicateRemote.
// CreatedAt is set to time.Now().Unix() if the caller did not provide one.
func (r *Registry) Register(p Project) error {
	if !IsValidSlug(p.Slug) {
		return fmt.Errorf("register: invalid slug %q", p.Slug)
	}
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("register: name is empty")
	}
	if strings.TrimSpace(p.Remote) == "" {
		return fmt.Errorf("register: remote is empty")
	}
	if p.CreatedAt == 0 {
		p.CreatedAt = time.Now().Unix()
	}
	_, err := r.db.Exec(
		`INSERT INTO projects (slug, name, remote, created_at) VALUES (?,?,?,?)`,
		p.Slug, p.Name, p.Remote, p.CreatedAt)
	if err != nil {
		msg := err.Error()
		// mattn/go-sqlite3 surfaces UNIQUE violations as "UNIQUE constraint
		// failed: projects.<col>".
		if strings.Contains(msg, "UNIQUE") && strings.Contains(msg, "remote") {
			return ErrDuplicateRemote
		}
		if strings.Contains(msg, "UNIQUE") && strings.Contains(msg, "slug") {
			return fmt.Errorf("register: slug %q already exists", p.Slug)
		}
		return fmt.Errorf("register: %w", err)
	}
	return nil
}

// Get returns the project with the given slug, or ErrNotFound.
func (r *Registry) Get(slug string) (*Project, error) {
	row := r.db.QueryRow(
		`SELECT slug, name, remote, created_at FROM projects WHERE slug=?`, slug)
	return scanProject(row)
}

// GetByRemote returns the project with the given remote, or ErrNotFound.
func (r *Registry) GetByRemote(remote string) (*Project, error) {
	row := r.db.QueryRow(
		`SELECT slug, name, remote, created_at FROM projects WHERE remote=?`, remote)
	return scanProject(row)
}

// List returns all registered projects ordered by created_at ascending.
// Returns an empty slice (not nil) when the registry is empty.
func (r *Registry) List() ([]*Project, error) {
	rows, err := r.db.Query(
		`SELECT slug, name, remote, created_at FROM projects ORDER BY created_at ASC, slug ASC`)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	out := []*Project{}
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.Slug, &p.Name, &p.Remote, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("list projects: scan: %w", err)
		}
		out = append(out, &p)
	}
	return out, rows.Err()
}

// Delete removes a project row by slug. Returns ErrNotFound if no row
// matched. This does NOT touch the per-project data dir on disk — callers
// that want the files gone must rm -rf separately (see `projects delete
// --purge`).
func (r *Registry) Delete(slug string) error {
	res, err := r.db.Exec(`DELETE FROM projects WHERE slug=?`, slug)
	if err != nil {
		return fmt.Errorf("delete project: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete project: rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func scanProject(row *sql.Row) (*Project, error) {
	var p Project
	err := row.Scan(&p.Slug, &p.Name, &p.Remote, &p.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan project: %w", err)
	}
	return &p, nil
}
