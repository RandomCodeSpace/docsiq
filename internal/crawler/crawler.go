// Package crawler discovers and fetches pages from documentation websites.
// It supports sitemap.xml discovery (MkDocs, Docusaurus, Sphinx) and falls
// back to BFS link-following within the same origin.
package crawler

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"

	"github.com/RandomCodeSpace/docsiq/internal/loader"
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
		urls, err = discoverSitemap(ctx, client, base)
		if err != nil {
			slog.Debug("🔍 sitemap not found, falling back to BFS", "url", rootURL, "reason", err)
		} else {
			slog.Info("🔍 sitemap discovered", "url", rootURL, "urls", len(urls))
		}
	}

	// Fall back to BFS
	if len(urls) == 0 {
		slog.Info("🌐 starting BFS crawl", "url", rootURL, "max_pages", opts.MaxPages, "max_depth", opts.MaxDepth)
		urls = bfsCrawl(ctx, client, base, opts)
		slog.Info("✅ BFS crawl complete", "url", rootURL, "pages_found", len(urls))
	}

	// Cap
	if opts.MaxPages > 0 && len(urls) > opts.MaxPages {
		slog.Debug("⏭️ capping URLs at max_pages", "total", len(urls), "max_pages", opts.MaxPages)
		urls = urls[:opts.MaxPages]
	}

	slog.Info("🌐 fetching pages", "count", len(urls), "concurrency", opts.Concurrency)

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
			if err != nil {
				slog.Debug("⚠️ failed to fetch page", "url", pageURL, "err", err)
			}
			pages[idx] = p
			errs[idx] = err
		}(i, u)
	}
	wg.Wait()

	// Collect successful fetches
	var result []*Page
	var fetchErrs int
	for i, p := range pages {
		if errs[i] == nil && p != nil && strings.TrimSpace(p.Content) != "" {
			result = append(result, p)
		} else if errs[i] != nil {
			fetchErrs++
		}
	}

	if fetchErrs > 0 {
		slog.Warn("⚠️ some pages failed to fetch", "failed", fetchErrs, "succeeded", len(result))
	}
	slog.Info("✅ crawl finished", "url", rootURL, "pages_fetched", len(result))
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

func discoverSitemap(ctx context.Context, client *http.Client, base *url.URL) ([]string, error) {
	candidates := []string{
		base.Scheme + "://" + base.Host + "/sitemap.xml",
		base.String() + "/sitemap.xml",
		base.String() + "sitemap.xml",
	}

	for _, candidate := range candidates {
		urls, err := parseSitemap(ctx, client, candidate, base)
		if err == nil && len(urls) > 0 {
			slog.Debug("🔍 sitemap parsed", "url", candidate, "entries", len(urls))
			return urls, nil
		}
	}
	return nil, fmt.Errorf("no sitemap found")
}

func parseSitemap(ctx context.Context, client *http.Client, sitemapURL string, base *url.URL) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sitemapURL, nil)
	if err != nil {
		return nil, fmt.Errorf("sitemap request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			_ = resp.Body.Close()
		}
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
		slog.Debug("🔍 sitemap index found", "url", sitemapURL, "sub_sitemaps", len(idx.Sitemaps))
		var all []string
		for _, s := range idx.Sitemaps {
			sub, err := parseSitemap(ctx, client, s.Loc, base)
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
			slog.Debug("🛑 BFS crawl cancelled by context", "pages_found", len(found))
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

		links := extractLinks(ctx, client, item.u, base)
		slog.Debug("🔗 BFS page links extracted", "url", item.u, "depth", item.depth, "links", len(links))
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

func extractLinks(ctx context.Context, client *http.Client, pageURL string, base *url.URL) []string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		slog.Debug("⚠️ failed to build request for link extraction", "url", pageURL, "err", err)
		return nil
	}
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if err != nil {
			slog.Debug("⚠️ failed to fetch page for link extraction", "url", pageURL, "err", err)
		}
		if resp != nil {
			_ = resp.Body.Close()
		}
		return nil
	}
	defer resp.Body.Close()

	doc, err := html.Parse(resp.Body)
	if err != nil {
		slog.Debug("⚠️ failed to parse HTML", "url", pageURL, "err", err)
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
	if strings.HasPrefix(href, "#") {
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
	// Reject any href with an explicit scheme other than http/https.
	// This allow-list covers mailto:, javascript:, data:, vbscript:,
	// tel:, file:, blob:, and anything else we don't crawl.
	if h.Scheme != "" {
		s := strings.ToLower(h.Scheme)
		if s != "http" && s != "https" {
			return ""
		}
	}
	resolved := b.ResolveReference(h)
	// Belt-and-braces: also reject if the base URL had a non-http(s)
	// scheme. The crawler only receives http(s) base URLs in production,
	// but fuzzing proved resolveURL itself must enforce the invariant.
	if rs := strings.ToLower(resolved.Scheme); rs != "http" && rs != "https" {
		return ""
	}
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

