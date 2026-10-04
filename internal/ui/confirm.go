package ui

import (
	tea "charm.land/bubbletea/v2"
)

// Confirm is a simple yes/no confirmation dialog, ported from avredit's
// internal/model/confirm.go.
type Confirm struct {
	Message string
}

// NewConfirm creates a confirmation dialog with the given message.
func NewConfirm(message string) Confirm {
	return Confirm{Message: message}
}

// HandleKey processes a keypress. Returns (confirmed, handled); handled is
// false if the key wasn't y/n/esc and the dialog should keep waiting.
func (c Confirm) HandleKey(msg tea.KeyPressMsg) (confirmed bool, handled bool) {
	switch msg.String() {
	case "y", "Y":
		return true, true
	case "n", "N", "esc", "ctrl+c":
		return false, true
	}
	return false, false
}

// View renders the confirmation dialog.
func (c Confirm) View() string {
	return styleWarning.Render(c.Message) + "\n" + styleDim.Render("  [y]es / [n]o")
}
