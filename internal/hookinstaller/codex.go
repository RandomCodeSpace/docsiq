package hookinstaller

import (
	"encoding/json"
	"path/filepath"
)

// CodexInstaller targets ~/.codex/hooks.json (per kgraph reference).
//
// Shape — kgraph wrote a flat map:
//
//	{"hooks":{"SessionStart":"<cmd>"}}
//
// We keep the same shape for wire compatibility. Codex CLI's hook API
// is less stable than Claude Code's, so this is also flagged as a
// best-effort integration.
type CodexInstaller struct{}

func (CodexInstaller) Name() string { return "codex" }

func (CodexInstaller) ConfigPath() (string, error) {
	home, err := homeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".codex", "hooks.json"), nil
}

func (c CodexInstaller) Install(hookPath string) error {
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
	hooks := map[string]string{}
	if raw, ok := settings["hooks"]; ok && len(raw) > 0 {
		_ = json.Unmarshal(raw, &hooks)
	}
	hooks["SessionStart"] = hookPath
	enc, err := json.Marshal(hooks)
	if err != nil {
		return err
	}
	settings["hooks"] = enc
	return writeJSONMapAtomic(path, settings)
}

func (c CodexInstaller) Uninstall() error {
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
	hooks := map[string]string{}
	_ = json.Unmarshal(raw, &hooks)
	if cur, ok := hooks["SessionStart"]; ok && looksLikeDocsiqHook(cur) {
		delete(hooks, "SessionStart")
	}
	if len(hooks) == 0 {
		delete(settings, "hooks")
	} else {
		enc, _ := json.Marshal(hooks)
		settings["hooks"] = enc
	}
	return writeJSONMapAtomic(path, settings)
}

func (c CodexInstaller) Status() (bool, string) {
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
	hooks := map[string]string{}
	if err := json.Unmarshal(raw, &hooks); err != nil {
		return false, "malformed hooks section"
	}
	if cur, ok := hooks["SessionStart"]; ok && looksLikeDocsiqHook(cur) {
		return true, path
	}
	return false, "not installed (" + path + ")"
}
