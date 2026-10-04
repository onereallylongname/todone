package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/onereallylongname/todone/internal/store"
)

// detailRows computes the flat, navigable row list for a task's detail
// view: the Summary field, one row per Tags item, a trailing "+ add tag"
// row, one row per Links item, and a trailing "+ add link" row. Tags/Links
// are real lists this way — j/k walks every item, not just the three
// fields — per the "lists should actually be lists" UX request.
func detailRows(t store.Task) []detailRow {
	rows := make([]detailRow, 0, 2+len(t.Tags)+len(t.Links))
	rows = append(rows, detailRow{Kind: rowSummary})
	for i := range t.Tags {
		rows = append(rows, detailRow{Kind: rowTagItem, Item: i})
	}
	rows = append(rows, detailRow{Kind: rowTagAdd})
	for i := range t.Links {
		rows = append(rows, detailRow{Kind: rowLinkItem, Item: i})
	}
	rows = append(rows, detailRow{Kind: rowLinkAdd})
	return rows
}

// clampDetailRow keeps a.detailRow in range after Tags/Links length
// changes (add/delete item) or when switching tasks (H/L).
func (a *App) clampDetailRow(t store.Task) {
	n := len(detailRows(t))
	if a.detailRow >= n {
		a.detailRow = n - 1
	}
	if a.detailRow < 0 {
		a.detailRow = 0
	}
}

// renderDetail shows the selected task as a navigable row list: Summary,
// then every Tags item, a "+ add tag" row, every Links item, and a
// "+ add link" row — the active row highlighted. Ported styling approach
// from list.go: build each line as plain text, then wrap the *whole* line
// in exactly one Style.Render call (avoids lipgloss's nested-ANSI-reset
// truncation bug).
func (a App) renderDetail(height int) string {
	_ = height // detail view doesn't scroll in v1 — task field lists are short
	idx := a.selectedIdx()
	if idx < 0 {
		return styleDim.Render("No task selected.")
	}
	t := a.st.Tasks[idx]
	rows := detailRows(t)
	cur := a.detailRow
	if cur >= len(rows) {
		cur = len(rows) - 1
	}
	if cur < 0 {
		cur = 0
	}

	status := "pending"
	if t.Done {
		status = "done"
	}
	header := styleAccent.Render("Task "+t.ID) + "\n" +
		styleDim.Render(fmt.Sprintf("status: %s · created: %s", status, t.Date))
	if t.Updated != "" {
		header += styleDim.Render(" · updated: " + t.Updated)
	}

	width := a.width - 4
	line := func(i int, text string, muted bool) string {
		marker := "  "
		if i == cur {
			marker = "▸ "
		}
		full := marker + text
		switch {
		case i == cur:
			if width > 0 && lipgloss.Width(full) < width {
				full += strings.Repeat(" ", width-lipgloss.Width(full))
			}
			return styleSelected.Render(full)
		case muted:
			return styleMuted.Render(full)
		default:
			return styleBase.Render(full)
		}
	}

	var b strings.Builder
	b.WriteString(header)
	b.WriteString("\n\n")

	ri := 0
	summary := t.Summary
	if summary == "" {
		summary = "—"
	}
	b.WriteString(line(ri, "Summary: "+summary, false) + "\n\n")
	ri++

	b.WriteString(styleDim.Render("  Tags:") + "\n")
	for _, tag := range t.Tags {
		b.WriteString(line(ri, "#"+tag, false) + "\n")
		ri++
	}
	b.WriteString(line(ri, "+ add tag", true) + "\n\n")
	ri++

	b.WriteString(styleDim.Render("  Links:") + "\n")
	for _, link := range t.Links {
		b.WriteString(line(ri, link, false) + "\n")
		ri++
	}
	b.WriteString(line(ri, "+ add link", true) + "\n\n")
	ri++

	b.WriteString(styleDim.Render("e/Enter edit · a add · d delete · y yank · o open link · H/L next/prev task · J/K move item · Esc/q back"))
	return b.String()
}

// overlay renders content as a bordered box centered over a width x height
// canvas, used for the help and confirm modals. Matches avredit's modal
// pattern of fully replacing the content area while an overlay is active.
func overlay(base, content string, width, height int) string {
	_ = base // overlay intentionally replaces, not composites — see avredit renderOverlay
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorPrimary).
		Padding(1, 2).
		Render(content)
	if width < 1 {
		width = lipgloss.Width(box)
	}
	if height < 1 {
		height = lipgloss.Height(box)
	}
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}
