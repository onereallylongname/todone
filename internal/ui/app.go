// Package ui implements the todone Bubble Tea v2 TUI: a single flat-list
// task manager with vim-like modal navigation, inspired directly by
// avredit's App/Editor/Confirm/StatusBar pattern (see docs/PLAN.md).
//
// There is deliberately one Model (App) rather than a tree of sub-models —
// same architectural call avredit made ("no view-model separation per
// panel... avoid sync bugs"), and doubly true here since there's only one
// real panel (the list) plus an inline detail view over the same data.
package ui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/onereallylongname/todone/internal/clipboard"
	"github.com/onereallylongname/todone/internal/config"
	"github.com/onereallylongname/todone/internal/search"
	"github.com/onereallylongname/todone/internal/store"
)

// App is the root Bubble Tea model.
type App struct {
	cfg config.Config
	st  *store.Store

	width, height int
	ready         bool

	mode Mode

	// List state
	filtered []int // indexes into st.Tasks, in display order
	cursor   int
	queryRaw string
	query    search.Query
	sortMode search.SortMode
	showDone bool

	// Detail view
	detailOpen bool
	detailRow  int // index into detailRows(currentTask) — see mode.go

	// Insert-mode editing
	insertTarget insertTarget
	editItem     int // item index being edited/inserted, for tag/link item targets
	editor       Editor

	// Search / command bars (each just an Editor + a committed value)
	searchEditor Editor
	cmdEditor    Editor

	// Persisted search/command history (arrow-key recall), ported from
	// avredit's internal/config/history.go.
	searchHist    []string
	searchHistIdx int // -1 means not browsing history
	cmdHist       []string
	cmdHistIdx    int // -1 means not browsing history

	// Command-mode Tab completion state, ported from avredit's
	// completeWord cycling behavior.
	cmdCompletionActive bool
	cmdCompletions      []string
	cmdCompletionIdx    int
	cmdCompletionPrefix string

	// Tag Tab-completion state (search/insert fields) — see tagcomplete.go.
	// Only ever active in one editor at a time since insert/search/command
	// modes are mutually exclusive, so one shared state suffices.
	tagCompletionActive bool
	tagCompletions      []string
	tagCompletionIdx    int
	tagCompletionPrefix string

	// Overlays
	helpOpen   bool
	helpScroll int
	tagsOpen   bool
	tagsScroll int
	confirm    *Confirm
	onDelete   int // index (into st.Tasks) pending delete confirmation

	// Jumplist (ctrl+o/ctrl+i), populated only by following a task-id
	// link — not by routine cursor movement. See link.go.
	jumpBack []jumpLoc
	jumpFwd  []jumpLoc

	// vim operator pending state: 'g' waiting for 'g', or 'y' waiting for
	// its motion (y/l/i).
	pendingKey rune

	// Undo/redo command stack (see undo.go).
	history CmdHistory

	flash      string
	flashIsErr bool

	quitting bool
}

// New constructs the initial App model.
func New(cfg config.Config, st *store.Store) App {
	applyTheme(config.LoadTheme())
	hist := config.LoadHistory()
	a := App{
		cfg:           cfg,
		st:            st,
		sortMode:      search.ParseSortMode(cfg.DefaultSort),
		showDone:      cfg.ShowDone,
		searchHist:    hist.Searches,
		searchHistIdx: -1,
		cmdHist:       hist.Commands,
		cmdHistIdx:    -1,
	}
	a.refilter()
	return a
}

// Init implements tea.Model.
func (a App) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.ready = true
		return a, nil

	case tea.KeyPressMsg:
		a.flash = ""
		a.flashIsErr = false

		// Many users instinctively hit ctrl+c expecting it to quit, like a
		// shell. todone treats it as a no-op (same as vim) but — unlike a
		// silent no-op — tells you how to actually quit, mirroring
		// neovim's "Type :quit<Enter> to exit" hint.
		if msg.String() == "ctrl+c" {
			a.setFlash("use q or :q to quit")
			return a, nil
		}

		if a.helpOpen {
			return a.updateHelp(msg)
		}

		if a.tagsOpen {
			return a.updateTagsOverlay(msg)
		}

		if a.confirm != nil {
			confirmed, handled := a.confirm.HandleKey(msg)
			if !handled {
				return a, nil
			}
			idx := a.onDelete
			a.confirm = nil
			if confirmed {
				return a.doDelete(idx)
			}
			return a, nil
		}

		switch a.mode {
		case ModeInsert:
			return a.updateInsert(msg)
		case ModeSearch:
			return a.updateSearch(msg)
		case ModeCommand:
			return a.updateCommand(msg)
		default:
			if a.detailOpen {
				return a.updateDetailNormal(msg)
			}
			return a.updateNormal(msg)
		}

	case tea.PasteMsg:
		// Bracketed-paste text from the terminal (Shift+Insert, right-click
		// paste, etc.) — route it into whichever editor is active. A no-op
		// outside the three editor-owning modes.
		a.pasteText(msg.Content)
		return a, nil
	}
	return a, nil
}

