package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/onereallylongname/todone/internal/clipboard"
	"github.com/onereallylongname/todone/internal/search"
	"github.com/onereallylongname/todone/internal/store"
)

// --- Insert mode (add / edit a single value or list item) ------------------

func (a App) updateInsert(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// Any key other than Tab ends a tag-completion cycling sequence (same
	// convention as command-mode completion — see updateCommand).
	if msg.String() != "tab" {
		a.tagCompletionActive = false
	}
	switch msg.String() {
	case "enter":
		return a.commitInsert()
	case "esc":
		a.mode = ModeNormal
		a.insertTarget = insertNone
		return a, nil
	case "ctrl+v", "shift+insert":
		return a.pasteFromNativeClipboard()
	case "tab":
		// Tags-list add/edit treats the whole field as one tag; quick-add
		// free text only completes a trailing #tag token. Every other
		// field (summary, links) has no tag concept, so Tab is a no-op.
		switch a.insertTarget {
		case insertAddTagItem, insertEditTagItem:
			a.completeTag(&a.editor, true)
		case insertAdd:
			a.completeTag(&a.editor, false)
		}
		return a, nil
	default:
		a.editor.HandleKey(msg)
		return a, nil
	}
}

// pasteFromNativeClipboard is the explicit "paste" fallback (ctrl+v or
// shift+insert) for terminals/sessions where bracketed-paste doesn't reach
// the app (e.g. no tea.PasteMsg fires). Silently no-ops if no native
// clipboard tool is available — this is a best-effort convenience, not the
// primary path.
func (a App) pasteFromNativeClipboard() (tea.Model, tea.Cmd) {
	text, err := clipboard.ReadNative()
	if err != nil {
		return a, nil
	}
	a.pasteText(text)
	return a, nil
}

func (a App) commitInsert() (tea.Model, tea.Cmd) {
	text := a.editor.Value()
	target := a.insertTarget
	itemIdx := a.editItem
	a.mode = ModeNormal
	a.insertTarget = insertNone

	switch target {
	case insertAdd:
		summary, tags, links := parseAddInput(text)
		if summary == "" {
			a.setErr("empty summary, not added")
			return a, nil
		}
		t := store.Task{
			ID:      store.NewID(),
			Summary: summary,
			Date:    time.Now().Format(time.RFC3339),
			Tags:    tags,
			Links:   links,
		}
		newIdx := len(a.st.Tasks)
		st := a.st
		a.history.Execute(Command{
			Desc: fmt.Sprintf("added %q", truncate(summary, 30)),
			Do:   func() { st.Insert(newIdx, t) },
			Undo: func() { st.Delete(newIdx) },
			Idx:  newIdx,
		})
		a.refilter()
		if i := indexOf(a.filtered, newIdx); i >= 0 {
			a.cursor = i
		}
		a.save()
		a.setFlash("added")

	case insertEditSummary:
		idx := a.selectedIdx()
		if idx < 0 {
			return a, nil
		}
		if strings.TrimSpace(text) == "" {
			a.setErr("summary can't be empty")
			return a, nil
		}
		prev := a.st.Tasks[idx]
		next := prev
		next.Summary = text
		next.Updated = time.Now().Format(time.RFC3339)
		st := a.st
		a.history.Execute(Command{
			Desc: "edited summary",
			Do:   func() { st.Set(idx, next) },
			Undo: func() { st.Set(idx, prev) },
			Idx:  idx,
		})
		a.refilter()
		a.followIdx(idx)
		a.save()
		a.setFlash("saved")

	case insertEditTagItem, insertAddTagItem, insertEditLinkItem, insertAddLinkItem:
		return a.commitItemEdit(target, itemIdx, text)
	}
	return a, nil
}

