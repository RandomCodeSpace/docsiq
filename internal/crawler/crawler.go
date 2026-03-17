// Package crawler discovers and fetches pages from documentation websites.
// It supports sitemap.xml discovery (MkDocs, Docusaurus, Sphinx) and falls
// back to BFS link-following within the same origin.
package crawler

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"

	"github.com/RandomCodeSpace/docsgraphcontext/internal/loader"
)

// Page is a fetched documentation page.
type Page = loader.RawDocument

// Options controls crawl behaviour.
type Options struct {
	MaxPages    int  // 0 = unlimited
	MaxDepth    int  // 0 = unlimited BFS depth
	Concurrency int  // parallel fetchers (default 4)
	SkipSitemap bool // force BFS even if sitemap.xml exists
}

// Crawl fetches all pages reachable from rootURL within the same origin.
// It tries sitemap.xml first; if absent it falls back to BFS link following.
func Crawl(ctx context.Context, rootURL string, opts Options) ([]*Page, error) {
	if opts.Concurrency <= 0 {
		opts.Concurrency = 4
	}
	if opts.MaxPages <= 0 {
		opts.MaxPages = 500
	}

	base, err := url.Parse(rootURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	base.Fragment = ""
	base.RawQuery = ""

	client := &http.Client{Timeout: 30 * time.Second}
	wl := loader.NewWebLoader()

	// Try sitemap first
	var urls []string
	if !opts.SkipSitemap {
		urls, _ = discoverSitemap(client, base)
	}

	// Fall back to BFS
	if len(urls) == 0 {
		urls = bfsCrawl(ctx, client, base, opts)
	}

	// Cap
	if opts.MaxPages > 0 && len(urls) > opts.MaxPages {
		urls = urls[:opts.MaxPages]
	}

	// Fetch pages concurrently
	pages := make([]*Page, len(urls))
	errs := make([]error, len(urls))
	sem := make(chan struct{}, opts.Concurrency)
	var wg sync.WaitGroup

	for i, u := range urls {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, pageURL string) {
			defer wg.Done()
			defer func() { <-sem }()
			p, err := wl.LoadURL(pageURL)
			pages[idx] = p
			errs[idx] = err
		}(i, u)
	}
	wg.Wait()

	// Collect successful fetches
	var result []*Page
	for i, p := range pages {
		if errs[i] == nil && p != nil && strings.TrimSpace(p.Content) != "" {
			result = append(result, p)
		}
	}
	return result, nil
}

// ── Sitemap discovery ─────────────────────────────────────────────────────────

type sitemapIndex struct {
	Sitemaps []struct {
		Loc string `xml:"loc"`
	} `xml:"sitemap"`
}

type urlSet struct {
	URLs []struct {
		Loc string `xml:"loc"`
	} `xml:"url"`
}

func discoverSitemap(client *http.Client, base *url.URL) ([]string, error) {
	candidates := []string{
		base.Scheme + "://" + base.Host + "/sitemap.xml",
		base.String() + "/sitemap.xml",
		base.String() + "sitemap.xml",
	}

	for _, candidate := range candidates {
		urls, err := parseSitemap(client, candidate, base)
		if err == nil && len(urls) > 0 {
			return urls, nil
		}
	}
	return nil, fmt.Errorf("no sitemap found")
}

func parseSitemap(client *http.Client, sitemapURL string, base *url.URL) ([]string, error) {
	resp, err := client.Get(sitemapURL)
	if err != nil || resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sitemap not found")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20))
	if err != nil {
		return nil, err
	}

	// Try sitemap index first
	var idx sitemapIndex
	if err := xml.Unmarshal(body, &idx); err == nil && len(idx.Sitemaps) > 0 {
		var all []string
		for _, s := range idx.Sitemaps {
			sub, err := parseSitemap(client, s.Loc, base)
			if err == nil {
				all = append(all, sub...)
			}
		}
		return all, nil
	}

	// Try urlset
	var us urlSet
	if err := xml.Unmarshal(body, &us); err != nil {
		return nil, err
	}

	var urls []string
	for _, u := range us.URLs {
		if isSameOrigin(u.Loc, base) {
			urls = append(urls, u.Loc)
		}
	}
	return urls, nil
}

// ── BFS link crawler ──────────────────────────────────────────────────────────

func bfsCrawl(ctx context.Context, client *http.Client, base *url.URL, opts Options) []string {
	visited := map[string]bool{}
	queue := []struct {
		u     string
		depth int
	}{{base.String(), 0}}
	var found []string

	for len(queue) > 0 && (opts.MaxPages == 0 || len(found) < opts.MaxPages) {
		select {
		case <-ctx.Done():
			return found
		default:
		}

		item := queue[0]
		queue = queue[1:]

		if visited[item.u] {
			continue
		}
		visited[item.u] = true
		found = append(found, item.u)

		if opts.MaxDepth > 0 && item.depth >= opts.MaxDepth {
			continue
		}

		links := extractLinks(client, item.u, base)
		for _, l := range links {
			if !visited[l] {
				queue = append(queue, struct {
					u     string
					depth int
				}{l, item.depth + 1})
			}
		}
	}
	return found
}

func extractLinks(client *http.Client, pageURL string, base *url.URL) []string {
	resp, err := client.Get(pageURL)
	if err != nil || resp.StatusCode != http.StatusOK {
		return nil
	}
	defer resp.Body.Close()

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return nil
	}

	seen := map[string]bool{}
	var links []string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, a := range n.Attr {
				if a.Key == "href" {
					resolved := resolveURL(pageURL, a.Val)
					if resolved != "" && isSameOrigin(resolved, base) && !seen[resolved] {
						seen[resolved] = true
						links = append(links, resolved)
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return links
}

func resolveURL(base, href string) string {
	if strings.HasPrefix(href, "#") || strings.HasPrefix(href, "mailto:") ||
		strings.HasPrefix(href, "javascript:") {
		return ""
	}
	b, err := url.Parse(base)
	if err != nil {
		return ""
	}
	h, err := url.Parse(href)
	if err != nil {
		return ""
	}
	resolved := b.ResolveReference(h)
	resolved.Fragment = ""
	resolved.RawQuery = ""
	return resolved.String()
}

func isSameOrigin(rawURL string, base *url.URL) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return u.Host == base.Host && strings.HasPrefix(u.Path, base.Path)
}
