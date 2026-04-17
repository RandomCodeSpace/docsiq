package notes

import (
	"sort"
	"strings"
)

// Graph is the wikilink-derived relation between notes in a project.
type Graph struct {
	Nodes []NoteNode `json:"nodes"`
	Edges []Edge     `json:"edges"`
}

// NoteNode is a lean projection of a Note for graph rendering.
type NoteNode struct {
	Key    string   `json:"key"`
	Title  string   `json:"title"`
	Folder string   `json:"folder"`
	Tags   []string `json:"tags,omitempty"`
}

// Edge represents a directed `[[wikilink]]` reference from Source → Target.
type Edge struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

// BuildGraph walks every note in notesDir, extracts wikilinks from the
// body, and returns the resulting node+edge graph. Edges may point to
// nonexistent targets (dangling wikilinks) — that is intentional, and
// matches kgraph's behavior.
func BuildGraph(notesDir string) (*Graph, error) {
	allNotes, err := List(notesDir)
	if err != nil {
		return nil, err
	}
	g := &Graph{
		Nodes: make([]NoteNode, 0, len(allNotes)),
		Edges: []Edge{},
	}
	for _, n := range allNotes {
		node := NoteNode{
			Key:    n.Key,
			Title:  titleFromKey(n.Key),
			Folder: folderFromKey(n.Key),
			Tags:   append([]string(nil), n.Tags...),
		}
		g.Nodes = append(g.Nodes, node)
		for _, target := range ExtractWikilinks([]byte(n.Content)) {
			g.Edges = append(g.Edges, Edge{Source: n.Key, Target: target})
		}
	}
	sort.Slice(g.Nodes, func(i, j int) bool { return g.Nodes[i].Key < g.Nodes[j].Key })
	sort.SliceStable(g.Edges, func(i, j int) bool {
		if g.Edges[i].Source != g.Edges[j].Source {
			return g.Edges[i].Source < g.Edges[j].Source
		}
		return g.Edges[i].Target < g.Edges[j].Target
	})
	return g, nil
}

// Related returns the set of note keys directly connected to `key`, in
// either direction (outlinks + backlinks). Self-links are deduped. The
// result is sorted for deterministic API output.
func Related(g *Graph, key string) []string {
	if g == nil {
		return nil
	}
	seen := map[string]struct{}{}
	for _, e := range g.Edges {
		if e.Source == key && e.Target != key {
			seen[e.Target] = struct{}{}
		}
		if e.Target == key && e.Source != key {
			seen[e.Source] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func titleFromKey(key string) string {
	if i := strings.LastIndex(key, "/"); i >= 0 {
		return key[i+1:]
	}
	return key
}

func folderFromKey(key string) string {
	if i := strings.LastIndex(key, "/"); i >= 0 {
		return key[:i]
	}
	return ""
}
