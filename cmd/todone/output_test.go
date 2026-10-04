package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/onereallylongname/todone/internal/store"
)

// captureStdout redirects os.Stdout for the duration of fn and returns
// everything written to it, so renderTasks/cmdShow/cmdFollow's fmt.Println
// calls can be asserted on directly instead of needing an injectable
// writer threaded through every function.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = orig
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func TestParseFieldsPresets(t *testing.T) {
	cases := map[string][]string{
		"name":    {"summary"},
		"default": {"id", "done", "summary", "tags"},
		"all":     {"id", "done", "summary", "date", "updated", "tags", "links"},
		"":        {"id", "done", "summary", "date", "updated", "tags", "links"},
	}
	for in, want := range cases {
		got, err := parseFields(in)
		if err != nil {
			t.Fatalf("parseFields(%q): unexpected error: %v", in, err)
		}
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("parseFields(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestParseFieldsCustomListIsCanonicallyOrdered(t *testing.T) {
	got, err := parseFields("links,id,tags")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"id", "tags", "links"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("got %v, want %v (canonical order regardless of input order)", got, want)
	}
}

func TestParseFieldsRejectsUnknownKey(t *testing.T) {
	if _, err := parseFields("id,bogus"); err == nil {
		t.Fatal("expected an error for an unknown field key")
	}
}

func TestParseFormatValidation(t *testing.T) {
	for _, f := range []string{"table", "json", "yaml", "TABLE"} {
		if _, err := parseFormat(f); err != nil {
			t.Errorf("parseFormat(%q): unexpected error: %v", f, err)
		}
	}
	if _, err := parseFormat("csv"); err == nil {
		t.Fatal("expected an error for an unsupported format")
	}
}

func TestResolveFormatAndFieldsFieldsImpliesTableFormat(t *testing.T) {
	format, fields, err := resolveFormatAndFields("", "name")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if format != "table" {
		t.Fatalf("expected --fields without --format to imply table, got %q", format)
	}
	if len(fields) != 1 || fields[0] != "summary" {
		t.Fatalf("expected fields=[summary], got %v", fields)
	}
}

func TestResolveFormatAndFieldsDefaultsToLegacyRow(t *testing.T) {
	format, fields, err := resolveFormatAndFields("", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if format != "" {
		t.Fatalf("expected empty format (legacy row) when neither flag given, got %q", format)
	}
	if len(fields) != len(allFieldKeys) {
		t.Fatalf("expected default fields = all, got %v", fields)
	}
}

func sampleTask() store.Task {
	return store.Task{ID: "abc1234567", Done: true, Summary: "Buy milk", Tags: []string{"errand"}, Links: []string{"https://x.test"}, Date: "2026-01-01T00:00:00Z"}
}

func TestRenderTasksTableIncludesHeaderAndSelectedFields(t *testing.T) {
	out := captureStdout(t, func() {
		if err := renderTasks([]store.Task{sampleTask()}, "table", []string{"id", "summary"}, false); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "ID") || !strings.Contains(out, "SUMMARY") {
		t.Fatalf("expected a header row with ID/SUMMARY, got:\n%s", out)
	}
	if !strings.Contains(out, "abc1234567") || !strings.Contains(out, "Buy milk") {
		t.Fatalf("expected the task's id/summary in the table, got:\n%s", out)
	}
	if strings.Contains(out, "errand") {
		t.Fatalf("did not request the tags field, should not appear:\n%s", out)
	}
}

func TestRenderTasksJSONSingleIsAnObjectNotArray(t *testing.T) {
	out := captureStdout(t, func() {
		if err := renderTasks([]store.Task{sampleTask()}, "json", []string{"id", "summary"}, true); err != nil {
			t.Fatal(err)
		}
	})
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("expected a single JSON object, got parse error %v for:\n%s", err, out)
	}
	if m["id"] != "abc1234567" {
		t.Fatalf("expected id=abc1234567, got %v", m["id"])
	}
}

func TestRenderTasksJSONListIsAnArray(t *testing.T) {
	out := captureStdout(t, func() {
		if err := renderTasks([]store.Task{sampleTask(), sampleTask()}, "json", []string{"id"}, false); err != nil {
			t.Fatal(err)
		}
	})
	var arr []map[string]any
	if err := json.Unmarshal([]byte(out), &arr); err != nil {
		t.Fatalf("expected a JSON array, got parse error %v for:\n%s", err, out)
	}
	if len(arr) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(arr))
	}
}

