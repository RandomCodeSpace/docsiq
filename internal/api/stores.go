package api

import (
	"fmt"
	"sync"

	"github.com/RandomCodeSpace/docsiq/internal/store"
)

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

func newProjectStores(dataDir string) *projectStores {
	return &projectStores{
		dataDir: dataDir,
		stores:  map[string]*store.Store{},
	}
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
