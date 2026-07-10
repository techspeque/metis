// Package ledger manages the slice ledger — the dispatch state for Metis.
package ledger

import (
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
