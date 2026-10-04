package ui

import (
	"fmt"
	"strings"
)

// tagsOverlay renders a scrollable list of every distinct tag in use,
// opened by the `:tags` command. Mirrors helpOverlay's layout/scrolling
// conventions (see help.go) so both overlays feel consistent: j/k scrolls,
// esc/q closes (see app.go's updateTagsOverlay).
func tagsOverlay(tags []string, scroll, maxLines int) string {
	var lines []string
	lines = append(lines, styleAccent.Render(fmt.Sprintf("Tags (%d)", len(tags))), "")
	if len(tags) == 0 {
		lines = append(lines, styleDim.Render("  (no tags yet)"))
	}
	for _, t := range tags {
		lines = append(lines, "  "+styleWarning.Render("#"+t))
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

	hint := "esc/q close · j/k scroll"
	if scroll > 0 {
		hint += " ↑"
	}
	if scroll < maxScroll {
		hint += " ↓"
	}
	visible = append(visible, "", styleDim.Render(hint))
	return strings.Join(visible, "\n")
}
