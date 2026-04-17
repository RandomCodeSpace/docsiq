package notes

import (
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
