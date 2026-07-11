package ledger

import (
	"fmt"

	"github.com/techspeque/metis/internal/slice"
)

// Validate checks the ledger for structural errors.
// agentSlugs is the set of valid agent slugs from the config (can be nil to skip slug checks).
func (l *Ledger) Validate(agentSlugs map[string]bool) []error {
	var errs []error

	// Check for unique IDs
	seen := make(map[string]bool)
	for i := range l.Slices {
		if l.Slices[i].ID == "" {
			errs = append(errs, fmt.Errorf("slice has empty ID"))
			continue
		}
		if seen[l.Slices[i].ID] {
			errs = append(errs, fmt.Errorf("duplicate slice ID: %s", l.Slices[i].ID))
		}
		seen[l.Slices[i].ID] = true
	}

	// Validate each slice
	for i := range l.Slices {
		s := &l.Slices[i]

		// Delegate structural validation to slice
		sliceErrs := s.Validate()
		errs = append(errs, sliceErrs...)

		// Check agent slugs against config if provided
		if agentSlugs != nil {
			if s.Coder != "" && !agentSlugs[s.Coder] {
				errs = append(errs, fmt.Errorf("%s: coder %q not in agents map", s.ID, s.Coder))
			}
			if s.Reviewer != "" && !agentSlugs[s.Reviewer] {
				errs = append(errs, fmt.Errorf("%s: reviewer %q not in agents map", s.ID, s.Reviewer))
			}
		}

		// Validate ID format
		if s.ID != "" && !slice.IsValidID(s.ID) {
			errs = append(errs, fmt.Errorf("%s: invalid ID format (must be lowercase alphanumeric + hyphens/dots)", s.ID))
		}

		// Validate blocked_by references exist
		for _, dep := range s.BlockedBy {
			if !seen[dep] {
				errs = append(errs, fmt.Errorf("%s: blocked_by references unknown slice %q", s.ID, dep))
			}
		}
	}

	// Check for circular dependencies
	if cyclicErrs := l.detectCycles(); len(cyclicErrs) > 0 {
		errs = append(errs, cyclicErrs...)
	}

	return errs
}

// detectCycles checks for circular dependencies in blocked_by.
func (l *Ledger) detectCycles() []error {
	var errs []error

	// Build adjacency map: ID -> IDs it's blocked by
	deps := make(map[string][]string)
	for i := range l.Slices {
		if len(l.Slices[i].BlockedBy) > 0 {
			deps[l.Slices[i].ID] = l.Slices[i].BlockedBy
		}
	}

	// DFS-based cycle detection
	const (
		white = 0 // unvisited
		gray  = 1 // in current path
		black = 2 // fully explored
	)
	color := make(map[string]int)

	var visit func(id string) bool
	visit = func(id string) bool {
		color[id] = gray
		for _, dep := range deps[id] {
			switch color[dep] {
			case gray:
				errs = append(errs, fmt.Errorf("circular dependency detected involving %s and %s", id, dep))
				return true
			case white:
				if visit(dep) {
					return true
				}
			}
		}
		color[id] = black
		return false
	}

	for i := range l.Slices {
		if color[l.Slices[i].ID] == white {
			visit(l.Slices[i].ID)
		}
	}

	return errs
}

// ValidateArchive checks the archive for structural integrity.
// All entries must be fully done.
func ValidateArchive(a *Archive) []error {
	var errs []error
	for i := range a.Slices {
		if !a.Slices[i].IsDone() {
			errs = append(errs, fmt.Errorf("archive entry %s is not fully done (coded=%v, reviewed=%v)",
				a.Slices[i].ID, a.Slices[i].Coded, a.Slices[i].Reviewed))
		}
	}
	return errs
}
