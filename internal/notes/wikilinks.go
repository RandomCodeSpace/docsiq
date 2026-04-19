package notes

import "regexp"

// crossProjectRe matches targets of the form projects/<slug>/<rest> where
// slug is alphanumeric plus hyphens/underscores (mirrors IsValidSlug minus
// the lowercase restriction — we match case-insensitively at the call site).
var crossProjectRe = regexp.MustCompile(`^projects/([a-zA-Z0-9_-]+)/(.+)$`)

// Wikilink holds a parsed wikilink target broken into its constituent parts.
type Wikilink struct {
	Target       string // raw target string, unchanged
	Project      string // "" for same-project; slug of the target project otherwise
	Key          string // the note key within that project (or the raw target for same-project)
	CrossProject bool   // true iff target is a cross-project reference
}

// ParseWikilink parses a raw wikilink target (as returned by ExtractWikilinks)
// into a Wikilink. A target matches the cross-project pattern when it is of
// the form "projects/<slug>/<rest>" and <slug> is a non-empty string of
// alphanumerics, hyphens, and underscores. Any other shape is treated as a
// same-project key.
func ParseWikilink(target string) Wikilink {
	if m := crossProjectRe.FindStringSubmatch(target); m != nil {
		// m[1] = slug, m[2] = key — both are guaranteed non-empty by the regex.
		return Wikilink{
			Target:       target,
			Project:      m[1],
			Key:          m[2],
			CrossProject: true,
		}
	}
	return Wikilink{Target: target, Key: target}
}

// wikilinkRe matches [[target]] and [[target|Alias]] forms. We greedily
// disallow ']' and '|' inside the target slug so nested/unterminated
// links don't produce garbage.
var wikilinkRe = regexp.MustCompile(`\[\[([^\]|]+?)(?:\|[^\]]*)?\]\]`)

// ExtractWikilinks returns the de-duplicated list of note keys referenced
// by `[[wikilink]]` or `[[wikilink|alias]]` occurrences in body, in the
// order of first appearance. Whitespace inside a target is preserved
// verbatim (Obsidian-style keys can contain spaces, though we normalize
// them to paths at the call site).
func ExtractWikilinks(body []byte) []string {
	matches := wikilinkRe.FindAllSubmatch(body, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(matches))
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		target := string(m[1])
		if target == "" {
			continue
		}
		if _, ok := seen[target]; ok {
			continue
		}
		seen[target] = struct{}{}
		out = append(out, target)
	}
	return out
}
