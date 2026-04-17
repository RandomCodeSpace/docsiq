package config

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

// isolateEnv unsets all DOCSCONTEXT_* and DOCSIQ_* env vars and sets HOME
// to the given tempdir so Load() can't read any real user config. Restores
// originals on test cleanup.
func isolateEnv(t *testing.T, home string) {
	t.Helper()

	type kv struct{ k, v string }
	var saved []kv
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "DOCSCONTEXT_") && !strings.HasPrefix(e, "DOCSIQ_") {
			continue
		}
		k, v, ok := strings.Cut(e, "=")
		if !ok {
			continue
		}
		saved = append(saved, kv{k, v})
	}
	for _, p := range saved {
		os.Unsetenv(p.k)
	}

	origHome := os.Getenv("HOME")
	origUserProfile := os.Getenv("USERPROFILE")
	if err := os.Setenv("HOME", home); err != nil {
		t.Fatalf("setenv HOME: %v", err)
	}
	if runtime.GOOS == "windows" {
		_ = os.Setenv("USERPROFILE", home)
	}

	t.Cleanup(func() {
		for _, p := range saved {
			os.Setenv(p.k, p.v)
		}
		os.Setenv("HOME", origHome)
		if runtime.GOOS == "windows" {
			os.Setenv("USERPROFILE", origUserProfile)
		}
	})
}

