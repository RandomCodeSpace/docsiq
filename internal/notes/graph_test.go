package notes

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func TestBuildGraph_Isolated(t *testing.T) {
	dir := t.TempDir()
	for _, k := range []string{"a", "b", "c"} {
		if err := Write(dir, &Note{Key: k, Content: "nothing"}); err != nil {
			t.Fatal(err)
		}
	}
	g, err := BuildGraph(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Nodes) != 3 {
		t.Errorf("nodes = %d, want 3", len(g.Nodes))
	}
	if len(g.Edges) != 0 {
		t.Errorf("edges = %d, want 0", len(g.Edges))
	}
}

func TestBuildGraph_Linked(t *testing.T) {
	dir := t.TempDir()
	Write(dir, &Note{Key: "a", Content: "see [[b]] and [[c]]"})
	Write(dir, &Note{Key: "b", Content: "see [[c]]"})
	Write(dir, &Note{Key: "c", Content: "end"})
	g, err := BuildGraph(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Edges) != 3 {
		t.Errorf("edges = %d, want 3 (a→b, a→c, b→c)", len(g.Edges))
	}
}

func TestBuildGraph_Cycle(t *testing.T) {
	dir := t.TempDir()
	Write(dir, &Note{Key: "a", Content: "[[b]]"})
	Write(dir, &Note{Key: "b", Content: "[[c]]"})
	Write(dir, &Note{Key: "c", Content: "[[a]]"})
	g, err := BuildGraph(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Edges) != 3 {
		t.Errorf("edges = %d, want 3", len(g.Edges))
	}
	related := Related(g, "a")
	sort.Strings(related)
	want := []string{"b", "c"}
	if !reflect.DeepEqual(related, want) {
		t.Errorf("Related(a) = %v, want %v", related, want)
	}
}

func TestBuildGraph_SelfLink(t *testing.T) {
	dir := t.TempDir()
	Write(dir, &Note{Key: "a", Content: "I link [[a]] to myself"})
	g, err := BuildGraph(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Edges) != 1 {
		t.Errorf("edges = %d, want 1", len(g.Edges))
	}
	// Self-link is de-duped out of Related.
	if r := Related(g, "a"); len(r) != 0 {
		t.Errorf("Related(a) = %v, want empty (self-link)", r)
	}
}

func TestBuildGraph_DanglingLink(t *testing.T) {
	dir := t.TempDir()
	Write(dir, &Note{Key: "a", Content: "[[does-not-exist]]"})
	g, err := BuildGraph(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(g.Edges) != 1 {
		t.Errorf("edges = %d, want 1 (dangling kept)", len(g.Edges))
	}
	if g.Edges[0].Target != "does-not-exist" {
		t.Errorf("target = %q", g.Edges[0].Target)
	}
}

func TestRelated_Bidirectional(t *testing.T) {
	dir := t.TempDir()
	Write(dir, &Note{Key: "a", Content: "[[b]]"}) // a → b
	Write(dir, &Note{Key: "b", Content: "nothing"})
	Write(dir, &Note{Key: "c", Content: "[[b]]"}) // c → b
	g, _ := BuildGraph(dir)
	r := Related(g, "b")
	sort.Strings(r)
	if !reflect.DeepEqual(r, []string{"a", "c"}) {
		t.Errorf("Related(b) = %v, want [a c]", r)
	}
}

func TestRelated_NilGraph(t *testing.T) {
	if r := Related(nil, "x"); r != nil {
		t.Errorf("Related(nil) = %v, want nil", r)
	}
}

func TestBuildGraph_WikilinksInFrontmatterBodyOnly(t *testing.T) {
	// Wikilinks in frontmatter should NOT produce edges — only body
	// content is scanned. This matches kgraph.
	dir := t.TempDir()
	n := &Note{
		Key:         "a",
		Content:     "body with no link",
		Frontmatter: map[string]any{"refs": "[[ghost]]"},
	}
	Write(dir, n)
	g, _ := BuildGraph(dir)
	if len(g.Edges) != 0 {
		t.Errorf("edges = %d, want 0 (fm refs must not produce edges)", len(g.Edges))
	}
}

// TestBuildGraph_CrossProject verifies that a wikilink of the form
// [[projects/B/x]] in project A produces a cross-project edge and a stub
// node marked with project="B". When the target note exists on disk, the
// stub must NOT be marked missing.
func TestBuildGraph_CrossProject(t *testing.T) {
	// Build a fake projectsRoot with two projects: A and B.
	root := t.TempDir()

	dirA := filepath.Join(root, "A", "notes")
	dirB := filepath.Join(root, "B", "notes")
	if err := os.MkdirAll(dirA, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dirB, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write note "a" in project A linking to B/x via cross-project wikilink.
	if err := Write(dirA, &Note{Key: "a", Content: "see [[projects/B/x]]"}); err != nil {
		t.Fatal(err)
	}
	// Write note "x" in project B so it exists on disk.
	if err := Write(dirB, &Note{Key: "x", Content: "I am x"}); err != nil {
		t.Fatal(err)
	}

	g, err := BuildGraph(dirA, root)
	if err != nil {
		t.Fatal(err)
	}

	// Must have exactly one edge: a → projects/B/x, cross_project=true.
	if len(g.Edges) != 1 {
		t.Fatalf("edges = %d, want 1; edges=%v", len(g.Edges), g.Edges)
	}
	edge := g.Edges[0]
	if edge.Source != "a" {
		t.Errorf("edge.Source = %q, want %q", edge.Source, "a")
	}
	if edge.Target != "projects/B/x" {
		t.Errorf("edge.Target = %q, want %q", edge.Target, "projects/B/x")
	}
	if !edge.CrossProject {
		t.Errorf("edge.CrossProject = false, want true")
	}

	// Find the stub node for project B.
	var stub *NoteNode
	for i := range g.Nodes {
		if g.Nodes[i].Key == "projects/B/x" {
			stub = &g.Nodes[i]
			break
		}
	}
	if stub == nil {
		t.Fatalf("no node for key %q in graph; nodes=%v", "projects/B/x", g.Nodes)
	}
	if stub.Project != "B" {
		t.Errorf("stub.Project = %q, want %q", stub.Project, "B")
	}
	// Target exists on disk → must NOT be marked missing.
	if stub.Missing {
		t.Errorf("stub.Missing = true, want false (note exists on disk)")
	}
}

// TestBuildGraph_CrossProjectMissing verifies that when the target note does
// not exist on disk, the stub node is marked missing=true.
func TestBuildGraph_CrossProjectMissing(t *testing.T) {
	root := t.TempDir()
	dirA := filepath.Join(root, "A", "notes")
	if err := os.MkdirAll(dirA, 0o755); err != nil {
		t.Fatal(err)
	}
	// Project B's notes dir is never created — target is absent.
	if err := Write(dirA, &Note{Key: "a", Content: "[[projects/B/ghost]]"}); err != nil {
		t.Fatal(err)
	}

	g, err := BuildGraph(dirA, root)
	if err != nil {
		t.Fatal(err)
	}

	var stub *NoteNode
	for i := range g.Nodes {
		if g.Nodes[i].Key == "projects/B/ghost" {
			stub = &g.Nodes[i]
			break
		}
	}
	if stub == nil {
		t.Fatal("stub node not found")
	}
	if !stub.Missing {
		t.Errorf("stub.Missing = false, want true (note absent on disk)")
	}
	if stub.Project != "B" {
		t.Errorf("stub.Project = %q, want %q", stub.Project, "B")
	}
}