// commitItemEdit applies an add/edit of one Tags or Links item. Every case
// builds a brand-new slice (never mutates in place) so prev/next Task
// values stay independent snapshots for undo/redo.
func (a App) commitItemEdit(target insertTarget, itemIdx int, text string) (tea.Model, tea.Cmd) {
	idx := a.selectedIdx()
	if idx < 0 {
		return a, nil
	}
	text = strings.TrimSpace(text)
	if text == "" {
		a.setErr("empty value, not saved")
		return a, nil
	}
	prev := a.st.Tasks[idx]
	next := prev
	var desc string
	switch target {
	case insertEditTagItem:
		if itemIdx < 0 || itemIdx >= len(prev.Tags) {
			return a, nil
		}
		newTags := append([]string{}, prev.Tags...)
		newTags[itemIdx] = text
		next.Tags = newTags
		desc = fmt.Sprintf("edited tag %q", text)
	case insertAddTagItem:
		next.Tags = append(append([]string{}, prev.Tags...), text)
		desc = fmt.Sprintf("added tag %q", text)
	case insertEditLinkItem:
		if itemIdx < 0 || itemIdx >= len(prev.Links) {
			return a, nil
		}
		text = normalizeLink(text)
		newLinks := append([]string{}, prev.Links...)
		newLinks[itemIdx] = text
		next.Links = newLinks
		desc = fmt.Sprintf("edited link %q", truncate(text, 30))
	case insertAddLinkItem:
		text = normalizeLink(text)
		next.Links = append(append([]string{}, prev.Links...), text)
		desc = fmt.Sprintf("added link %q", truncate(text, 30))
	}
	next.Updated = time.Now().Format(time.RFC3339)
	st := a.st
	a.history.Execute(Command{
		Desc: desc,
		Do:   func() { st.Set(idx, next) },
		Undo: func() { st.Set(idx, prev) },
		Idx:  idx,
	})
	a.refilter()
	a.followIdx(idx)
	a.save()

	switch target {
	case insertAddTagItem:
		a.detailRow = tagItemRow(len(next.Tags) - 1)
	case insertAddLinkItem:
		a.detailRow = linkItemRow(next, len(next.Links)-1)
	}
	a.clampDetailRow(next)
	a.setFlash(desc)
	return a, nil
}

// tagItemRow/linkItemRow compute a row index directly (matching
// detailRows' fixed construction order: Summary, Tags items, +add tag,
// Links items, +add link) so callers can land the cursor on a
// just-added/edited item without re-scanning the whole row list.
func tagItemRow(item int) int { return 1 + item }
func linkItemRow(t store.Task, item int) int {
	return 1 + len(t.Tags) + 1 + item
}

// parseAddInput splits "Buy milk #errand @./notes.md https://x.test" into
// summary="Buy milk", tags=["errand"], links=["./notes.md","https://x.test"]
// — a quick-capture shortcut so adding a tagged/linked task doesn't need a
// second prompt. A @token is any kind of link (URL, filesystem path, or
// another task's id) stored raw — classification happens lazily when it's
// opened (see classifyLink). A word can escape its leading sigil with a
// backslash (\#, \|, \@) to be treated as plain summary text instead.
// Mirrors cmd/todone's parseAddText.
func parseAddInput(text string) (summary string, tags []string, links []string) {
	var words []string
	for _, w := range strings.Fields(text) {
		if lit, escaped := unescapeSigil(w); escaped {
			words = append(words, lit)
			continue
		}
		switch {
		case strings.HasPrefix(w, "#") && len(w) > 1:
			tags = append(tags, w[1:])
		case strings.HasPrefix(w, "@") && len(w) > 1:
			links = append(links, w[1:])
		case strings.HasPrefix(w, "http://") || strings.HasPrefix(w, "https://"):
			links = append(links, w)
		default:
			words = append(words, w)
		}
	}
	return strings.Join(words, " "), tags, links
}

// unescapeSigil reports whether word starts with a backslash-escaped sigil
// (\#, \|, or \@) and, if so, returns the literal word with the backslash
// stripped — so e.g. "\#1" is treated as plain text "#1" instead of a tag.
// Any other use of "\" passes through unchanged (no support yet for
// escaping a literal backslash itself). Mirrors cmd/todone's unescapeSigil.
func unescapeSigil(word string) (string, bool) {
	if len(word) >= 2 && word[0] == '\\' {
		switch word[1] {
		case '#', '|', '@':
			return word[1:], true
		}
	}
	return word, false
}

func indexOf(s []int, v int) int {
	for i, x := range s {
		if x == v {
			return i
		}
	}
	return -1
}

// --- Search mode -------------------------------------------------------------

