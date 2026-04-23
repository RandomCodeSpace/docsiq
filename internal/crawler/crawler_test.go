package crawler

import "testing"

func TestResolveURL_SchemeAllowList(t *testing.T) {
	const base = "https://example.com/docs/"
	cases := []struct {
		name string
		href string
		want string
	}{
		{"relative path", "guide.html", "https://example.com/docs/guide.html"},
		{"absolute http", "http://example.com/x", "http://example.com/x"},
		{"absolute https", "https://example.com/x", "https://example.com/x"},
		{"fragment only", "#anchor", ""},
		{"mailto", "mailto:a@b.c", ""},
		{"javascript", "javascript:alert(1)", ""},
		{"javascript case", "JavaScript:alert(1)", ""},
		{"data uri", "data:text/html,<script>alert(1)</script>", ""},
		{"vbscript", "vbscript:msgbox(1)", ""},
		{"tel", "tel:+15555555555", ""},
		{"file", "file:///etc/passwd", ""},
		{"blob", "blob:https://example.com/abc", ""},
		{"ftp", "ftp://example.com/file", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveURL(base, tc.href)
			if got != tc.want {
				t.Errorf("resolveURL(%q, %q) = %q, want %q", base, tc.href, got, tc.want)
			}
		})
	}
}
