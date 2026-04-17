// Package sqlitevec ships the asg017/sqlite-vec C extension embedded in
// the binary and extracts it to $DATA_DIR/ext/ at runtime so the mattn
// driver can LOAD EXTENSION it.
//
// Windows is explicitly unsupported — there is no windows embed here.
package sqlitevec

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

//go:embed assets/vec0-linux-amd64.so
//go:embed assets/vec0-linux-arm64.so
//go:embed assets/vec0-darwin-amd64.dylib
//go:embed assets/vec0-darwin-arm64.dylib
var assetsFS embed.FS

// ErrUnsupportedPlatform is returned when GOOS/GOARCH doesn't have an
// embedded sqlite-vec binary. Callers should treat this as "fall back to
// brute-force / pure-Go HNSW".
var ErrUnsupportedPlatform = errors.New("sqlitevec: unsupported GOOS/GOARCH (no embedded vec0 binary)")

// ErrEmptyExtension is returned when the embedded asset is a 0-byte
// placeholder (i.e. the real release binary was not dropped in at build
// time). Callers should treat this as "fall back".
var ErrEmptyExtension = errors.New("sqlitevec: embedded vec0 asset is empty (placeholder — real binary not bundled)")

// assetName returns the embedded asset path for the current platform, or
// "" if the platform is unsupported.
func assetName() string {
	switch runtime.GOOS {
	case "linux":
		switch runtime.GOARCH {
		case "amd64":
			return "assets/vec0-linux-amd64.so"
		case "arm64":
			return "assets/vec0-linux-arm64.so"
		}
	case "darwin":
		switch runtime.GOARCH {
		case "amd64":
			return "assets/vec0-darwin-amd64.dylib"
		case "arm64":
			return "assets/vec0-darwin-arm64.dylib"
		}
	}
	return ""
}

// extForGOOS returns the shared-object file extension (without dot) for
// the current GOOS. "" for unsupported.
func extForGOOS() string {
	switch runtime.GOOS {
	case "linux":
		return "so"
	case "darwin":
		return "dylib"
	}
	return ""
}

// Extract writes the embedded sqlite-vec shared object for the current
// GOOS/GOARCH into $dataDir/ext/vec0.<ext> and returns the absolute path.
//
// Idempotent: if the destination already exists and its size matches the
// embedded asset, the existing file is kept. Mode is 0o755.
//
// Returns ErrUnsupportedPlatform if GOOS/GOARCH has no embedded asset,
// and ErrEmptyExtension if the embedded asset is a 0-byte placeholder.
// In both cases callers should fall back to pure-Go search.
func Extract(dataDir string) (string, error) {
	if dataDir == "" {
		return "", errors.New("sqlitevec: data dir is empty")
	}
	name := assetName()
	if name == "" {
		return "", fmt.Errorf("%w (%s/%s)", ErrUnsupportedPlatform, runtime.GOOS, runtime.GOARCH)
	}
	data, err := assetsFS.ReadFile(name)
	if err != nil {
		return "", fmt.Errorf("sqlitevec: read embedded %s: %w", name, err)
	}
	if len(data) == 0 {
		return "", ErrEmptyExtension
	}

	extDir := filepath.Join(dataDir, "ext")
	if err := os.MkdirAll(extDir, 0o755); err != nil {
		return "", fmt.Errorf("sqlitevec: mkdir %s: %w", extDir, err)
	}
	dst := filepath.Join(extDir, "vec0."+extForGOOS())

	// Idempotency check: if file exists and size matches, skip write. We
	// intentionally do not hash — size match is sufficient for a single
	// frozen embedded asset per binary build.
	if info, err := os.Stat(dst); err == nil && info.Size() == int64(len(data)) {
		return dst, nil
	}

	// Write atomically: tmp file then rename.
	tmp, err := os.CreateTemp(extDir, "vec0-*.tmp")
	if err != nil {
		return "", fmt.Errorf("sqlitevec: create temp: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }
	if _, err := io.Copy(tmp, bytes.NewReader(data)); err != nil {
		_ = tmp.Close()
		cleanup()
		return "", fmt.Errorf("sqlitevec: write temp: %w", err)
	}
	if err := tmp.Chmod(0o755); err != nil {
		_ = tmp.Close()
		cleanup()
		return "", fmt.Errorf("sqlitevec: chmod temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return "", fmt.Errorf("sqlitevec: close temp: %w", err)
	}
	if err := os.Rename(tmpName, dst); err != nil {
		cleanup()
		return "", fmt.Errorf("sqlitevec: rename %s → %s: %w", tmpName, dst, err)
	}
	return dst, nil
}