func (a App) updateSearch(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// Any key other than Tab ends a tag-completion cycling sequence (same
	// convention as command-mode completion — see updateCommand).
	if msg.String() != "tab" {
		a.tagCompletionActive = false
	}
	switch msg.String() {
	case "enter":
		a.mode = ModeNormal
		a.commitSearchHistory()
		return a, nil
	case "esc":
		a.mode = ModeNormal
		a.queryRaw = ""
		a.query = search.Query{}
		a.searchHistIdx = -1
		a.refilter()
		return a, nil
	case "up":
		a.recallSearchHistory(-1)
		return a, nil
	case "down":
		a.recallSearchHistory(1)
		return a, nil
	case "ctrl+v", "shift+insert":
		return a.pasteFromNativeClipboard()
	case "tab":
		a.completeTag(&a.searchEditor, false)
		a.queryRaw = a.searchEditor.Value()
		a.query = search.Parse(a.queryRaw)
		a.searchHistIdx = -1
		a.refilter()
		return a, nil
	default:
		a.searchEditor.HandleKey(msg)
		a.queryRaw = a.searchEditor.Value()
		a.query = search.Parse(a.queryRaw)
		a.searchHistIdx = -1
		a.refilter()
		return a, nil
	}
}

// commitSearchHistory records the just-entered query for future Up/Down
// recall, skipping empty queries and immediate duplicates.
func (a *App) commitSearchHistory() {
	q := a.queryRaw
	if q == "" {
		return
	}
	if len(a.searchHist) == 0 || a.searchHist[len(a.searchHist)-1] != q {
		a.searchHist = append(a.searchHist, q)
	}
	a.searchHistIdx = -1
}

// recallSearchHistory moves through searchHist by delta (-1 = older/Up,
// +1 = newer/Down), ported from avredit's search-bar history browsing.
func (a *App) recallSearchHistory(delta int) {
	if len(a.searchHist) == 0 {
		return
	}
	if delta < 0 {
		if a.searchHistIdx < 0 {
			a.searchHistIdx = len(a.searchHist) - 1
		} else if a.searchHistIdx > 0 {
			a.searchHistIdx--
		}
		a.searchEditor = NewEditor(a.searchHist[a.searchHistIdx], editorStyle)
	} else {
		if a.searchHistIdx < 0 {
			return
		}
		if a.searchHistIdx < len(a.searchHist)-1 {
			a.searchHistIdx++
			a.searchEditor = NewEditor(a.searchHist[a.searchHistIdx], editorStyle)
		} else {
			a.searchHistIdx = -1
			a.searchEditor = NewEditor("", editorStyle)
		}
	}
	a.queryRaw = a.searchEditor.Value()
	a.query = search.Parse(a.queryRaw)
	a.refilter()
}

// --- Command mode -------------------------------------------------------------

// knownCommands/knownSortArgs drive Tab completion in command mode.
var knownCommands = []string{"w", "q", "q!", "wq", "sort", "all", "tags"}
var knownSortArgs = []string{"date", "updated", "alpha"}

func (a App) updateCommand(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// Any key other than Tab ends a completion-cycling sequence, so the
	// next Tab press starts a fresh match instead of advancing the cycle
	// (ported from avredit's completeCommand/completeWord behavior).
	if msg.String() != "tab" {
		a.cmdCompletionActive = false
	}
	switch msg.String() {
	case "enter":
		raw := a.cmdEditor.Value()
		a.mode = ModeNormal
		a.commitCmdHistory(raw)
		return a.executeCommand(raw)
	case "esc":
		a.mode = ModeNormal
		a.cmdHistIdx = -1
		return a, nil
	case "tab":
		a.completeCommand()
		return a, nil
	case "up":
		a.recallCmdHistory(-1)
		return a, nil
	case "down":
		a.recallCmdHistory(1)
		return a, nil
	case "ctrl+v", "shift+insert":
		return a.pasteFromNativeClipboard()
	default:
		a.cmdEditor.HandleKey(msg)
		a.cmdHistIdx = -1
		return a, nil
	}
}

func (a *App) commitCmdHistory(cmd string) {
	if cmd == "" {
		return
	}
	if len(a.cmdHist) == 0 || a.cmdHist[len(a.cmdHist)-1] != cmd {
		a.cmdHist = append(a.cmdHist, cmd)
	}
	a.cmdHistIdx = -1
}

func (a *App) recallCmdHistory(delta int) {
	if len(a.cmdHist) == 0 {
		return
	}
	if delta < 0 {
		if a.cmdHistIdx < 0 {
			a.cmdHistIdx = len(a.cmdHist) - 1
		} else if a.cmdHistIdx > 0 {
			a.cmdHistIdx--
		}
		a.cmdEditor = NewEditor(a.cmdHist[a.cmdHistIdx], editorStyle)
	} else {
		if a.cmdHistIdx < 0 {
			return
		}
		if a.cmdHistIdx < len(a.cmdHist)-1 {
			a.cmdHistIdx++
			a.cmdEditor = NewEditor(a.cmdHist[a.cmdHistIdx], editorStyle)
		} else {
			a.cmdHistIdx = -1
			a.cmdEditor = NewEditor("", editorStyle)
		}
	}
}

