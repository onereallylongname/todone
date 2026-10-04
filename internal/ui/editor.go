package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Editor is a lightweight inline single-line text editor. Ported from
// avredit's internal/model/editor.go (same readline-ish bindings), trimmed
// to single-line use since todone's fields (summary, tags, links) are all
// one line.
type Editor struct {
	value  []rune
	cursor int
	width  int
	style  EditorStyle
}

// EditorStyle holds the visual styles for the editor.
type EditorStyle struct {
	Text   lipgloss.Style
	Cursor lipgloss.Style
}

// NewEditor creates an editor with the given initial value, cursor at end.
func NewEditor(initial string, style EditorStyle) Editor {
	runes := []rune(initial)
	return Editor{value: runes, cursor: len(runes), width: 60, style: style}
}

// Value returns the current text content.
func (e *Editor) Value() string { return string(e.value) }

// InsertText inserts s at the cursor, used for pasted text (bracketed
// paste or an explicit clipboard read). Newlines are flattened to spaces
// since every todone field (summary, tag, link) is single-line.
func (e *Editor) InsertText(s string) {
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	for _, r := range s {
		e.value = append(e.value[:e.cursor], append([]rune{r}, e.value[e.cursor:]...)...)
		e.cursor++
	}
}

// SetWidth sets the visible width (used for future scroll-within-field support).
func (e *Editor) SetWidth(w int) { e.width = w }

// HandleKey processes a keypress. Returns true if the key was consumed.
func (e *Editor) HandleKey(msg tea.KeyPressMsg) bool {
	switch msg.String() {
	case "left":
		if e.cursor > 0 {
			e.cursor--
		}
	case "right":
		if e.cursor < len(e.value) {
			e.cursor++
		}
	case "home", "ctrl+a":
		e.cursor = 0
	case "end", "ctrl+e":
		e.cursor = len(e.value)
	case "backspace":
		if e.cursor > 0 {
			e.value = append(e.value[:e.cursor-1], e.value[e.cursor:]...)
			e.cursor--
		}
	case "delete":
		if e.cursor < len(e.value) {
			e.value = append(e.value[:e.cursor], e.value[e.cursor+1:]...)
		}
	case "ctrl+u":
		e.value = e.value[e.cursor:]
		e.cursor = 0
	case "ctrl+k":
		e.value = e.value[:e.cursor]
	case "ctrl+w":
		if e.cursor > 0 {
			pos := e.cursor - 1
			for pos > 0 && e.value[pos] == ' ' {
				pos--
			}
			for pos > 0 && e.value[pos-1] != ' ' {
				pos--
			}
			e.value = append(e.value[:pos], e.value[e.cursor:]...)
			e.cursor = pos
		}
	default:
		if msg.Text != "" {
			for _, r := range msg.Text {
				e.value = append(e.value[:e.cursor], append([]rune{r}, e.value[e.cursor:]...)...)
				e.cursor++
			}
			return true
		}
		return false
	}
	return true
}

// View renders the editor content with a visible cursor block.
func (e Editor) View() string {
	var sb strings.Builder
	text := e.value
	cursorPos := e.cursor

	if cursorPos > 0 {
		sb.WriteString(e.style.Text.Render(string(text[:cursorPos])))
	}
	if cursorPos < len(text) {
		sb.WriteString(e.style.Cursor.Render(string(text[cursorPos])))
		if cursorPos+1 < len(text) {
			sb.WriteString(e.style.Text.Render(string(text[cursorPos+1:])))
		}
	} else {
		sb.WriteString(e.style.Cursor.Render(" "))
	}
	return sb.String()
}
