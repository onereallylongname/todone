package ui

// Command represents a reversible mutation on the store: Do applies the
// change, Undo reverses it. Mirrors avredit's internal/command.Command,
// ported here as closures rather than a factory-per-mutation-kind since
// todone's mutations are all simple "replace/insert/delete one Task" shapes.
// Idx is the affected store index (-1 if not a single fixed slot, e.g. a
// delete/insert where the task's position itself is what changed) — it
// lets callers re-sync the cursor onto the same task after a resort.
type Command struct {
	Do   func()
	Undo func()
	Desc string
	Idx  int
}

// CmdHistory manages undo/redo stacks of executed commands, directly
// ported from avredit's internal/command.History (same default depth).
type CmdHistory struct {
	undo []Command
	redo []Command
}

const cmdHistoryLimit = 500

// Execute runs cmd.Do, pushes it onto the undo stack, and clears redo (a
// fresh action invalidates any previously-undone branch).
func (h *CmdHistory) Execute(cmd Command) {
	cmd.Do()
	h.undo = append(h.undo, cmd)
	if len(h.undo) > cmdHistoryLimit {
		h.undo = h.undo[1:]
	}
	h.redo = nil
}

// Undo reverses the last command, if any. Returns its description and
// affected store index (-1 if not applicable).
func (h *CmdHistory) Undo() (string, int, bool) {
	if len(h.undo) == 0 {
		return "", -1, false
	}
	cmd := h.undo[len(h.undo)-1]
	h.undo = h.undo[:len(h.undo)-1]
	cmd.Undo()
	h.redo = append(h.redo, cmd)
	return cmd.Desc, cmd.Idx, true
}

// Redo re-applies the last undone command, if any. Returns its description
// and affected store index (-1 if not applicable).
func (h *CmdHistory) Redo() (string, int, bool) {
	if len(h.redo) == 0 {
		return "", -1, false
	}
	cmd := h.redo[len(h.redo)-1]
	h.redo = h.redo[:len(h.redo)-1]
	cmd.Do()
	h.undo = append(h.undo, cmd)
	return cmd.Desc, cmd.Idx, true
}

// CanUndo reports whether there's anything to undo.
func (h *CmdHistory) CanUndo() bool { return len(h.undo) > 0 }

// CanRedo reports whether there's anything to redo.
func (h *CmdHistory) CanRedo() bool { return len(h.redo) > 0 }
