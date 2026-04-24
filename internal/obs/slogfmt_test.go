package obs

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"
)

func TestStripLeadingEmoji(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want string
	}{
		{"✅ all good", "all good"},            // white heavy check mark
		{"❌ panic recovered", "panic recovered"}, // cross mark
		{"⚠️ auth disabled", "auth disabled"}, // warning + VS16
		{"\U0001f6d1 shutting down...", "shutting down..."},
		{"⚙️ LLM provider initialised", "LLM provider initialised"},
		{"plain log line", "plain log line"},
		{"", ""},
		{"  leading spaces", "  leading spaces"}, // no emoji → untouched
	}
	for _, c := range cases {
		got := stripLeadingEmoji(c.in)
		if got != c.want {
			t.Errorf("stripLeadingEmoji(%q)=%q want %q", c.in, got, c.want)
		}
	}
}

func TestProductionHandler_JSONOutputNoEmoji(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	h := NewProductionHandler(slog.NewJSONHandler(&buf, nil))
	logger := slog.New(h)

	logger.Info("✅ ready", "port", 8080)
	logger.Error("❌ connection failed", "err", "timeout")

	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	if len(lines) != 2 {
		t.Fatalf("got %d lines want 2", len(lines))
	}
	for _, line := range lines {
		var rec map[string]any
		if err := json.Unmarshal(line, &rec); err != nil {
			t.Fatalf("not JSON: %v — raw=%q", err, line)
		}
		msg, _ := rec["msg"].(string)
		if msg == "" {
			t.Errorf("missing msg: %s", line)
		}
		for _, r := range msg {
			if isEmojiLike(r) {
				t.Errorf("msg contains emoji %q; msg=%q", r, msg)
				break
			}
		}
	}
}
