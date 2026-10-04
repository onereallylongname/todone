package ui

// Mode is the current input mode, following the same Normal/Insert/Search/
// Command split as avredit's AppMode.
type Mode int

const (
	ModeNormal Mode = iota
	ModeInsert
	ModeSearch
	ModeCommand
)

func (m Mode) String() string {
	switch m {
	case ModeInsert:
		return "INSERT"
	case ModeSearch:
		return "SEARCH"
	case ModeCommand:
		return "COMMAND"
	default:
		return "NORMAL"
	}
}

// insertTarget records what an active Insert-mode edit is for, so Enter
// knows how to commit it.
type insertTarget int

const (
	insertNone insertTarget = iota
	insertAdd
	insertEditSummary
	insertEditTagItem
	insertAddTagItem
	insertEditLinkItem
	insertAddLinkItem
)

// detailRowKind enumerates the kinds of rows the detail view can show:
// the single Summary field, one row per Tags/Links item, and a trailing
// "+ add" placeholder row for each list — this is what makes Tags/Links
// real navigable lists instead of one comma-separated line.
type detailRowKind int

const (
	rowSummary detailRowKind = iota
	rowTagItem
	rowTagAdd
	rowLinkItem
	rowLinkAdd
)

// detailRow is one navigable row in the detail view; Item is the index
// into Tags/Links when Kind is a *Item kind, unused otherwise.
type detailRow struct {
	Kind detailRowKind
	Item int
}

