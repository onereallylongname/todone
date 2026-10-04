package ui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/onereallylongname/todone/internal/linkkind"
	"github.com/onereallylongname/todone/internal/opener"
	"github.com/onereallylongname/todone/internal/search"
	"github.com/onereallylongname/todone/internal/store"
)

// linkKind classifies a Links entry so 'o' can decide what to do with it.
// Aliased to the shared internal/linkkind package so the TUI and the CLI's
// `follow` command use exactly one classification implementation.
type linkKind = linkkind.Kind

const (
	linkGarbage = linkkind.Garbage // not an id, not an existing path, not a URL
	linkTaskID  = linkkind.TaskID  // matches another task's id exactly
	linkPath    = linkkind.Path    // resolves to an existing file or directory
	linkURL     = linkkind.URL     // looks like scheme://... (http, file, mailto, ...)
)

// classifyLink decides what kind of thing link is, in the priority order
// the user asked for: an exact task-id match first, then an existing
// filesystem path, then a URL, else garbage. st may be nil (id matching
// then always misses).
func classifyLink(link string, st *store.Store) (linkKind, string) {
	return linkkind.Classify(link, st)
}

// expandHome expands a leading "~" or "~/..." to the user's home
// directory; anything else passes through unchanged.
func expandHome(p string) string {
	return linkkind.ExpandHome(p)
}

// normalizeLink absolutizes a path-like link at the moment it's typed in
// (add/edit), so a relative path resolved against today's cwd doesn't
// silently break if todone is later launched from somewhere else. It only
// rewrites strings that currently resolve to a real file or directory;
// URLs, task ids, garbage, and not-yet-existing paths pass through
// completely unchanged.
func normalizeLink(s string) string {
	return linkkind.Normalize(s)
}

// jumpLoc captures enough state to restore a navigation position: which
// task (by store index) was selected, and whether the detail view was
// open, so ctrl+o/ctrl+i can retrace a task-id link jump.
type jumpLoc struct {
	idx        int
	detailOpen bool
}

// pushJump records the current location onto the back-jump stack and
// clears the forward stack — a fresh jump invalidates "redo" history, the
// same as vim's jumplist or a browser's back/forward list.
func (a *App) pushJump() {
	cur := a.selectedIdx()
	if cur < 0 {
		return
	}
	a.jumpBack = append(a.jumpBack, jumpLoc{idx: cur, detailOpen: a.detailOpen})
	a.jumpFwd = nil
}

// jumpBackward moves to the previous location on the jumplist (ctrl+o).
func (a App) jumpBackward() (tea.Model, tea.Cmd) {
	if len(a.jumpBack) == 0 {
		a.setErr("no older jumps")
		return a, nil
	}
	loc := a.jumpBack[len(a.jumpBack)-1]
	a.jumpBack = a.jumpBack[:len(a.jumpBack)-1]
	if cur := a.selectedIdx(); cur >= 0 {
		a.jumpFwd = append(a.jumpFwd, jumpLoc{idx: cur, detailOpen: a.detailOpen})
	}
	a.restoreJump(loc)
	return a, nil
}

// jumpForward moves to the next location on the jumplist (ctrl+i).
func (a App) jumpForward() (tea.Model, tea.Cmd) {
	if len(a.jumpFwd) == 0 {
		a.setErr("no newer jumps")
		return a, nil
	}
	loc := a.jumpFwd[len(a.jumpFwd)-1]
	a.jumpFwd = a.jumpFwd[:len(a.jumpFwd)-1]
	if cur := a.selectedIdx(); cur >= 0 {
		a.jumpBack = append(a.jumpBack, jumpLoc{idx: cur, detailOpen: a.detailOpen})
	}
	a.restoreJump(loc)
	return a, nil
}

// restoreJump lands on loc's task (clearing the filter first if needed)
// and restores whether the detail view was open.
func (a *App) restoreJump(loc jumpLoc) {
	if !a.gotoTaskIdx(loc.idx) {
		a.setErr("jump target no longer exists")
		return
	}
	a.detailOpen = loc.detailOpen
	if a.detailOpen {
		a.detailRow = 0
	}
	a.setFlash("jumped")
}

// gotoTaskIdx moves the cursor to the task at store-index idx, clearing
// the active filter first if idx is currently hidden by it (so a link
// jump or jumplist restore always lands somewhere visible). Returns false
// if idx doesn't exist in the store at all.
func (a *App) gotoTaskIdx(idx int) bool {
	if idx < 0 || idx >= len(a.st.Tasks) {
		return false
	}
	pos := indexOfInt(a.filtered, idx)
	if pos < 0 {
		a.queryRaw = ""
		a.query = search.Query{}
		a.refilter()
		pos = indexOfInt(a.filtered, idx)
		if pos < 0 {
			return false
		}
	}
	a.cursor = pos
	return true
}

func indexOfInt(xs []int, v int) int {
	for i, x := range xs {
		if x == v {
			return i
		}
	}
	return -1
}

// openLink classifies link and acts on it:
//   - a task-id jumps to that task, staying in whichever view (list or
//     detail) is currently active, clearing the filter first if the
//     target is hidden by it;
//   - an existing path or a URL is handed to the OS opener;
//   - anything else is reported as an error without shelling out.
func (a App) openLink(link string) (tea.Model, tea.Cmd) {
	kind, resolved := classifyLink(link, a.st)
	switch kind {
	case linkTaskID:
		target := a.st.Index(resolved)
		if target < 0 {
			a.setErr("linked task not found")
			return a, nil
		}
		wasHidden := indexOfInt(a.filtered, target) < 0
		a.pushJump()
		if !a.gotoTaskIdx(target) {
			a.setErr("linked task not found")
			return a, nil
		}
		if a.detailOpen {
			a.detailRow = 0
		}
		if wasHidden {
			a.setFlash("jumped to task (filter cleared)")
		} else {
			a.setFlash("jumped to task")
		}
		return a, nil
	case linkPath, linkURL:
		if err := opener.Open(resolved); err != nil {
			a.setErr("open failed: " + err.Error())
			return a, nil
		}
		if kind == linkPath {
			a.setFlash("opened path")
		} else {
			a.setFlash("opened link")
		}
		return a, nil
	default:
		a.setErr("not a recognizable link")
		return a, nil
	}
}
