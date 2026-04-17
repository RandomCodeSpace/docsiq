package notes

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseFrontmatter(t *testing.T) {
	t.Run("no_frontmatter", func(t *testing.T) {
		fm, body, err := ParseFrontmatter([]byte("just body\nnothing fancy\n"))
		if err != nil {
			t.Fatal(err)
		}
		if len(fm) != 0 {
			t.Errorf("expected empty map, got %v", fm)
		}
		if string(body) != "just body\nnothing fancy\n" {
			t.Errorf("body mismatch: %q", body)
		}
	})

	t.Run("empty_input", func(t *testing.T) {
		fm, body, err := ParseFrontmatter(nil)
		if err != nil {
			t.Fatal(err)
		}
		if fm == nil || len(fm) != 0 {
			t.Errorf("expected empty map, got %v", fm)
		}
		if len(body) != 0 {
			t.Errorf("expected empty body, got %q", body)
		}
	})

	t.Run("simple_fm", func(t *testing.T) {
		in := []byte("---\nauthor: alice\ntags:\n  - a\n  - b\n---\nhello\n")
		fm, body, err := ParseFrontmatter(in)
		if err != nil {
			t.Fatal(err)
		}
		if fm["author"] != "alice" {
			t.Errorf("author = %v", fm["author"])
		}
		tags, _ := fm["tags"].([]any)
		if len(tags) != 2 || tags[0] != "a" || tags[1] != "b" {
			t.Errorf("tags = %v", fm["tags"])
		}
		if string(body) != "hello\n" {
			t.Errorf("body = %q", body)
		}
	})

	t.Run("malformed_yaml", func(t *testing.T) {
		in := []byte("---\nauthor: [unclosed\n---\nbody\n")
		_, _, err := ParseFrontmatter(in)
		if err == nil {
			t.Fatal("expected error for malformed yaml")
		}
	})

	t.Run("no_closing_delim", func(t *testing.T) {
		// gray-matter compat: treat as "no frontmatter".
		in := []byte("---\nfoo: bar\nrest of file\n")
		fm, body, err := ParseFrontmatter(in)
		if err != nil {
			t.Fatal(err)
		}
		if len(fm) != 0 {
			t.Errorf("expected empty fm, got %v", fm)
		}
		if !bytes.Equal(body, in) {
			t.Errorf("expected whole input as body")
		}
	})

	t.Run("unicode_values", func(t *testing.T) {
		in := []byte("---\ntitle: \"日本語 emoji 🎉\"\n---\nbody\n")
		fm, _, err := ParseFrontmatter(in)
		if err != nil {
			t.Fatal(err)
		}
		if fm["title"] != "日本語 emoji 🎉" {
			t.Errorf("title = %v", fm["title"])
		}
	})

	t.Run("nested_maps", func(t *testing.T) {
		in := []byte("---\nmeta:\n  a: 1\n  b:\n    c: 2\n---\n")
		fm, _, err := ParseFrontmatter(in)
		if err != nil {
			t.Fatal(err)
		}
		meta, ok := fm["meta"].(map[string]any)
		if !ok {
			t.Fatalf("meta not a map: %T", fm["meta"])
		}
		if meta["a"] != 1 {
			t.Errorf("meta.a = %v", meta["a"])
		}
	})

	t.Run("very_long_value", func(t *testing.T) {
		long := strings.Repeat("x", 100_000)
		in := []byte("---\nblob: " + long + "\n---\nb\n")
		fm, _, err := ParseFrontmatter(in)
		if err != nil {
			t.Fatal(err)
		}
		if fm["blob"] != long {
			t.Error("long value not round-tripped")
		}
	})

	t.Run("only_fm_no_body", func(t *testing.T) {
		in := []byte("---\na: 1\n---\n")
		fm, body, err := ParseFrontmatter(in)
		if err != nil {
			t.Fatal(err)
		}
		if fm["a"] != 1 {
			t.Errorf("a = %v", fm["a"])
		}
		if len(body) != 0 {
			t.Errorf("expected empty body, got %q", body)
		}
	})

	t.Run("body_contains_triple_dashes", func(t *testing.T) {
		// Edge: kgraph doesn't test this. Our impl consumes only the
		// first delimiter pair; inner `---` lines stay in the body.
		in := []byte("---\nk: v\n---\nbefore\n---\nafter\n")
		fm, body, err := ParseFrontmatter(in)
		if err != nil {
			t.Fatal(err)
		}
		if fm["k"] != "v" {
			t.Errorf("k = %v", fm["k"])
		}
		if !bytes.Contains(body, []byte("before")) || !bytes.Contains(body, []byte("after")) {
			t.Errorf("body lost content: %q", body)
		}
	})

	t.Run("crlf_line_endings", func(t *testing.T) {
		in := []byte("---\r\nfoo: bar\r\n---\r\nhi\r\n")
		fm, _, err := ParseFrontmatter(in)
		if err != nil {
			t.Fatal(err)
		}
		if fm["foo"] != "bar" {
			t.Errorf("foo = %v", fm["foo"])
		}
	})
}

func TestEncodeFrontmatter(t *testing.T) {
	t.Run("empty_map_returns_body", func(t *testing.T) {
		out, err := EncodeFrontmatter(map[string]any{}, []byte("hi"))
		if err != nil {
			t.Fatal(err)
		}
		if string(out) != "hi" {
			t.Errorf("got %q", out)
		}
	})

	t.Run("round_trip", func(t *testing.T) {
		in := map[string]any{"author": "bob", "tags": []string{"x", "y"}}
		encoded, err := EncodeFrontmatter(in, []byte("body\n"))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.HasPrefix(encoded, []byte("---\n")) {
			t.Errorf("missing leading delim: %q", encoded)
		}
		fm, body, err := ParseFrontmatter(encoded)
		if err != nil {
			t.Fatal(err)
		}
		if fm["author"] != "bob" {
			t.Errorf("author = %v", fm["author"])
		}
		if string(body) != "body\n" {
			t.Errorf("body = %q", body)
		}
	})

	t.Run("unicode_round_trip", func(t *testing.T) {
		in := map[string]any{"title": "日本語 🎉"}
		enc, err := EncodeFrontmatter(in, []byte("x"))
		if err != nil {
			t.Fatal(err)
		}
		fm, _, err := ParseFrontmatter(enc)
		if err != nil {
			t.Fatal(err)
		}
		if fm["title"] != "日本語 🎉" {
			t.Errorf("title = %v", fm["title"])
		}
	})
}
