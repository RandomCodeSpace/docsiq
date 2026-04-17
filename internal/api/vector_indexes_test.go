package api

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/RandomCodeSpace/docsiq/internal/store"
	"github.com/RandomCodeSpace/docsiq/internal/vectorindex"
)

// TestVectorIndexes_ForProject_SingleFlight is a regression test for
// P0-4. 10 goroutines racing on the first ForProject call for the
// same slug must cause exactly ONE BuildFromStore invocation; the
// others must receive the same cached result.
func TestVectorIndexes_ForProject_SingleFlight(t *testing.T) {
	v := NewVectorIndexes()

	// Replace the build hook with one that counts invocations and
	// sleeps briefly to amplify the race window.
	var buildCount atomic.Int32
	fakeIdx := vectorindex.NewDefaultHNSW()
	v.build = func(ctx context.Context, st *store.Store) (vectorindex.Index, error) {
		buildCount.Add(1)
		// Give other goroutines enough time to observe the cache miss.
		time.Sleep(50 * time.Millisecond)
		return fakeIdx, nil
	}

	const goroutines = 10
	var wg sync.WaitGroup
	results := make([]vectorindex.Index, goroutines)
	// Barrier so all goroutines call ForProject ~simultaneously.
	start := make(chan struct{})
	for i := range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results[i] = v.ForProject("same-slug", nil)
		}()
	}
	close(start)
	wg.Wait()

	if got := buildCount.Load(); got != 1 {
		t.Fatalf("BuildFromStore called %d times; want 1", got)
	}
	for i, r := range results {
		if r != fakeIdx {
			t.Errorf("goroutine %d got %v; want the deduplicated fakeIdx", i, r)
		}
	}

	// Second call (after cache is populated) should not trigger any
	// new build.
	_ = v.ForProject("same-slug", nil)
	if got := buildCount.Load(); got != 1 {
		t.Errorf("after cache hit, BuildFromStore called %d times; want still 1", got)
	}

	// After Invalidate, a new build should run exactly once under
	// concurrent callers.
	v.Invalidate("same-slug")
	var wg2 sync.WaitGroup
	start2 := make(chan struct{})
	for range goroutines {
		wg2.Add(1)
		go func() {
			defer wg2.Done()
			<-start2
			_ = v.ForProject("same-slug", nil)
		}()
	}
	close(start2)
	wg2.Wait()
	if got := buildCount.Load(); got != 2 {
		t.Errorf("after Invalidate + concurrent re-touch, build called %d times total; want 2", got)
	}
}
