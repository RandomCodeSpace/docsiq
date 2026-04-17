package api

import (
	"github.com/RandomCodeSpace/docsiq/internal/store"
)

// testSingleStore builds a *ProjectStores that returns the same pre-
// opened *store.Store for every slug. This is the minimal adapter
// tests need to call NewRouter after the Wave-2 signature change:
//
//	h := NewRouter(nil, nil, cfg, registry,
//	    WithProjectStores(testSingleStore(cfg.DataDir, st, "testproj")))
//
// The cache is "real" (a *projectStores instance) but overridden to
// bypass disk re-opens — we seed it with the caller's handle.
func testSingleStore(dataDir string, st *store.Store, slugs ...string) *ProjectStores {
	p := newProjectStores(dataDir)
	for _, slug := range slugs {
		p.stores[slug] = st
	}
	return (*ProjectStores)(p)
}
