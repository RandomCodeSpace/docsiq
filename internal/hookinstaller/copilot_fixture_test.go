package hookinstaller

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestCopilot_FixtureTransform verifies the CopilotInstaller transform.
// GitHub Copilot CLI has no documented SessionStart hook API as of
// 2026-04-17; this test pins the current (unverified, kgraph-derived)
// shape so we can detect any accidental schema change.
func TestCopilot_FixtureTransform(t *testing.T) {
	tmp := t.TempDir()
	before, err := os.ReadFile(filepath.Join("fixtures", "copilot", "before.json"))
	if err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(tmp, "hooks.json")
	if err := os.WriteFile(cfg, before, 0o644); err != nil {
		t.Fatal(err)
	}

	inst := newCopilotInstallerWithPath(cfg)
	if err := inst.Install("/path/to/hook.sh"); err != nil {
		t.Fatal(err)
	}

	got, _ := os.ReadFile(cfg)
	want, _ := os.ReadFile(filepath.Join("fixtures", "copilot", "after.json"))

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
