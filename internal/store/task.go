// Package store manages the on-disk task list: a flat YAML file with the
// same record shape as the legacy todo.nu Nushell module (id, done, summary,
// date, updated, tags, links), so a file can be shared between the two tools
// if the user ever points todone at it.
package store

import (
	"sort"
	"time"
)

// dateLayouts are tried in order when parsing a date/updated field read from
// disk. RFC3339 is what todone itself writes; the second layout matches
// todo.nu's Nushell-formatted timestamps ("%Y-%m-%d %H:%M:%S%.f %:z"), so a
// file written by the legacy script can still be opened.
var dateLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02 15:04:05.999999999 -07:00",
	"2006-01-02 15:04:05 -07:00",
}

// Task is a single to-do item.
type Task struct {
	ID      string   `yaml:"id"`
	Done    bool     `yaml:"done"`
	Summary string   `yaml:"summary"`
	Date    string   `yaml:"date"`
	Updated string   `yaml:"updated"`
	Tags    []string `yaml:"tags"`
	Links   []string `yaml:"links"`
}

// CreatedAt parses Date, trying every known layout. Returns the zero time if
// the field is empty or unparseable.
func (t Task) CreatedAt() time.Time {
	return parseDate(t.Date)
}

// UpdatedAt parses Updated, trying every known layout. Returns the zero time
// if the field is empty (never updated) or unparseable.
func (t Task) UpdatedAt() time.Time {
	return parseDate(t.Updated)
}

// SortTime returns UpdatedAt if set, otherwise CreatedAt — "most recently
// touched" for the default sort order.
func (t Task) SortTime() time.Time {
	if u := t.UpdatedAt(); !u.IsZero() {
		return u
	}
	return t.CreatedAt()
}

func parseDate(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	for _, layout := range dateLayouts {
		if tm, err := time.Parse(layout, s); err == nil {
			return tm
		}
	}
	return time.Time{}
}

// HasTag reports whether the task has a tag containing needle
// (case-insensitive substring match).
func (t Task) HasTag(needle string) bool {
	for _, tag := range t.Tags {
		if containsFold(tag, needle) {
			return true
		}
	}
	return false
}

// HasLink reports whether the task has a link containing needle
// (case-insensitive substring match), mirroring HasTag — used by the `@`
// search prefix.
func (t Task) HasLink(needle string) bool {
	for _, link := range t.Links {
		if containsFold(link, needle) {
			return true
		}
	}
	return false
}

// UniqueSortedTags returns every distinct tag used across tasks, sorted
// alphabetically. This is the single source of truth for "what tags
// exist" — shared by the CLI `tags` command, the TUI `:tags` overlay, and
// tag Tab-completion, so all three never drift out of sync.
func UniqueSortedTags(tasks []Task) []string {
	seen := map[string]bool{}
	var tags []string
	for _, t := range tasks {
		for _, tag := range t.Tags {
			if !seen[tag] {
				seen[tag] = true
				tags = append(tags, tag)
			}
		}
	}
	sort.Strings(tags)
	return tags
}
