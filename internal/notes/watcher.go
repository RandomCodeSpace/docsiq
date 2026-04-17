package notes

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Watch scans notesDir every second and fires onChange(key) whenever a
// `.md` file's mtime changes, a new `.md` file appears, or an existing
// one is removed. The returned `stop` func halts the background goroutine.
//
// This is a polling watcher — chosen over fsnotify to keep the watcher
// dependency-free and cross-platform. kgraph uses fs.watch (Node), which
// is similarly coarse in practice; the 1-second cadence is adequate for
// the "re-index after a user edits a note in their IDE" use case.
func Watch(notesDir string, onChange func(key string)) (func(), error) {
	if onChange == nil {
		onChange = func(string) {}
	}
	// Ensure the directory exists so the first tick doesn't silently
	// treat a typo as "no notes yet" forever.
	if err := os.MkdirAll(notesDir, 0o755); err != nil {
		return nil, err
	}

	state := snapshotMTimes(notesDir)
	stopCh := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stopCh:
				return
			case <-ticker.C:
				next := snapshotMTimes(notesDir)
				for key, mt := range next {
					if old, ok := state[key]; !ok || !old.Equal(mt) {
						onChange(key)
					}
				}
				for key := range state {
					if _, ok := next[key]; !ok {
						onChange(key)
					}
				}
				state = next
			}
		}
	}()
	stop := func() {
		select {
		case <-stopCh:
			// already stopped
		default:
			close(stopCh)
		}
		wg.Wait()
	}
	return stop, nil
}

func snapshotMTimes(notesDir string) map[string]time.Time {
	out := map[string]time.Time{}
	_ = filepath.WalkDir(notesDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".md") {
			return nil
		}
		info, ierr := d.Info()
		if ierr != nil {
			return nil
		}
		rel, rerr := filepath.Rel(notesDir, path)
		if rerr != nil {
			return nil
		}
		key := strings.TrimSuffix(filepath.ToSlash(rel), ".md")
		out[key] = info.ModTime()
		return nil
	})
	return out
}
