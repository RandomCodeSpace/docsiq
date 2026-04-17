package store

import (
	"testing"
)

// TestOpen_BusyTimeoutPragma is a regression test for P1-2.
//
// The fix explicitly sets `_busy_timeout=5000` in the DSN so SQLite
// waits up to 5 s for a lock before returning SQLITE_BUSY. The
// mattn/go-sqlite3 driver does default to 5000 ms when the DSN param
// is omitted — but relying on an unspecified driver default is fragile
// and a future driver upgrade could change it. This test locks the
// value in via the observable `PRAGMA busy_timeout` readback.
func TestOpen_BusyTimeoutPragma(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenForProject(dir, "busytest")
	if err != nil {
		t.Fatalf("OpenForProject: %v", err)
	}
	defer s.Close()

	var got int
	if err := s.DB().QueryRow(`PRAGMA busy_timeout`).Scan(&got); err != nil {
		t.Fatalf("PRAGMA busy_timeout: %v", err)
	}
	if got < 5000 {
		t.Fatalf("busy_timeout = %d ms; want >= 5000 (DSN must set _busy_timeout explicitly)", got)
	}
}
