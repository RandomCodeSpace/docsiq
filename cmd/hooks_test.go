package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RandomCodeSpace/docsiq/internal/config"
)

// withFakeHome points DOCSIQ_FAKE_HOME at a tempdir for the test's life.
func withFakeHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	prev, had := os.LookupEnv("DOCSIQ_FAKE_HOME")
	_ = os.Setenv("DOCSIQ_FAKE_HOME", dir)
	t.Cleanup(func() {
		if had {
			_ = os.Setenv("DOCSIQ_FAKE_HOME", prev)
		} else {
			_ = os.Unsetenv("DOCSIQ_FAKE_HOME")
		}
	})
	return dir
}

// withCfg installs a package-level cfg pointing at a tempdir data_dir
// so hookDestPath() resolves predictably.
func withCfg(t *testing.T) string {
	t.Helper()
	prev := cfg
	dataDir := t.TempDir()
	cfg = &config.Config{DataDir: dataDir, DefaultProject: config.DefaultProjectSlug}
	t.Cleanup(func() { cfg = prev })
	return dataDir
}

// resetFlags zeroes the package-level flag vars between tests so one
// test's --client doesn't leak into the next.
func resetFlags(t *testing.T) {
	t.Helper()
	prevC, prevD, prevU := hooksClient, hooksDryRun, hooksHookURL
	hooksClient = "all"
	hooksDryRun = false
	hooksHookURL = ""
	t.Cleanup(func() {
		hooksClient, hooksDryRun, hooksHookURL = prevC, prevD, prevU
	})
}

func TestHooksInstallDryRun(t *testing.T) {
	withFakeHome(t)
	withCfg(t)
	resetFlags(t)

	hooksClient = "claude"
	hooksDryRun = true

	var buf bytes.Buffer
	if err := runInstall(&buf); err != nil {
		t.Fatalf("runInstall: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "[dry-run]") {
		t.Errorf("dry-run marker missing: %s", out)
	}
	if !strings.Contains(out, "claude") {
		t.Errorf("claude client not in output: %s", out)
	}
	// Dry-run must not have created the hook file.
	if _, err := os.Stat(hookDestPath()); err == nil {
		t.Errorf("dry-run wrote hook file at %s", hookDestPath())
	}
}

func TestHooksInstallAndUninstallRoundtrip(t *testing.T) {
	withFakeHome(t)
	withCfg(t)
	resetFlags(t)
	hooksClient = "claude"

	var buf bytes.Buffer
	if err := runInstall(&buf); err != nil {
		t.Fatalf("install: %v", err)
	}
	if _, err := os.Stat(hookDestPath()); err != nil {
		t.Fatalf("hook file not extracted: %v", err)
	}

	// Status should report installed for claude.
	buf.Reset()
	if err := runStatus(&buf); err != nil {
		t.Fatalf("status: %v", err)
	}
	if !strings.Contains(buf.String(), "claude") {
		t.Errorf("status missing claude: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "installed") {
		t.Errorf("status did not include installed: %s", buf.String())
	}

	// Uninstall.
	buf.Reset()
	if err := runUninstall(&buf); err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	if !strings.Contains(buf.String(), "uninstalled") {
		t.Errorf("uninstall output missing success marker: %s", buf.String())
	}
}

func TestHooksInstallUnknownClient(t *testing.T) {
	withFakeHome(t)
	withCfg(t)
	resetFlags(t)
	hooksClient = "emacs"

	var buf bytes.Buffer
	if err := runInstall(&buf); err == nil {
		t.Fatal("expected error for unknown client")
	}
}

func TestHooksInstallAll(t *testing.T) {
	withFakeHome(t)
	withCfg(t)
	resetFlags(t)
	hooksClient = "all"

	var buf bytes.Buffer
	if err := runInstall(&buf); err != nil {
		t.Fatalf("install all: %v", err)
	}
	// Every installer should print a success line.
	for _, name := range []string{"claude", "cursor", "copilot", "codex"} {
		if !strings.Contains(buf.String(), name) {
			t.Errorf("install-all output missing %s: %s", name, buf.String())
		}
	}
}

func TestHooksStatusBeforeInstall(t *testing.T) {
	withFakeHome(t)
	withCfg(t)
	resetFlags(t)

	var buf bytes.Buffer
	if err := runStatus(&buf); err != nil {
		t.Fatalf("status: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "not installed") {
		t.Errorf("status before install should show 'not installed': %s", out)
	}
	if !strings.Contains(out, "not extracted") {
		t.Errorf("status before install should flag missing hook script: %s", out)
	}
}

func TestHooksUninstallDryRun(t *testing.T) {
	withFakeHome(t)
	withCfg(t)
	resetFlags(t)
	hooksClient = "codex"
	hooksDryRun = true

	var buf bytes.Buffer
	if err := runUninstall(&buf); err != nil {
		t.Fatalf("uninstall dry-run: %v", err)
	}
	if !strings.Contains(buf.String(), "[dry-run]") {
		t.Errorf("dry-run marker missing: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "codex") {
		t.Errorf("client codex not in output: %s", buf.String())
	}
}

func TestHookDestPathUnderDataDir(t *testing.T) {
	withCfg(t)
	want := filepath.Join(cfg.DataDir, "hooks", "hook.sh")
	if got := hookDestPath(); got != want {
		t.Errorf("hookDestPath = %q, want %q", got, want)
	}
}
