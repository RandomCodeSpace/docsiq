package hookinstaller

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestClaude_FixtureTransform verifies the ClaudeInstaller transforms
// a realistic pre-existing settings.json (with unrelated top-level keys
// AND an unrelated sibling hook event) into the expected post-Install
// state. The expected "after" shape is documented by Claude Code:
// https://docs.claude.com/en/docs/claude-code/hooks fetched 2026-04-17.
func TestClaude_FixtureTransform(t *testing.T) {
	tmp := t.TempDir()
	before, err := os.ReadFile(filepath.Join("fixtures", "claude", "before.json"))
	if err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(tmp, "settings.json")
	if err := os.WriteFile(cfg, before, 0o644); err != nil {
		t.Fatal(err)
	}

	inst := newClaudeInstallerWithPath(cfg)
	if err := inst.Install("/path/to/hook.sh"); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("fixtures", "claude", "after.json"))
	if err != nil {
		t.Fatal(err)
	}

	var gotMap, wantMap map[string]any
	if err := json.Unmarshal(got, &gotMap); err != nil {
		t.Fatalf("unmarshal got: %v", err)
	}
	if err := json.Unmarshal(want, &wantMap); err != nil {
		t.Fatalf("unmarshal want: %v", err)
	}
	if !reflect.DeepEqual(gotMap, wantMap) {
		gb, _ := json.MarshalIndent(gotMap, "", "  ")
		wb, _ := json.MarshalIndent(wantMap, "", "  ")
		t.Errorf("fixture mismatch:\n got=%s\nwant=%s", gb, wb)
	}
}

// TestClaude_FixtureIdempotent runs Install twice and confirms the
// second invocation does not duplicate the SessionStart entry.
func TestClaude_FixtureIdempotent(t *testing.T) {
	tmp := t.TempDir()
	before, _ := os.ReadFile(filepath.Join("fixtures", "claude", "before.json"))
	cfg := filepath.Join(tmp, "settings.json")
	if err := os.WriteFile(cfg, before, 0o644); err != nil {
		t.Fatal(err)
	}

	inst := newClaudeInstallerWithPath(cfg)
	if err := inst.Install("/path/to/hook.sh"); err != nil {
		t.Fatal(err)
	}
	if err := inst.Install("/path/to/hook.sh"); err != nil {
		t.Fatal(err)
	}

	raw, _ := os.ReadFile(cfg)
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatal(err)
	}
	hooks := parsed["hooks"].(map[string]any)
	ss := hooks["SessionStart"].([]any)
	if len(ss) != 1 {
		t.Errorf("SessionStart should have 1 entry after 2 Installs, got %d", len(ss))
	}
}
