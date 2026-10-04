package ledger

import (
	"fmt"
	"time"

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
	s.Waiting = nil
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
	s.Waiting = nil
	return nil
}

// Block rejects a slice during review: resets coded, increments review_cycles.
// BlockedForReview reports whether a review has already blocked the slice
// in its current cycle: another blocking finding joins that cycle.
func (l *Ledger) BlockedForReview(id string) bool {
	s := l.FindByID(id)
	return s != nil && !s.Coded && !s.Reviewed && s.ReviewCycles > 0
}

// Wait parks a slice on something outside the repository until it is
// cleared, or until the given instant when one is set.
func (l *Ledger) Wait(id, reason string, until time.Time, now time.Time) error {
	s := l.FindByID(id)
	if s == nil {
		return fmt.Errorf("slice %q not found", id)
	}
	if s.IsDone() {
		return fmt.Errorf("slice %q is already done", id)
	}
	if reason == "" {
		return fmt.Errorf("slice %q: a wait needs a reason", id)
	}
	w := &slice.Wait{Reason: reason, Since: now.UTC().Format(time.RFC3339)}
	if !until.IsZero() {
		if !until.After(now) {
			return fmt.Errorf("slice %q: --until %s is not in the future", id, until.UTC().Format(time.RFC3339))
		}
		w.Until = until.UTC().Format(time.RFC3339)
	}
	s.Waiting = w
	return nil
}

// ClearWait returns a parked slice to dispatch.
func (l *Ledger) ClearWait(id string) error {
	s := l.FindByID(id)
	if s == nil {
		return fmt.Errorf("slice %q not found", id)
	}
	if s.Waiting == nil {
		return fmt.Errorf("slice %q is not waiting", id)
	}
	s.Waiting = nil
	return nil
}

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

// Retire moves a slice the plan no longer needs to the archive, marked
// removed with the reason in its notes. Nothing is erased — the audit trail
// keeps the slice and why it left. Dependents' blocked_by lists drop the
// retired ID so they dispatch instead of deadlocking (same rule as Archive).
func (l *Ledger) Retire(archive *Archive, id, reason string) error {
	s := l.FindByID(id)
	if s == nil {
		return fmt.Errorf("slice %q not found", id)
	}
	if s.IsDone() {
		return fmt.Errorf("slice %q is done — completed work is archived by 'metis archive', not removed", id)
	}
	s.Removed = true
	if reason != "" {
		if s.Notes != "" {
			s.Notes += "; "
		}
		s.Notes += "removed: " + reason
	}
	archive.Slices = append(archive.Slices, *s)
	_ = l.Remove(id)
	for i := range l.Slices {
		var deps []string
		for _, dep := range l.Slices[i].BlockedBy {
			if dep != id {
				deps = append(deps, dep)
			}
		}
		l.Slices[i].BlockedBy = deps
	}
	return nil
}

// Archive moves all done slices from the ledger to the archive.
// Returns the IDs of archived slices.
func (l *Ledger) Archive(archive *Archive) []string {
	var archived []string
	var remaining []slice.Slice

	for i := range l.Slices {
		if l.Slices[i].IsDone() {
			archive.Slices = append(archive.Slices, l.Slices[i])
			archived = append(archived, l.Slices[i].ID)
		} else {
			remaining = append(remaining, l.Slices[i])
		}
	}

	if remaining == nil {
		remaining = []slice.Slice{}
	}

	// Archived slices are done by definition: strip them from remaining
	// blocked_by lists so dependents (gates especially) dispatch instead of
	// deadlocking on IDs that no longer exist in the active ledger.
	archivedSet := map[string]bool{}
	for _, id := range archived {
		archivedSet[id] = true
	}
	for i := range remaining {
		var deps []string
		for _, dep := range remaining[i].BlockedBy {
			if !archivedSet[dep] {
				deps = append(deps, dep)
			}
		}
		remaining[i].BlockedBy = deps
	}

	l.Slices = remaining
	return archived
}
