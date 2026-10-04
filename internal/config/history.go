package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// History holds persistent command and search history, ported from
// avredit's internal/config/history.go so todone's `/` and `:` bars get the
// same arrow-key recall across restarts.
type History struct {
	Commands []string `json:"commands"`
	Searches []string `json:"searches"`
}

const maxHistoryItems = 100

// HistoryPath returns the full path to the history file.
func HistoryPath() string {
	return filepath.Join(Dir(), "history.json")
}

// LoadHistory reads persistent history from disk. Returns empty history if
// the file doesn't exist.
func LoadHistory() History {
	var h History
	data, err := os.ReadFile(HistoryPath())
	if err != nil {
		return h
	}
	_ = json.Unmarshal(data, &h)
	return h
}

// SaveHistory writes history to disk, creating the config dir if needed,
// trimmed to the most recent maxHistoryItems entries each.
func SaveHistory(h History) {
	if len(h.Commands) > maxHistoryItems {
		h.Commands = h.Commands[len(h.Commands)-maxHistoryItems:]
	}
	if len(h.Searches) > maxHistoryItems {
		h.Searches = h.Searches[len(h.Searches)-maxHistoryItems:]
	}
	dir := Dir()
	_ = os.MkdirAll(dir, 0o755)
	data, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(HistoryPath(), data, 0o644)
}