// completeCommand dispatches to completeWord with the right candidate set
// depending on how many words are typed so far — command name first, then
// (for `sort`) its argument. Ported from avredit's completeCommand.
func (a *App) completeCommand() {
	input := a.cmdEditor.Value()
	fields := strings.Fields(input)
	endsWithSpace := strings.HasSuffix(input, " ")

	if len(fields) == 0 {
		return
	}
	if len(fields) == 1 && !endsWithSpace {
		a.completeWord(input, knownCommands)
		return
	}
	if fields[0] == "sort" && ((len(fields) == 1 && endsWithSpace) || (len(fields) == 2 && !endsWithSpace)) {
		a.completeWord(input, knownSortArgs)
	}
}

// completeWord completes the final whitespace-delimited word of input
// against candidates: a single match completes fully; multiple matches
// fill in their longest common prefix on the first Tab and cycle through
// each candidate on repeated presses. Ported near-verbatim from avredit's
// completeWord.
func (a *App) completeWord(input string, candidates []string) {
	if a.cmdCompletionActive && len(a.cmdCompletions) > 0 {
		a.cmdCompletionIdx++
		if a.cmdCompletionIdx >= len(a.cmdCompletions) {
			a.cmdCompletionIdx = 0
		}
		a.cmdEditor = NewEditor(a.cmdCompletionPrefix+a.cmdCompletions[a.cmdCompletionIdx], editorStyle)
		return
	}

	prefix := ""
	partial := input
	if i := strings.LastIndex(input, " "); i >= 0 {
		prefix = input[:i+1]
		partial = input[i+1:]
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
	sort.Strings(matches)
	if len(matches) == 1 {
		a.cmdEditor = NewEditor(prefix+matches[0], editorStyle)
		a.cmdCompletionActive = false
		a.cmdCompletions = nil
		return
	}

	common := matches[0]
	for _, m := range matches[1:] {
		for !strings.HasPrefix(m, common) {
			common = common[:len(common)-1]
		}
	}
	a.cmdCompletions = matches
	a.cmdCompletionPrefix = prefix
	a.cmdCompletionActive = true
	if len(common) > len(partial) {
		a.cmdCompletionIdx = -1
		a.cmdEditor = NewEditor(prefix+common, editorStyle)
	} else {
		a.cmdCompletionIdx = 0
		a.cmdEditor = NewEditor(prefix+matches[0], editorStyle)
	}
}

func (a App) executeCommand(raw string) (tea.Model, tea.Cmd) {
	fields := strings.Fields(strings.TrimSpace(raw))
	if len(fields) == 0 {
		return a, nil
	}
	switch fields[0] {
	case "w":
		a.save()
		if !a.flashIsErr {
			a.setFlash("saved")
		}
	case "q", "q!":
		return a.quit()
	case "wq":
		a.save()
		return a.quit()
	case "sort":
		if len(fields) > 1 {
			a.sortMode = search.ParseSortMode(fields[1])
			a.refilter()
			a.setFlash("sort: " + a.sortMode.String())
		}
	case "all":
		a.showDone = !a.showDone
		a.refilter()
	case "tags":
		a.tagsOpen = true
		a.tagsScroll = 0
	default:
		a.setErr("unknown command: " + fields[0])
	}
	return a, nil
}

// --- Detail view (normal sub-mode) -------------------------------------------

func (a App) updateDetailNormal(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		idx := a.selectedIdx()
		if idx < 0 {
			return a, nil
		}
		if n := len(detailRows(a.st.Tasks[idx])); a.detailRow < n-1 {
			a.detailRow++
		}
		return a, nil
	case "k", "up":
		if a.detailRow > 0 {
			a.detailRow--
		}
		return a, nil
	case "H":
		a.gotoAdjacentTask(-1)
		return a, nil
	case "L":
		a.gotoAdjacentTask(1)
		return a, nil
	case "J":
		return a.moveRowItem(1)
	case "K":
		return a.moveRowItem(-1)
	case "e", "enter":
		return a.beginRowEdit()
	case "a":
		return a.beginRowAdd()
	case "d":
		return a.deleteRowItem()
	case "y":
		return a.yankRow()
	case "o":
		return a.openDetailLink()
	case "ctrl+o":
		return a.jumpBackward()
	case "ctrl+i":
		return a.jumpForward()
	case "u":
		if desc, idx, ok := a.history.Undo(); ok {
			a.refilter()
			a.followIdx(idx)
			a.save()
			a.setFlash("undid: " + desc)
			if sel := a.selectedIdx(); sel >= 0 {
				a.clampDetailRow(a.st.Tasks[sel])
			}
		} else {
			a.setErr("nothing to undo")
		}
		return a, nil
	case "ctrl+r":
		if desc, idx, ok := a.history.Redo(); ok {
			a.refilter()
			a.followIdx(idx)
			a.save()
			a.setFlash("redid: " + desc)
			if sel := a.selectedIdx(); sel >= 0 {
				a.clampDetailRow(a.st.Tasks[sel])
			}
		} else {
			a.setErr("nothing to redo")
		}
		return a, nil
	case "esc", "q":
		a.detailOpen = false
		return a, nil
	case "?":
		a.helpOpen = true
		a.helpScroll = 0
		return a, nil
	}
	return a, nil
}

