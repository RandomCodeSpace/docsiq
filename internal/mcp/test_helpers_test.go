package mcp

import (
	"sync"

	"github.com/RandomCodeSpace/docsiq/internal/store"
)

// fakeStorer is the test-only Storer used by the MCP tool test suite.
// It returns the same *store.Store for every requested slug so tests
// that hit a docs tool without specifying a "project" arg (i.e. they
// hit the _default fallback) still get a usable handle.
type fakeStorer struct {
	st *store.Store
}

func newFakeStorer(st *store.Store) *fakeStorer { return &fakeStorer{st: st} }

// ForProject satisfies Storer. Returns the bound store for every slug.
func (f *fakeStorer) ForProject(slug string) (*store.Store, error) {
	return f.st, nil
}

// lazyStorer opens a per-project store on demand under dataDir, caches
// the handle, and returns it on subsequent calls. Tests that exercise
// the notes tools (write_note → read_note → search_notes) need a real
// SQLite file per slug since the handlers drive FTS5 indexing.
type lazyStorer struct {
	dataDir string
	mu      sync.Mutex
	stores  map[string]*store.Store
}

func newLazyStorer(dataDir string) *lazyStorer {
	return &lazyStorer{dataDir: dataDir, stores: map[string]*store.Store{}}
}

// ForProject opens (and caches) a store for slug under dataDir.
func (l *lazyStorer) ForProject(slug string) (*store.Store, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if st, ok := l.stores[slug]; ok {
		return st, nil
	}
	st, err := store.OpenForProject(l.dataDir, slug)
	if err != nil {
		return nil, err
	}
	l.stores[slug] = st
	return st, nil
}

// Close releases every cached handle. Tests should register this via
// t.Cleanup on the returned lazyStorer.
func (l *lazyStorer) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	for k, st := range l.stores {
		_ = st.Close()
		delete(l.stores, k)
	}
}
