package ui

import (
	"image/color"

	"charm.land/lipgloss/v2"

	"github.com/onereallylongname/todone/internal/config"
)

// Built-in defaults. Colors are ANSI-256 indices, which lipgloss.Color
// accepts the same as hex strings — chosen to look reasonable on both
// light and dark terminal backgrounds without extra configuration.
const (
	defaultFg         = "252"
	defaultDim        = "244"
	defaultMuted      = "238"
	defaultPrimary    = "75"  // blue
	defaultSuccess    = "114" // green
	defaultWarning    = "179" // yellow
	defaultError      = "203" // red
	defaultAccent     = "183" // purple
	defaultContrastFg = "0"   // text drawn on top of a colored background
)

// Package-level style vars. Built once at startup (and again whenever a
// theme.json override is applied) by applyTheme — nothing else should
// assign to these directly.
var (
	colorFg      color.Color
	colorDim     color.Color
	colorMuted   color.Color
	colorPrimary color.Color
	colorSuccess color.Color
	colorWarning color.Color
	colorError   color.Color
	colorAccent  color.Color

	styleBase    lipgloss.Style
	styleDim     lipgloss.Style
	styleMuted   lipgloss.Style
	stylePrimary lipgloss.Style
	styleSuccess lipgloss.Style
	styleWarning lipgloss.Style
	styleError   lipgloss.Style
	styleAccent  lipgloss.Style

	styleSelected lipgloss.Style
	styleTagBadge lipgloss.Style
	styleDone     lipgloss.Style

	styleModeNormal  lipgloss.Style
	styleModeInsert  lipgloss.Style
	styleModeSearch  lipgloss.Style
	styleModeCommand lipgloss.Style

	styleStatusBar lipgloss.Style

	editorStyle EditorStyle
)

func init() {
	applyTheme(config.ThemeOverrides{})
}

// colorOr returns override if non-empty, else fallback, wrapped as a
// lipgloss.Color. override may be a hex string ("#7aa2f7") or an ANSI-256
// index ("75") — both pass straight through to lipgloss untouched.
func colorOr(override, fallback string) color.Color {
	if override != "" {
		return lipgloss.Color(override)
	}
	return lipgloss.Color(fallback)
}

// applyTheme (re)builds every package-level color and style from built-in
// defaults, overridden field-by-field by a theme.json (if any field is
// set). Called once at startup with the real loaded overrides; also
// called once with a zero-value ThemeOverrides by init() so the UI has a
// fully-built default palette even before config is consulted (and so
// tests that never call ui.New() still see sane styles).
func applyTheme(o config.ThemeOverrides) {
	colorFg = colorOr(o.Fg, defaultFg)
	colorDim = colorOr(o.Dim, defaultDim)
	colorMuted = colorOr(o.Muted, defaultMuted)
	colorPrimary = colorOr(o.Primary, defaultPrimary)
	colorSuccess = colorOr(o.Success, defaultSuccess)
	colorWarning = colorOr(o.Warning, defaultWarning)
	colorError = colorOr(o.Error, defaultError)
	colorAccent = colorOr(o.Accent, defaultAccent)
	contrastFg := colorOr(o.ContrastFg, defaultContrastFg)

	styleBase = lipgloss.NewStyle().Foreground(colorFg)
	styleDim = lipgloss.NewStyle().Foreground(colorDim)
	styleMuted = lipgloss.NewStyle().Foreground(colorMuted)
	stylePrimary = lipgloss.NewStyle().Foreground(colorPrimary)
	styleSuccess = lipgloss.NewStyle().Foreground(colorSuccess)
	styleWarning = lipgloss.NewStyle().Foreground(colorWarning).Bold(true)
	styleError = lipgloss.NewStyle().Foreground(colorError)
	styleAccent = lipgloss.NewStyle().Foreground(colorAccent)

	styleSelected = lipgloss.NewStyle().Foreground(contrastFg).Background(colorPrimary).Bold(true)
	styleTagBadge = lipgloss.NewStyle().Foreground(colorAccent)
	styleDone = lipgloss.NewStyle().Foreground(colorMuted).Strikethrough(true)

	styleModeNormal = lipgloss.NewStyle().Foreground(contrastFg).Background(colorPrimary).Bold(true).Padding(0, 1)
	styleModeInsert = lipgloss.NewStyle().Foreground(contrastFg).Background(colorSuccess).Bold(true).Padding(0, 1)
	styleModeSearch = lipgloss.NewStyle().Foreground(contrastFg).Background(colorAccent).Bold(true).Padding(0, 1)
	styleModeCommand = lipgloss.NewStyle().Foreground(contrastFg).Background(colorWarning).Bold(true).Padding(0, 1)

	styleStatusBar = lipgloss.NewStyle().Foreground(colorFg)

	editorStyle = EditorStyle{
		Text:   styleBase,
		Cursor: lipgloss.NewStyle().Foreground(contrastFg).Background(colorFg),
	}
}
