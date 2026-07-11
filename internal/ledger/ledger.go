// Package ledger manages the slice ledger — the dispatch state for Metis.
package ledger

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/techspeque/metis/internal/slice"
)

// Ledger represents the active slice ledger (.metis/slices.yaml).
type Ledger struct {
	Version int           `yaml:"version"`
	Slices  []slice.Slice `yaml:"slices"`
}

// Archive represents the completed slice archive (.metis/slices-done.yaml).
type Archive struct {
	Version int           `yaml:"version"`
	Slices  []slice.Slice `yaml:"slices"`
}

// Load reads and parses the ledger from a YAML file.
func Load(path string) (*Ledger, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Ledger{Version: 1, Slices: []slice.Slice{}}, nil
		}
		return nil, fmt.Errorf("reading ledger: %w", err)
	}
	return Parse(data)
}

// Parse parses ledger content from bytes.
func Parse(data []byte) (*Ledger, error) {
	var l Ledger
	if err := yaml.Unmarshal(data, &l); err != nil {
		return nil, fmt.Errorf("parsing ledger: %w", err)
	}
	if l.Slices == nil {
		l.Slices = []slice.Slice{}
	}
	return &l, nil
}

// Save writes the ledger to a YAML file.
func (l *Ledger) Save(path string) error {
	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating ledger directory: %w", err)
	}

	data, err := yaml.Marshal(l)
	if err != nil {
		return fmt.Errorf("marshaling ledger: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing ledger: %w", err)
	}
	return nil
}

// LoadArchive reads and parses the archive file.
func LoadArchive(path string) (*Archive, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Archive{Version: 1, Slices: []slice.Slice{}}, nil
		}
		return nil, fmt.Errorf("reading archive: %w", err)
	}
	var a Archive
	if err := yaml.Unmarshal(data, &a); err != nil {
		return nil, fmt.Errorf("parsing archive: %w", err)
	}
	if a.Slices == nil {
		a.Slices = []slice.Slice{}
	}
	return &a, nil
}

// SaveArchive writes the archive to a YAML file.
func (a *Archive) Save(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating archive directory: %w", err)
	}

	data, err := yaml.Marshal(a)
	if err != nil {
		return fmt.Errorf("marshaling archive: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing archive: %w", err)
	}
	return nil
}

// FindByID returns a pointer to the slice with the given ID, or nil if not found.
func (l *Ledger) FindByID(id string) *slice.Slice {
	for i := range l.Slices {
		if l.Slices[i].ID == id {
			return &l.Slices[i]
		}
	}
	return nil
}

// IDs returns all slice IDs in the ledger.
func (l *Ledger) IDs() []string {
	ids := make([]string, len(l.Slices))
	for i := range l.Slices {
		ids[i] = l.Slices[i].ID
	}
	return ids
}

// Add appends a new slice to the ledger.
func (l *Ledger) Add(s *slice.Slice) error {
	if l.FindByID(s.ID) != nil {
		return fmt.Errorf("slice ID %q already exists", s.ID)
	}
	l.Slices = append(l.Slices, *s)
	return nil
}

// AddAfter inserts a new slice after the given reference ID.
func (l *Ledger) AddAfter(s *slice.Slice, afterID string) error {
	if l.FindByID(s.ID) != nil {
		return fmt.Errorf("slice ID %q already exists", s.ID)
	}
	for i := range l.Slices {
		if l.Slices[i].ID == afterID {
			// Insert after position i
			l.Slices = append(l.Slices[:i+1], append([]slice.Slice{*s}, l.Slices[i+1:]...)...)
			return nil
		}
	}
	return fmt.Errorf("reference slice %q not found", afterID)
}

// AddBefore inserts a new slice before the given reference ID.
func (l *Ledger) AddBefore(s *slice.Slice, beforeID string) error {
	if l.FindByID(s.ID) != nil {
		return fmt.Errorf("slice ID %q already exists", s.ID)
	}
	for i := range l.Slices {
		if l.Slices[i].ID == beforeID {
			// Insert before position i
			l.Slices = append(l.Slices[:i], append([]slice.Slice{*s}, l.Slices[i:]...)...)
			return nil
		}
	}
	return fmt.Errorf("reference slice %q not found", beforeID)
}

// Remove deletes a slice from the ledger by ID.
func (l *Ledger) Remove(id string) error {
	for i := range l.Slices {
		if l.Slices[i].ID == id {
			l.Slices = append(l.Slices[:i], l.Slices[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("slice %q not found", id)
}
