package chunker

import (
	"strings"
	"testing"
)

func FuzzChunker(f *testing.F) {
	seeds := []string{
		"",
		"hello world",
		"one two three four five",
		"paragraph one.\n\nparagraph two.",
		strings.Repeat("a", 2048),
		"\x00\x00\x00",
		"unicode: 你好 世界 𝕌𝕟𝕚𝕔𝕠𝕕𝕖",
		strings.Repeat("word ", 1024),
		"line\nline\nline\nline",
		"mixed\r\nwindows\r\nendings",
	}
	for _, s := range seeds {
		f.Add(s, 256, 32)
	}

	f.Fuzz(func(t *testing.T, text string, size, overlap int) {
		if size <= 0 || size > 4096 || overlap < 0 || overlap >= size {
			t.Skip()
		}
		c := New(size, overlap)
		chunks := c.Split(text)
		for i, ch := range chunks {
			if ch.Index != i {
				t.Fatalf("chunk %d has wrong Index=%d", i, ch.Index)
			}
			if ch.Tokens < 0 {
				t.Fatalf("chunk %d negative token count: %d", i, ch.Tokens)
			}
		}
	})
}
