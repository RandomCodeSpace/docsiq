package store

import (
	"context"
	"testing"
)

func TestDocumentIndexedMtimeRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenForProject(dir, "testproj")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()
	ctx := context.Background()

	doc := &Document{
		ID: "d1", Path: "/tmp/x.md", Title: "x", DocType: "md",
		FileHash: "h1", IsLatest: true, IndexedMtime: 12345,
	}
	if err := s.UpsertDocument(ctx, doc); err != nil {
		t.Fatalf("UpsertDocument: %v", err)
	}

	got, err := s.GetDocument(ctx, "d1")
	if err != nil {
		t.Fatalf("GetDocument: %v", err)
	}
	if got == nil {
		t.Fatal("GetDocument returned nil")
	}
	if got.IndexedMtime != 12345 {
		t.Errorf("IndexedMtime = %d, want 12345", got.IndexedMtime)
	}
}

func TestDeleteDocument(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenForProject(dir, "testproj")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()
	ctx := context.Background()

	if err := s.UpsertDocument(ctx, &Document{
		ID: "d1", Path: "/tmp/x.md", DocType: "md", FileHash: "h1", IsLatest: true,
	}); err != nil {
		t.Fatalf("UpsertDocument: %v", err)
	}
	n, err := s.DeleteDocument(ctx, "d1")
	if err != nil {
		t.Fatalf("DeleteDocument: %v", err)
	}
	if n != 1 {
		t.Errorf("rows affected = %d, want 1", n)
	}
	got, err := s.GetDocument(ctx, "d1")
	if err != nil {
		t.Fatalf("GetDocument: %v", err)
	}
	if got != nil {
		t.Error("doc still present after delete")
	}
}

func TestAllDocuments(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenForProject(dir, "testproj")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()
	ctx := context.Background()

	for i, id := range []string{"a", "b", "c"} {
		if err := s.UpsertDocument(ctx, &Document{
			ID: id, Path: "/tmp/" + id + ".md", DocType: "md",
			FileHash: id + "h", IsLatest: true, Version: i + 1,
		}); err != nil {
			t.Fatalf("UpsertDocument: %v", err)
		}
	}
	docs, err := s.AllDocuments(ctx)
	if err != nil {
		t.Fatalf("AllDocuments: %v", err)
	}
	if len(docs) != 3 {
		t.Errorf("len = %d, want 3", len(docs))
	}
}
