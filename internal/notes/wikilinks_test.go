package notes

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseWikilink(t *testing.T) {
	cases := []struct {
		name         string
		target       string
		wantProject  string
		wantKey      string
		wantCross    bool
	}{
		// same-project: plain key
		{"same_plain", "foo", "", "foo", false},
		// cross-project: valid slug + key
		{"cross_valid", "projects/docsiq/internal/pipeline", "docsiq", "internal/pipeline", true},
		// NOT cross-project: no key part after slug
		{"no_key", "projects/docsiq", "", "projects/docsiq", false},
		// NOT cross-project: empty slug
		{"empty_slug", "projects//foo", "", "projects//foo", false},
		// same-project: folder key
		{"same_folder", "architecture/overview", "", "architecture/overview", false},
		// cross-project: underscore and hyphen in slug
		{"cross_slug_chars", "projects/my-proj_1/notes/design", "my-proj_1", "notes/design", true},
		// same-project: only "projects" with no slash after
		{"projects_no_slash", "projects", "", "projects", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseWikilink(tc.target)
			if got.Target != tc.target {
				t.Errorf("Target = %q, want %q", got.Target, tc.target)
			}
			if got.Project != tc.wantProject {
				t.Errorf("Project = %q, want %q", got.Project, tc.wantProject)
			}
			if got.Key != tc.wantKey {
				t.Errorf("Key = %q, want %q", got.Key, tc.wantKey)
			}
			if got.CrossProject != tc.wantCross {
				t.Errorf("CrossProject = %v, want %v", got.CrossProject, tc.wantCross)
			}
		})
	}
}

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