// pasteText inserts s into whichever inline editor is currently active.
func (a *App) pasteText(s string) {
	if s == "" {
		return
	}
	switch a.mode {
	case ModeInsert:
		a.editor.InsertText(s)
	case ModeSearch:
		a.searchEditor.InsertText(s)
		a.queryRaw = a.searchEditor.Value()
		a.refilter()
	case ModeCommand:
		a.cmdEditor.InsertText(s)
	}
}

// updateHelp handles key input while the `?` overlay is open: esc/?/q
// closes it, j/k (or arrows) scroll the (potentially long) keybinding
// reference — ported from avredit's OverlayHelp key handling.
func (a App) updateHelp(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "?", "q":
		a.helpOpen = false
		a.helpScroll = 0
	case "j", "down":
		a.helpScroll++
	case "k", "up":
		if a.helpScroll > 0 {
			a.helpScroll--
		}
	}
	return a, nil
}

// updateTagsOverlay handles key input while the `:tags` overlay is open —
// same esc/q-closes, j/k-scrolls pattern as updateHelp.
func (a App) updateTagsOverlay(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		a.tagsOpen = false
		a.tagsScroll = 0
	case "j", "down":
		a.tagsScroll++
	case "k", "up":
		if a.tagsScroll > 0 {
			a.tagsScroll--
		}
	}
	return a, nil
}

// View implements tea.Model.
func (a App) View() tea.View {
	if !a.ready {
		return tea.NewView("todone — loading…")
	}

	// The status bar is always the last line (matches vim's permanent
	// statusline); the search/command/insert prompt, when active, is an
	// extra line drawn *above* it rather than replacing it.
	barHeight := 1
	if a.mode == ModeSearch || a.mode == ModeCommand || a.mode == ModeInsert {
		barHeight = 2
	}
	contentHeight := a.height - barHeight
	if contentHeight < 1 {
		contentHeight = 1
	}

	var content string
	switch {
	case a.detailOpen:
		content = a.renderDetail(contentHeight)
	default:
		content = a.renderList(contentHeight)
	}

	if a.helpOpen {
		content = overlay(content, helpOverlay(a.helpScroll, contentHeight-8), a.width, contentHeight)
	} else if a.tagsOpen {
		content = overlay(content, tagsOverlay(store.UniqueSortedTags(a.st.Tasks), a.tagsScroll, contentHeight-6), a.width, contentHeight)
	} else if a.confirm != nil {
		content = overlay(content, a.confirm.View(), a.width, contentHeight)
	}
	// Pad/clip to exactly contentHeight lines so the status bar always
	// sits on the screen's last line, even when the list/detail view has
	// fewer rows than the terminal is tall.
	content = padLines(content, contentHeight)

	bottom := a.renderBottomBar()
	full := lipgloss.JoinVertical(lipgloss.Left, content, bottom)

	v := tea.NewView(full)
	v.AltScreen = true
	return v
}

