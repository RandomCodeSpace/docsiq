//go:build integration

package api_test

import (
	"fmt"
	"net/http"
	"sync"
	"testing"

	"github.com/RandomCodeSpace/docsiq/internal/api/itest"
)

// TestConcurrency_100ParallelNotePUTsSameProject fires 100 parallel
// PUTs against distinct keys in _default. Every PUT must return 200
// and no goroutine may race (checked implicitly by `-race`).
func TestConcurrency_100ParallelNotePUTsSameProject(t *testing.T) {
	e := itest.New(t)

	const n = 100
	var wg sync.WaitGroup
	wg.Add(n)
	errCh := make(chan error, n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("concur/key-%03d", i)
			resp := e.PUTNote(t, "_default", key, fmt.Sprintf("content-%d", i), nil)
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				errCh <- fmt.Errorf("PUT %s: status %d", key, resp.StatusCode)
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Error(err)
	}
}

// TestConcurrency_50ReadsDuringWrites pounds one key with 50 writes
// interleaved with 50 reads, ensuring the handler chain survives
// read/write contention without panic, race, or 5xx.
func TestConcurrency_50ReadsDuringWrites(t *testing.T) {
	e := itest.New(t)

	// Seed the key so initial reads do not 404.
	if resp, body := e.PUTNoteBody(t, "_default", "hot", "seed", nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("seed: %d body=%s", resp.StatusCode, string(body))
	}

	const n = 50
	var wg sync.WaitGroup
	wg.Add(2 * n)

	errCh := make(chan error, 4*n)

	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			resp := e.PUTNote(t, "_default", "hot", fmt.Sprintf("content-%d", i), nil)
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				errCh <- fmt.Errorf("write %d: status %d", i, resp.StatusCode)
			}
		}(i)
		go func(i int) {
			defer wg.Done()
			resp, _ := e.GET(t, "/api/projects/_default/notes/hot")
			if resp.StatusCode >= 500 {
				errCh <- fmt.Errorf("read %d: status %d", i, resp.StatusCode)
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Error(err)
	}
}
