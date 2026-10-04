package search

import (
	"sort"
	"strings"

	"github.com/onereallylongname/todone/internal/store"
)

// SortMode controls the order used when FreeText isn't driving a fuzzy
// ranking (an active fuzzy search always wins — it has its own relevance
// order).
type SortMode int

const (
	SortUpdated SortMode = iota // most recently touched first (default)
	SortDate                    // most recently created first
	SortAlpha                   // summary, A-Z
)

// ParseSortMode maps a :sort command argument / config value to a SortMode.
// Defaults to SortUpdated for anything unrecognized.
func ParseSortMode(s string) SortMode {
	switch strings.ToLower(s) {
	case "date", "added", "created":
		return SortDate
	case "alpha", "alphabetical", "name":
		return SortAlpha
	default:
		return SortUpdated
	}
}

func (m SortMode) String() string {
	switch m {
	case SortDate:
		return "date"
	case SortAlpha:
		return "alpha"
	default:
		return "updated"
	}
}

// Next cycles updated -> date -> alpha -> updated, used by the `s` key.
func (m SortMode) Next() SortMode {
	return (m + 1) % 3
}

// SortIndexes reorders idx (indexes into tasks) in place according to mode.
// Stable, so equal keys keep their relative order.
func SortIndexes(tasks []store.Task, idx []int, mode SortMode) {
	switch mode {
	case SortDate:
		sort.SliceStable(idx, func(a, b int) bool {
			return tasks[idx[a]].CreatedAt().After(tasks[idx[b]].CreatedAt())
		})
	case SortAlpha:
		sort.SliceStable(idx, func(a, b int) bool {
			return strings.ToLower(tasks[idx[a]].Summary) < strings.ToLower(tasks[idx[b]].Summary)
		})
	default: // SortUpdated
		sort.SliceStable(idx, func(a, b int) bool {
			return tasks[idx[a]].SortTime().After(tasks[idx[b]].SortTime())
		})
	}
}
