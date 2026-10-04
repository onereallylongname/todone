package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/onereallylongname/todone/internal/search"
)

// StatusBar renders the bottom status bar: mode badge, the in-use data
// file, counts, sort/filter state, and a help hint. Loosely ported from
// avredit's statusbar.go, trimmed to what a flat task list needs (no node
// path).
type StatusBar struct {
	Mode       Mode
	Width      int
	File       string // path to the data file in use; shown abbreviated, degrading to basename if it doesn't fit
	Total      int
	Shown      int
	Cursor     int // 1-based row position within the shown/filtered list, 0 when empty
	Done       int
	ShowDone   bool
	Sort       search.SortMode
	Query      string // active filter text, shown so an active filter is never silently forgotten
	Flash      string
	FlashIsErr bool
}

func (s StatusBar) View() string {
	var modeStyle lipgloss.Style
	switch s.Mode {
	case ModeInsert:
		modeStyle = styleModeInsert
	case ModeSearch:
		modeStyle = styleModeSearch
	case ModeCommand:
		modeStyle = styleModeCommand
	default:
		modeStyle = styleModeNormal
	}
	modeBadge := modeStyle.Render(s.Mode.String())

	visibility := "active"
	if s.ShowDone {
		visibility = "all"
	}
	pos := fmt.Sprintf("%d/%d", s.Cursor, s.Shown)
	counts := fmt.Sprintf("%s · %d total · %d done · sort:%s · %s", pos, s.Total, s.Done, s.Sort, visibility)
	if s.Query != "" {
		counts += " · filter:" + s.Query
	}

	var right string
	if s.Flash != "" {
		st := styleSuccess
		if s.FlashIsErr {
			st = styleError
		}
		right = st.Render(s.Flash)
	} else {
		right = styleDim.Render("q:quit ?:help")
	}

	sep := styleDim.Render(" | ")
	buildLeft := func(file string) string {
		left := modeBadge
		if file != "" {
			left += sep + styleDim.Render(file)
		}
		return left + sep + styleStatusBar.Render(counts)
	}

	// Prefer a ~-abbreviated full path; degrade to just the basename if
	// the terminal is too narrow to fit everything on one line.
	left := buildLeft(abbreviateHome(s.File))
	if gap := s.Width - lipgloss.Width(left) - lipgloss.Width(right); gap < 1 && s.File != "" {
		left = buildLeft(filepath.Base(s.File))
	}

	gap := s.Width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

// abbreviateHome replaces a leading home-directory prefix with "~" for
// compact, recognizable status-bar display (e.g.
// "/home/alice/.config/todone/todo.yaml" → "~/.config/todone/todo.yaml").
// Returns path unchanged if it isn't under the home dir, or the home dir
// can't be determined.
func abbreviateHome(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if path == home {
		return "~"
	}
	if strings.HasPrefix(path, home+string(os.PathSeparator)) {
		return "~" + path[len(home):]
	}
	return path
}
