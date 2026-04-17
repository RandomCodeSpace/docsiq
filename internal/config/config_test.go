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

// isolateEnv unsets all DOCSIQ_* env vars and sets HOME to the given
// tempdir so Load() can't read any real user config. Restores originals
// on test cleanup.
func isolateEnv(t *testing.T, home string) {
	t.Helper()

	type kv struct{ k, v string }
	var saved []kv
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "DOCSIQ_") {
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
		t.Setenv("DOCSIQ_SERVER_PORT", "9999")

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
		t.Setenv("DOCSIQ_LLM_AZURE_CHAT_MODEL", "gpt-5")

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
		t.Setenv("DOCSIQ_LLM_AZURE_API_KEY", "shared-secret-abc")

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
		t.Setenv("DOCSIQ_LLM_AZURE_API_KEY", "shared-key")
		t.Setenv("DOCSIQ_LLM_AZURE_CHAT_API_KEY", "chat-specific-key")

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
		t.Setenv("DOCSIQ_SERVER_PORT", "7777")

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
		// HOME is a fresh tempdir with no ~/.docsiq.
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
		home := t.TempDir()
		isolateEnv(t, home)
		t.Setenv("DOCSIQ_SERVER_PORT", "0")

		cfg, err := Load("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Server.Port != 0 {
			t.Errorf("Server.Port = %d, want 0 (currently accepted)", cfg.Server.Port)
		}
	})

	t.Run("non_numeric_port_falls_back_to_default", func(t *testing.T) {
		home := t.TempDir()
		isolateEnv(t, home)
		t.Setenv("DOCSIQ_SERVER_PORT", "not-a-number")

		cfg, err := Load("")
		if err != nil {
			return
		}
		if cfg.Server.Port == 8080 {
			t.Errorf("non-numeric port silently kept default 8080; got=%d", cfg.Server.Port)
		}
		if cfg.Server.Port != 0 {
			t.Logf("observed non-numeric port -> %d (documenting)", cfg.Server.Port)
		}
	})

	t.Run("unicode_and_long_string_env_values", func(t *testing.T) {
		home := t.TempDir()
		isolateEnv(t, home)

		unicode := "テスト-模型-🚀"
		long := strings.Repeat("a", 4096)
		t.Setenv("DOCSIQ_LLM_AZURE_CHAT_MODEL", unicode)
		t.Setenv("DOCSIQ_LLM_AZURE_API_KEY", long)

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
		home := t.TempDir()
		isolateEnv(t, home)

		_, err := Load(filepath.Join(home, "does-not-exist.yaml"))
		if err == nil {
			t.Fatal("expected error for nonexistent explicit config path, got nil")
		}
	})

	// ---------------------------------------------------------------------
	// DOCSIQ_ prefix behavior
	// ---------------------------------------------------------------------

	t.Run("docsiq_server_port_no_warning", func(t *testing.T) {
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

	t.Run("docsiq_azure_chat_api_key", func(t *testing.T) {
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

	t.Run("docsiq_api_key_shortcut", func(t *testing.T) {
		// DOCSIQ_API_KEY is an explicit BindEnv alias for server.api_key.
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

	t.Run("docsiq_env_beats_config_file", func(t *testing.T) {
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

	t.Run("lowercase_env_not_loaded", func(t *testing.T) {
		// Document viper behavior: env matching is case-sensitive on Unix.
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

	t.Run("edge_very_long_value_10kb", func(t *testing.T) {
		home := t.TempDir()
		isolateEnv(t, home)
		long := strings.Repeat("x", 10*1024)
		t.Setenv("DOCSIQ_LLM_AZURE_CHAT_API_KEY", long)

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
		t.Setenv("DOCSIQ_LLM_AZURE_CHAT_API_KEY", u)

		cfg, err := Load("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.LLM.Azure.Chat.APIKey != u {
			t.Errorf("unicode mangled: %q != %q", cfg.LLM.Azure.Chat.APIKey, u)
		}
	})
}

func TestProjectDBPath(t *testing.T) {
	cfg := &Config{DataDir: "/tmp/docsiq-data"}
	got := cfg.ProjectDBPath("my-slug")
	want := filepath.Join("/tmp/docsiq-data", "projects", "my-slug", "docsiq.db")
	if got != want {
		t.Errorf("ProjectDBPath = %q, want %q", got, want)
	}
}

func TestProjectDBPath_EmptySlug(t *testing.T) {
	// ProjectDBPath does NOT validate the slug — callers should.
	cfg := &Config{DataDir: "/d"}
	got := cfg.ProjectDBPath("")
	want := filepath.Join("/d", "projects", "docsiq.db")
	if got != want {
		t.Errorf("ProjectDBPath(\"\") = %q, want %q", got, want)
	}
}

func TestDefaultProject_Default(t *testing.T) {
	home := t.TempDir()
	isolateEnv(t, home)

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.DefaultProject != DefaultProjectSlug {
		t.Errorf("DefaultProject = %q, want %q", cfg.DefaultProject, DefaultProjectSlug)
	}
	if DefaultProjectSlug != "_default" {
		t.Errorf("DefaultProjectSlug constant = %q, want _default (locked by Phase-1 spec)", DefaultProjectSlug)
	}
}

func TestDefaultProject_EnvOverride(t *testing.T) {
	home := t.TempDir()
	isolateEnv(t, home)
	if err := os.Setenv("DOCSIQ_DEFAULT_PROJECT", "custom_default"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Unsetenv("DOCSIQ_DEFAULT_PROJECT") })

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.DefaultProject != "custom_default" {
		t.Errorf("DefaultProject = %q, want custom_default", cfg.DefaultProject)
	}
}

// ---------------------------------------------------------------------------
// slog capture helper.
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
// buffer for the duration of the test.
func captureSlog(t *testing.T) *syncBuffer {
	t.Helper()
	buf := &syncBuffer{}
	h := slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	prev := slog.Default()
	slog.SetDefault(slog.New(h))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return buf
}
