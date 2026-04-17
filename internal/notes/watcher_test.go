package notes

import (
	"sync"
	"testing"
	"time"
)

func TestWatcher_FiresOnWrite(t *testing.T) {
	dir := t.TempDir()
	var mu sync.Mutex
	seen := map[string]int{}
	stop, err := Watch(dir, func(key string) {
		mu.Lock()
		defer mu.Unlock()
		seen[key]++
	})
	if err != nil {
		t.Fatal(err)
	}
	defer stop()

	// Wait for the initial snapshot to settle.
	time.Sleep(150 * time.Millisecond)

	if err := Write(dir, &Note{Key: "k", Content: "v1"}); err != nil {
		t.Fatal(err)
	}

	// Watcher polls once per second; allow up to 3s for it to notice.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := seen["k"]
		mu.Unlock()
		if n > 0 {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("watcher did not fire within 3s")
}

func TestWatcher_StopIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	stop, err := Watch(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	stop()
	stop() // must not panic / must not deadlock
}
