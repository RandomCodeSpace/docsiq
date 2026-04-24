package obs

import (
	"context"
	"log/slog"
	"strings"
	"unicode/utf8"
)

// NewProductionHandler wraps an inner slog.Handler and strips a
// leading emoji + trailing space from each record's Message. docsiq
// uses emoji prefixes (OK KO WARN etc.) as visual cues in dev text
// format; in JSON these collide with log-aggregator indexing
// (Elasticsearch tokeniser, fluentd grep rules) and obscure the actual
// message string. The handler mutates only Message — attrs pass
// through.
func NewProductionHandler(inner slog.Handler) slog.Handler {
	return &prodHandler{inner: inner}
}

type prodHandler struct{ inner slog.Handler }

func (h *prodHandler) Enabled(ctx context.Context, lvl slog.Level) bool {
	return h.inner.Enabled(ctx, lvl)
}

func (h *prodHandler) Handle(ctx context.Context, r slog.Record) error {
	r.Message = stripLeadingEmoji(r.Message)
	return h.inner.Handle(ctx, r)
}

func (h *prodHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &prodHandler{inner: h.inner.WithAttrs(attrs)}
}

func (h *prodHandler) WithGroup(name string) slog.Handler {
	return &prodHandler{inner: h.inner.WithGroup(name)}
}

// stripLeadingEmoji removes the first rune from msg if it is in a
// Unicode emoji-like range, plus any immediately-following whitespace.
// Also strips a VS16 variation selector (U+FE0F) that often follows
// warning signs etc. We intentionally do NOT use a dependency like
// mattn/go-emoji; docsiq ships under air-gap rules (see build.md).
func stripLeadingEmoji(msg string) string {
	if msg == "" {
		return msg
	}
	r, size := utf8.DecodeRuneInString(msg)
	if r == utf8.RuneError {
		return msg
	}
	if !isEmojiLike(r) {
		return msg
	}
	rest := msg[size:]
	rest = strings.TrimLeft(rest, " \t")
	if len(rest) > 0 {
		r2, size2 := utf8.DecodeRuneInString(rest)
		if r2 == 0xFE0F {
			rest = strings.TrimLeft(rest[size2:], " \t")
		}
	}
	return rest
}

// isEmojiLike is a conservative test for the emoji-range runes that
// appear in docsiq log messages today. Covers BMP symbols
// (U+2600-U+27BF), miscellaneous pictographs (U+1F300-U+1F6FF), and
// supplemental symbols (U+1F900-U+1F9FF).
func isEmojiLike(r rune) bool {
	switch {
	case r >= 0x2600 && r <= 0x27BF:
		return true
	case r >= 0x1F300 && r <= 0x1F6FF:
		return true
	case r >= 0x1F900 && r <= 0x1F9FF:
		return true
	}
	return false
}