func TestRenderTasksJSONNilSlicesBecomeEmptyArrays(t *testing.T) {
	task := store.Task{ID: "x", Summary: "no tags or links"}
	out := captureStdout(t, func() {
		if err := renderTasks([]store.Task{task}, "json", []string{"tags", "links"}, true); err != nil {
			t.Fatal(err)
		}
	})
	if strings.Contains(out, "null") {
		t.Fatalf("expected nil tags/links to marshal as [], not null:\n%s", out)
	}
}

func TestRenderTasksYAMLSingleIsAMapping(t *testing.T) {
	out := captureStdout(t, func() {
		if err := renderTasks([]store.Task{sampleTask()}, "yaml", []string{"id", "summary"}, true); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "id: abc1234567") {
		t.Fatalf("expected a YAML mapping with id, got:\n%s", out)
	}
}

func TestRenderTasksUnknownFormatErrors(t *testing.T) {
	err := renderTasks([]store.Task{sampleTask()}, "bogus", allFieldKeys, false)
	if err == nil {
		t.Fatal("expected an error for an unknown format")
	}
}

func TestCmdShowAnyDoneState(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/todo.yaml"
	st := &store.Store{Path: path}
	st.Add("Buy milk", nil, nil)
	done := st.Add("Write report", nil, nil)
	st.ToggleDone(st.Index(done.ID))
	if err := st.Save(); err != nil {
		t.Fatal(err)
	}

	out := captureStdout(t, func() {
		if err := cmdShow(path, []string{done.ID}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, "Write report") {
		t.Fatalf("expected the done task's summary in output, got:\n%s", out)
	}
}

func TestCmdShowNoArgsErrors(t *testing.T) {
	if err := cmdShow("/dev/null", nil); err == nil {
		t.Fatal("expected a usage error with no query")
	}
}

func TestCmdFollowOpensPathLink(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/todo.yaml"
	st := &store.Store{Path: path}
	target := dir + "/notes.md"
	if err := os.WriteFile(target, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	st.Add("Read notes", nil, []string{target})
	if err := st.Save(); err != nil {
		t.Fatal(err)
	}

	var opened string
	origOpen := openFunc
	openFunc = func(s string) error { opened = s; return nil }
	defer func() { openFunc = origOpen }()

	out := captureStdout(t, func() {
		if err := cmdFollow(path, []string{"notes"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if opened != target {
		t.Fatalf("expected openFunc called with %q, got %q", target, opened)
	}
	if !strings.Contains(out, "opened path") {
		t.Fatalf("expected an 'opened path' message, got:\n%s", out)
	}
}

func TestCmdFollowTaskIDLinkPrintsTargetInsteadOfOpening(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/todo.yaml"
	st := &store.Store{Path: path}
	target := st.Add("Target task", nil, nil)
	st.Add("Linker task", nil, []string{target.ID})
	if err := st.Save(); err != nil {
		t.Fatal(err)
	}

	var openCalled bool
	origOpen := openFunc
	openFunc = func(s string) error { openCalled = true; return nil }
	defer func() { openFunc = origOpen }()

	out := captureStdout(t, func() {
		if err := cmdFollow(path, []string{"Linker"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if openCalled {
		t.Fatal("a task-id link must not be shelled out to the OS opener")
	}
	if !strings.Contains(out, "Target task") {
		t.Fatalf("expected the linked task's row to be printed, got:\n%s", out)
	}
}

// TestCmdFollowTaskIDLinkHonorsFormatAndFields guards against a regression
// where --fields/--format were swallowed into the needle (treated as part
// of the search query instead of flags) because cmdFollow never called
// extractFormatFieldFlags.
func TestCmdFollowTaskIDLinkHonorsFormatAndFields(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/todo.yaml"
	st := &store.Store{Path: path}
	target := st.Add("Target task", nil, nil)
	st.Add("Linker task", nil, []string{target.ID})
	if err := st.Save(); err != nil {
		t.Fatal(err)
	}

	origOpen := openFunc
	openFunc = func(s string) error { t.Fatal("must not shell out for a task-id link"); return nil }
	defer func() { openFunc = origOpen }()

	out := captureStdout(t, func() {
		if err := cmdFollow(path, []string{"Linker", "--format", "json", "--fields", "id,summary"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, `"summary": "Target task"`) {
		t.Fatalf("expected json output with only id/summary fields, got:\n%s", out)
	}
	if strings.Contains(out, "tags") {
		t.Fatalf("expected --fields id,summary to exclude tags, got:\n%s", out)
	}
}

// TestCmdFollowPathLinkAcceptsFormatFlagsWithoutError guards the other half
// of the same regression: --format/--fields must still parse cleanly (and
// simply go unused) even when the resolved link isn't a task id.
func TestCmdFollowPathLinkAcceptsFormatFlagsWithoutError(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/todo.yaml"
	st := &store.Store{Path: path}
	target := dir + "/notes.md"
	if err := os.WriteFile(target, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	st.Add("Read notes", nil, []string{target})
	if err := st.Save(); err != nil {
		t.Fatal(err)
	}

	var opened string
	origOpen := openFunc
	openFunc = func(s string) error { opened = s; return nil }
	defer func() { openFunc = origOpen }()

	out := captureStdout(t, func() {
		if err := cmdFollow(path, []string{"notes", "--fields", "default"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if opened != target {
		t.Fatalf("expected openFunc called with %q, got %q", target, opened)
	}
	if !strings.Contains(out, "opened path") {
		t.Fatalf("expected an 'opened path' message, got:\n%s", out)
	}
}

func TestCmdFollowGarbageLinkErrors(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/todo.yaml"
	st := &store.Store{Path: path}
	st.Add("Bad link task", nil, []string{"not-a-real-anything"})
	if err := st.Save(); err != nil {
		t.Fatal(err)
	}

	if err := cmdFollow(path, []string{"Bad link"}); err == nil {
		t.Fatal("expected an error for a garbage link")
	}
}

func TestCmdFollowIndexFlagPicksOtherLink(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/todo.yaml"
	st := &store.Store{Path: path}
	st.Add("Multi link task", nil, []string{"https://first.test", "https://second.test"})
	if err := st.Save(); err != nil {
		t.Fatal(err)
	}

	var opened string
	origOpen := openFunc
	openFunc = func(s string) error { opened = s; return nil }
	defer func() { openFunc = origOpen }()

	captureStdout(t, func() {
		if err := cmdFollow(path, []string{"--index", "2", "Multi link"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if opened != "https://second.test" {
		t.Fatalf("expected --index 2 to open the second link, opened %q", opened)
	}
}

func TestCmdFollowIndexOutOfRangeErrors(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/todo.yaml"
	st := &store.Store{Path: path}
	st.Add("One link task", nil, []string{"https://only.test"})
	if err := st.Save(); err != nil {
		t.Fatal(err)
	}

	if err := cmdFollow(path, []string{"--index", "5", "One link"}); err == nil {
		t.Fatal("expected an out-of-range error")
	}
}

func TestCmdFollowNoLinksErrors(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/todo.yaml"
	st := &store.Store{Path: path}
	st.Add("No links task", nil, nil)
	if err := st.Save(); err != nil {
		t.Fatal(err)
	}

	if err := cmdFollow(path, []string{"No links"}); err == nil {
		t.Fatal("expected an error when the task has no links")
	}
}
