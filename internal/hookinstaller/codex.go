package hookinstaller

import (
	"encoding/json"
	"log/slog"
	"path/filepath"
)

// CodexInstaller targets ~/.codex/hooks.json.
//
// UNVERIFIED — OpenAI Codex CLI does not publicly document a SessionStart
// hook API as of 2026-04-17. Doc sources checked:
//   - https://github.com/openai/codex (repo root)
//   - https://github.com/openai/codex/tree/main/docs
//   - https://github.com/openai/codex/blob/main/docs/config.md
//     (documents TOML config at ~/.codex/config.toml; only a "Notify"
//     post-turn notification hook is documented — no SessionStart)
//
// Schema below mirrors kgraph's original guess (wrong config format
// AND wrong event name relative to real Codex, but preserved for wire
// compatibility with existing kgraph users until Codex publishes a
// stable hook API):
//
//	config path: ~/.codex/hooks.json
//	shape:       {"hooks": {"SessionStart": "<cmd>"}}
type CodexInstaller struct {
	testPath string
}

func newCodexInstallerWithPath(p string) CodexInstaller {
	return CodexInstaller{testPath: p}
}

func (CodexInstaller) Name() string { return "codex" }

func (c CodexInstaller) ConfigPath() (string, error) {
	if c.testPath != "" {
		return c.testPath, nil
	}
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
	slog.Warn("⚠️ installing unverified hook for codex",
		"reason", "no documented SessionStart hook API as of 2026-04-17")
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
