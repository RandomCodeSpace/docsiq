package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

// TestLogFormatJSON is a compact smoke-test for the JSON handler — the
// real --log-format wiring lives in initConfig which hits os.Stderr +
// viper, neither of which is worth faking here. Exercising the handler
// directly is enough to catch regressions where we accidentally write a
// text handler for format=json.
func TestLogFormatJSON(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
	logger.Info("hello", "k", "v")

	var decoded map[string]any
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("log output is not valid JSON: %v\n%s", err, buf.String())
	}
	if decoded["msg"] != "hello" {
		t.Errorf("msg = %v, want hello", decoded["msg"])
	}
	if decoded["k"] != "v" {
		t.Errorf("k = %v, want v", decoded["k"])
	}
}

// TestBuildLogHandler_JSONStripsEmoji confirms buildLogHandler returns
// a handler chain that strips emoji from the message when format=json.
// buildLogHandler writes to os.Stderr so we redirect it through a pipe
// for the test (NOT parallel — shares global os.Stderr).
func TestBuildLogHandler_JSONStripsEmoji(t *testing.T) {
	origStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = origStderr })

	h := buildLogHandler(slog.LevelInfo, "json")
	slog.New(h).Info("✅ ready", "k", "v")
	_ = w.Close()

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	var rec map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(out), &rec); err != nil {
		t.Fatalf("not JSON: %v — raw=%q", err, out)
	}
	msg, _ := rec["msg"].(string)
	if msg != "ready" {
		t.Errorf("msg=%q want 'ready' (emoji stripped)", msg)
	}
}

// TestInitConfig_LogFormatFromConfigFile asserts the config file is
// consulted when neither --log-format nor DOCSIQ_LOG_FORMAT is set.
// NOT parallel — mutates env + HOME + package-level flags.
func TestInitConfig_LogFormatFromConfigFile(t *testing.T) {
	origHome := os.Getenv("HOME")
	origLogFormat := os.Getenv("DOCSIQ_LOG_FORMAT")
	t.Cleanup(func() {
		logLevel = "info"
		logFormat = ""
		cfgFile = ""
		cfg = nil
		os.Setenv("HOME", origHome)
		if origLogFormat != "" {
			os.Setenv("DOCSIQ_LOG_FORMAT", origLogFormat)
		}
	})

	dir := t.TempDir()
	os.Setenv("HOME", dir)
	os.Unsetenv("DOCSIQ_LOG_FORMAT")

	yaml := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(yaml, []byte("log:\n  format: json\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfgFile = yaml
	logFormat = ""

	initConfig()

	if cfg == nil {
		t.Fatal("initConfig produced nil cfg")
	}
	if cfg.Log.Format != "json" {
		t.Errorf("cfg.Log.Format=%q want json", cfg.Log.Format)
	}
}
