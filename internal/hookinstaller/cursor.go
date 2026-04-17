package hookinstaller

import (
	"encoding/json"
	"path/filepath"
)

// CursorInstaller writes to ~/.cursor/docsiq-hooks.json.
//
// NOTE — schema guess flagged for verification: kgraph's install.ts does
// not wire Cursor into hook registration (only into MCP registration).
// Cursor has no canonical "SessionStart hook" primitive the way Claude
// Code does, so we stash our entry in a sibling docsiq-specific file
// and document that a user-level shell wrapper / workspace automation
// is expected to call hook.sh themselves. The JSON we write tracks
// WHICH hook is registered, so `status` and `uninstall` still work.
//
// Shape:
//
//	{
//	  "hooks": { "SessionStart": { "command": "/path/to/hook.sh" } }
//	}
type CursorInstaller struct{}

func (CursorInstaller) Name() string { return "cursor" }

func (CursorInstaller) ConfigPath() (string, error) {
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