// padLines pads s with blank lines (or clips it) so it is exactly height
// lines tall.
func padLines(s string, height int) string {
	if height < 1 {
		return s
	}
	lines := strings.Split(s, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

// renderBottomBar always ends with the status bar; an active
// search/command/insert prompt is prepended as its own line above it.
func (a App) renderBottomBar() string {
	bar := a.renderStatusBar()
	prompt := a.renderPromptLine()
	if prompt == "" {
		return bar
	}
	return prompt + "\n" + bar
}

func (a App) renderPromptLine() string {
	switch a.mode {
	case ModeSearch:
		return stylePrimary.Render("/") + a.searchEditor.View()
	case ModeCommand:
		line := styleWarning.Render(":") + a.cmdEditor.View()
		if a.cmdCompletionActive && len(a.cmdCompletions) > 0 {
			line += "  " + styleDim.Render(strings.Join(a.cmdCompletions, " "))
		}
		return line
	case ModeInsert:
		return stylePrimary.Render(a.insertPrompt()) + a.editor.View()
	default:
		return ""
	}
}

func (a App) renderStatusBar() string {
	cursor := 0
	if len(a.filtered) > 0 {
		cursor = a.cursor + 1
	}
	sb := StatusBar{
		Mode:       a.mode,
		Width:      a.width,
		File:       a.st.Path,
		Total:      len(a.st.Tasks),
		Shown:      len(a.filtered),
		Cursor:     cursor,
		Done:       a.countDone(),
		ShowDone:   a.showDone,
		Sort:       a.sortMode,
		Query:      a.queryRaw,
		Flash:      a.flash,
		FlashIsErr: a.flashIsErr,
	}
	return sb.View()
}

func (a App) insertPrompt() string {
	switch a.insertTarget {
	case insertAdd:
		return "add> "
	case insertEditSummary:
		return "summary> "
	case insertEditTagItem:
		return "tag> "
	case insertAddTagItem:
		return "new tag> "
	case insertEditLinkItem:
		return "link> "
	case insertAddLinkItem:
		return "new link> "
	default:
		return "> "
	}
}

func (a App) countDone() int {
	n := 0
	for _, t := range a.st.Tasks {
		if t.Done {
			n++
		}
	}
	return n
}

// refilter recomputes a.filtered from the current query + showDone, and
// re-sorts unless an active fuzzy free-text search already provides a
// relevance order.
func (a *App) refilter() {
	q := a.query
	if q.Done == nil && !a.showDone {
		f := false
		q.Done = &f
	}
	a.filtered = search.Apply(a.st.Tasks, q)
	if !q.HasFreeText() {
		search.SortIndexes(a.st.Tasks, a.filtered, a.sortMode)
	}
	if a.cursor >= len(a.filtered) {
		a.cursor = len(a.filtered) - 1
	}
	if a.cursor < 0 {
		a.cursor = 0
	}
}

// selectedIdx returns the index into st.Tasks for the currently highlighted
// row, or -1 if the list is empty.
func (a App) selectedIdx() int {
	if a.cursor < 0 || a.cursor >= len(a.filtered) {
		return -1
	}
	return a.filtered[a.cursor]
}

// followIdx re-syncs a.cursor onto the given store index after a mutation
// + refilter may have reshuffled the sort order out from under it (e.g.
// sort:updated re-ranks whatever task was just edited) — without this, the
// cursor silently keeps pointing at an unrelated task. No-op if idx isn't
// in the filtered set (filtered out, or an insert/delete where tracking a
// fixed slot doesn't make sense).
func (a *App) followIdx(idx int) {
	if idx < 0 {
		return
	}
	if i := indexOf(a.filtered, idx); i >= 0 {
		a.cursor = i
	}
}

func (a *App) save() {
	if err := a.st.Save(); err != nil {
		a.flash = "save failed: " + err.Error()
		a.flashIsErr = true
	}
}

func (a *App) setFlash(msg string) {
	a.flash = msg
	a.flashIsErr = false
}

func (a *App) setErr(msg string) {
	a.flash = msg
	a.flashIsErr = true
}

// --- Normal mode -----------------------------------------------------------

func (a App) updateNormal(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Resolve any pending two-key sequence first ("gg", "y"+motion).
	if a.pendingKey != 0 {
		pending := a.pendingKey
		a.pendingKey = 0
		switch pending {
		case 'g':
			if key == "g" {
				a.cursor = 0
			}
			return a, nil
		case 'y':
			return a.handleYankMotion(key)
		}
	}

	switch key {
	case "q":
		return a.quit()
	case "?":
		a.helpOpen = true
		a.helpScroll = 0
		return a, nil
	case "j", "down":
		if a.cursor < len(a.filtered)-1 {
			a.cursor++
		}
		return a, nil
	case "k", "up":
		if a.cursor > 0 {
			a.cursor--
		}
		return a, nil
	case "g":
		a.pendingKey = 'g'
		return a, nil
	case "G":
		a.cursor = len(a.filtered) - 1
		if a.cursor < 0 {
			a.cursor = 0
		}
		return a, nil
	case "ctrl+d", "f":
		a.cursor += a.pageStep()
		a.clampCursor()
		return a, nil
	case "ctrl+u", "b":
		a.cursor -= a.pageStep()
		a.clampCursor()
		return a, nil
	case "space":
		return a.doToggle()
	case "enter":
		if a.selectedIdx() >= 0 {
			a.detailOpen = true
			a.detailRow = 0
		}
		return a, nil
	case "a":
		a.mode = ModeInsert
		a.insertTarget = insertAdd
		a.editor = NewEditor("", editorStyle)
		return a, nil
	case "d":
		idx := a.selectedIdx()
		if idx < 0 {
			return a, nil
		}
		a.onDelete = idx
		c := NewConfirm(fmt.Sprintf("Delete %q?", truncate(a.st.Tasks[idx].Summary, 50)))
		a.confirm = &c
		return a, nil
	case "u":
		if desc, idx, ok := a.history.Undo(); ok {
			a.refilter()
			a.followIdx(idx)
			a.save()
			a.setFlash("undid: " + desc)
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
		} else {
			a.setErr("nothing to redo")
		}
		return a, nil
	case "y":
		a.pendingKey = 'y'
		return a, nil
	case "o":
		return a.openSelectedLink()
	case "ctrl+o":
		return a.jumpBackward()
	case "ctrl+i":
		return a.jumpForward()
	case "A":
		a.showDone = !a.showDone
		a.refilter()
		return a, nil
	case "s":
		a.sortMode = a.sortMode.Next()
		a.refilter()
		a.setFlash("sort: " + a.sortMode.String())
		return a, nil
	case "/":
		a.mode = ModeSearch
		a.searchEditor = NewEditor(a.queryRaw, editorStyle)
		a.searchHistIdx = -1
		return a, nil
	case ":":
		a.mode = ModeCommand
		a.cmdEditor = NewEditor("", editorStyle)
		a.cmdHistIdx = -1
		return a, nil
	case "esc":
		if a.queryRaw != "" {
			a.queryRaw = ""
			a.query = search.Query{}
			a.refilter()
		}
		return a, nil
	}
	return a, nil
}

// quit persists search/command history before exiting — same pattern as
// avredit's App.quit().
func (a App) quit() (tea.Model, tea.Cmd) {
	config.SaveHistory(config.History{
		Commands: a.cmdHist,
		Searches: a.searchHist,
	})
	return a, tea.Quit
}

// pageStep returns how many tasks a half-page (ctrl+d/ctrl+u, f/b) should
// jump: half of however many tasks actually fit on screen, which is
// contentHeight/linesPerTask since each task now renders as two lines.
func (a *App) pageStep() int {
	h := a.height - 1
	if h < 2 {
		return 1
	}
	step := h / linesPerTask / 2
	if step < 1 {
		step = 1
	}
	return step
}

func (a *App) clampCursor() {
	if a.cursor < 0 {
		a.cursor = 0
	}
	if a.cursor > len(a.filtered)-1 {
		a.cursor = len(a.filtered) - 1
	}
	if a.cursor < 0 {
		a.cursor = 0
	}
}

func (a App) doToggle() (tea.Model, tea.Cmd) {
	idx := a.selectedIdx()
	if idx < 0 {
		return a, nil
	}
	prev := a.st.Tasks[idx]
	next := prev
	next.Done = !next.Done
	next.Updated = time.Now().Format(time.RFC3339)
	st := a.st
	verb := "marked done"
	if !next.Done {
		verb = "marked pending"
	}
	a.history.Execute(Command{
		Desc: fmt.Sprintf("%s %q", verb, truncate(prev.Summary, 30)),
		Do:   func() { st.Set(idx, next) },
		Undo: func() { st.Set(idx, prev) },
		Idx:  idx,
	})
	a.refilter()
	a.followIdx(idx)
	a.save()
	return a, nil
}

func (a App) doDelete(idx int) (tea.Model, tea.Cmd) {
	if idx < 0 || idx >= len(a.st.Tasks) {
		return a, nil
	}
	removed := a.st.Tasks[idx]
	st := a.st
	a.history.Execute(Command{
		Desc: fmt.Sprintf("deleted %q", truncate(removed.Summary, 30)),
		Do:   func() { st.Delete(idx) },
		Undo: func() { st.Insert(idx, removed) },
		Idx:  -1,
	})
	a.refilter()
	a.save()
	a.setFlash("deleted")
	return a, nil
}

func (a App) handleYankMotion(key string) (tea.Model, tea.Cmd) {
	idx := a.selectedIdx()
	if idx < 0 {
		return a, nil
	}
	t := a.st.Tasks[idx]
	var text, label string
	switch key {
	case "y":
		text, label = t.Summary, "summary"
	case "l":
		text, label = strings.Join(t.Links, "\n"), "links"
	case "i":
		text, label = t.ID, "id"
	default:
		return a, nil
	}
	if text == "" {
		a.setErr("nothing to yank")
		return a, nil
	}
	a.setFlash("yanked " + label)
	return a, copyToClipboard(text)
}

func (a App) openSelectedLink() (tea.Model, tea.Cmd) {
	idx := a.selectedIdx()
	if idx < 0 || len(a.st.Tasks[idx].Links) == 0 {
		a.setErr("no links")
		return a, nil
	}
	return a.openLink(a.st.Tasks[idx].Links[0])
}

// copyToClipboard tries both OSC52 (works over SSH, no deps) and a
// best-effort native tool (works without terminal OSC52 support). See
// docs/PLAN.md's clipboard decision.
func copyToClipboard(text string) tea.Cmd {
	return tea.Batch(
		tea.SetClipboard(text),
		func() tea.Msg {
			_ = clipboard.WriteNative(text)
			return nil
		},
	)
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