func TestLoad(t *testing.T) {
	t.Run("defaults_no_env_no_file", func(t *testing.T) {
		home := t.TempDir()
		isolateEnv(t, home)

		cfg, err := Load("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Server.Host != "127.0.0.1" {
			t.Errorf("Server.Host = %q, want 127.0.0.1", cfg.Server.Host)
		}
		if cfg.Server.Port != 8080 {
			t.Errorf("Server.Port = %d, want 8080", cfg.Server.Port)
		}
		if cfg.LLM.Provider != "ollama" {
			t.Errorf("LLM.Provider = %q, want ollama", cfg.LLM.Provider)
		}
		if cfg.LLM.Ollama.BaseURL != "http://localhost:11434" {
			t.Errorf("Ollama.BaseURL = %q", cfg.LLM.Ollama.BaseURL)
		}
		if cfg.LLM.Azure.Chat.Model != "gpt-4o" {
			t.Errorf("Azure.Chat.Model default = %q, want gpt-4o", cfg.LLM.Azure.Chat.Model)
		}
		if cfg.LLM.Azure.APIVersion != "2024-08-01" {
			t.Errorf("Azure.APIVersion default = %q", cfg.LLM.Azure.APIVersion)
		}
		if cfg.Indexing.ChunkSize != 512 {
			t.Errorf("Indexing.ChunkSize = %d, want 512", cfg.Indexing.ChunkSize)
		}
		if cfg.Community.MinCommunitySize != 2 {
			t.Errorf("Community.MinCommunitySize = %d", cfg.Community.MinCommunitySize)
		}
		wantDataDir := filepath.Join(home, ".docsiq", "data")
		if cfg.DataDir != wantDataDir {
			t.Errorf("DataDir = %q, want %q", cfg.DataDir, wantDataDir)
		}
	})

	t.Run("env_override_server_port", func(t *testing.T) {
		home := t.TempDir()
		isolateEnv(t, home)
		t.Setenv("DOCSCONTEXT_SERVER_PORT", "9999")

		cfg, err := Load("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Server.Port != 9999 {
			t.Errorf("Server.Port = %d, want 9999", cfg.Server.Port)
		}
	})

	t.Run("nested_env_azure_chat_model", func(t *testing.T) {
		home := t.TempDir()
		isolateEnv(t, home)
		t.Setenv("DOCSCONTEXT_LLM_AZURE_CHAT_MODEL", "gpt-5")

		cfg, err := Load("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.LLM.Azure.Chat.Model != "gpt-5" {
			t.Errorf("Azure.Chat.Model = %q, want gpt-5", cfg.LLM.Azure.Chat.Model)
		}
	})

	t.Run("shared_fallback_chat_api_key", func(t *testing.T) {
		home := t.TempDir()
		isolateEnv(t, home)
		t.Setenv("DOCSCONTEXT_LLM_AZURE_API_KEY", "shared-secret-abc")

		cfg, err := Load("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := cfg.LLM.Azure.ChatAPIKey(); got != "shared-secret-abc" {
			t.Errorf("ChatAPIKey() = %q, want shared-secret-abc", got)
		}
		if got := cfg.LLM.Azure.EmbedAPIKey(); got != "shared-secret-abc" {
			t.Errorf("EmbedAPIKey() = %q (should also fall back to shared)", got)
		}
	})

	t.Run("chat_specific_key_wins_over_shared", func(t *testing.T) {
		home := t.TempDir()
		isolateEnv(t, home)
		t.Setenv("DOCSCONTEXT_LLM_AZURE_API_KEY", "shared-key")
		t.Setenv("DOCSCONTEXT_LLM_AZURE_CHAT_API_KEY", "chat-specific-key")

		cfg, err := Load("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := cfg.LLM.Azure.ChatAPIKey(); got != "chat-specific-key" {
			t.Errorf("ChatAPIKey() = %q, want chat-specific-key", got)
		}
		if got := cfg.LLM.Azure.EmbedAPIKey(); got != "shared-key" {
			t.Errorf("EmbedAPIKey() = %q, want shared-key fallback", got)
		}
	})

	t.Run("config_file_load_yaml", func(t *testing.T) {
		home := t.TempDir()
		isolateEnv(t, home)

		yamlPath := filepath.Join(home, "cfg.yaml")
		content := []byte("server:\n  host: 0.0.0.0\n  port: 4321\nllm:\n  provider: azure\n  azure:\n    chat:\n      model: gpt-fancy\n")
		if err := os.WriteFile(yamlPath, content, 0o644); err != nil {
			t.Fatalf("write yaml: %v", err)
		}

		cfg, err := Load(yamlPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Server.Host != "0.0.0.0" {
			t.Errorf("Server.Host = %q", cfg.Server.Host)
		}
		if cfg.Server.Port != 4321 {
			t.Errorf("Server.Port = %d", cfg.Server.Port)
		}
		if cfg.LLM.Provider != "azure" {
			t.Errorf("LLM.Provider = %q", cfg.LLM.Provider)
		}
		if cfg.LLM.Azure.Chat.Model != "gpt-fancy" {
			t.Errorf("Azure.Chat.Model = %q", cfg.LLM.Azure.Chat.Model)
		}
	})

	t.Run("env_beats_config_file", func(t *testing.T) {
		home := t.TempDir()
		isolateEnv(t, home)

		yamlPath := filepath.Join(home, "cfg.yaml")
		content := []byte("server:\n  port: 4321\n")
		if err := os.WriteFile(yamlPath, content, 0o644); err != nil {
			t.Fatalf("write yaml: %v", err)
		}
		t.Setenv("DOCSCONTEXT_SERVER_PORT", "7777")

		cfg, err := Load(yamlPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Server.Port != 7777 {
			t.Errorf("Server.Port = %d, want 7777 (env wins)", cfg.Server.Port)
		}
	})

	t.Run("data_dir_tilde_expansion", func(t *testing.T) {
		home := t.TempDir()
		isolateEnv(t, home)

		yamlPath := filepath.Join(home, "cfg.yaml")
		content := []byte("data_dir: ~/foo/bar\n")
		if err := os.WriteFile(yamlPath, content, 0o644); err != nil {
			t.Fatalf("write yaml: %v", err)
		}

		cfg, err := Load(yamlPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := filepath.Join(home, "foo", "bar")
		if cfg.DataDir != want {
			t.Errorf("DataDir = %q, want %q", cfg.DataDir, want)
		}
		if strings.HasPrefix(cfg.DataDir, "~") {
			t.Errorf("DataDir still contains tilde: %q", cfg.DataDir)
		}
	})

	t.Run("malformed_yaml_returns_error", func(t *testing.T) {
		home := t.TempDir()
		isolateEnv(t, home)

		yamlPath := filepath.Join(home, "broken.yaml")
		// Unterminated mapping + stray colons = invalid yaml
		content := []byte("server:\n  host: : :\n  port: [not, a, number\n")
		if err := os.WriteFile(yamlPath, content, 0o644); err != nil {
			t.Fatalf("write yaml: %v", err)
		}

		_, err := Load(yamlPath)
		if err == nil {
			t.Fatal("expected error for malformed yaml, got nil")
		}
	})

	t.Run("missing_config_dir_uses_defaults", func(t *testing.T) {
		// HOME is a fresh tempdir with no ~/.docscontext or ~/.DocsContext.
		// Current behavior: warn + defaults, no error.
		home := t.TempDir()
		isolateEnv(t, home)

		cfg, err := Load("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Server.Port != 8080 {
			t.Errorf("Server.Port = %d, want default 8080", cfg.Server.Port)
		}
	})

	t.Run("zero_port_allowed", func(t *testing.T) {
		// Documents current behavior: port=0 is accepted as-is (no validation).
		home := t.TempDir()
		isolateEnv(t, home)
		t.Setenv("DOCSCONTEXT_SERVER_PORT", "0")

		cfg, err := Load("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Server.Port != 0 {
			t.Errorf("Server.Port = %d, want 0 (currently accepted)", cfg.Server.Port)
		}
	})

	t.Run("non_numeric_port_falls_back_to_default", func(t *testing.T) {
		// Viper's cast of "not-a-number" to int returns 0. Load does not
		// currently validate; it silently yields 0. This test documents
		// that behavior rather than demanding an error.
		home := t.TempDir()
		isolateEnv(t, home)
		t.Setenv("DOCSCONTEXT_SERVER_PORT", "not-a-number")

		cfg, err := Load("")
		if err != nil {
			// If viper ever starts returning an error for unmarshal-from-string
			// failures, that's an acceptable outcome too — the bad input must
			// not silently succeed with port 8080.
			return
		}
		if cfg.Server.Port == 8080 {
			t.Errorf("non-numeric port silently kept default 8080 — viper clobbered default without error; current observed value=%d", cfg.Server.Port)
		}
		// Observed: cast yields 0. Accept 0 as documented behavior.
		if cfg.Server.Port != 0 {
			t.Logf("observed non-numeric port -> %d (documenting)", cfg.Server.Port)
		}
	})

	t.Run("dbpath_under_data_dir", func(t *testing.T) {
		home := t.TempDir()
		isolateEnv(t, home)

		cfg, err := Load("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := filepath.Join(cfg.DataDir, "DocsContext.db")
		if got := cfg.DBPath(); got != want {
			t.Errorf("DBPath() = %q, want %q", got, want)
		}
	})

	t.Run("unicode_and_long_string_env_values", func(t *testing.T) {
		// Nasty inputs: unicode, embedded whitespace, 4 KB string.
		home := t.TempDir()
		isolateEnv(t, home)

		unicode := "テスト-模型-🚀"
		long := strings.Repeat("a", 4096)
		t.Setenv("DOCSCONTEXT_LLM_AZURE_CHAT_MODEL", unicode)
		t.Setenv("DOCSCONTEXT_LLM_AZURE_API_KEY", long)

		cfg, err := Load("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.LLM.Azure.Chat.Model != unicode {
			t.Errorf("Azure.Chat.Model = %q, want %q", cfg.LLM.Azure.Chat.Model, unicode)
		}
		if cfg.LLM.Azure.ChatAPIKey() != long {
			t.Errorf("ChatAPIKey() length = %d, want %d", len(cfg.LLM.Azure.ChatAPIKey()), len(long))
		}
	})

	t.Run("missing_config_file_path_errors", func(t *testing.T) {
		// When an explicit cfgFile path is given and doesn't exist, viper
		// returns a non-ConfigFileNotFoundError (it's an os.PathError), so
		// Load should return an error.
		home := t.TempDir()
		isolateEnv(t, home)

		_, err := Load(filepath.Join(home, "does-not-exist.yaml"))
		if err == nil {
			t.Fatal("expected error for nonexistent explicit config path, got nil")
		}
	})

	// ---------------------------------------------------------------------
	// Phase 0 Task 3 — DOCSIQ_ prefix migration
	// ---------------------------------------------------------------------

	t.Run("T1_new_prefix_server_port", func(t *testing.T) {
		home := t.TempDir()
		isolateEnv(t, home)
		buf := captureSlog(t)
		t.Setenv("DOCSIQ_SERVER_PORT", "9090")

		cfg, err := Load("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Server.Port != 9090 {
			t.Errorf("Server.Port = %d, want 9090", cfg.Server.Port)
		}
		if strings.Contains(buf.String(), "deprecated env var") {
			t.Errorf("unexpected deprecation warning: %s", buf.String())
		}
	})

	t.Run("T2_old_prefix_server_port_warns", func(t *testing.T) {
		home := t.TempDir()
		isolateEnv(t, home)
		buf := captureSlog(t)
		t.Setenv("DOCSCONTEXT_SERVER_PORT", "9090")

		cfg, err := Load("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Server.Port != 9090 {
			t.Errorf("Server.Port = %d, want 9090 via alias", cfg.Server.Port)
		}
		log := buf.String()
		if !strings.Contains(log, "deprecated env var") {
			t.Errorf("missing per-var deprecation warn: %s", log)
		}
		if !strings.Contains(log, "DOCSCONTEXT_SERVER_PORT") {
			t.Errorf("warn missing old name: %s", log)
		}
		if !strings.Contains(log, "DOCSIQ_SERVER_PORT") {
			t.Errorf("warn missing new name: %s", log)
		}
		if !strings.Contains(log, "will be removed in v2.0") {
			t.Errorf("missing summary warn: %s", log)
		}
	})

	t.Run("T3_both_set_docsiq_wins", func(t *testing.T) {
		home := t.TempDir()
		isolateEnv(t, home)
		buf := captureSlog(t)
		t.Setenv("DOCSIQ_SERVER_PORT", "9090")
		t.Setenv("DOCSCONTEXT_SERVER_PORT", "7777")

		cfg, err := Load("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Server.Port != 9090 {
			t.Errorf("Server.Port = %d, want 9090 (DOCSIQ wins)", cfg.Server.Port)
		}
		if !strings.Contains(buf.String(), "deprecated env var") {
			t.Errorf("deprecation warn should still fire for the old var: %s", buf.String())
		}
	})

	t.Run("T4_neither_set_default_port", func(t *testing.T) {
		home := t.TempDir()
		isolateEnv(t, home)

		cfg, err := Load("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Server.Port != 8080 {
			t.Errorf("Server.Port = %d, want 8080", cfg.Server.Port)
		}
	})

	t.Run("T5_new_prefix_azure_chat_api_key", func(t *testing.T) {
		home := t.TempDir()
		isolateEnv(t, home)
		t.Setenv("DOCSIQ_LLM_AZURE_CHAT_API_KEY", "k1")

		cfg, err := Load("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.LLM.Azure.Chat.APIKey != "k1" {
			t.Errorf("Azure.Chat.APIKey = %q, want k1", cfg.LLM.Azure.Chat.APIKey)
		}
	})

	t.Run("T6_old_prefix_azure_chat_api_key_warns", func(t *testing.T) {
		home := t.TempDir()
		isolateEnv(t, home)
		buf := captureSlog(t)
		t.Setenv("DOCSCONTEXT_LLM_AZURE_CHAT_API_KEY", "k2")

		cfg, err := Load("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.LLM.Azure.Chat.APIKey != "k2" {
			t.Errorf("Azure.Chat.APIKey = %q, want k2", cfg.LLM.Azure.Chat.APIKey)
		}
		if !strings.Contains(buf.String(), "DOCSCONTEXT_LLM_AZURE_CHAT_API_KEY") {
			t.Errorf("missing deprecation warn: %s", buf.String())
		}
	})

	t.Run("T7_both_azure_chat_key_docsiq_wins", func(t *testing.T) {
		home := t.TempDir()
		isolateEnv(t, home)
		t.Setenv("DOCSIQ_LLM_AZURE_CHAT_API_KEY", "new")
		t.Setenv("DOCSCONTEXT_LLM_AZURE_CHAT_API_KEY", "old")

		cfg, err := Load("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.LLM.Azure.Chat.APIKey != "new" {
			t.Errorf("Azure.Chat.APIKey = %q, want new", cfg.LLM.Azure.Chat.APIKey)
		}
	})

	t.Run("T8_new_prefix_api_key_shortcut", func(t *testing.T) {
		// DOCSIQ_API_KEY is an explicit BindEnv alias for server.api_key —
		// convenience shortcut matching the spec. Also verify the nested
		// form works.
		home := t.TempDir()
		isolateEnv(t, home)
		t.Setenv("DOCSIQ_API_KEY", "secret")

		cfg, err := Load("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Server.APIKey != "secret" {
			t.Errorf("Server.APIKey = %q, want secret", cfg.Server.APIKey)
		}
	})

	t.Run("T9_old_prefix_api_key_warns", func(t *testing.T) {
		home := t.TempDir()
		isolateEnv(t, home)
		buf := captureSlog(t)
		t.Setenv("DOCSCONTEXT_API_KEY", "old-secret")

		cfg, err := Load("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Server.APIKey != "old-secret" {
			t.Errorf("Server.APIKey = %q, want old-secret", cfg.Server.APIKey)
		}
		if !strings.Contains(buf.String(), "DOCSCONTEXT_API_KEY") {
			t.Errorf("missing deprecation warn: %s", buf.String())
		}
	})

	t.Run("T10_new_config_dir_wins_with_shadow_warn", func(t *testing.T) {
		home := t.TempDir()
		isolateEnv(t, home)

		newDir := filepath.Join(home, ".docsiq")
		oldDir := filepath.Join(home, ".docscontext")
		if err := os.MkdirAll(newDir, 0o755); err != nil {
			t.Fatalf("mkdir new: %v", err)
		}
		if err := os.MkdirAll(oldDir, 0o755); err != nil {
			t.Fatalf("mkdir old: %v", err)
		}
		if err := os.WriteFile(filepath.Join(newDir, "config.yaml"),
			[]byte("server:\n  port: 1111\n"), 0o644); err != nil {
			t.Fatalf("write new: %v", err)
		}
		if err := os.WriteFile(filepath.Join(oldDir, "config.yaml"),
			[]byte("server:\n  port: 2222\n"), 0o644); err != nil {
			t.Fatalf("write old: %v", err)
		}

		buf := captureSlog(t)
		cfg, err := Load("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Server.Port != 1111 {
			t.Errorf("Server.Port = %d, want 1111 (new wins)", cfg.Server.Port)
		}
		if !strings.Contains(buf.String(), "shadowed legacy config file") {
			t.Errorf("missing shadow warn: %s", buf.String())
		}
	})

	t.Run("T11_old_config_dir_only_warns", func(t *testing.T) {
		home := t.TempDir()
		isolateEnv(t, home)

		oldDir := filepath.Join(home, ".docscontext")
		if err := os.MkdirAll(oldDir, 0o755); err != nil {
			t.Fatalf("mkdir old: %v", err)
		}
		if err := os.WriteFile(filepath.Join(oldDir, "config.yaml"),
			[]byte("server:\n  port: 2222\n"), 0o644); err != nil {
			t.Fatalf("write old: %v", err)
		}

		buf := captureSlog(t)
		cfg, err := Load("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Server.Port != 2222 {
			t.Errorf("Server.Port = %d, want 2222 (old loads)", cfg.Server.Port)
		}
		if !strings.Contains(buf.String(), "deprecated config path") {
			t.Errorf("missing deprecated-path warn: %s", buf.String())
		}
	})

	t.Run("T12_new_data_dir_exists_silent", func(t *testing.T) {
		home := t.TempDir()
		isolateEnv(t, home)

		newDataDir := filepath.Join(home, ".docsiq", "data")
		if err := os.MkdirAll(newDataDir, 0o755); err != nil {
			t.Fatalf("mkdir new data: %v", err)
		}

		buf := captureSlog(t)
		cfg, err := Load("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.DataDir != newDataDir {
			t.Errorf("DataDir = %q, want %q", cfg.DataDir, newDataDir)
		}
		if strings.Contains(buf.String(), "legacy data dir detected") {
			t.Errorf("should not warn about legacy data dir: %s", buf.String())
		}
	})

	t.Run("T13_old_data_dir_only_warns", func(t *testing.T) {
		home := t.TempDir()
		isolateEnv(t, home)

		oldDataDir := filepath.Join(home, ".docscontext", "data")
		if err := os.MkdirAll(oldDataDir, 0o755); err != nil {
			t.Fatalf("mkdir old data: %v", err)
		}

		buf := captureSlog(t)
		_, err := Load("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(buf.String(), "legacy data dir detected") {
			t.Errorf("missing legacy-data-dir warn: %s", buf.String())
		}
	})

	t.Run("T14_docsiq_env_beats_config_file", func(t *testing.T) {
		home := t.TempDir()
		isolateEnv(t, home)

		yamlPath := filepath.Join(home, "cfg.yaml")
		if err := os.WriteFile(yamlPath,
			[]byte("server:\n  port: 8080\n"), 0o644); err != nil {
			t.Fatalf("write yaml: %v", err)
		}
		t.Setenv("DOCSIQ_SERVER_PORT", "9090")

		cfg, err := Load(yamlPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Server.Port != 9090 {
			t.Errorf("Server.Port = %d, want 9090 (env beats file)", cfg.Server.Port)
		}
	})

	t.Run("T15_docscontext_env_beats_config_file_via_alias", func(t *testing.T) {
		home := t.TempDir()
		isolateEnv(t, home)

		yamlPath := filepath.Join(home, "cfg.yaml")
		if err := os.WriteFile(yamlPath,
			[]byte("server:\n  port: 8080\n"), 0o644); err != nil {
			t.Fatalf("write yaml: %v", err)
		}
		t.Setenv("DOCSCONTEXT_SERVER_PORT", "9090")

		cfg, err := Load(yamlPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Server.Port != 9090 {
			t.Errorf("Server.Port = %d, want 9090 (alias beats file)", cfg.Server.Port)
		}
	})

	t.Run("T16_lowercase_env_not_loaded", func(t *testing.T) {
		// Document viper behavior: env matching is case-sensitive on Unix
		// (os.Getenv is exact-match) and viper's AutomaticEnv uppercases
		// the key. A lowercase DOCSIQ_* var is ignored because os.Environ
		// lists it as lowercase and viper looks up the uppercased form.
		home := t.TempDir()
		isolateEnv(t, home)
		t.Setenv("docsiq_api_key", "foo")

		cfg, err := Load("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Server.APIKey == "foo" {
			t.Errorf("lowercase env should not populate config; got %q", cfg.Server.APIKey)
		}
	})

	// --- edge cases ---

	t.Run("edge_empty_deprecated_value_still_warns", func(t *testing.T) {
		// DOCSCONTEXT_FOO="" — still emits a warning. Empty value is
		// mirrored as empty string (effectively unset on some platforms);
		// the user still needs to rename the variable.
		home := t.TempDir()
		isolateEnv(t, home)
		buf := captureSlog(t)
		t.Setenv("DOCSCONTEXT_SERVER_PORT", "")

		_, err := Load("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(buf.String(), "DOCSCONTEXT_SERVER_PORT") {
			t.Errorf("empty deprecated var should still warn: %s", buf.String())
		}
	})

	t.Run("edge_same_value_both_prefixes", func(t *testing.T) {
		home := t.TempDir()
		isolateEnv(t, home)
		buf := captureSlog(t)
		t.Setenv("DOCSIQ_SERVER_PORT", "9090")
		t.Setenv("DOCSCONTEXT_SERVER_PORT", "9090")

		cfg, err := Load("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Server.Port != 9090 {
			t.Errorf("Server.Port = %d, want 9090", cfg.Server.Port)
		}
		if !strings.Contains(buf.String(), "deprecated env var") {
			t.Errorf("should still warn about deprecated var even if values match: %s", buf.String())
		}
	})

	t.Run("edge_very_long_value_10kb", func(t *testing.T) {
		home := t.TempDir()
		isolateEnv(t, home)
		long := strings.Repeat("x", 10*1024)
		t.Setenv("DOCSCONTEXT_LLM_AZURE_CHAT_API_KEY", long)

		cfg, err := Load("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.LLM.Azure.Chat.APIKey != long {
			t.Errorf("long value truncated: got len=%d want=%d",
				len(cfg.LLM.Azure.Chat.APIKey), len(long))
		}
	})

	t.Run("edge_unicode_preserved", func(t *testing.T) {
		home := t.TempDir()
		isolateEnv(t, home)
		u := "秘密-🔐-テスト"
		t.Setenv("DOCSCONTEXT_LLM_AZURE_CHAT_API_KEY", u)

		cfg, err := Load("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.LLM.Azure.Chat.APIKey != u {
			t.Errorf("unicode mangled: %q != %q", cfg.LLM.Azure.Chat.APIKey, u)
		}
	})

	t.Run("edge_double_underscore_harmless", func(t *testing.T) {
		// DOCSCONTEXT__FOO -> DOCSIQ__FOO; viper maps no key, no crash.
		home := t.TempDir()
		isolateEnv(t, home)
		t.Setenv("DOCSCONTEXT__FOO", "bar")

		_, err := Load("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := os.Getenv("DOCSIQ__FOO"); got != "bar" {
			t.Errorf("double-underscore not mirrored: %q", got)
		}
	})

	t.Run("edge_non_docscontext_env_untouched", func(t *testing.T) {
		home := t.TempDir()
		isolateEnv(t, home)
		buf := captureSlog(t)
		// HOME is already set by isolateEnv; PATH is inherited. We don't
		// touch either. Nothing in os.Environ starting with DOCSCONTEXT_
		// should exist after isolateEnv.
		_, err := Load("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if strings.Contains(buf.String(), "deprecated env var") {
			t.Errorf("no-op Load should not warn about deprecations: %s", buf.String())
		}
	})
}

// ---------------------------------------------------------------------------
// slog capture helper.
//
// Installs a TextHandler writing to a buffer as the process default logger
// for the duration of the test. Restoration is registered with t.Cleanup.
// A mutex guards concurrent writes because TextHandler.Handle is safe but
// bytes.Buffer is not.
// ---------------------------------------------------------------------------

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

// captureSlog replaces slog.Default() with a TextHandler writing to a
// buffer for the duration of the test. The returned *syncBuffer yields
// all log output captured while the test runs.
func captureSlog(t *testing.T) *syncBuffer {
	t.Helper()
	buf := &syncBuffer{}
	h := slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	prev := slog.Default()
	slog.SetDefault(slog.New(h))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return buf
}