// gotoAdjacentTask moves the list cursor by delta within a.filtered (the
// current filter+sort order) and opens the same task's detail view fresh,
// so "next/prev task" from the detail view respects whatever you were
// just browsing.
func (a *App) gotoAdjacentTask(delta int) {
	if len(a.filtered) == 0 {
		return
	}
	next := a.cursor + delta
	if next < 0 || next >= len(a.filtered) {
		a.setErr("no more tasks")
		return
	}
	a.cursor = next
	a.detailRow = 0
}

func (a App) beginRowEdit() (tea.Model, tea.Cmd) {
	idx := a.selectedIdx()
	if idx < 0 {
		return a, nil
	}
	t := a.st.Tasks[idx]
	rows := detailRows(t)
	if a.detailRow >= len(rows) {
		return a, nil
	}
	row := rows[a.detailRow]
	var initial string
	switch row.Kind {
	case rowSummary:
		a.insertTarget = insertEditSummary
		initial = t.Summary
	case rowTagItem:
		a.insertTarget = insertEditTagItem
		a.editItem = row.Item
		initial = t.Tags[row.Item]
	case rowTagAdd:
		a.insertTarget = insertAddTagItem
	case rowLinkItem:
		a.insertTarget = insertEditLinkItem
		a.editItem = row.Item
		initial = t.Links[row.Item]
	case rowLinkAdd:
		a.insertTarget = insertAddLinkItem
	}
	a.mode = ModeInsert
	a.editor = NewEditor(initial, editorStyle)
	return a, nil
}

// beginRowAdd handles the 'a' key in detail view: add a new item to
// whichever section (Tags or Links) the cursor is currently in.
func (a App) beginRowAdd() (tea.Model, tea.Cmd) {
	idx := a.selectedIdx()
	if idx < 0 {
		return a, nil
	}
	t := a.st.Tasks[idx]
	rows := detailRows(t)
	if a.detailRow >= len(rows) {
		return a, nil
	}
	switch rows[a.detailRow].Kind {
	case rowTagItem, rowTagAdd:
		a.insertTarget = insertAddTagItem
	case rowLinkItem, rowLinkAdd:
		a.insertTarget = insertAddLinkItem
	default:
		a.setErr("move to Tags or Links to add an item")
		return a, nil
	}
	a.mode = ModeInsert
	a.editor = NewEditor("", editorStyle)
	return a, nil
}

