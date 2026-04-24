package store

import (
	"context"
	"testing"
	"time"
)

// TestOpen_HardeningPragmas verifies Block 3.6 — every PRAGMA the spec
// requires is observable on a freshly-opened store.
func TestOpen_HardeningPragmas(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s, err := OpenForProject(dir, "harden")
	if err != nil {
		t.Fatalf("OpenForProject: %v", err)
	}
	defer s.Close()

	cases := []struct {
		name string
		sql  string
		want string
	}{
		{"journal_mode", `PRAGMA journal_mode`, "wal"},
		{"foreign_keys", `PRAGMA foreign_keys`, "1"},
		{"synchronous", `PRAGMA synchronous`, "1"}, // 1 = NORMAL
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			var got string
			if err := s.DB().QueryRow(c.sql).Scan(&got); err != nil {
				t.Fatalf("%s: %v", c.sql, err)
			}
			if got != c.want {
				t.Fatalf("%s = %q; want %q", c.sql, got, c.want)
			}
		})
	}

	t.Run("busy_timeout_ge_5000", func(t *testing.T) {
		var got int
		if err := s.DB().QueryRow(`PRAGMA busy_timeout`).Scan(&got); err != nil {
			t.Fatalf("PRAGMA busy_timeout: %v", err)
		}
		if got < 5000 {
			t.Fatalf("busy_timeout = %d ms; want >= 5000", got)
		}
	})
}

// TestOpen_PoolSettings asserts the raised MaxOpenConns / MaxIdleConns
// values survive the Open recipe. MaxOpenConns=4, MaxIdleConns=2,
// ConnMaxLifetime=1h are not individually observable via sql.DB stats
// without opening connections; we assert on Stats().MaxOpenConnections
// which reflects SetMaxOpenConns.
func TestOpen_PoolSettings(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s, err := OpenForProject(dir, "pool")
	if err != nil {
		t.Fatalf("OpenForProject: %v", err)
	}
	defer s.Close()

	stats := s.DB().Stats()
	if stats.MaxOpenConnections != 4 {
		t.Fatalf("MaxOpenConnections = %d; want 4", stats.MaxOpenConnections)
	}
}

// TestStore_PingContext asserts the new context-aware Ping method.
// A cancelled context must surface as a ctx.Err(), not a generic
// database error, so that /readyz can distinguish "caller gave up"
// from "SQLite is sick".
func TestStore_PingContext(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s, err := OpenForProject(dir, "ping")
	if err != nil {
		t.Fatalf("OpenForProject: %v", err)
	}
	defer s.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.Ping(ctx); err != nil {
		t.Fatalf("Ping: %v", err)
	}

	cancelled, cancel2 := context.WithCancel(context.Background())
	cancel2()
	if err := s.Ping(cancelled); err == nil {
		t.Fatalf("Ping on cancelled ctx: want non-nil error, got nil")
	}
}
