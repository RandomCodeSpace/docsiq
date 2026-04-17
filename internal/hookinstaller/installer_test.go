package hookinstaller

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fakeHome points DOCSIQ_FAKE_HOME at a tempdir and returns the path +
// a cleanup that restores the previous value. Every installer test uses
// this so we never touch the developer's real config.
func fakeHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	prev, had := os.LookupEnv("DOCSIQ_FAKE_HOME")
	if err := os.Setenv("DOCSIQ_FAKE_HOME", dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if had {
			_ = os.Setenv("DOCSIQ_FAKE_HOME", prev)
		} else {
			_ = os.Unsetenv("DOCSIQ_FAKE_HOME")
		}
	})
	return dir
}

// hookPath returns a valid absolute hook path living inside the fake
// home so looksLikeDocsiqHook matches.
func hookPath(home string) string {
	return filepath.Join(home, ".docsiq", "hooks", "hook.sh")
}

func TestValidateHookPath(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		wantErr bool
	}{
		{"absolute_ok", "/home/user/.docsiq/hooks/hook.sh", false},
		{"empty", "", true},
		{"relative", "hook.sh", true},
		{"path_traversal", "/home/user/../etc/passwd", true},
		{"null_byte", "/home/user/hook\x00.sh", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateHookPath(tc.in)
			if tc.wantErr && err == nil {
				t.Fatalf("want error for %q", tc.in)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestExtractHookScript(t *testing.T) {
	home := fakeHome(t)
	dest := filepath.Join(home, ".docsiq", "hooks", "hook.sh")
	if err := ExtractHookScript(dest); err != nil {
		t.Fatalf("ExtractHookScript: %v", err)
	}
	info, err := os.Stat(dest)
	if err != nil {
		t.Fatalf("stat dest: %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
		t.Errorf("hook.sh not executable: mode=%v", info.Mode())
	}
	content, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "docsiq") {
		t.Errorf("hook.sh missing docsiq marker; first 40 bytes=%q", string(content[:40]))
	}
}

// ----------------------------------------------------------------------
// Claude installer
// ----------------------------------------------------------------------

func TestClaudeInstaller(t *testing.T) {
	t.Run("install_fresh_writes_correct_json", func(t *testing.T) {
		home := fakeHome(t)
		hp := hookPath(home)
		inst := ClaudeInstaller{}
		if err := inst.Install(hp); err != nil {
			t.Fatalf("Install: %v", err)
		}
		path, _ := inst.ConfigPath()
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var parsed map[string]any
		if err := json.Unmarshal(raw, &parsed); err != nil {
			t.Fatalf("parse: %v", err)
		}
		hooks, ok := parsed["hooks"].(map[string]any)
		if !ok {
			t.Fatalf("missing hooks: %v", parsed)
		}
		ss, ok := hooks["SessionStart"].([]any)
		if !ok || len(ss) != 1 {
			t.Fatalf("SessionStart not a 1-elem array: %v", hooks["SessionStart"])
		}
	})

	t.Run("install_preserves_unrelated_keys", func(t *testing.T) {
		home := fakeHome(t)
		hp := hookPath(home)
		inst := ClaudeInstaller{}
		path, _ := inst.ConfigPath()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		seed := `{"theme":"dark","mcpServers":{"foo":{"command":"x"}},"hooks":{"PreTool":[{"type":"command","command":"/bin/true"}]}}`
		if err := os.WriteFile(path, []byte(seed), 0o644); err != nil {
			t.Fatal(err)
		}

		if err := inst.Install(hp); err != nil {
			t.Fatalf("Install: %v", err)
		}
		raw, _ := os.ReadFile(path)
		var parsed map[string]any
		if err := json.Unmarshal(raw, &parsed); err != nil {
			t.Fatal(err)
		}
		if parsed["theme"] != "dark" {
			t.Errorf("lost theme key: %v", parsed)
		}
		mcp, ok := parsed["mcpServers"].(map[string]any)
		if !ok || mcp["foo"] == nil {
			t.Errorf("lost mcpServers: %v", parsed["mcpServers"])
		}
		hooks := parsed["hooks"].(map[string]any)
		if hooks["PreTool"] == nil {
			t.Errorf("lost PreTool: %v", hooks)
		}
		if hooks["SessionStart"] == nil {
			t.Errorf("SessionStart not installed: %v", hooks)
		}
	})

	t.Run("install_twice_is_idempotent", func(t *testing.T) {
		home := fakeHome(t)
		hp := hookPath(home)
		inst := ClaudeInstaller{}
		if err := inst.Install(hp); err != nil {
			t.Fatal(err)
		}
		if err := inst.Install(hp); err != nil {
			t.Fatal(err)
		}
		path, _ := inst.ConfigPath()
		raw, _ := os.ReadFile(path)
		var parsed map[string]any
		_ = json.Unmarshal(raw, &parsed)
		ss := parsed["hooks"].(map[string]any)["SessionStart"].([]any)
		if len(ss) != 1 {
			t.Fatalf("expected 1 entry after 2 installs, got %d: %v", len(ss), ss)
		}
	})

	t.Run("uninstall_removes_our_entry_only", func(t *testing.T) {
		home := fakeHome(t)
		hp := hookPath(home)
		inst := ClaudeInstaller{}
		path, _ := inst.ConfigPath()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		// Seed with a user-owned SessionStart entry
		seed := `{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"/usr/local/bin/user-script.sh"}]}]}}`
		if err := os.WriteFile(path, []byte(seed), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := inst.Install(hp); err != nil {
			t.Fatal(err)
		}
		if err := inst.Uninstall(); err != nil {
			t.Fatal(err)
		}
		raw, _ := os.ReadFile(path)
		s := string(raw)
		if !strings.Contains(s, "user-script.sh") {
			t.Errorf("uninstall lost the user's entry: %s", s)
		}
		if strings.Contains(s, hp) {
			t.Errorf("uninstall did not remove docsiq entry: %s", s)
		}
	})

	t.Run("uninstall_when_not_installed_is_noop", func(t *testing.T) {
		fakeHome(t)
		inst := ClaudeInstaller{}
		if err := inst.Uninstall(); err != nil {
			t.Fatalf("Uninstall on missing file should not error: %v", err)
		}
	})

	t.Run("status_before_and_after_install", func(t *testing.T) {
		home := fakeHome(t)
		hp := hookPath(home)
		inst := ClaudeInstaller{}
		installed, _ := inst.Status()
		if installed {
			t.Fatal("Status reports installed before install")
		}
		if err := inst.Install(hp); err != nil {
			t.Fatal(err)
		}
		installed, _ = inst.Status()
		if !installed {
			t.Fatal("Status reports not installed after install")
		}
	})

	t.Run("malformed_json_does_not_clobber", func(t *testing.T) {
		home := fakeHome(t)
		hp := hookPath(home)
		inst := ClaudeInstaller{}
		path, _ := inst.ConfigPath()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := inst.Install(hp); err == nil {
			t.Fatal("Install should refuse malformed JSON")
		}
		// File should still contain the original garbage.
		raw, _ := os.ReadFile(path)
		if !strings.Contains(string(raw), "not json") {
			t.Errorf("clobbered malformed file: %q", raw)
		}
	})

	t.Run("nonexistent_parent_dir_is_created", func(t *testing.T) {
		fakeHome(t)
		inst := ClaudeInstaller{}
		path, _ := inst.ConfigPath()
		if _, err := os.Stat(filepath.Dir(path)); err == nil {
			t.Fatalf("parent dir should not exist yet: %s", filepath.Dir(path))
		}
		if err := inst.Install("/tmp/.docsiq/hooks/hook.sh"); err != nil {
			t.Fatalf("Install: %v", err)
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("file not written: %v", err)
		}
	})

	t.Run("symlinked_config_is_written_through", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("symlink support requires admin on Windows")
		}
		home := fakeHome(t)
		hp := hookPath(home)
		inst := ClaudeInstaller{}
		path, _ := inst.ConfigPath()

		// Create the real file at an alternate location, then symlink.
		realDir := t.TempDir()
		realFile := filepath.Join(realDir, "settings.json")
		if err := os.WriteFile(realFile, []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(realFile, path); err != nil {
			t.Fatal(err)
		}
		if err := inst.Install(hp); err != nil {
			t.Fatalf("Install through symlink: %v", err)
		}
		// The symlink should still exist and point at a file with our entry.
		raw, _ := os.ReadFile(path)
		if !strings.Contains(string(raw), hp) {
			t.Errorf("symlink write did not land: %s", raw)
		}
	})

	t.Run("rejects_path_traversal_hook_path", func(t *testing.T) {
		fakeHome(t)
		inst := ClaudeInstaller{}
		if err := inst.Install("/foo/../../etc/passwd"); err == nil {
			t.Fatal("expected rejection for path-traversal hook path")
		}
	})
}

// ----------------------------------------------------------------------
// Cursor installer
// ----------------------------------------------------------------------

func TestCursorInstaller(t *testing.T) {
	t.Run("install_and_status", func(t *testing.T) {
		home := fakeHome(t)
		hp := hookPath(home)
		inst := CursorInstaller{}
		if err := inst.Install(hp); err != nil {
			t.Fatal(err)
		}
		installed, _ := inst.Status()
		if !installed {
			t.Fatal("Status reports not installed after install")
		}
	})

	t.Run("uninstall", func(t *testing.T) {
		home := fakeHome(t)
		hp := hookPath(home)
		inst := CursorInstaller{}
		if err := inst.Install(hp); err != nil {
			t.Fatal(err)
		}
		if err := inst.Uninstall(); err != nil {
			t.Fatal(err)
		}
		installed, _ := inst.Status()
		if installed {
			t.Fatal("Status still reports installed after Uninstall")
		}
	})

	t.Run("uninstall_noop_on_missing_file", func(t *testing.T) {
		fakeHome(t)
		inst := CursorInstaller{}
		if err := inst.Uninstall(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("preserves_unrelated_keys", func(t *testing.T) {
		home := fakeHome(t)
		hp := hookPath(home)
		inst := CursorInstaller{}
		path, _ := inst.ConfigPath()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(`{"other":"keep"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := inst.Install(hp); err != nil {
			t.Fatal(err)
		}
		raw, _ := os.ReadFile(path)
		if !strings.Contains(string(raw), `"other"`) {
			t.Errorf("lost unrelated key: %s", raw)
		}
	})

	t.Run("config_path_under_fake_home", func(t *testing.T) {
		home := fakeHome(t)
		p, err := CursorInstaller{}.ConfigPath()
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(p, home) {
			t.Errorf("config path %q not under fake home %q", p, home)
		}
	})
}

// ----------------------------------------------------------------------
// Copilot installer
// ----------------------------------------------------------------------

func TestCopilotInstaller(t *testing.T) {
	t.Run("install_and_uninstall", func(t *testing.T) {
		home := fakeHome(t)
		hp := hookPath(home)
		inst := CopilotInstaller{}
		if err := inst.Install(hp); err != nil {
			t.Fatal(err)
		}
		installed, _ := inst.Status()
		if !installed {
			t.Fatal("not installed after Install")
		}
		if err := inst.Uninstall(); err != nil {
			t.Fatal(err)
		}
		installed, _ = inst.Status()
		if installed {
			t.Fatal("still installed after Uninstall")
		}
	})

	t.Run("config_path_under_fake_home", func(t *testing.T) {
		home := fakeHome(t)
		p, err := CopilotInstaller{}.ConfigPath()
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(p, home) {
			t.Errorf("config path %q not under fake home %q", p, home)
		}
	})
}

// ----------------------------------------------------------------------
// Codex installer
// ----------------------------------------------------------------------

func TestCodexInstaller(t *testing.T) {
	t.Run("install_and_uninstall", func(t *testing.T) {
		home := fakeHome(t)
		hp := hookPath(home)
		inst := CodexInstaller{}
		if err := inst.Install(hp); err != nil {
			t.Fatal(err)
		}
		installed, _ := inst.Status()
		if !installed {
			t.Fatal("not installed after Install")
		}
		if err := inst.Uninstall(); err != nil {
			t.Fatal(err)
		}
		installed, _ = inst.Status()
		if installed {
			t.Fatal("still installed after Uninstall")
		}
	})

	t.Run("config_path_under_fake_home", func(t *testing.T) {
		home := fakeHome(t)
		p, err := CodexInstaller{}.ConfigPath()
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(p, home) {
			t.Errorf("config path %q not under fake home %q", p, home)
		}
	})
}

// ----------------------------------------------------------------------
// Registry
// ----------------------------------------------------------------------

func TestRegistry(t *testing.T) {
	t.Run("all_returns_four", func(t *testing.T) {
		if got := len(All()); got != 4 {
			t.Fatalf("All() len = %d, want 4", got)
		}
	})

	t.Run("names_sorted", func(t *testing.T) {
		names := Names()
		if len(names) != 4 {
			t.Fatalf("Names() len = %d, want 4", len(names))
		}
		want := []string{"claude", "codex", "copilot", "cursor"}
		for i, n := range want {
			if names[i] != n {
				t.Errorf("Names()[%d] = %q, want %q", i, names[i], n)
			}
		}
	})

	t.Run("by_name_known", func(t *testing.T) {
		inst, err := ByName("claude")
		if err != nil {
			t.Fatal(err)
		}
		if inst.Name() != "claude" {
			t.Errorf("got %q", inst.Name())
		}
	})

	t.Run("by_name_unknown_errors", func(t *testing.T) {
		if _, err := ByName("emacs"); err == nil {
			t.Fatal("expected error for unknown client")
		}
	})
}

func TestHomeDirOverride(t *testing.T) {
	t.Run("fake_home_honored", func(t *testing.T) {
		dir := t.TempDir()
		prev := os.Getenv("DOCSIQ_FAKE_HOME")
		defer os.Setenv("DOCSIQ_FAKE_HOME", prev)
		_ = os.Setenv("DOCSIQ_FAKE_HOME", dir)
		h, err := homeDir()
		if err != nil {
			t.Fatal(err)
		}
		if h != dir {
			t.Errorf("homeDir = %q, want %q", h, dir)
		}
	})
}
