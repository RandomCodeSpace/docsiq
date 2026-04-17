package hookinstaller

import (
	"encoding/json"
	"log/slog"
	"path/filepath"
)

// CopilotInstaller — GitHub Copilot CLI integration.
//
// UNVERIFIED — GitHub Copilot CLI does not publicly document a
// SessionStart hook API as of 2026-04-17. Doc sources checked:
//   - https://docs.github.com/en/copilot/how-tos/use-copilot-agents/use-copilot-in-the-cli (404)
//   - https://docs.github.com/en/copilot/github-copilot-in-the-cli/about-github-copilot-in-the-cli
//     (general responsible-use docs, no hook schema)
//   - https://docs.github.com/en/copilot/using-github-copilot/using-github-copilot-in-the-command-line
//     (notes the legacy `gh copilot` extension was retired and replaced
//     by the new Copilot CLI; no hook schema is published)
//
// Schema below mirrors kgraph's original guess:
//
//	config path: ~/.config/github-copilot/hooks.json
//	shape:       {"hooks": {"session-start": "<cmd>"}}
type CopilotInstaller struct {
	testPath string
}

func newCopilotInstallerWithPath(p string) CopilotInstaller {
	return CopilotInstaller{testPath: p}
}

func (CopilotInstaller) Name() string { return "copilot" }

func (c CopilotInstaller) ConfigPath() (string, error) {
	if c.testPath != "" {
		return c.testPath, nil
	}
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
	slog.Warn("⚠️ installing unverified hook for copilot",
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
