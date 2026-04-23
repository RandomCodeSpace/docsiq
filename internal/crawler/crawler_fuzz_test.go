package crawler

import (
	"net/url"
	"strings"
	"testing"
)

func FuzzResolveURL(f *testing.F) {
	seeds := []struct {
		base, href string
	}{
		{"https://example.com/", ""},
		{"https://example.com/docs/", "guide.html"},
		{"https://example.com/", "#anchor"},
		{"https://example.com/", "mailto:a@b.c"},
		{"https://example.com/", "javascript:alert(1)"},
		{"https://example.com/", "JavaScript:alert(1)"},
		{"https://example.com/", "data:text/html,<script>alert(1)</script>"},
		{"https://example.com/", "vbscript:msgbox(1)"},
		{"https://example.com/", "tel:+15555555555"},
		{"https://example.com/", "file:///etc/passwd"},
		{"https://example.com/", "//evil.com/x"},
		{"https://example.com/", "http://example.com/%"},
		{"https://example.com/", strings.Repeat("a", 4096)},
	}
	for _, s := range seeds {
		f.Add(s.base, s.href)
	}

	f.Fuzz(func(t *testing.T, base, href string) {
		got := resolveURL(base, href)
		if got == "" {
			return
		}
		// Any non-empty result MUST be a parseable http/https URL.
		u, err := url.Parse(got)
		if err != nil {
			t.Fatalf("resolveURL returned unparseable URL: base=%q href=%q got=%q err=%v",
				base, href, got, err)
		}
		scheme := strings.ToLower(u.Scheme)
		if scheme != "http" && scheme != "https" {
			t.Fatalf("resolveURL returned non-http(s) scheme: base=%q href=%q got=%q scheme=%q",
				base, href, got, scheme)
		}
	})
}
