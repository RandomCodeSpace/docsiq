package hookinstaller

import (
	"encoding/json"
	"path/filepath"
)

// CopilotInstaller — GitHub Copilot CLI/VSCode integration.
//
// NOTE — config path taken from kgraph's install.ts:
//
//	~/.config/github-copilot/hooks.json
//
// kgraph's shape was a flat {"hooks":{"session-start":"<cmd>"}} map.
// That's what we mirror. GitHub Copilot does not officially document
// a SessionStart hook surface area as of this writing — treat this as
// a best-effort placeholder until Copilot publishes stable hook APIs.
type CopilotInstaller struct{}

func (CopilotInstaller) Name() string { return "copilot" }

func (CopilotInstaller) ConfigPath() (string, error) {
	home, err := homeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "github-copilot", "hooks.json"), nil
}

func (c CopilotInstaller) Install(hookPath string) error {
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
	hooks["session-start"] = hookPath
	enc, err := json.Marshal(hooks)
	if err != nil {
		return err
	}
	settings["hooks"] = enc
	return writeJSONMapAtomic(path, settings)
}

func (c CopilotInstaller) Uninstall() error {
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
	if cur, ok := hooks["session-start"]; ok && looksLikeDocsiqHook(cur) {
		delete(hooks, "session-start")
	}
	if len(hooks) == 0 {
		delete(settings, "hooks")
	} else {
		enc, _ := json.Marshal(hooks)
		settings["hooks"] = enc
	}
	return writeJSONMapAtomic(path, settings)
}

func (c CopilotInstaller) Status() (bool, string) {
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
	if cur, ok := hooks["session-start"]; ok && looksLikeDocsiqHook(cur) {
		return true, path
	}
	return false, "not installed (" + path + ")"
}
