// Package project provides per-project identity, registry, and per-project
// SQLite storage for docsiq. A project is identified by a normalized git
// remote URL slug and persisted in $DATA_DIR/registry.db.
package project

// Project is the canonical identity of a scoped workspace in docsiq.
//
// Slug      — URL-safe identifier (charset [a-z0-9_-]) derived from Remote.
// Name      — human-readable display name (defaults to last path component of Remote).
// Remote    — normalized git remote URL, or "_default" / "legacy" for sentinel projects.
// CreatedAt — Unix epoch seconds when the project was first registered.
type Project struct {
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	Remote    string `json:"remote"`
	CreatedAt int64  `json:"created_at"`
}
