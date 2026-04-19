package notes

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Graph is the wikilink-derived relation between notes in a project.
type Graph struct {
	Nodes []NoteNode `json:"nodes"`
	Edges []Edge     `json:"edges"`
}

// NoteNode is a lean projection of a Note for graph rendering.
// The optional Project field is non-empty only for cross-project nodes.
// The optional Missing field is true when the target note does not exist on disk.
type NoteNode struct {
	Key     string   `json:"key"`
	Title   string   `json:"title"`
	Folder  string   `json:"folder"`
	Tags    []string `json:"tags,omitempty"`
	Project string   `json:"project,omitempty"` // non-empty for cross-project nodes
	Missing bool     `json:"missing,omitempty"` // true when target file is absent
}

// Edge represents a directed `[[wikilink]]` reference from Source → Target.
// The optional CrossProject field is true when Source and Target belong to
// different projects.
type Edge struct {
	Source       string `json:"source"`
	Target       string `json:"target"`
	CrossProject bool   `json:"cross_project,omitempty"`
}

// BuildGraph walks every note in notesDir, extracts wikilinks from the
// body, and returns the resulting node+edge graph. Edges may point to
// nonexistent targets (dangling wikilinks) — that is intentional, and
// matches kgraph's behavior.
//
// projectsRoot, when non-empty, is the parent directory that contains all
// project data dirs (i.e. the directory whose children are <slug>/notes/).
// It is used to resolve cross-project wikilinks. Pass "" to disable
// cross-project resolution (backward-compatible: same-project only).
func BuildGraph(notesDir string, projectsRoot ...string) (*Graph, error) {
	allNotes, err := List(notesDir)
	if err != nil {
		return nil, err
	}

	root := ""
	if len(projectsRoot) > 0 {
		root = projectsRoot[0]
	}

	g := &Graph{
		Nodes: make([]NoteNode, 0, len(allNotes)),
		Edges: []Edge{},
	}

	// Index existing node keys so we can detect cross-project stubs later.
	nodeIndex := make(map[string]struct{}, len(allNotes))

	for _, n := range allNotes {
		node := NoteNode{
			Key:    n.Key,
			Title:  titleFromKey(n.Key),
			Folder: folderFromKey(n.Key),
			Tags:   append([]string(nil), n.Tags...),
		}
		g.Nodes = append(g.Nodes, node)
		nodeIndex[n.Key] = struct{}{}
	}

	// Track cross-project stub nodes we have already added (keyed by node key).
	crossStubs := make(map[string]struct{})

	for _, n := range allNotes {
		for _, target := range ExtractWikilinks([]byte(n.Content)) {
			wl := ParseWikilink(target)
			if !wl.CrossProject {
				// Same-project link — emit as before (dangling is fine).
				g.Edges = append(g.Edges, Edge{Source: n.Key, Target: target})
				continue
			}

			// Cross-project link. The node key in the graph is the full
			// cross-project target string (e.g. "projects/B/x").
			nodeKey := target
			if _, exists := crossStubs[nodeKey]; !exists {
				crossStubs[nodeKey] = struct{}{}
				stub := NoteNode{
					Key:     nodeKey,
					Title:   titleFromKey(wl.Key),
					Folder:  folderFromKey(wl.Key),
					Project: wl.Project,
				}
				if root != "" {
					// Check whether the note file exists on disk.
					candidate := filepath.Join(root, wl.Project, "notes", wl.Key+".md")
					if _, serr := os.Stat(candidate); os.IsNotExist(serr) {
						stub.Missing = true
					}
				} else {
					// No root provided — mark missing conservatively.
					stub.Missing = true
				}
				g.Nodes = append(g.Nodes, stub)
				nodeIndex[nodeKey] = struct{}{}
			}

			g.Edges = append(g.Edges, Edge{
				Source:       n.Key,
				Target:       nodeKey,
				CrossProject: true,
			})
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
