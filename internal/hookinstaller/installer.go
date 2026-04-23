// Package hookinstaller registers docsiq's SessionStart hook with the
// various AI clients (Claude Code, Cursor, GitHub Copilot, Codex CLI).
//
// Port of kgraph's hooks/install.ts. Key differences:
//   - POSIX-only: no hook.mjs, no hook.ps1, no Windows branches.
//   - Stdlib JSON with json.RawMessage for deep-merge safety.
//   - Atomic writes (temp-file + rename).
//   - Per-client "recognize our entry" is driven by the hook command
//     path — so a user who moves ~/.docsiq/hooks/hook.sh can still
//     uninstall the stale entry by rerunning against the old path.
package hookinstaller

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Installer is the contract every per-client installer satisfies.
//
// Name        — short client identifier ("claude", "cursor", ...).
// ConfigPath  — absolute path the installer will read/write. Returning
//
//	an error here means "config dir cannot be determined"
//	(e.g. $HOME is unset); this is not a user-visible error
//	unless Install/Status is actually called.
//
// Install     — writes the hook entry to the config, merging with any
//
//	pre-existing content. Idempotent.
//
// Uninstall   — removes ONLY our entry; leaves unrelated keys intact.
// Status      — returns (installed, detail) where detail is a short
//
//	human-readable string (path, reason missing, etc).
type Installer interface {
	Name() string
	ConfigPath() (string, error)
	Install(hookPath string) error
	Uninstall() error
	Status() (installed bool, detail string)
}

// ErrConfigUnavailable is returned by ConfigPath when the client's config
// directory cannot be derived (no $HOME, etc).
var ErrConfigUnavailable = errors.New("config path unavailable")

// homeDir returns the user's home directory, honoring the DOCSIQ_FAKE_HOME
// override used in tests. Returning an empty string is an error — all
// callers wrap this with ErrConfigUnavailable.
func homeDir() (string, error) {
	if h := os.Getenv("DOCSIQ_FAKE_HOME"); h != "" {
		return h, nil
	}
	h, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(h) == "" {
		return "", fmt.Errorf("%w: cannot determine home directory: %v",
			ErrConfigUnavailable, err)
	}
	return h, nil
}

// readJSONMap reads a JSON file and returns it as a generic map. If the
// file does not exist, it returns an empty map and nil error (first-write
// case). Any OTHER error — including malformed JSON — is returned unchanged
// so the caller does NOT silently clobber a corrupt config.
func readJSONMap(path string) (map[string]json.RawMessage, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]json.RawMessage{}, nil
		}
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	raw, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	// Treat an empty file as "no config" — many editors create empty files.
	if len(strings.TrimSpace(string(raw))) == 0 {
		return map[string]json.RawMessage{}, nil
	}
	out := map[string]json.RawMessage{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return out, nil
}

// writeJSONMapAtomic writes data to path via temp-file + rename so a
// crash mid-write cannot corrupt the user's config. Creates parent
// directories as needed. Follows symlinks (per spec — we write through
// the symlink so user-chosen locations keep working).
func writeJSONMapAtomic(path string, data map[string]json.RawMessage) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	buf, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	buf = append(buf, '\n')

	// Resolve symlinks so the temp file lands on the same filesystem as
	// the real target; otherwise os.Rename across mountpoints fails.
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil || realPath == "" {
		realPath = path
	}

	tmp, err := os.CreateTemp(filepath.Dir(realPath), ".docsiq-hook-*.json")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmp.Name()
	// Cleanup on any error path below.
	cleanup := func() { _ = os.Remove(tmpPath) }

	if _, err := tmp.Write(buf); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close temp: %w", err)
	}
	// 0o600 — user-only read/write. No group/world bits.
	if err := os.Chmod(tmpPath, 0o600); err != nil {
		cleanup()
		return fmt.Errorf("chmod temp: %w", err)
	}
	if err := os.Rename(tmpPath, realPath); err != nil {
		cleanup()
		return fmt.Errorf("rename temp: %w", err)
	}
	return nil
}

// validateHookPath rejects blatant path-traversal / null-byte garbage
// before we write it into a user config. Callers pass whatever they got
// from the CLI (--hook-path style overrides) so this is the gatekeeper.
func validateHookPath(p string) error {
	p = strings.TrimSpace(p)
	if p == "" {
		return fmt.Errorf("hook path is empty")
	}
	if strings.ContainsRune(p, 0) {
		return fmt.Errorf("hook path contains NUL byte")
	}
	if strings.Contains(p, "..") {
		return fmt.Errorf("hook path must be absolute and clean, got %q", p)
	}
	if !filepath.IsAbs(p) {
		return fmt.Errorf("hook path must be absolute, got %q", p)
	}
	return nil
}

// ExtractHookScript writes the embedded hook.sh to dest with 0o700 perms.
// Creates the parent directory if missing. Overwrites existing files —
// callers that want to preserve a user-modified hook should check first.
func ExtractHookScript(dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(dest), err)
	}
	if err := os.WriteFile(dest, HookScript, 0o700); err != nil {
		return fmt.Errorf("write %s: %w", dest, err)
	}
	// WriteFile masks perms through umask on create; reset explicitly so
	// the resulting file is reliably executable for the owner only.
	if err := os.Chmod(dest, 0o700); err != nil {
		return fmt.Errorf("chmod %s: %w", dest, err)
	}
	return nil
}
