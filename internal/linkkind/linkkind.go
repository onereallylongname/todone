// Package linkkind classifies a task's Links entry (task id, filesystem
// path, URL, or garbage) and normalizes path-like links to an absolute
// form. It has no TUI dependency, so both the TUI's 'o' keybinding and the
// CLI's `follow` command share exactly one classification implementation.
package linkkind

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/onereallylongname/todone/internal/store"
)

// Kind classifies a Links entry so callers can decide what to do with it.
type Kind int

const (
	Garbage Kind = iota // not an id, not an existing path, not a URL
	TaskID              // matches another task's id exactly
	Path                // resolves to an existing file or directory
	URL                 // looks like scheme://... (http, file, mailto, ...)
)

// urlSchemeRe matches a leading URI scheme ("https://"; "mailto:" doesn't
// have "//" so it's intentionally not matched — todone's links are
// expected to be browser/opener-style URLs, not every RFC 3986 scheme).
var urlSchemeRe = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.-]*://`)

// Classify decides what kind of thing link is, in the priority order the
// user asked for: an exact task-id match first, then an existing
// filesystem path, then a URL, else garbage. st may be nil (id matching
// then always misses).
func Classify(link string, st *store.Store) (Kind, string) {
	link = strings.TrimSpace(link)
	if link == "" {
		return Garbage, link
	}
	if st != nil && st.Index(link) >= 0 {
		return TaskID, link
	}
	if p := ResolvePath(link); p != "" {
		return Path, p
	}
	if urlSchemeRe.MatchString(link) {
		return URL, link
	}
	return Garbage, link
}

// ResolvePath returns the existing filesystem path link resolves to
// (expanding a leading ~, and falling back to resolving relative to the
// current working directory for links that are still relative — e.g.
// hand-edited YAML), or "" if nothing exists at that path. Always returns
// an absolute path on success.
func ResolvePath(link string) string {
	p := ExpandHome(link)
	if filepath.IsAbs(p) {
		if _, err := os.Stat(p); err == nil {
			return p
		}
		return ""
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return ""
	}
	if _, err := os.Stat(abs); err == nil {
		return abs
	}
	return ""
}

// ExpandHome expands a leading "~" or "~/..." to the user's home
// directory; anything else passes through unchanged.
func ExpandHome(p string) string {
	if p == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
		return p
	}
	if strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, p[2:])
		}
	}
	return p
}

// Normalize absolutizes a path-like link at the moment it's typed in
// (add/edit), so a relative path resolved against today's cwd doesn't
// silently break if todone is later launched from somewhere else. It only
// rewrites strings that currently resolve to a real file or directory;
// URLs, task ids, garbage, and not-yet-existing paths pass through
// completely unchanged.
func Normalize(s string) string {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" || urlSchemeRe.MatchString(trimmed) {
		return s
	}
	expanded := ExpandHome(trimmed)
	abs, err := filepath.Abs(expanded)
	if err != nil {
		return s
	}
	if _, err := os.Stat(abs); err != nil {
		return s
	}
	return abs
}
