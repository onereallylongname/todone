package ui

import "strings"

// KeyBinding describes a single key binding, ported from avredit's
// internal/model.KeyBinding so the `?` overlay stays table-aligned instead
// of a wrapped paragraph.
type KeyBinding struct {
	Key  string
	Desc string
}

// KeySection groups related keybindings under a heading.
type KeySection struct {
	Title    string
	Bindings []KeyBinding
}

// Keybindings returns every keybinding section shown in the `?` overlay.
func Keybindings() []KeySection {
	return []KeySection{
		{
			Title: "Global",
			Bindings: []KeyBinding{
				{"q", "Quit"},
				{"?", "Show this help"},
				{"/", "Search / filter"},
				{":", "Command mode"},
				{"esc", "Cancel / close"},
				{"ctrl+c", "No-op (hints to use q or :q — todone never force-quits)"},
			},
		},
		{
			Title: "List (normal mode)",
			Bindings: []KeyBinding{
				{"j/k, ↓/↑", "Move"},
				{"gg / G", "Jump to top / bottom"},
				{"ctrl+d/u, f/b", "Half page down/up"},
				{"space", "Toggle done"},
				{"enter", "Open detail"},
				{"a", "Add task"},
				{"d", "Delete task (y/n confirm)"},
				{"u / ctrl+r", "Undo / redo"},
				{"yy / yl / yi", "Yank summary / links / id"},
				{"o", "Open link: task-id jumps, path/URL opens"},
				{"ctrl+o / ctrl+i", "Jump back / forward (after a link jump)"},
				{"A", "Toggle show-done"},
				{"s", "Cycle sort"},
			},
		},
		{
			Title: "Detail view",
			Bindings: []KeyBinding{
				{"j/k", "Move row"},
				{"H/L", "Next / prev task (filtered order)"},
				{"J/K", "Move Tags/Links item up/down (set default link)"},
				{"e / enter", "Edit row"},
				{"a", "Add item to Tags or Links"},
				{"d", "Delete item (instant, undo is the safety net)"},
				{"y", "Yank row"},
				{"o", "Open link: task-id jumps, path/URL opens"},
				{"ctrl+o / ctrl+i", "Jump back / forward (after a link jump)"},
				{"u / ctrl+r", "Undo / redo"},
				{"esc / q", "Back to list"},
			},
		},
		{
			Title: "Insert mode",
			Bindings: []KeyBinding{
				{"enter", "Confirm"},
				{"esc", "Cancel"},
				{"← → home end", "Move cursor"},
				{"backspace / delete", "Delete char"},
				{"ctrl+a/e", "Start / end of line"},
				{"ctrl+u/k", "Clear before / after cursor"},
				{"ctrl+w", "Delete previous word"},
				{"ctrl+v / shift+insert", "Paste from clipboard"},
				{"tab", "Complete tag (Tags field, or a #tag token in free text)"},
			},
		},
		{
			Title: "Search / command history",
			Bindings: []KeyBinding{
				{"↑/↓", "Recall previous search or command"},
			},
		},
		{
			Title: "Search prefixes",
			Bindings: []KeyBinding{
				{"t:tag or #tag", "Filter by tag (tab completes the tag name)"},
				{"@link", "Filter by link (substring match)"},
				{"d:done | d:pending", "Filter by status"},
				{"(plain text)", "Fuzzy match on summary — tokens combine (AND)"},
				{"a | b", "OR: match 'a' OR 'b' (tag/link/status filters still apply to both)"},
				{"\\# \\| \\@", "Escape a sigil as literal text, e.g. \\#1 searches for \"#1\""},
			},
		},
		{
			Title: "Command mode",
			Bindings: []KeyBinding{
				{":w", "Save"},
				{":q / :q!", "Quit / force quit"},
				{":wq", "Save and quit"},
				{":sort date|updated|alpha", "Change sort order"},
				{":all", "Toggle show-done"},
				{":tags", "List every tag in use (scrollable overlay)"},
				{"tab", "Complete command name, or sort's argument"},
			},
		},
	}
}

// helpOverlay renders a scrollable, table-aligned keybinding reference,
// clamped to maxLines (ported from avredit's OverlayHelp rendering: j/k
// scrolls, esc/?/q closes — see app.go's updateHelp).
func helpOverlay(scroll, maxLines int) string {
	var lines []string
	lines = append(lines, styleAccent.Render("Keybindings"), "")
	for _, sec := range Keybindings() {
		lines = append(lines, styleDim.Render("── "+sec.Title+" ──"))
		for _, b := range sec.Bindings {
			lines = append(lines, "  "+styleWarning.Render(padRight(b.Key, 24))+styleBase.Render(b.Desc))
		}
		lines = append(lines, "")
	}

	if maxLines < 5 {
		maxLines = 5
	}
	maxScroll := len(lines) - maxLines
	if maxScroll < 0 {
		maxScroll = 0
	}
	if scroll > maxScroll {
		scroll = maxScroll
	}
	if scroll < 0 {
		scroll = 0
	}
	end := scroll + maxLines
	if end > len(lines) {
		end = len(lines)
	}
	visible := lines[scroll:end]

	hint := "esc/? close · j/k scroll"
	if scroll > 0 {
		hint += " ↑"
	}
	if scroll < maxScroll {
		hint += " ↓"
	}
	visible = append(visible, "", styleDim.Render(hint))
	return strings.Join(visible, "\n")
}

// KeybindingsText renders the same keybinding reference as helpOverlay, but
// as plain text (no ANSI styling, no scrolling/clamping) — used by `todone
// --help` so the full TUI keybinding reference is available without
// launching the TUI. Ported from avredit's internal/model.KeybindingsText.
func KeybindingsText() string {
	var sb strings.Builder
	for _, sec := range Keybindings() {
		sb.WriteString("  " + sec.Title + ":\n")
		for _, b := range sec.Bindings {
			sb.WriteString("    " + padRight(b.Key, 24) + b.Desc + "\n")
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s + " "
	}
	return s + strings.Repeat(" ", width-len(s))
}
