package ledger

import (
	"fmt"

	"github.com/techspeque/metis/internal/slice"
)

// FlipCoded marks a slice as coded. Validates preconditions.
func (l *Ledger) FlipCoded(id string) error {
	s := l.FindByID(id)
	if s == nil {
		return fmt.Errorf("slice %q not found", id)
	}
	if s.Coded {
		return fmt.Errorf("slice %q is already coded", id)
	}
	s.Coded = true
	return nil
}

// FlipReviewed marks a slice as reviewed. Validates preconditions.
func (l *Ledger) FlipReviewed(id string, agent string) error {
	s := l.FindByID(id)
	if s == nil {
		return fmt.Errorf("slice %q not found", id)
	}
	if !s.Coded {
		return fmt.Errorf("slice %q is not yet coded — cannot review", id)
	}
	if s.Reviewed {
		return fmt.Errorf("slice %q is already reviewed", id)
	}
	if agent != "" && agent == s.Coder {
		return fmt.Errorf("slice %q: reviewer (%s) cannot be the same as coder (%s)", id, agent, s.Coder)
	}
	s.Reviewed = true
	return nil
}

// Block rejects a slice during review: resets coded, increments review_cycles.
func (l *Ledger) Block(id string) error {
	s := l.FindByID(id)
	if s == nil {
		return fmt.Errorf("slice %q not found", id)
	}
	if !s.Coded {
		return fmt.Errorf("slice %q is not coded — nothing to block", id)
	}
	if s.Reviewed {
		return fmt.Errorf("slice %q is already reviewed — cannot block", id)
	}
	s.Coded = false
	s.ReviewCycles++
	return nil
}

// Skip marks a slice as done without implementation (both coded and reviewed = true).
func (l *Ledger) Skip(id string, reason string) error {
	s := l.FindByID(id)
	if s == nil {
		return fmt.Errorf("slice %q not found", id)
	}
	if s.IsDone() {
		return fmt.Errorf("slice %q is already done", id)
	}
	s.Coded = true
	s.Reviewed = true
	if reason != "" {
		if s.Notes != "" {
			s.Notes += "; "
		}
		s.Notes += "skipped: " + reason
	}
	return nil
}

// Reopen resets a slice to uncoded/unreviewed state for re-implementation.
func (l *Ledger) Reopen(id string, reason string) error {
	s := l.FindByID(id)
	if s == nil {
		return fmt.Errorf("slice %q not found", id)
	}
	s.Coded = false
	s.Reviewed = false
	if reason != "" {
		if s.Notes != "" {
			s.Notes += "; "
		}
		s.Notes += "reopened: " + reason
	}
	return nil
}

// Archive moves all done slices from the ledger to the archive.
// Returns the IDs of archived slices.
func (l *Ledger) Archive(archive *Archive) []string {
	var archived []string
	var remaining []slice.Slice

	for _, s := range l.Slices {
		if s.IsDone() {
			archive.Slices = append(archive.Slices, s)
			archived = append(archived, s.ID)
		} else {
			remaining = append(remaining, s)
		}
	}

	if remaining == nil {
		remaining = []slice.Slice{}
	}
	l.Slices = remaining
	return archived
}
