package project

import (
	"strings"
	"testing"
)

func TestSlugNormalization(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		// NOTE: slug charset is [a-z0-9_-]; dots in hostnames become '-'.
		{"https_github_dotgit", "https://github.com/Owner/Repo.git", "github-com-owner-repo", false},
		{"https_github_no_dotgit", "https://github.com/owner/repo", "github-com-owner-repo", false},
		{"http_plain", "http://example.com/org/proj", "example-com-org-proj", false},
		{"scp_style_github", "git@github.com:owner/repo.git", "github-com-owner-repo", false},
		{"scp_style_gitlab_nested", "git@gitlab.com:group/sub/repo.git", "gitlab-com-group-sub-repo", false},
		{"ssh_url_port", "ssh://git@github.com:22/owner/repo.git", "github-com-owner-repo", false},
		{"ssh_user_path", "ssh://user@host.example/path/to/repo.git", "host-example-path-to-repo", false},
		{"git_protocol", "git://github.com/owner/repo.git", "github-com-owner-repo", false},
		{"file_url", "file:///home/user/repo", "home-user-repo", false},
		{"mixed_case_collapse", "HTTPS://Host.COM/OWNER/REPO.GIT", "host-com-owner-repo", false},
		{"underscore_preserved", "https://host/some_repo", "host-some_repo", false},
		{"hyphen_preserved", "https://host/some-repo", "host-some-repo", false},
		{"dot_becomes_dash_in_path", "git@github.com:owner/name.dotted.git", "github-com-owner-name-dotted", false},
		// Unicode letters are non-[a-z0-9_-] so they collapse to '-'.
		{"unicode_letters_stripped", "https://host/Ömer/café", "host-mer-caf", false},
		{"emoji_stripped", "https://host/🚀/repo", "host-repo", false},
		{"trailing_slash", "https://host/owner/repo/", "host-owner-repo", false},
		{"multiple_dots_in_host", "https://git.sr.ht/~user/proj.git", "git-sr-ht-user-proj", false},
		{"percent_encoded", "https://host/own%20er/repo", "host-own-er-repo", false},
		{"whitespace_trimmed", "   git@github.com:a/b.git  ", "github-com-a-b", false},
		{"only_slashes", "///", "", true},
		{"empty", "", "", true},
		{"only_whitespace", "   \t\n", "", true},
		{"only_dotgit", ".git", "", true},
		{"null_byte", "git@host:owner/repo\x00evil", "", true},
		{"shortpath_no_host", "owner/repo", "owner-repo", false},
		{"single_token", "repo", "repo", false},
		{"already_slug", "my-project_v2", "my-project_v2", false},
		{"very_long", "https://host/" + strings.Repeat("a", 300), "", false}, // want checked separately
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Slug(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("Slug(%q) = %q, want error", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Slug(%q) error: %v", tc.in, err)
			}
			if tc.name == "very_long" {
				if len(got) == 0 || len(got) > maxSlugLen {
					t.Fatalf("very_long slug len=%d, want in (0,%d]", len(got), maxSlugLen)
				}
				return
			}
			if got != tc.want {
				t.Fatalf("Slug(%q) = %q, want %q", tc.in, got, tc.want)
			}
			if !IsValidSlug(got) {
				t.Fatalf("Slug(%q) produced %q which fails IsValidSlug", tc.in, got)
			}
		})
	}
}

func TestIsValidSlug(t *testing.T) {
	cases := []struct {
		s  string
		ok bool
	}{
		{"", false},
		{"a", true},
		{"_default", true},
		{"my-slug_2", true},
		{"UPPER", false},
		{"with space", false},
		{"with/slash", false},
		{"with\\back", false},
		{"with\x00nul", false},
		{"dot.ted", false}, // dots not allowed in slug charset
		{strings.Repeat("a", maxSlugLen), true},
		{strings.Repeat("a", maxSlugLen+1), false},
	}
	for _, tc := range cases {
		if got := IsValidSlug(tc.s); got != tc.ok {
			t.Errorf("IsValidSlug(%q) = %v, want %v", tc.s, got, tc.ok)
		}
	}
}

func TestSlug_Deterministic(t *testing.T) {
	// Same input always produces the same slug.
	remote := "git@github.com:Owner/Repo.git"
	first, err := Slug(remote)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		got, err := Slug(remote)
		if err != nil {
			t.Fatal(err)
		}
		if got != first {
			t.Fatalf("Slug not deterministic: %q vs %q", first, got)
		}
	}
}

func TestSlug_NoPathTraversal(t *testing.T) {
	// A remote containing ".." must not produce a slug with ".." — the
	// slug charset excludes '.', so this is covered, but we assert it
	// explicitly because it's a security-relevant invariant.
	remotes := []string{
		"https://host/../../etc/passwd",
		"git@host:../escape.git",
		"file:///..//..//root",
	}
	for _, r := range remotes {
		got, err := Slug(r)
		if err != nil {
			continue
		}
		if strings.Contains(got, "..") || strings.Contains(got, "/") {
			t.Errorf("Slug(%q) = %q, leaks path traversal", r, got)
		}
	}
}
