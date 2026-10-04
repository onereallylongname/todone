package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withHome points HOME at a temp dir for the duration of the test so
// Dir()/ThemePath() resolve under our control, then restores it.
func withHome(t *testing.T, dir string) {
	t.Helper()
	orig, had := os.LookupEnv("HOME")
	os.Setenv("HOME", dir)
	t.Cleanup(func() {
		if had {
			os.Setenv("HOME", orig)
		} else {
			os.Unsetenv("HOME")
		}
	})
}

func TestShippedThemePresetsParse(t *testing.T) {
	entries, err := os.ReadDir(filepath.Join("..", "..", "themes"))
	if err != nil {
		t.Fatalf("reading themes/ dir: %v", err)
	}
	found := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		found++
		home := t.TempDir()
		withHome(t, home)
		dir := Dir()
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(filepath.Join("..", "..", "themes", e.Name()))
		if err != nil {
			t.Fatalf("%s: read: %v", e.Name(), err)
		}
		if err := os.WriteFile(ThemePath(), data, 0o644); err != nil {
			t.Fatal(err)
		}
		o := LoadTheme()
		if o == (ThemeOverrides{}) {
			t.Errorf("%s: parsed to zero-value overrides (bad JSON?)", e.Name())
		}
		if o.Fg == "" || o.Primary == "" || o.ContrastFg == "" {
			t.Errorf("%s: expected fg/primary/contrast_fg to be set, got %+v", e.Name(), o)
		}
	}
	if found == 0 {
		t.Fatal("no theme presets found in themes/")
	}
}

func TestLoadThemeMissingFile(t *testing.T) {
	withHome(t, t.TempDir())
	o := LoadTheme()
	if o != (ThemeOverrides{}) {
		t.Fatalf("expected zero-value overrides for missing file, got %+v", o)
	}
}

func TestLoadThemeBadJSON(t *testing.T) {
	home := t.TempDir()
	withHome(t, home)
	dir := Dir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ThemePath(), []byte("{not valid json"), 0o644); err != nil {
		t.Fatal(err)
	}
	o := LoadTheme()
	if o != (ThemeOverrides{}) {
		t.Fatalf("expected zero-value overrides for malformed JSON, got %+v", o)
	}
}

func TestLoadThemePartialOverride(t *testing.T) {
	home := t.TempDir()
	withHome(t, home)
	dir := Dir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	data := `{"primary": "#7aa2f7", "error": "203"}`
	if err := os.WriteFile(filepath.Join(dir, "theme.json"), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	o := LoadTheme()
	if o.Primary != "#7aa2f7" {
		t.Errorf("Primary = %q, want #7aa2f7", o.Primary)
	}
	if o.Error != "203" {
		t.Errorf("Error = %q, want 203", o.Error)
	}
	if o.Fg != "" || o.Dim != "" || o.Accent != "" {
		t.Errorf("unset fields should stay empty, got %+v", o)
	}
}
