package project

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// slugValid is the canonical charset a docsiq project slug is allowed to
// contain: lowercase ASCII alphanumerics, underscore, and hyphen.
var slugValid = regexp.MustCompile(`^[a-z0-9_-]+$`)

// nonSlug matches any rune that is NOT part of the slug charset; Slug()
// replaces every match with a single '-'.
var nonSlug = regexp.MustCompile(`[^a-z0-9_-]+`)

// collapseDashes turns runs of '-' into a single '-'. Applied after
// normalization so "git@host:owner/repo.git" doesn't produce double-dashes.
var collapseDashes = regexp.MustCompile(`-{2,}`)

// maxSlugLen caps the slug to a reasonable filesystem-friendly length.
// ext4/APFS tolerate longer names, but 200 is a conservative ceiling that
// stays well under 255-byte path component limits on all supported FSes.
const maxSlugLen = 200

// IsValidSlug returns true iff s matches the canonical slug charset and
// length bounds. Used by OpenForProject and registry consumers.
func IsValidSlug(s string) bool {
	if s == "" || len(s) > maxSlugLen {
		return false
	}
	// Reject NUL and any embedded whitespace — the regex would also catch
	// these, but this short-circuit makes the error path obvious.
	if strings.ContainsAny(s, "\x00 \t\n\r/\\") {
		return false
	}
	return slugValid.MatchString(s)
}

// Slug normalizes a git remote URL into a filesystem-safe, URL-safe slug.
//
// Supported input shapes:
//
//	https://github.com/owner/repo[.git]
//	http://host/owner/repo[.git]
//	git@github.com:owner/repo[.git]
//	ssh://git@host:22/owner/repo[.git]
//	ssh://user@host/path/to/repo.git
//	owner/repo  (already-path-ish)
//
// Rules applied in order:
//  1. Strip surrounding whitespace; reject empty.
//  2. Strip trailing ".git".
//  3. Drop the user@ portion and protocol (ssh://, https://, git://, file://).
//  4. SCP-style `git@host:owner/repo` → `host/owner/repo`.
//  5. URL-decode the path so %-escapes don't leak into the slug.
//  6. Lowercase.
//  7. Replace every non-[a-z0-9_-] rune with '-'.
//  8. Collapse runs of '-' and trim leading/trailing '-'.
//  9. Reject empty result; cap length.
//
// Returns an error if the remote is empty, produces an empty slug after
// normalization, or contains only non-slug characters.
func Slug(remote string) (string, error) {
	raw := strings.TrimSpace(remote)
	if raw == "" {
		return "", fmt.Errorf("slug: remote is empty")
	}

	// NUL byte in the middle of a git remote URL is pathological; refuse up
	// front rather than silently smuggling it through url.Parse.
	if strings.ContainsRune(raw, 0) {
		return "", fmt.Errorf("slug: remote contains NUL byte")
	}

	s := raw

	// Strip trailing ".git" (case-insensitive).
	if len(s) >= 4 && strings.EqualFold(s[len(s)-4:], ".git") {
		s = s[:len(s)-4]
	}

	// Handle SCP-style "user@host:path" (no slash before the colon).
	// Convert to "host/path" for downstream handling.
	if !strings.Contains(s, "://") {
		if at := strings.Index(s, "@"); at >= 0 {
			if colon := strings.Index(s[at+1:], ":"); colon >= 0 {
				host := s[at+1 : at+1+colon]
				path := s[at+1+colon+1:]
				s = host + "/" + path
			}
		}
	}

	// Strip known protocols.
	for _, scheme := range []string{"ssh://", "https://", "http://", "git://", "file://"} {
		if strings.HasPrefix(strings.ToLower(s), scheme) {
			s = s[len(scheme):]
			break
		}
	}

	// Drop embedded "user@" (after protocol strip).
	if at := strings.Index(s, "@"); at >= 0 {
		// Only strip if the segment before '@' looks like a user (no '/').
		if !strings.ContainsAny(s[:at], "/") {
			s = s[at+1:]
		}
	}

	// Drop ":port" if present between host and path.
	if colon := strings.Index(s, ":"); colon >= 0 {
		slashAfter := strings.Index(s[colon:], "/")
		if slashAfter > 0 {
			// Everything between ':' and the next '/' is the port — drop it.
			s = s[:colon] + s[colon+slashAfter:]
		}
	}

	// URL-decode any %-escapes in the path portion.
	if decoded, err := url.PathUnescape(s); err == nil {
		s = decoded
	}

	// Lowercase (Unicode-aware).
	s = strings.ToLower(s)

	// Replace every non-slug rune with '-'.
	s = nonSlug.ReplaceAllString(s, "-")

	// Collapse dash runs and trim.
	s = collapseDashes.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-_")

	if s == "" {
		return "", fmt.Errorf("slug: normalized to empty string for remote %q", remote)
	}

	// Length cap. Trim trailing '-' left over from a mid-byte cut.
	if len(s) > maxSlugLen {
		s = strings.TrimRight(s[:maxSlugLen], "-_")
	}

	if !slugValid.MatchString(s) {
		return "", fmt.Errorf("slug: produced invalid slug %q for remote %q", s, remote)
	}
	return s, nil
}
