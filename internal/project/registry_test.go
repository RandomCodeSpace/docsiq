package project

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

func newTestRegistry(t *testing.T) (*Registry, string) {
	t.Helper()
	dir := t.TempDir()
	r, err := OpenRegistry(dir)
	if err != nil {
		t.Fatalf("OpenRegistry: %v", err)
	}
	t.Cleanup(func() { _ = r.Close() })
	return r, dir
}

func TestOpenRegistry_EmptyDir(t *testing.T) {
	if _, err := OpenRegistry(""); err == nil {
		t.Fatal("OpenRegistry(\"\") = nil, want error")
	}
	if _, err := OpenRegistry("   "); err == nil {
		t.Fatal("OpenRegistry(whitespace) = nil, want error")
	}
}

func TestOpenRegistry_CreatesDir(t *testing.T) {
	parent := t.TempDir()
	nested := filepath.Join(parent, "a", "b", "c")
	r, err := OpenRegistry(nested)
	if err != nil {
		t.Fatalf("OpenRegistry nested: %v", err)
	}
	defer r.Close()
	if _, err := os.Stat(filepath.Join(nested, "registry.db")); err != nil {
		t.Fatalf("registry.db not created: %v", err)
	}
}

func TestOpenRegistry_ReadOnlyDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		// TODO(#65): environmental skip (windows chmod semantics); tracked in flake-register.
		t.Skip("chmod semantics differ on windows")
	}
	if os.Getuid() == 0 {
		// TODO(#65): environmental skip (root bypasses chmod 0555); tracked in flake-register.
		t.Skip("running as root; chmod 0555 does not block writes")
	}
	parent := t.TempDir()
	ro := filepath.Join(parent, "ro")
	if err := os.MkdirAll(ro, 0o555); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(ro, 0o755) })
	inside := filepath.Join(ro, "cannot-create")
	if _, err := OpenRegistry(inside); err == nil {
		t.Fatal("OpenRegistry read-only parent = nil, want error")
	}
}

func TestRegistry_RegisterGetListDelete(t *testing.T) {
	r, _ := newTestRegistry(t)

	p := Project{Slug: "my-project", Name: "My Project", Remote: "git@github.com:owner/my-project.git"}
	if err := r.Register(p); err != nil {
		t.Fatalf("Register: %v", err)
	}

	got, err := r.Get("my-project")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Slug != p.Slug || got.Name != p.Name || got.Remote != p.Remote {
		t.Fatalf("Get = %+v, want %+v", got, p)
	}
	if got.CreatedAt == 0 {
		t.Error("CreatedAt not set")
	}

	byRemote, err := r.GetByRemote(p.Remote)
	if err != nil {
		t.Fatalf("GetByRemote: %v", err)
	}
	if byRemote.Slug != "my-project" {
		t.Fatalf("GetByRemote.Slug = %q, want my-project", byRemote.Slug)
	}

	list, err := r.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("List len = %d, want 1", len(list))
	}

	if err := r.Delete("my-project"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := r.Get("my-project"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get after Delete: %v, want ErrNotFound", err)
	}
	if err := r.Delete("my-project"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete nonexistent: %v, want ErrNotFound", err)
	}
}

func TestRegistry_EmptyList(t *testing.T) {
	r, _ := newTestRegistry(t)
	list, err := r.List()
	if err != nil {
		t.Fatal(err)
	}
	if list == nil {
		t.Fatal("List() = nil, want empty slice")
	}
	if len(list) != 0 {
		t.Fatalf("List len = %d, want 0", len(list))
	}
}

func TestRegistry_DuplicateRemote(t *testing.T) {
	r, _ := newTestRegistry(t)
	p1 := Project{Slug: "a", Name: "a", Remote: "git@host:x/y.git"}
	p2 := Project{Slug: "b", Name: "b", Remote: "git@host:x/y.git"}
	if err := r.Register(p1); err != nil {
		t.Fatalf("Register p1: %v", err)
	}
	err := r.Register(p2)
	if !errors.Is(err, ErrDuplicateRemote) {
		t.Fatalf("Register duplicate remote: %v, want ErrDuplicateRemote", err)
	}
}

func TestRegistry_DuplicateSlug(t *testing.T) {
	r, _ := newTestRegistry(t)
	p1 := Project{Slug: "same", Name: "a", Remote: "r1"}
	p2 := Project{Slug: "same", Name: "b", Remote: "r2"}
	if err := r.Register(p1); err != nil {
		t.Fatal(err)
	}
	if err := r.Register(p2); err == nil {
		t.Fatal("Register duplicate slug = nil, want error")
	}
}

func TestRegistry_InvalidInputs(t *testing.T) {
	r, _ := newTestRegistry(t)
	cases := []struct {
		name string
		p    Project
	}{
		{"invalid_slug_chars", Project{Slug: "NOT/Good", Name: "n", Remote: "r"}},
		{"empty_slug", Project{Slug: "", Name: "n", Remote: "r"}},
		{"empty_name", Project{Slug: "ok", Name: "", Remote: "r"}},
		{"empty_remote", Project{Slug: "ok", Name: "n", Remote: ""}},
		{"uppercase_slug", Project{Slug: "UPPER", Name: "n", Remote: "r"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := r.Register(tc.p); err == nil {
				t.Fatalf("Register(%+v) = nil, want error", tc.p)
			}
		})
	}
}

func TestRegistry_ConcurrentWrites(t *testing.T) {
	r, _ := newTestRegistry(t)
	const N = 20
	var wg sync.WaitGroup
	errs := make(chan error, N)
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			slug := "p-" + string(rune('a'+i%26)) + string(rune('a'+(i/26)%26)) + "-" + string(rune('0'+i%10))
			p := Project{
				Slug:   slug,
				Name:   slug,
				Remote: "remote-" + slug,
			}
			if err := r.Register(p); err != nil {
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("concurrent register: %v", err)
	}

	list, err := r.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != N {
		t.Fatalf("List len = %d, want %d", len(list), N)
	}
}

func TestRegistry_CloseIdempotent(t *testing.T) {
	r, _ := newTestRegistry(t)
	if err := r.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	// A second Close on an already-closed *sql.DB returns an error on some
	// drivers; we tolerate either nil or non-nil, but must not panic.
	_ = r.Close()
}

func TestRegistry_ReopenPersists(t *testing.T) {
	dir := t.TempDir()
	r1, err := OpenRegistry(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := r1.Register(Project{Slug: "persist", Name: "p", Remote: "r"}); err != nil {
		t.Fatal(err)
	}
	if err := r1.Close(); err != nil {
		t.Fatal(err)
	}

	r2, err := OpenRegistry(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer r2.Close()

	got, err := r2.Get("persist")
	if err != nil {
		t.Fatalf("Get after reopen: %v", err)
	}
	if got.Remote != "r" {
		t.Fatalf("Remote = %q, want r", got.Remote)
	}
}

func TestDetectRemote_EmptyCwd(t *testing.T) {
	if _, err := DetectRemote(""); err == nil {
		t.Fatal("DetectRemote(\"\") = nil, want error")
	}
}

func TestDetectRemote_NonGitDir(t *testing.T) {
	dir := t.TempDir()
	if _, err := DetectRemote(dir); err == nil {
		t.Fatal("DetectRemote(non-git) = nil, want error")
	}
}
