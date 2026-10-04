// Package config's theme.go handles the optional single-file color theme:
// ~/.config/todone/theme.json. If present, any non-empty field overrides
// the corresponding built-in default color; the file is otherwise ignored
// entirely (missing file, empty file, or parse error all just mean "use
// defaults") — matches the rest of this package's "always start up clean"
// philosophy. Deliberately colors-only (no structural overrides like
// avredit's fuller theme system) per the suckless "one sensible default,
// small override surface" approach.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// ThemeOverrides holds optional color overrides. Every field accepts
// anything lipgloss.Color understands (hex "#7aa2f7" or an ANSI 256-color
// index "75") since values just get passed straight through. Empty string
// means "keep the built-in default" for that slot.
type ThemeOverrides struct {
	Fg         string `json:"fg,omitempty"`
	Dim        string `json:"dim,omitempty"`
	Muted      string `json:"muted,omitempty"`
	Primary    string `json:"primary,omitempty"`
	Success    string `json:"success,omitempty"`
	Warning    string `json:"warning,omitempty"`
	Error      string `json:"error,omitempty"`
	Accent     string `json:"accent,omitempty"`
	ContrastFg string `json:"contrast_fg,omitempty"`
}

// ThemePath returns the full path to theme.json.
func ThemePath() string {
	return filepath.Join(Dir(), "theme.json")
}

// LoadTheme reads overrides from the standard path. Returns a zero-value
// ThemeOverrides (i.e. "use every default") if the file doesn't exist or
// fails to parse.
func LoadTheme() ThemeOverrides {
	var o ThemeOverrides
	data, err := os.ReadFile(ThemePath())
	if err != nil {
		return o
	}
	_ = json.Unmarshal(data, &o)
	return o
}
