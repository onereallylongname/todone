package main

import (
	"strings"
	"testing"

	"github.com/onereallylongname/todone/internal/store"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	return &store.Store{}
}

func TestResolveToggleTargetExactID(t *testing.T) {
	st := newTestStore(t)
	a := st.Add("Buy milk", nil, nil)
	st.Add("Buy bread", nil, nil)

	idx, err := resolveToggleTarget(st, a.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if st.Tasks[idx].ID != a.ID {
		t.Fatalf("expected to resolve to %q, got %q", a.ID, st.Tasks[idx].ID)
	}
}

func TestResolveToggleTargetIDPrefix(t *testing.T) {
	st := newTestStore(t)
	a := st.Add("Buy milk", nil, nil)
	st.Add("Buy bread", nil, nil)

	prefix := a.ID[:4]
	idx, err := resolveToggleTarget(st, prefix)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if st.Tasks[idx].ID != a.ID {
		t.Fatalf("expected to resolve to %q, got %q", a.ID, st.Tasks[idx].ID)
	}
}

func TestResolveToggleTargetAmbiguousIDPrefix(t *testing.T) {
	st := newTestStore(t)
	// Force a shared id prefix by editing the stored ids directly.
	st.Add("Task A", nil, nil)
	st.Add("Task B", nil, nil)
	st.Tasks[0].ID = "abc123"
	st.Tasks[1].ID = "abc456"

	_, err := resolveToggleTarget(st, "abc")
	if err == nil {
		t.Fatal("expected an error for an ambiguous id prefix")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("expected an ambiguous-match error, got: %v", err)
	}
}

func TestResolveToggleTargetExactSummaryWinsOverDoneState(t *testing.T) {
	st := newTestStore(t)
	st.Add("Buy milk", nil, nil)
	st.ToggleDone(0) // mark it done

	// An exact (case-insensitive) summary match should be accepted even
	// though the only matching task is already done — this is the
	// user-requested exception to the "substring match is pending-only"
	// rule.
	idx, err := resolveToggleTarget(st, "buy milk")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if st.Tasks[idx].Summary != "Buy milk" {
		t.Fatalf("expected to resolve to 'Buy milk', got %q", st.Tasks[idx].Summary)
	}
}

func TestResolveToggleTargetSubstringPendingOnly(t *testing.T) {
	st := newTestStore(t)
	st.Add("Buy milk and eggs", nil, nil)
	st.Add("Buy milk for cereal", nil, nil)
	st.ToggleDone(1) // the second match is already done

	// Only one pending task contains "milk", so the substring match
	// should resolve unambiguously, skipping the done one.
	idx, err := resolveToggleTarget(st, "milk")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if st.Tasks[idx].Summary != "Buy milk and eggs" {
		t.Fatalf("expected to resolve to the pending task, got %q", st.Tasks[idx].Summary)
	}
}

func TestResolveToggleTargetAmbiguousSubstring(t *testing.T) {
	st := newTestStore(t)
	st.Add("Buy milk", nil, nil)
	st.Add("Buy milkshake", nil, nil)

	_, err := resolveToggleTarget(st, "milk")
	if err == nil {
		t.Fatal("expected an error for an ambiguous substring match")
	}
}

func TestResolveToggleTargetNoMatch(t *testing.T) {
	st := newTestStore(t)
	st.Add("Buy milk", nil, nil)

	_, err := resolveToggleTarget(st, "xyz")
	if err == nil {
		t.Fatal("expected an error when nothing matches")
	}
}

func TestParseAddTextExtractsTagsAndLinks(t *testing.T) {
	summary, tags, links := parseAddText("Fix kafka #backend #infra https://x.test/issue @./notes.md @3964bffd95")
	if summary != "Fix kafka" {
		t.Fatalf("expected summary 'Fix kafka', got %q", summary)
	}
	wantTags := []string{"backend", "infra"}
	if len(tags) != len(wantTags) || tags[0] != wantTags[0] || tags[1] != wantTags[1] {
		t.Fatalf("expected tags %v, got %v", wantTags, tags)
	}
	wantLinks := []string{"https://x.test/issue", "./notes.md", "3964bffd95"}
	if len(links) != len(wantLinks) {
		t.Fatalf("expected links %v, got %v", wantLinks, links)
	}
	for i := range wantLinks {
		if links[i] != wantLinks[i] {
			t.Fatalf("expected links %v, got %v", wantLinks, links)
		}
	}
}

func TestParseAddTextEscapedSigilsStayLiteral(t *testing.T) {
	summary, tags, links := parseAddText(`Ticket \#1 \@mention \|pipe #real`)
	wantSummary := `Ticket #1 @mention |pipe`
	if summary != wantSummary {
		t.Fatalf("expected summary %q, got %q", wantSummary, summary)
	}
	if len(tags) != 1 || tags[0] != "real" {
		t.Fatalf("expected only the unescaped #real tag, got %v", tags)
	}
	if len(links) != 0 {
		t.Fatalf("expected no links from escaped tokens, got %v", links)
	}
}

func TestParseAddTextBareAtSignIsIgnored(t *testing.T) {
	// A lone "@" (len == 1) has nothing to link to, so it passes through
	// as plain text, mirroring the existing "#" len>1 guard.
	summary, _, links := parseAddText("ping @ here")
	if summary != "ping @ here" {
		t.Fatalf("expected lone '@' to stay in summary, got %q", summary)
	}
	if len(links) != 0 {
		t.Fatalf("expected no links, got %v", links)
	}
}

func TestResolveShowTargetSubstringMatchesAnyDoneState(t *testing.T) {
	st := newTestStore(t)
	st.Add("Buy milk and eggs", nil, nil)
	st.Add("Buy milk for cereal", nil, nil)
	st.ToggleDone(1) // the second match is already done

	// Unlike resolveToggleTarget, resolveShowTarget's substring tier must
	// still be ambiguous here — show/follow are read-only lookups, so a
	// done task competing for the match is not silently excluded.
	_, err := resolveShowTarget(st, "milk")
	if err == nil {
		t.Fatal("expected ambiguous match across both done-states")
	}
}

func TestResolveShowTargetSubstringCanResolveToADoneTask(t *testing.T) {
	st := newTestStore(t)
	st.Add("Buy milk", nil, nil)
	st.ToggleDone(0)

	// toggle's pending-only tier would never find this; show must.
	idx, err := resolveShowTarget(st, "milk")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !st.Tasks[idx].Done {
		t.Fatal("expected to resolve to the done task")
	}
	if _, err := resolveToggleTarget(st, "milk"); err == nil {
		t.Fatal("expected resolveToggleTarget to NOT find a done-only match (sanity check on the pending-only tier)")
	}
}
