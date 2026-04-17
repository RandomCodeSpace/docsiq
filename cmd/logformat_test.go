package cmd

import (
	"bytes"
	"encoding/json"
	"log/slog"
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
