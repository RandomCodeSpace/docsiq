package notes

import "regexp"

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
