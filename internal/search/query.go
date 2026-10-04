// Package search implements the `/` filter query language: space-separated
// tokens that are either a hard filter prefix (t:/#tag for tag, @link for
// link, d:/s: for done-state) or free text, which is fuzzy-matched against
// the task summary. A bare `|` token splits free text into OR'd
// alternatives (e.g. "int | work #now" matches tasks tagged #now whose
// summary fuzzy-matches "int" OR "work"); hard filters (tags/links/done-
// state) always apply as AND, regardless of which side of a `|` they
// appear on. A leading sigil (#, |, @) can be escaped with a backslash
// (\#, \|, \@) to search for it as literal free text instead. Directly
// inspired by avredit's n:/t:/ns: search prefixes.
package search

import (
	"sort"
	"strings"

	"github.com/onereallylongname/todone/internal/store"
	"github.com/sahilm/fuzzy"
)

// Query is a parsed `/` filter.
type Query struct {
	Tags           []string // substrings that must appear in some tag (AND'ed)
	Links          []string // substrings that must appear in some link (AND'ed)
	Done           *bool    // nil = any state
	FreeTextGroups []string // OR'd fuzzy alternatives; each is one "|"-separated phrase, fuzzy-matched against the summary
}

// Parse splits raw input into hard-filter tokens and free-text OR groups.
func Parse(raw string) Query {
	var q Query
	var free []string
	var groups []string

	for _, tok := range strings.Fields(raw) {
		if lit, escaped := unescapeSigil(tok); escaped {
			free = append(free, lit)
			continue
		}
		switch {
		case tok == "|":
			groups = appendGroup(groups, free)
			free = nil
		case strings.HasPrefix(tok, "#") && len(tok) > 1:
			q.Tags = append(q.Tags, tok[1:])
		case strings.HasPrefix(tok, "@") && len(tok) > 1:
			q.Links = append(q.Links, tok[1:])
		case strings.HasPrefix(tok, "t:") && len(tok) > 2:
			q.Tags = append(q.Tags, tok[2:])
		case strings.HasPrefix(tok, "d:") && len(tok) > 2:
			setDone(&q, tok[2:])
		case strings.HasPrefix(tok, "s:") && len(tok) > 2:
			setDone(&q, tok[2:])
		default:
			free = append(free, tok)
		}
	}
	q.FreeTextGroups = appendGroup(groups, free)
	return q
}

// unescapeSigil reports whether tok starts with a backslash-escaped sigil
// (\#, \|, or \@) and, if so, returns the literal token with the backslash
// stripped — so e.g. "\#1" is searched as literal free text "#1" instead
// of being parsed as a tag filter, and "\|" is a literal pipe character
// instead of the OR-group separator. Any other use of "\" passes through
// unchanged (no support yet for escaping a literal backslash itself).
func unescapeSigil(tok string) (string, bool) {
	if len(tok) >= 2 && tok[0] == '\\' {
		switch tok[1] {
		case '#', '|', '@':
			return tok[1:], true
		}
	}
	return tok, false
}

// appendGroup joins words into one OR-branch and appends it to groups,
// skipping empty branches (e.g. a leading/trailing/doubled "|") so they
// don't silently become an always-match wildcard.
func appendGroup(groups []string, words []string) []string {
	if len(words) == 0 {
		return groups
	}
	return append(groups, strings.Join(words, " "))
}

func setDone(q *Query, val string) {
	switch strings.ToLower(val) {
	case "done", "true", "yes", "y":
		v := true
		q.Done = &v
	case "pending", "false", "no", "n", "todo", "open":
		v := false
		q.Done = &v
	}
}

// HasFreeText reports whether the query has any free-text (fuzzy) filter —
// a single phrase, or several "|"-separated OR alternatives.
func (q Query) HasFreeText() bool {
	return len(q.FreeTextGroups) > 0
}

// Empty reports whether the query has no effect (would match everything,
// in original order).
func (q Query) Empty() bool {
	return len(q.Tags) == 0 && len(q.Links) == 0 && q.Done == nil && !q.HasFreeText()
}

func (q Query) hardMatch(t store.Task) bool {
	if q.Done != nil && t.Done != *q.Done {
		return false
	}
	for _, tag := range q.Tags {
		if !t.HasTag(tag) {
			return false
		}
	}
	for _, link := range q.Links {
		if !t.HasLink(link) {
			return false
		}
	}
	return true
}

// Apply filters tasks and returns the matching indexes (into tasks), in
// display order: fuzzy-ranked by FreeTextGroups if present (a task
// matching several OR groups is ranked by its best-scoring group),
// otherwise in the original slice order.
func Apply(tasks []store.Task, q Query) []int {
	candidates := make([]int, 0, len(tasks))
	for i, t := range tasks {
		if q.hardMatch(t) {
			candidates = append(candidates, i)
		}
	}

	if !q.HasFreeText() {
		return candidates
	}

	summaries := make([]string, len(candidates))
	for i, idx := range candidates {
		summaries[i] = tasks[idx].Summary
	}

	// Each group is an independent fuzzy pattern; a task matches if it
	// matches ANY group ("OR"), ranked by whichever group scores it best.
	bestScore := make(map[int]int) // index into candidates -> best fuzzy score
	for _, group := range q.FreeTextGroups {
		for _, m := range fuzzy.Find(group, summaries) {
			if score, ok := bestScore[m.Index]; !ok || m.Score > score {
				bestScore[m.Index] = m.Score
			}
		}
	}

	matched := make([]int, 0, len(bestScore))
	for ci := range bestScore {
		matched = append(matched, ci)
	}
	sort.Slice(matched, func(i, j int) bool { return bestScore[matched[i]] > bestScore[matched[j]] })

	out := make([]int, len(matched))
	for i, ci := range matched {
		out[i] = candidates[ci]
	}
	return out
}
