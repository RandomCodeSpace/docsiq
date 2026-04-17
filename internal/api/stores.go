package api

import (
	"fmt"
	"sync"

	"github.com/RandomCodeSpace/docsiq/internal/store"
)

// Storer is the narrow resolver contract that doc handlers and MCP doc
// tools use to obtain a per-project *store.Store on every request. The
// implementation returned by newProjectStores is the production one;
// tests may substitute a map-backed fake.
//
// Wave-2 policy: callers do NOT hold a long-lived *store.Store. Every
// handler invocation calls ForProject(slug) to get the correct DB for
// the request's project scope.
type Storer interface {
	// ForProject opens (or returns the cached) *store.Store for slug.
	// The caller MUST NOT call Close on the returned handle — the
	// Storer owns lifecycle.
	ForProject(slug string) (*store.Store, error)
}

// projectStores is a lazy, mutex-guarded cache of per-project *store.Store
// handles. One entry per slug; Close() on shutdown walks the map and
// closes every open DB. No TTL eviction yet — projects are long-lived
// and the per-store memory footprint is tiny (SQLite with one open
// connection per store, WAL on disk).
type projectStores struct {
	mu      sync.Mutex
	dataDir string
	stores  map[string]*store.Store
}

// newProjectStores constructs an empty cache rooted at dataDir. Stores
// are opened lazily on first ForProject / Get call.
func newProjectStores(dataDir string) *projectStores {
	return &projectStores{
		dataDir: dataDir,
		stores:  map[string]*store.Store{},
	}
}

// NewProjectStores is the public constructor used by cmd/serve to build
// a single cache that owns every per-project *store.Store handle for
// the server's lifetime. Closing the returned value releases every
// opened DB. Tests should use newProjectStores inside this package.
func NewProjectStores(dataDir string) *ProjectStores {
	return (*ProjectStores)(newProjectStores(dataDir))
}

// ProjectStores is the exported alias of *projectStores. Fields are
// intentionally unexported; callers interact with it through Storer
// (ForProject) and Close only.
type ProjectStores projectStores

// ForProject implements Storer on the exported alias.
func (p *ProjectStores) ForProject(slug string) (*store.Store, error) {
	return (*projectStores)(p).ForProject(slug)
}

// Close shuts down every cached store.
func (p *ProjectStores) Close() error {
	return (*projectStores)(p).Close()
}

// Slugs returns a snapshot of currently-cached project slugs.
func (p *ProjectStores) Slugs() []string {
	return (*projectStores)(p).Slugs()
}

// inner converts the exported alias back into the unexported concrete
// type used by NewRouter via WithProjectStores.
func (p *ProjectStores) inner() *projectStores { return (*projectStores)(p) }

// ForProject implements the Storer contract. Identical semantics to
// Get — ForProject is the public API; Get predates this refactor and
// is retained for in-package callers (metrics scrape path).
func (p *projectStores) ForProject(slug string) (*store.Store, error) {
	return p.Get(slug)
}

// Get returns the *store.Store for slug, opening (and caching) one on
// first access. The caller MUST NOT call Close on the returned handle —
// the cache owns its lifecycle.
func (p *projectStores) Get(slug string) (*store.Store, error) {
	if slug == "" {
		return nil, fmt.Errorf("project stores: empty slug")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if s, ok := p.stores[slug]; ok {
		return s, nil
	}
	s, err := store.OpenForProject(p.dataDir, slug)
	if err != nil {
		return nil, fmt.Errorf("open store for %q: %w", slug, err)
	}
	p.stores[slug] = s
	return s, nil
}

// Slugs returns a snapshot of currently-cached project slugs, sorted
// order not guaranteed. Used by shutdown/metric iteration paths.
func (p *projectStores) Slugs() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]string, 0, len(p.stores))
	for slug := range p.stores {
		out = append(out, slug)
	}
	return out
}

// Close walks the cache and closes every open handle. Errors are
// aggregated into a single string; callers may wrap or log as needed.
func (p *projectStores) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	var firstErr error
	for slug, s := range p.stores {
		if err := s.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("close store %q: %w", slug, err)
		}
		delete(p.stores, slug)
	}
	return firstErr
}
