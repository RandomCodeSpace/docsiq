package hookinstaller

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestCodex_FixtureTransform verifies the CodexInstaller transform.
// OpenAI Codex CLI has no documented SessionStart hook API as of
// 2026-04-17 (its real config is TOML at ~/.codex/config.toml and only
// documents a post-turn "Notify" hook). This test pins the current
// (unverified, kgraph-derived) shape so we can detect any accidental
// schema change.
func TestCodex_FixtureTransform(t *testing.T) {
	tmp := t.TempDir()
	before, err := os.ReadFile(filepath.Join("fixtures", "codex", "before.json"))
	if err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(tmp, "hooks.json")
	if err := os.WriteFile(cfg, before, 0o644); err != nil {
		t.Fatal(err)
	}

	inst := newCodexInstallerWithPath(cfg)
	if err := inst.Install("/path/to/hook.sh"); err != nil {
		t.Fatal(err)
	}

	got, _ := os.ReadFile(cfg)
	want, _ := os.ReadFile(filepath.Join("fixtures", "codex", "after.json"))

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
