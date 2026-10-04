package search

import (
	"strings"
	"testing"

	"github.com/onereallylongname/todone/internal/store"
)

func sampleTasks() []store.Task {
	return []store.Task{
		{Summary: "Fix kafka client retry", Tags: []string{"kafka", "backend"}, Done: false, Links: []string{"https://github.com/example/kafka/issues/42"}},
		{Summary: "Write pricing docs", Tags: []string{"pricing"}, Done: true},
		{Summary: "Upgrade kafka broker", Tags: []string{"kafka", "infra"}, Done: false, Links: []string{"3964bffd95"}},
		{Summary: "Unrelated chore", Tags: nil, Done: false},
	}
}

func TestParsePrefixes(t *testing.T) {
	q := Parse("#kafka d:pending retry")
	if len(q.Tags) != 1 || q.Tags[0] != "kafka" {
		t.Fatalf("expected tag 'kafka', got %+v", q.Tags)
	}
	if q.Done == nil || *q.Done != false {
		t.Fatalf("expected Done=false, got %+v", q.Done)
	}
	if len(q.FreeTextGroups) != 1 || q.FreeTextGroups[0] != "retry" {
		t.Fatalf("expected one free-text group 'retry', got %+v", q.FreeTextGroups)
	}
}

func TestParseOrGroups(t *testing.T) {
	q := Parse("int | work #now")
	if len(q.Tags) != 1 || q.Tags[0] != "now" {
		t.Fatalf("expected tag 'now' to apply regardless of | position, got %+v", q.Tags)
	}
	if len(q.FreeTextGroups) != 2 || q.FreeTextGroups[0] != "int" || q.FreeTextGroups[1] != "work" {
		t.Fatalf("expected two OR groups ['int','work'], got %+v", q.FreeTextGroups)
	}
}

func TestParseOrGroupsSkipsEmptyBranches(t *testing.T) {
	// Leading/trailing/doubled "|" shouldn't produce an empty group that
	// would otherwise behave like an always-match wildcard.
	q := Parse("| work | |")
	if len(q.FreeTextGroups) != 1 || q.FreeTextGroups[0] != "work" {
		t.Fatalf("expected empty branches to be dropped, got %+v", q.FreeTextGroups)
	}
}

func TestApplyCombinesTagStatusAndFuzzy(t *testing.T) {
	tasks := sampleTasks()
	q := Parse("#kafka d:pending retry")
	idx := Apply(tasks, q)
	if len(idx) != 1 || tasks[idx[0]].Summary != "Fix kafka client retry" {
		t.Fatalf("expected only the matching kafka+pending+retry task, got %v", idx)
	}
}

func TestApplyTagOnlyKeepsBothKafkaTasks(t *testing.T) {
	tasks := sampleTasks()
	idx := Apply(tasks, Parse("t:kafka"))
	if len(idx) != 2 {
		t.Fatalf("expected 2 kafka-tagged tasks, got %d (%v)", len(idx), idx)
	}
}

func TestApplyOrGroupsMatchesEitherAlternative(t *testing.T) {
	tasks := sampleTasks()
	// "retry" only matches the first task; "broker" only matches the
	// third — an OR query should return both, with the hard #kafka
	// filter still applied to each.
	idx := Apply(tasks, Parse("retry | broker #kafka"))
	if len(idx) != 2 {
		t.Fatalf("expected 2 tasks matching either OR branch, got %d (%v)", len(idx), idx)
	}
	summaries := map[string]bool{}
	for _, i := range idx {
		summaries[tasks[i].Summary] = true
	}
	if !summaries["Fix kafka client retry"] || !summaries["Upgrade kafka broker"] {
		t.Fatalf("expected both kafka tasks to match one OR branch each, got %v", idx)
	}
}

func TestApplyOrGroupsExcludesNonMatches(t *testing.T) {
	tasks := sampleTasks()
	idx := Apply(tasks, Parse("retry | broker"))
	for _, i := range idx {
		if tasks[i].Summary == "Write pricing docs" || tasks[i].Summary == "Unrelated chore" {
			t.Fatalf("did not expect non-matching task %q in OR results", tasks[i].Summary)
		}
	}
}

func TestEmptyQueryMatchesEverythingInOrder(t *testing.T) {
	tasks := sampleTasks()
	q := Parse("")
	if !q.Empty() {
		t.Fatal("expected empty query to report Empty()==true")
	}
	idx := Apply(tasks, q)
	if len(idx) != len(tasks) {
		t.Fatalf("expected all %d tasks, got %d", len(tasks), len(idx))
	}
	for i, v := range idx {
		if v != i {
			t.Fatalf("expected original order, got %v", idx)
		}
	}
}

func TestSortAlpha(t *testing.T) {
	tasks := sampleTasks()
	idx := []int{0, 1, 2, 3}
	SortIndexes(tasks, idx, SortAlpha)
	want := "Fix kafka client retry" // alphabetically first
	if tasks[idx[0]].Summary != want {
		t.Fatalf("expected first = %q, got %q", want, tasks[idx[0]].Summary)
	}
}

func TestParseLinkPrefix(t *testing.T) {
	q := Parse("@github retry")
	if len(q.Links) != 1 || q.Links[0] != "github" {
		t.Fatalf("expected link filter 'github', got %+v", q.Links)
	}
	if len(q.FreeTextGroups) != 1 || q.FreeTextGroups[0] != "retry" {
		t.Fatalf("expected free-text group 'retry', got %+v", q.FreeTextGroups)
	}
}

func TestApplyLinkFilterMatchesSubstring(t *testing.T) {
	tasks := sampleTasks()
	idx := Apply(tasks, Parse("@kafka/issues"))
	if len(idx) != 1 || tasks[idx[0]].Summary != "Fix kafka client retry" {
		t.Fatalf("expected only the task with a matching link, got %v", idx)
	}
}

func TestApplyLinkFilterMatchesTaskIDLink(t *testing.T) {
	tasks := sampleTasks()
	idx := Apply(tasks, Parse("@3964bffd95"))
	if len(idx) != 1 || tasks[idx[0]].Summary != "Upgrade kafka broker" {
		t.Fatalf("expected only the task linking to that id, got %v", idx)
	}
}

func TestParseEscapedSigilsAreLiteralFreeText(t *testing.T) {
	q := Parse(`\#1 \|piped \@mention`)
	if len(q.Tags) != 0 || len(q.Links) != 0 {
		t.Fatalf("escaped sigils must not become filters, got tags=%v links=%v", q.Tags, q.Links)
	}
	if len(q.FreeTextGroups) != 1 {
		t.Fatalf("expected a single OR group, got %+v", q.FreeTextGroups)
	}
	got := q.FreeTextGroups[0]
	for _, w := range []string{"#1", "|piped", "@mention"} {
		if !strings.Contains(got, w) {
			t.Fatalf("expected free text %q to contain %q", got, w)
		}
	}
}

func TestParseEscapedPipeIsNotAnOrSeparator(t *testing.T) {
	q := Parse(`a \| b`)
	if len(q.FreeTextGroups) != 1 {
		t.Fatalf("expected one OR group (escaped pipe is literal, not a separator), got %+v", q.FreeTextGroups)
	}
}
