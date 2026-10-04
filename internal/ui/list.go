package ui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/onereallylongname/todone/internal/store"
)

// linesPerTask is how many terminal rows each task occupies in the list:
// a summary line, then a dim/indented subtitle line with its tags+age.
// Splitting these onto two lines (rather than cramming tags into a column
// on the summary line) keeps the summary readable at a glance regardless
// of how many/long the tags are.
const linesPerTask = 2

// renderList draws the visible window of a.filtered, scrolling to keep the
// cursor in view. Windowing counts in tasks (not terminal lines), since
// each task now renders as linesPerTask lines.
func (a App) renderList(height int) string {
	if len(a.filtered) == 0 {
		msg := "No tasks yet — press 'a' to add one."
		if a.queryRaw != "" || !a.query.Empty() {
			msg = "No tasks match the current filter."
		}
		return styleDim.Render(msg)
	}

	visible := height / linesPerTask
	if visible < 1 {
		visible = 1
	}

	start := 0
	if a.cursor >= visible {
		start = a.cursor - visible + 1
	}
	end := start + visible
	if end > len(a.filtered) {
		end = len(a.filtered)
		start = end - visible
		if start < 0 {
			start = 0
		}
	}

	var b strings.Builder
	for i := start; i < end; i++ {
		idx := a.filtered[i]
		b.WriteString(a.renderRow(a.st.Tasks[idx], i == a.cursor, a.width))
		if i != end-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

const (
	checkboxW = 4 // "[ ] "
	timeW     = 5 // right-aligned age, e.g. "12mo"
)

// renderRow renders one task as two lines: "[ ] Summary" on top, and a
// dim, indented "#tag #tag ... age" subtitle line underneath (indented to
// align under the summary text, not the checkbox). The whole two-line
// block is highlighted together when selected, so the cursor is never
// ambiguous about which subtitle belongs to which summary.
func (a App) renderRow(t store.Task, selected bool, width int) string {
	if width <= 0 {
		width = 80
	}
	checkbox := "[ ]"
	if t.Done {
		checkbox = "[x]"
	}

	summaryW := width - checkboxW
	if summaryW < 1 {
		summaryW = 1
	}
	line1 := checkbox + " " + fixedWidth(t.Summary, summaryW)
	if lipgloss.Width(line1) > width {
		line1 = truncate(line1, width)
	}

	indent := strings.Repeat(" ", checkboxW)
	tagsText := ""
	if len(t.Tags) > 0 {
		tags := make([]string, len(t.Tags))
		for i, tg := range t.Tags {
			tags[i] = "#" + tg
		}
		tagsText = strings.Join(tags, " ")
	}
	age := relativeTime(t.SortTime())
	subW := width - len(indent) - timeW - 1
	if subW < 1 {
		subW = 1
	}
	line2 := indent + fixedWidth(tagsText, subW) + " " + padLeft(age, timeW)
	if lipgloss.Width(line2) > width {
		line2 = truncate(line2, width)
	}

	pad := func(s string) string {
		if n := width - lipgloss.Width(s); n > 0 {
			s += strings.Repeat(" ", n)
		}
		return s
	}

	switch {
	case selected:
		return styleSelected.Render(pad(line1)) + "\n" + styleSelected.Render(pad(line2))
	case t.Done:
		return styleDone.Render(line1) + "\n" + styleMuted.Render(line2)
	default:
		return styleBase.Render(line1) + "\n" + styleMuted.Render(line2)
	}
}

// fixedWidth truncates s (adding an ellipsis if clipped) or right-pads it
// with spaces so the next column always starts at the same offset.
func fixedWidth(s string, w int) string {
	if w <= 0 {
		return ""
	}
	n := lipgloss.Width(s)
	if n > w {
		return truncate(s, w)
	}
	return s + strings.Repeat(" ", w-n)
}

// padLeft right-aligns s within width w by prepending spaces.
func padLeft(s string, w int) string {
	n := lipgloss.Width(s)
	if n >= w {
		return s
	}
	return strings.Repeat(" ", w-n) + s
}

// relativeTime renders a coarse, human-friendly age ("3h", "2d", "5mo").
// Empty string for a zero time (unparseable/unset).
func relativeTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	switch {
	case d < 0:
		return "now"
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%dmo", int(d.Hours()/24/30))
	default:
		return fmt.Sprintf("%dy", int(d.Hours()/24/365))
	}
}