// deleteRowItem removes the selected Tags/Links item instantly (no
// confirm — undo is the safety net here, per the user's choice).
func (a App) deleteRowItem() (tea.Model, tea.Cmd) {
	idx := a.selectedIdx()
	if idx < 0 {
		return a, nil
	}
	t := a.st.Tasks[idx]
	rows := detailRows(t)
	if a.detailRow >= len(rows) {
		return a, nil
	}
	row := rows[a.detailRow]

	prev := t
	next := t
	var desc string
	switch row.Kind {
	case rowTagItem:
		removed := t.Tags[row.Item]
		next.Tags = append(append([]string{}, t.Tags[:row.Item]...), t.Tags[row.Item+1:]...)
		desc = fmt.Sprintf("removed tag %q", removed)
	case rowLinkItem:
		removed := t.Links[row.Item]
		next.Links = append(append([]string{}, t.Links[:row.Item]...), t.Links[row.Item+1:]...)
		desc = fmt.Sprintf("removed link %q", truncate(removed, 30))
	default:
		a.setErr("nothing to delete here")
		return a, nil
	}
	next.Updated = time.Now().Format(time.RFC3339)
	st := a.st
	a.history.Execute(Command{
		Desc: desc,
		Do:   func() { st.Set(idx, next) },
		Undo: func() { st.Set(idx, prev) },
		Idx:  idx,
	})
	a.refilter()
	a.followIdx(idx)
	a.save()
	a.clampDetailRow(next)
	a.setFlash(desc)
	return a, nil
}

// moveRowItem swaps the selected Tags/Links item with its neighbor at
// delta (+1/-1), reordering within that list. This is the mechanism for
// "pick a default link": whichever link sits at index 0 is the one 'o'
// opens when the cursor isn't directly on a link row, so moving a link to
// the top promotes it to the default. No-op (silent) off Tags/Links rows
// or at a list boundary — reordering has nothing meaningful to do there.
func (a App) moveRowItem(delta int) (tea.Model, tea.Cmd) {
	idx := a.selectedIdx()
	if idx < 0 {
		return a, nil
	}
	t := a.st.Tasks[idx]
	rows := detailRows(t)
	if a.detailRow >= len(rows) {
		return a, nil
	}
	row := rows[a.detailRow]

	prev := t
	next := t
	var desc string
	var newRow int
	switch row.Kind {
	case rowTagItem:
		j := row.Item + delta
		if j < 0 || j >= len(t.Tags) {
			return a, nil
		}
		tags := append([]string{}, t.Tags...)
		tags[row.Item], tags[j] = tags[j], tags[row.Item]
		next.Tags = tags
		desc = fmt.Sprintf("moved tag %q", t.Tags[row.Item])
		newRow = tagItemRow(j)
	case rowLinkItem:
		j := row.Item + delta
		if j < 0 || j >= len(t.Links) {
			return a, nil
		}
		links := append([]string{}, t.Links...)
		links[row.Item], links[j] = links[j], links[row.Item]
		next.Links = links
		desc = fmt.Sprintf("moved link %q", truncate(t.Links[row.Item], 30))
		newRow = linkItemRow(t, j)
	default:
		return a, nil
	}
	next.Updated = time.Now().Format(time.RFC3339)
	st := a.st
	a.history.Execute(Command{
		Desc: desc,
		Do:   func() { st.Set(idx, next) },
		Undo: func() { st.Set(idx, prev) },
		Idx:  idx,
	})
	a.refilter()
	a.followIdx(idx)
	a.save()
	a.detailRow = newRow
	a.setFlash(desc)
	return a, nil
}

func (a App) yankRow() (tea.Model, tea.Cmd) {
	idx := a.selectedIdx()
	if idx < 0 {
		return a, nil
	}
	t := a.st.Tasks[idx]
	rows := detailRows(t)
	if a.detailRow >= len(rows) {
		return a, nil
	}
	row := rows[a.detailRow]
	var text, label string
	switch row.Kind {
	case rowSummary:
		text, label = t.Summary, "summary"
	case rowTagItem:
		text, label = t.Tags[row.Item], "tag"
	case rowLinkItem:
		text, label = t.Links[row.Item], "link"
	default:
		a.setErr("nothing to yank")
		return a, nil
	}
	if text == "" {
		a.setErr("nothing to yank")
		return a, nil
	}
	a.setFlash("yanked " + label)
	return a, copyToClipboard(text)
}

// openDetailLink opens the link under the cursor if it's on a Links row,
// otherwise falls back to the task's first link (same fallback the list
// view's 'o' uses).
func (a App) openDetailLink() (tea.Model, tea.Cmd) {
	idx := a.selectedIdx()
	if idx < 0 {
		return a, nil
	}
	t := a.st.Tasks[idx]
	rows := detailRows(t)
	link := ""
	if a.detailRow < len(rows) && rows[a.detailRow].Kind == rowLinkItem {
		link = t.Links[rows[a.detailRow].Item]
	} else if len(t.Links) > 0 {
		link = t.Links[0]
	}
	if link == "" {
		a.setErr("no links")
		return a, nil
	}
	return a.openLink(link)
}
