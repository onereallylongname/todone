package linkkind

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

func TestClassifyTaskID(t *testing.T) {
	st := testStore(t, "abc1234567", "def7654321")
	kind, resolved := Classify("abc1234567", st)
	if kind != TaskID {
		t.Fatalf("kind = %v, want TaskID", kind)
	}
	if resolved != "abc1234567" {
		t.Errorf("resolved = %q, want abc1234567", resolved)
	}
}

func TestClassifyExistingPath(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(file, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	st := testStore(t, "abc1234567")
	kind, resolved := Classify(file, st)
	if kind != Path {
		t.Fatalf("kind = %v, want Path", kind)
	}
	if resolved != file {
		t.Errorf("resolved = %q, want %q", resolved, file)
	}
}

func TestClassifyRelativePathResolvesAgainstCwd(t *testing.T) {
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
	kind, resolved := Classify("rel.txt", st)
	if kind != Path {
		t.Fatalf("kind = %v, want Path", kind)
	}
	want, _ := filepath.Abs(filepath.Join(dir, "rel.txt"))
	if resolved != want {
		t.Errorf("resolved = %q, want %q", resolved, want)
	}
}

func TestClassifyURL(t *testing.T) {
	st := testStore(t)
	kind, resolved := Classify("https://example.com/x", st)
	if kind != URL {
		t.Fatalf("kind = %v, want URL", kind)
	}
	if resolved != "https://example.com/x" {
		t.Errorf("resolved = %q, want unchanged URL", resolved)
	}
}

func TestClassifyGarbage(t *testing.T) {
	st := testStore(t)
	kind, _ := Classify("not-a-real-id-or-path-or-url", st)
	if kind != Garbage {
		t.Fatalf("kind = %v, want Garbage", kind)
	}
}

func TestClassifyPriorityIDBeforePath(t *testing.T) {
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
	kind, resolved := Classify(id, st)
	if kind != TaskID {
		t.Fatalf("kind = %v, want TaskID (id must win over path)", kind)
	}
	if resolved != id {
		t.Errorf("resolved = %q, want %q", resolved, id)
	}
}

func TestClassifyEmpty(t *testing.T) {
	st := testStore(t)
	kind, _ := Classify("   ", st)
	if kind != Garbage {
		t.Fatalf("kind = %v, want Garbage for blank input", kind)
	}
}

func TestNormalizeExistingRelativePath(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	origWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origWd)

	got := Normalize("a.md")
	want, _ := filepath.Abs(filepath.Join(dir, "a.md"))
	if got != want {
		t.Errorf("Normalize = %q, want %q", got, want)
	}
}

func TestNormalizeNonExistentPathUnchanged(t *testing.T) {
	got := Normalize("./does/not/exist.md")
	if got != "./does/not/exist.md" {
		t.Errorf("Normalize should leave non-existent paths untouched, got %q", got)
	}
}

func TestNormalizeURLUnchanged(t *testing.T) {
	got := Normalize("https://example.com")
	if got != "https://example.com" {
		t.Errorf("Normalize should leave URLs untouched, got %q", got)
	}
}

func TestNormalizeTaskIDUnchanged(t *testing.T) {
	got := Normalize("abc1234567")
	if got != "abc1234567" {
		t.Errorf("Normalize should leave non-path-like strings untouched, got %q", got)
	}
}

func TestNormalizeAlreadyAbsoluteExistingDir(t *testing.T) {
	dir := t.TempDir()
	got := Normalize(dir)
	want, _ := filepath.Abs(dir)
	if got != want {
		t.Errorf("Normalize = %q, want %q", got, want)
	}
}

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir available")
	}
	if got := ExpandHome("~"); got != home {
		t.Errorf("ExpandHome(~) = %q, want %q", got, home)
	}
	want := filepath.Join(home, "notes", "a.md")
	if got := ExpandHome("~/notes/a.md"); got != want {
		t.Errorf("ExpandHome(~/notes/a.md) = %q, want %q", got, want)
	}
	if got := ExpandHome("relative.txt"); got != "relative.txt" {
		t.Errorf("ExpandHome should leave non-tilde paths alone, got %q", got)
	}
}
