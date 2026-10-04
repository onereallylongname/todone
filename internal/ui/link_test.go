package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/onereallylongname/todone/internal/store"
)

func testStore(t *testing.T, ids ...string) *store.Store {
	t.Helper()
	st := &store.Store{}
	for _, id := range ids {
		st.Tasks = append(st.Tasks, store.Task{ID: id, Summary: "task " + id})
	}
	return st
}

func TestClassifyLinkTaskID(t *testing.T) {
	st := testStore(t, "abc1234567", "def7654321")
	kind, resolved := classifyLink("abc1234567", st)
	if kind != linkTaskID {
		t.Fatalf("kind = %v, want linkTaskID", kind)
	}
	if resolved != "abc1234567" {
		t.Errorf("resolved = %q, want abc1234567", resolved)
	}
}

func TestClassifyLinkExistingPath(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(file, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	st := testStore(t, "abc1234567")
	kind, resolved := classifyLink(file, st)
	if kind != linkPath {
		t.Fatalf("kind = %v, want linkPath", kind)
	}
	if resolved != file {
		t.Errorf("resolved = %q, want %q", resolved, file)
	}
}

func TestClassifyLinkRelativePathResolvesAgainstCwd(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "rel.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origWd)

	st := testStore(t)
	kind, resolved := classifyLink("rel.txt", st)
	if kind != linkPath {
		t.Fatalf("kind = %v, want linkPath", kind)
	}
	want, _ := filepath.Abs(filepath.Join(dir, "rel.txt"))
	if resolved != want {
		t.Errorf("resolved = %q, want %q", resolved, want)
	}
}

func TestClassifyLinkURL(t *testing.T) {
	st := testStore(t)
	kind, resolved := classifyLink("https://example.com/x", st)
	if kind != linkURL {
		t.Fatalf("kind = %v, want linkURL", kind)
	}
	if resolved != "https://example.com/x" {
		t.Errorf("resolved = %q, want unchanged URL", resolved)
	}
}

func TestClassifyLinkGarbage(t *testing.T) {
	st := testStore(t)
	kind, _ := classifyLink("not-a-real-id-or-path-or-url", st)
	if kind != linkGarbage {
		t.Fatalf("kind = %v, want linkGarbage", kind)
	}
}

func TestClassifyLinkPriorityIDBeforePath(t *testing.T) {
	// An id takes priority even if a same-named file happens to exist.
	dir := t.TempDir()
	id := "feedfeed01"
	if err := os.WriteFile(filepath.Join(dir, id), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	origWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origWd)

	st := testStore(t, id)
	kind, resolved := classifyLink(id, st)
	if kind != linkTaskID {
		t.Fatalf("kind = %v, want linkTaskID (id must win over path)", kind)
	}
	if resolved != id {
		t.Errorf("resolved = %q, want %q", resolved, id)
	}
}

func TestClassifyLinkEmpty(t *testing.T) {
	st := testStore(t)
	kind, _ := classifyLink("   ", st)
	if kind != linkGarbage {
		t.Fatalf("kind = %v, want linkGarbage for blank input", kind)
	}
}

func TestNormalizeLinkExistingRelativePath(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	origWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origWd)

	got := normalizeLink("a.md")
	want, _ := filepath.Abs(filepath.Join(dir, "a.md"))
	if got != want {
		t.Errorf("normalizeLink = %q, want %q", got, want)
	}
}

func TestNormalizeLinkNonExistentPathUnchanged(t *testing.T) {
	got := normalizeLink("./does/not/exist.md")
	if got != "./does/not/exist.md" {
		t.Errorf("normalizeLink should leave non-existent paths untouched, got %q", got)
	}
}

func TestNormalizeLinkURLUnchanged(t *testing.T) {
	got := normalizeLink("https://example.com")
	if got != "https://example.com" {
		t.Errorf("normalizeLink should leave URLs untouched, got %q", got)
	}
}

func TestNormalizeLinkTaskIDUnchanged(t *testing.T) {
	got := normalizeLink("abc1234567")
	if got != "abc1234567" {
		t.Errorf("normalizeLink should leave non-path-like strings untouched, got %q", got)
	}
}

func TestNormalizeLinkAlreadyAbsoluteExistingDir(t *testing.T) {
	dir := t.TempDir()
	got := normalizeLink(dir)
	want, _ := filepath.Abs(dir)
	if got != want {
		t.Errorf("normalizeLink = %q, want %q", got, want)
	}
}

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir available")
	}
	if got := expandHome("~"); got != home {
		t.Errorf("expandHome(~) = %q, want %q", got, home)
	}
	want := filepath.Join(home, "notes", "a.md")
	if got := expandHome("~/notes/a.md"); got != want {
		t.Errorf("expandHome(~/notes/a.md) = %q, want %q", got, want)
	}
	if got := expandHome("relative.txt"); got != "relative.txt" {
		t.Errorf("expandHome should leave non-tilde paths alone, got %q", got)
	}
}
