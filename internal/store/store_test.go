package store

import (
	"path/filepath"
	"testing"
)

func TestAddLoadSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "todo.yaml")

	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load (missing file) error: %v", err)
	}
	if len(s.Tasks) != 0 {
		t.Fatalf("expected empty store, got %d tasks", len(s.Tasks))
	}

	task := s.Add("Buy milk", []string{"errand"}, []string{"https://example.com"})
	if task.ID == "" {
		t.Fatal("expected non-empty id")
	}
	if err := s.Save(); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	reloaded, err := Load(path)
	if err != nil {
		t.Fatalf("reload error: %v", err)
	}
	if len(reloaded.Tasks) != 1 {
		t.Fatalf("expected 1 task after reload, got %d", len(reloaded.Tasks))
	}
	got := reloaded.Tasks[0]
	if got.Summary != "Buy milk" || got.Done || len(got.Tags) != 1 || got.Tags[0] != "errand" {
		t.Fatalf("round-tripped task mismatch: %+v", got)
	}
	if got.CreatedAt().IsZero() {
		t.Fatal("expected CreatedAt to parse the written RFC3339 date")
	}
}

func TestLegacyDateFormatParses(t *testing.T) {
	task := Task{Date: "2025-02-24 23:37:39.481996 +00:00"}
	if task.CreatedAt().IsZero() {
		t.Fatal("expected legacy todo.nu date format to parse")
	}
}

func TestToggleDoneAndUndo(t *testing.T) {
	s := &Store{}
	s.Add("Task A", nil, nil)

	prev, ok := s.ToggleDone(0)
	if !ok {
		t.Fatal("ToggleDone returned ok=false")
	}
	if !s.Tasks[0].Done {
		t.Fatal("expected task to be marked done")
	}
	// Undo: restore previous state.
	s.Set(0, prev)
	if s.Tasks[0].Done {
		t.Fatal("expected undo to restore not-done state")
	}
}

func TestDeleteInsertUndo(t *testing.T) {
	s := &Store{}
	s.Add("A", nil, nil)
	s.Add("B", nil, nil)
	s.Add("C", nil, nil)

	removed, ok := s.Delete(1)
	if !ok || removed.Summary != "B" {
		t.Fatalf("expected to remove B, got %+v ok=%v", removed, ok)
	}
	if len(s.Tasks) != 2 {
		t.Fatalf("expected 2 tasks after delete, got %d", len(s.Tasks))
	}

	s.Insert(1, removed)
	if len(s.Tasks) != 3 || s.Tasks[1].Summary != "B" {
		t.Fatalf("expected undo to restore B at index 1, got %+v", s.Tasks)
	}
}

func TestIndexByID(t *testing.T) {
	s := &Store{}
	t1 := s.Add("A", nil, nil)
	s.Add("B", nil, nil)

	if idx := s.Index(t1.ID); idx != 0 {
		t.Fatalf("expected index 0, got %d", idx)
	}
	if idx := s.Index("missing"); idx != -1 {
		t.Fatalf("expected -1 for missing id, got %d", idx)
	}
}

func TestUniqueSortedTags(t *testing.T) {
	tasks := []Task{
		{Tags: []string{"work", "errand"}},
		{Tags: []string{"work", "home"}},
		{Tags: nil},
		{Tags: []string{"errand"}},
	}
	got := UniqueSortedTags(tasks)
	want := []string{"errand", "home", "work"}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected %v, got %v", want, got)
		}
	}
}

func TestUniqueSortedTagsEmpty(t *testing.T) {
	if got := UniqueSortedTags(nil); len(got) != 0 {
		t.Fatalf("expected no tags, got %v", got)
	}
}

func TestHasLink(t *testing.T) {
	task := Task{Links: []string{"https://example.com/issue/42", "3964bffd95"}}
	if !task.HasLink("example.com") {
		t.Fatal("expected substring match against a URL link")
	}
	if !task.HasLink("3964") {
		t.Fatal("expected substring match against a task-id link")
	}
	if !task.HasLink("EXAMPLE") {
		t.Fatal("expected case-insensitive match")
	}
	if task.HasLink("nonexistent") {
		t.Fatal("expected no match for an unrelated substring")
	}
}

func TestHasLinkNoLinks(t *testing.T) {
	task := Task{}
	if task.HasLink("anything") {
		t.Fatal("expected no match when the task has no links")
	}
}
