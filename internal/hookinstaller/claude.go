package hookinstaller

import (
	"encoding/json"
	"fmt"
	"path/filepath"
)

// ClaudeInstaller targets ~/.claude/settings.json.
//
// Config shape (Claude Code, current as of kgraph reference):
//
//	{
//	  "hooks": {
//	    "SessionStart": [
//	      { "hooks": [ { "type": "command", "command": "/path/to/hook.sh" } ] }
//	    ]
//	  }
//	}
//
// We preserve all other top-level keys (mcpServers, permissions, etc)
// and only mutate hooks.SessionStart. Within SessionStart, we also
// preserve any entries that aren't ours.
type ClaudeInstaller struct{}

func (ClaudeInstaller) Name() string { return "claude" }

func (ClaudeInstaller) ConfigPath() (string, error) {
	home, err := homeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", "settings.json"), nil
}

// claudeHookEntry is the group-style entry Claude Code expects.
type claudeHookEntry struct {
	Hooks []claudeHookCmd `json:"hooks"`
}

type claudeHookCmd struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

// isOurClaudeEntry reports whether e (a raw array element) is a docsiq
// group-format entry pointing at hookPath. We match by command path
// exactly — not by substring — to avoid stepping on a user's custom
// hook that happens to contain "hook.sh" in its name.
func isOurClaudeEntry(e json.RawMessage, hookPath string) bool {
	var group claudeHookEntry
	if err := json.Unmarshal(e, &group); err == nil {
		for _, h := range group.Hooks {
			if h.Command == hookPath {
				return true
			}
		}
	}
	// Also handle the legacy flat shape just in case a user migrated
	// from an older docsiq / kgraph version.
	var flat claudeHookCmd
	if err := json.Unmarshal(e, &flat); err == nil {
		if flat.Type == "command" && flat.Command == hookPath {
			return true
		}
	}
	return false
}

func (c ClaudeInstaller) Install(hookPath string) error {
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

	// Decode hooks into a typed shape, then re-encode. Using
	// map[string][]json.RawMessage preserves unknown sibling events.
	hooks := map[string][]json.RawMessage{}
	if raw, ok := settings["hooks"]; ok && len(raw) > 0 {
		if err := json.Unmarshal(raw, &hooks); err != nil {
			return fmt.Errorf("parse hooks from %s: %w", path, err)
		}
	}

	entries := hooks["SessionStart"]
	// Dedup: drop any pre-existing docsiq entry pointing at this path.
	cleaned := entries[:0]
	for _, e := range entries {
		if !isOurClaudeEntry(e, hookPath) {
			cleaned = append(cleaned, e)
		}
	}

	ours := claudeHookEntry{Hooks: []claudeHookCmd{{Type: "command", Command: hookPath}}}
	oursRaw, err := json.Marshal(ours)
	if err != nil {
		return err
	}
	cleaned = append(cleaned, oursRaw)
	hooks["SessionStart"] = cleaned

	encoded, err := json.Marshal(hooks)
	if err != nil {
		return err
	}
	settings["hooks"] = encoded
	return writeJSONMapAtomic(path, settings)
}

func (c ClaudeInstaller) Uninstall() error {
	path, err := c.ConfigPath()
	if err != nil {
		return err
	}
	settings, err := readJSONMap(path)
	if err != nil {
		return err
	}
	raw, ok := settings["hooks"]
	if !ok || len(raw) == 0 {
		return nil
	}
	hooks := map[string][]json.RawMessage{}
	if err := json.Unmarshal(raw, &hooks); err != nil {
		return fmt.Errorf("parse hooks from %s: %w", path, err)
	}

	// Remove any entry pointing at OUR hook (any path containing our
	// marker string). Because uninstall may be called without knowing
	// the exact install path, we match on the hook filename marker.
	entries := hooks["SessionStart"]
	cleaned := entries[:0]
	for _, e := range entries {
		if !isOurClaudeEntryByMarker(e) {
			cleaned = append(cleaned, e)
		}
	}
	if len(cleaned) == 0 {
		delete(hooks, "SessionStart")
	} else {
		hooks["SessionStart"] = cleaned
	}

	if len(hooks) == 0 {
		delete(settings, "hooks")
	} else {
		encoded, err := json.Marshal(hooks)
		if err != nil {
			return err
		}
		settings["hooks"] = encoded
	}
	return writeJSONMapAtomic(path, settings)
}

// isOurClaudeEntryByMarker matches any entry whose command path ends in
// "/docsiq/hooks/hook.sh" OR contains the dotfile "/.docsiq/". This is
// the Uninstall fallback when the exact install path isn't known.
func isOurClaudeEntryByMarker(e json.RawMessage) bool {
	var group claudeHookEntry
	if err := json.Unmarshal(e, &group); err == nil {
		for _, h := range group.Hooks {
			if looksLikeDocsiqHook(h.Command) {
				return true
			}
		}
	}
	var flat claudeHookCmd
	if err := json.Unmarshal(e, &flat); err == nil {
		if flat.Type == "command" && looksLikeDocsiqHook(flat.Command) {
			return true
		}
	}
	return false
}

func (c ClaudeInstaller) Status() (bool, string) {
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
	hooks := map[string][]json.RawMessage{}
	if err := json.Unmarshal(raw, &hooks); err != nil {
		return false, "malformed hooks section"
	}
	for _, e := range hooks["SessionStart"] {
		if isOurClaudeEntryByMarker(e) {
			return true, path
		}
	}
	return false, "not installed (" + path + ")"
}
