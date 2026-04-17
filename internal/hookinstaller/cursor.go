package hookinstaller

import (
	"encoding/json"
	"log/slog"
	"path/filepath"
)

// CursorInstaller writes to ~/.cursor/docsiq-hooks.json.
//
// UNVERIFIED — Cursor does not publicly document a SessionStart hook API
// as of 2026-04-17. Attempted doc sources returned empty / 404:
//   - https://docs.cursor.com/en/agent/hooks
//   - https://cursor.com/docs/agent/hooks
//   - https://docs.cursor.com/advanced/hooks
//
// Cursor's primary extensibility surface is MCP (not shell hooks) and
// kgraph's install.ts only wires Cursor into MCP registration. Schema
// below mirrors kgraph's original guess; we stash our entry in a
// sibling docsiq-specific file so we at least have a place to track
// `status` and `uninstall` without polluting ~/.cursor/settings.json.
//
// Shape (unverified guess):
//
//	{
//	  "hooks": { "SessionStart": { "command": "/path/to/hook.sh" } }
//	}
type CursorInstaller struct {
	testPath string
}

func newCursorInstallerWithPath(p string) CursorInstaller {
	return CursorInstaller{testPath: p}
}

func (CursorInstaller) Name() string { return "cursor" }

func (c CursorInstaller) ConfigPath() (string, error) {
	if c.testPath != "" {
		return c.testPath, nil
	}
	home, err := homeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cursor", "docsiq-hooks.json"), nil
}

type simpleHookEntry struct {
	Command string `json:"command"`
}

type simpleHookMap struct {
	SessionStart *simpleHookEntry `json:"SessionStart,omitempty"`
}

func (c CursorInstaller) Install(hookPath string) error {
	if err := validateHookPath(hookPath); err != nil {
		return err
	}
	slog.Warn("⚠️ installing unverified hook for cursor",
		"reason", "no documented SessionStart hook API as of 2026-04-17")
	path, err := c.ConfigPath()
	if err != nil {
		return err
	}
	settings, err := readJSONMap(path)
	if err != nil {
		return err
	}
	hooks := simpleHookMap{}
	if raw, ok := settings["hooks"]; ok && len(raw) > 0 {
		_ = json.Unmarshal(raw, &hooks)
	}
	hooks.SessionStart = &simpleHookEntry{Command: hookPath}
	encoded, err := json.Marshal(hooks)
	if err != nil {
		return err
	}
	settings["hooks"] = encoded
	return writeJSONMapAtomic(path, settings)
}

func (c CursorInstaller) Uninstall() error {
	path, err := c.ConfigPath()
	if err != nil {
		return err
	}
	settings, err := readJSONMap(path)
	if err != nil {
		return err
	}
	raw, ok := settings["hooks"]
	if !ok {
		return nil
	}
	hooks := simpleHookMap{}
	_ = json.Unmarshal(raw, &hooks)
	if hooks.SessionStart != nil && looksLikeDocsiqHook(hooks.SessionStart.Command) {
		hooks.SessionStart = nil
	}
	if hooks.SessionStart == nil {
		delete(settings, "hooks")
	} else {
		enc, _ := json.Marshal(hooks)
		settings["hooks"] = enc
	}
	return writeJSONMapAtomic(path, settings)
}

func (c CursorInstaller) Status() (bool, string) {
	path, err := c.ConfigPath()
	if err != nil {
		return false, "config path unavailable"
	}
	settings, err := readJSONMap(path)
	if err != nil {
		return false, err.Error()
	}
	raw, ok := settings["hooks"]
	if !ok {
		return false, "not installed (" + path + ")"
	}
	hooks := simpleHookMap{}
	if err := json.Unmarshal(raw, &hooks); err != nil {
		return false, "malformed hooks section"
	}
	if hooks.SessionStart != nil && looksLikeDocsiqHook(hooks.SessionStart.Command) {
		return true, path
	}
	return false, "not installed (" + path + ")"
}
