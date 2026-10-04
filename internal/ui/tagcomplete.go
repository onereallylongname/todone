package ui

import (
	"strings"

	"github.com/onereallylongname/todone/internal/store"
)

// completeTag runs Tab-completion against every known tag (see
// store.UniqueSortedTags) for the editor at ed, mirroring completeWord's
// longest-common-prefix-then-cycle behavior (see input_modes.go) but keyed
// off the tagCompletion* state and sourced from the live task list instead
// of a fixed command list.
//
// In "whole-value" fields (the detail view's Tags add/edit prompt) the
// entire current value is treated as the partial tag. In free-text fields
// (quick-add, search) only the last whitespace-delimited token is
// completed, and only if it's tag-shaped ("#foo" or "t:foo") — Tab on any
// other word is a no-op, leaving plain summary text/links/ids untouched.
func (a *App) completeTag(ed *Editor, wholeValue bool) {
	candidates := store.UniqueSortedTags(a.st.Tasks)
	if len(candidates) == 0 {
		return
	}

	current := ed.Value()
	var linePrefix, tokenPrefix, partial string
	if wholeValue {
		partial = current
	} else {
		word := current
		if i := strings.LastIndexByte(current, ' '); i >= 0 {
			linePrefix = current[:i+1]
			word = current[i+1:]
		}
		switch {
		case strings.HasPrefix(word, "#"):
			tokenPrefix = "#"
			partial = word[1:]
		case strings.HasPrefix(word, "t:"):
			tokenPrefix = "t:"
			partial = word[2:]
		default:
			return
		}
	}
	prefix := linePrefix + tokenPrefix

	if a.tagCompletionActive && len(a.tagCompletions) > 0 {
		a.tagCompletionIdx++
		if a.tagCompletionIdx >= len(a.tagCompletions) {
			a.tagCompletionIdx = 0
		}
		*ed = NewEditor(a.tagCompletionPrefix+a.tagCompletions[a.tagCompletionIdx], editorStyle)
		return
	}

	var matches []string
	for _, c := range candidates {
		if strings.HasPrefix(c, partial) {
			matches = append(matches, c)
		}
	}
	if len(matches) == 0 {
		return
	}
	if len(matches) == 1 {
		*ed = NewEditor(prefix+matches[0], editorStyle)
		a.tagCompletionActive = false
		a.tagCompletions = nil
		return
	}

	common := matches[0]
	for _, m := range matches[1:] {
		for !strings.HasPrefix(m, common) {
			common = common[:len(common)-1]
		}
	}
	a.tagCompletions = matches
	a.tagCompletionPrefix = prefix
	a.tagCompletionActive = true
	if len(common) > len(partial) {
		a.tagCompletionIdx = -1
		*ed = NewEditor(prefix+common, editorStyle)
	} else {
		a.tagCompletionIdx = 0
		*ed = NewEditor(prefix+matches[0], editorStyle)
	}
}
