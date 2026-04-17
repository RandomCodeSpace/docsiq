package store

import (
	"context"
	"testing"
)

// TestGetDocumentVersions_ScanColumnCount is a regression test for P0-1.
// Before the fix, the inline SELECT was missing indexed_mtime, causing
// "sql: expected 12 destination arguments in Scan, not 11".
func TestGetDocumentVersions_ScanColumnCount(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenForProject(dir, "testproj")
	if err != nil {
		t.Fatalf("OpenForProject: %v", err)
	}
	defer s.Close()

	ctx := context.Background()

	// v1 — canonical row
	v1 := &Document{
		ID:           "doc-v1",
		Path:         "/tmp/a.md",
		Title:        "v1",
		DocType:      "md",
		FileHash:     "hash-v1",
		Version:      1,
		IsLatest:     false,
		IndexedMtime: 1700000000,
	}
	if err := s.UpsertDocument(ctx, v1); err != nil {
		t.Fatalf("upsert v1: %v", err)
	}
	// v2 — points at v1 as canonical
	v2 := &Document{
		ID:           "doc-v2",
		Path:         "/tmp/a.md",
		Title:        "v2",
		DocType:      "md",
		FileHash:     "hash-v2",
		Version:      2,
		CanonicalID:  "doc-v1",
		IsLatest:     true,
		IndexedMtime: 1700000100,
	}
	if err := s.UpsertDocument(ctx, v2); err != nil {
		t.Fatalf("upsert v2: %v", err)
	}

	docs, err := s.GetDocumentVersions(ctx, "doc-v1")
	if err != nil {
		t.Fatalf("GetDocumentVersions: %v", err)
	}
	if len(docs) != 2 {
		t.Fatalf("expected 2 versions, got %d", len(docs))
	}
	if docs[0].Version != 1 || docs[1].Version != 2 {
		t.Fatalf("expected versions [1,2], got [%d,%d]", docs[0].Version, docs[1].Version)
	}
	if docs[0].IndexedMtime != 1700000000 {
		t.Errorf("indexed_mtime not round-tripped; got %d", docs[0].IndexedMtime)
	}
}
