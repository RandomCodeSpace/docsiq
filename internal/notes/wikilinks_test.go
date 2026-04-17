package notes

import (
	"reflect"
	"strings"
	"testing"
)

func TestExtractWikilinks(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"simple", "see [[foo]] and [[bar]]", []string{"foo", "bar"}},
		{"aliased", "alias [[foo|Display]]", []string{"foo"}},
		{"dedup_preserves_order", "[[b]] [[a]] [[b]]", []string{"b", "a"}},
		{"folder_key", "[[architecture/overview]]", []string{"architecture/overview"}},
		{"empty_input", "", nil},
		{"no_links", "just prose [single] and (parens)", nil},
		{"unterminated", "open [[no close and more text", nil},
		{"pipe_at_start", "[[|alias-only]]", nil}, // no target → skipped
		{"multiple_per_line", "[[a]][[b]][[c]]", []string{"a", "b", "c"}},
		{"unicode_target", "[[日本語/ノート]]", []string{"日本語/ノート"}},
		{"nested_brackets", "[[outer [inner]]]", []string{"outer [inner"}}, // regex stops at first ']'
		{"escaped_forms_noop", "\\[[not-a-link]]", []string{"not-a-link"}},  // we don't implement \ escaping
		{"very_long_target", "[[" + strings.Repeat("a", 500) + "]]", []string{strings.Repeat("a", 500)}},
		{"mixed_alias_and_plain", "[[x|X]] and [[x]]", []string{"x"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ExtractWikilinks([]byte(tc.in))
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}
