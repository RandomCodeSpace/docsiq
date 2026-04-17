//go:build cgo

package sqlitevec

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestAssetNameDetection(t *testing.T) {
	// Sanity: on a supported platform the name must end with the right
	// file extension, and must be non-empty. On an unsupported platform
	// (not linux/darwin), assetName() must return "".
	got := assetName()
	switch runtime.GOOS {
	case "linux":
		if !strings.HasSuffix(got, ".so") {
			t.Errorf("linux: assetName()=%q, want .so suffix", got)
		}
	case "darwin":
		if !strings.HasSuffix(got, ".dylib") {
			t.Errorf("darwin: assetName()=%q, want .dylib suffix", got)
		}
	default:
		if got != "" {
			t.Errorf("%s: assetName()=%q, want empty (unsupported)", runtime.GOOS, got)
		}
	}
}

func TestExtForGOOS(t *testing.T) {
	switch runtime.GOOS {
	case "linux":
		if extForGOOS() != "so" {
			t.Errorf("extForGOOS()=%q, want so", extForGOOS())
		}
	case "darwin":
		if extForGOOS() != "dylib" {
			t.Errorf("extForGOOS()=%q, want dylib", extForGOOS())
		}
	default:
		if extForGOOS() != "" {
			t.Errorf("extForGOOS()=%q, want empty", extForGOOS())
		}
	}
}

func TestExtract_emptyPlaceholder(t *testing.T) {
	// The embedded assets in this tree are 0-byte placeholders. Extract
	// must return ErrEmptyExtension cleanly on any supported platform,
	// and ErrUnsupportedPlatform on anything else.
	dir := t.TempDir()
	_, err := Extract(dir)
	if err == nil {
		t.Fatal("Extract on placeholder build: err=nil, want ErrEmptyExtension or ErrUnsupportedPlatform")
	}
	switch runtime.GOOS {
	case "linux", "darwin":
		if !errors.Is(err, ErrEmptyExtension) {
			t.Errorf("Extract: err=%v, want ErrEmptyExtension", err)
		}
	default:
		if !errors.Is(err, ErrUnsupportedPlatform) {
			t.Errorf("Extract on %s: err=%v, want ErrUnsupportedPlatform", runtime.GOOS, err)
		}
	}
}

func TestExtract_emptyDataDir(t *testing.T) {
	if _, err := Extract(""); err == nil {
		t.Fatal("Extract(\"\"): err=nil, want error")
	}
}

func TestLoadInto_missingFile(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open in-memory: %v", err)
	}
	defer db.Close()

	dir := t.TempDir()
	missing := filepath.Join(dir, "does-not-exist.so")
	err = LoadInto(db, missing)
	if !errors.Is(err, ErrExtensionMissing) {
		t.Errorf("LoadInto(missing): err=%v, want ErrExtensionMissing", err)
	}
}

func TestLoadInto_emptyFile(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open in-memory: %v", err)
	}
	defer db.Close()

	dir := t.TempDir()
	empty := filepath.Join(dir, "empty.so")
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatalf("write empty: %v", err)
	}

	// Must not panic.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("LoadInto panicked on empty file: %v", r)
		}
	}()

	err = LoadInto(db, empty)
	if !errors.Is(err, ErrExtensionEmpty) {
		t.Errorf("LoadInto(empty): err=%v, want ErrExtensionEmpty", err)
	}
}

func TestLoadInto_nilDB(t *testing.T) {
	if err := LoadInto(nil, "/some/path.so"); err == nil {
		t.Error("LoadInto(nil db): err=nil, want error")
	}
}

func TestLoadInto_emptyPath(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open in-memory: %v", err)
	}
	defer db.Close()
	if err := LoadInto(db, ""); err == nil {
		t.Error("LoadInto(empty path): err=nil, want error")
	}
}
