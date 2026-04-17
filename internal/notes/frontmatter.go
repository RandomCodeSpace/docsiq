// Package notes implements the disk-backed notes subsystem ported from
// kgraph. Notes are Markdown files on disk (source of truth) with an
// optional YAML frontmatter block. An FTS5 index in the per-project
// SQLite database is maintained as a derived, rebuildable view.
package notes

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"
)

// fmDelim is the three-dash YAML frontmatter delimiter line.
var fmDelim = []byte("---")

// ParseFrontmatter splits raw note bytes into a frontmatter map and a
// body. If the input does not begin with a `---` delimiter line the
// whole input is returned as the body with an empty map — this matches
// the kgraph gray-matter behavior (no frontmatter is not an error).
//
// A body that happens to contain `---` on its own line elsewhere is
// preserved verbatim; only the FIRST delimiter pair is consumed.
func ParseFrontmatter(raw []byte) (map[string]any, []byte, error) {
	data := map[string]any{}
	if len(raw) == 0 {
		return data, []byte{}, nil
	}

	// Must begin with `---` followed by \n or \r\n.
	rest, ok := trimFMDelim(raw)
	if !ok {
		return data, raw, nil
	}

	// Find the closing delimiter on its own line.
	closeIdx := findClosingDelim(rest)
	if closeIdx < 0 {
		// No closing delimiter — gray-matter treats this as "no frontmatter".
		// We do the same: return the entire original input as body.
		return map[string]any{}, raw, nil
	}

	yamlBytes := rest[:closeIdx]
	// Skip past the closing delimiter line.
	afterClose := rest[closeIdx:]
	afterClose, _ = trimFMDelim(afterClose)
	// Drop a single leading \n if present (canonicalize body start).
	if len(afterClose) > 0 && afterClose[0] == '\n' {
		afterClose = afterClose[1:]
	}

	if len(bytes.TrimSpace(yamlBytes)) > 0 {
		if err := yaml.Unmarshal(yamlBytes, &data); err != nil {
			return nil, nil, fmt.Errorf("frontmatter: parse yaml: %w", err)
		}
		if data == nil {
			data = map[string]any{}
		}
	}
	return data, afterClose, nil
}

// EncodeFrontmatter emits `---\n<yaml>\n---\n<body>`. An empty map emits
// the body verbatim (no leading delimiter). Keys are sorted by yaml.v3's
// default encoder (alphabetical); kgraph's gray-matter also sorts, so
// round-trips are stable.
func EncodeFrontmatter(data map[string]any, body []byte) ([]byte, error) {
	if len(data) == 0 {
		return append([]byte{}, body...), nil
	}
	var buf bytes.Buffer
	buf.Write(fmDelim)
	buf.WriteByte('\n')

	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(data); err != nil {
		return nil, fmt.Errorf("frontmatter: encode yaml: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("frontmatter: close yaml: %w", err)
	}
	// yaml.Encoder always ends with \n; add closing delimiter.
	buf.Write(fmDelim)
	buf.WriteByte('\n')
	buf.Write(body)
	return buf.Bytes(), nil
}

// trimFMDelim consumes a leading `---` + newline. Returns the rest and
// true on match.
func trimFMDelim(b []byte) ([]byte, bool) {
	if !bytes.HasPrefix(b, fmDelim) {
		return b, false
	}
	rest := b[len(fmDelim):]
	// Require immediate \n or \r\n. Trailing whitespace on the delim
	// line (e.g. "---   \n") is tolerated.
	i := 0
	for i < len(rest) && (rest[i] == ' ' || rest[i] == '\t') {
		i++
	}
	if i < len(rest) && rest[i] == '\r' {
		i++
	}
	if i >= len(rest) || rest[i] != '\n' {
		return b, false
	}
	return rest[i+1:], true
}

// findClosingDelim returns the byte index in `s` where a line that
// equals `---` (optionally with trailing whitespace) begins, or -1.
func findClosingDelim(s []byte) int {
	start := 0
	for start < len(s) {
		nl := bytes.IndexByte(s[start:], '\n')
		var line []byte
		var lineEnd int
		if nl < 0 {
			line = s[start:]
			lineEnd = len(s)
		} else {
			line = s[start : start+nl]
			lineEnd = start + nl + 1
		}
		trimmed := bytes.TrimRight(line, " \t\r")
		if bytes.Equal(trimmed, fmDelim) {
			return start
		}
		if nl < 0 {
			return -1
		}
		start = lineEnd
	}
	return -1
}
