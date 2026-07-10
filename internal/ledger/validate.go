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
	for _, s := range l.Slices {
		if s.ID == "" {
			errs = append(errs, fmt.Errorf("slice has empty ID"))
			continue
		}
		if seen[s.ID] {
			errs = append(errs, fmt.Errorf("duplicate slice ID: %s", s.ID))
		}
		seen[s.ID] = true
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
	for _, s := range l.Slices {
		if len(s.BlockedBy) > 0 {
			deps[s.ID] = s.BlockedBy
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

	for _, s := range l.Slices {
		if color[s.ID] == white {
			visit(s.ID)
		}
	}

	return errs
}

// ValidateArchive checks the archive for structural integrity.
// All entries must be fully done.
func ValidateArchive(a *Archive) []error {
	var errs []error
	for _, s := range a.Slices {
		if !s.IsDone() {
			errs = append(errs, fmt.Errorf("archive entry %s is not fully done (coded=%v, reviewed=%v)",
				s.ID, s.Coded, s.Reviewed))
		}
	}
	return errs
}
