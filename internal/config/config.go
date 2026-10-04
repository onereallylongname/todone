// Package config handles user configuration for todone.
// Configuration is loaded from ~/.config/todone/config.json (mirrors
// avredit's internal/config package for consistency across both tools).
package config

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/onereallylongname/todone/internal/store"
)

// Config holds user preferences.
type Config struct {
	// Store is the path to the YAML task file. Empty means "use the
	// default" (~/.config/todone/todo.yaml) — resolved by ResolveStorePath,
	// not baked in here, so the default can move without rewriting configs.
	Store string `json:"store,omitempty"`

	// DefaultSort is one of "updated", "date", "alpha".
	DefaultSort string `json:"default_sort"`

	// ShowDone starts the TUI with done tasks visible.
	ShowDone bool `json:"show_done"`
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		DefaultSort: "updated",
		ShowDone:    false,
	}
}

// Dir returns the config directory path (~/.config/todone).
func Dir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "todone")
}

// FilePath returns the full path to config.json.
func FilePath() string {
	return filepath.Join(Dir(), "config.json")
}

// Load reads the config from the standard path. Returns defaults if the
// file doesn't exist or fails to parse — todone always starts up clean.
func Load() Config {
	cfg := DefaultConfig()
	data, err := os.ReadFile(FilePath())
	if err != nil {
		return cfg
	}
	_ = json.Unmarshal(data, &cfg)
	return cfg
}

// ResolveStorePath picks the data file path using, in order: an explicit
// flag value, the $TODONE_FILE env var, the config's Store field, then the
// package default. flagVal should be "" when --file wasn't passed.
func (c Config) ResolveStorePath(flagVal string) string {
	if flagVal != "" {
		return flagVal
	}
	if env := os.Getenv("TODONE_FILE"); env != "" {
		return env
	}
	if c.Store != "" {
		return c.Store
	}
	return store.DefaultPath()
}

// Save writes the config to the standard path.
func (c Config) Save() error {
	dir := Dir()
	if dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(FilePath(), data, 0o644)
}
