package store

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Store holds the in-memory task list plus the path it was loaded from.
type Store struct {
	Path  string
	Tasks []Task
}

// DefaultPath returns ~/.config/todone/todo.yaml.
func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "todo.yaml"
	}
	return filepath.Join(home, ".config", "todone", "todo.yaml")
}

// Load reads the store from path. A missing file is not an error — it
// yields an empty Store so first-run works with zero setup.
func Load(path string) (*Store, error) {
	s := &Store{Path: path}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if len(data) == 0 {
		return s, nil
	}
	if err := yaml.Unmarshal(data, &s.Tasks); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return s, nil
}

// Save writes the store atomically: marshal, write to a temp file in the
// same directory, then rename over the real path. This avoids the
// read-whole-file/rewrite-whole-file corruption risk present in todo.nu
// (an interrupted write there can truncate the store).
func (s *Store) Save() error {
	dir := filepath.Dir(s.Path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	data, err := yaml.Marshal(s.Tasks)
	if err != nil {
		return fmt.Errorf("encode tasks: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".todo-*.yaml.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) // no-op if the rename below succeeded

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpPath, s.Path); err != nil {
		return fmt.Errorf("rename into place: %w", err)
	}
	return nil
}

// Add appends a new task and returns it.
func (s *Store) Add(summary string, tags, links []string) Task {
	now := time.Now().Format(time.RFC3339)
	t := Task{
		ID:      NewID(),
		Done:    false,
		Summary: summary,
		Date:    now,
		Updated: "",
		Tags:    tags,
		Links:   links,
	}
	s.Tasks = append(s.Tasks, t)
	return t
}

// Index returns the slice index of the task with the given id, or -1.
func (s *Store) Index(id string) int {
	for i, t := range s.Tasks {
		if t.ID == id {
			return i
		}
	}
	return -1
}

// ToggleDone flips the done state of the task at idx and stamps Updated.
// Returns the previous (Task, ok) for undo purposes.
func (s *Store) ToggleDone(idx int) (Task, bool) {
	if idx < 0 || idx >= len(s.Tasks) {
		return Task{}, false
	}
	prev := s.Tasks[idx]
	s.Tasks[idx].Done = !s.Tasks[idx].Done
	s.Tasks[idx].Updated = time.Now().Format(time.RFC3339)
	return prev, true
}

// Delete removes the task at idx. Returns the removed (Task, ok) for undo.
func (s *Store) Delete(idx int) (Task, bool) {
	if idx < 0 || idx >= len(s.Tasks) {
		return Task{}, false
	}
	prev := s.Tasks[idx]
	s.Tasks = append(s.Tasks[:idx], s.Tasks[idx+1:]...)
	return prev, true
}

// Insert re-inserts a task at idx (used by undo after Delete).
func (s *Store) Insert(idx int, t Task) {
	if idx < 0 {
		idx = 0
	}
	if idx > len(s.Tasks) {
		idx = len(s.Tasks)
	}
	s.Tasks = append(s.Tasks, Task{})
	copy(s.Tasks[idx+1:], s.Tasks[idx:])
	s.Tasks[idx] = t
}

// Set replaces the task at idx wholesale (used by edit and undo).
func (s *Store) Set(idx int, t Task) bool {
	if idx < 0 || idx >= len(s.Tasks) {
		return false
	}
	s.Tasks[idx] = t
	return true
}

// Update replaces fields on the task at idx and stamps Updated. Returns the
// previous Task for undo.
func (s *Store) Update(idx int, summary string, tags, links []string) (Task, bool) {
	if idx < 0 || idx >= len(s.Tasks) {
		return Task{}, false
	}
	prev := s.Tasks[idx]
	t := prev
	t.Summary = summary
	t.Tags = tags
	t.Links = links
	t.Updated = time.Now().Format(time.RFC3339)
	s.Tasks[idx] = t
	return prev, true
}
