package ledger

import (
	"github.com/techspeque/metis/internal/slice"
)

// DispatchResult contains the outcome of finding the next active slice.
type DispatchResult struct {
	Slice    *slice.Slice
	Role     slice.Role
	AgentSlug string
}

// Next finds the active slice using the dispatch algorithm:
// 1. Filter to unblocked slices (no blocked_by with incomplete deps)
// 2. Sort by priority (p0 > p1 > p2 > p3)
// 3. Within same priority, declaration order
// 4. First slice with coded=false -> role Coder
// 5. If coded but reviewed=false -> role Reviewer
// 6. If both true -> skip (should be archived)
//
// Returns nil if no active slice exists (backlog empty or all done).
func (l *Ledger) Next() *DispatchResult {
	// Build a set of completed slice IDs for blocked_by resolution
	doneIDs := make(map[string]bool)
	for _, s := range l.Slices {
		if s.IsDone() {
			doneIDs[s.ID] = true
		}
	}

	// Find the highest-priority unblocked actionable slice.
	// We iterate by priority level, then by declaration order within each level.
	var bestResult *DispatchResult
	bestRank := 100

	for i := range l.Slices {
		s := &l.Slices[i]

		// Skip done slices
		if s.IsDone() {
			continue
		}

		// Check if blocked
		if isBlocked(s, doneIDs) {
			continue
		}

		// Determine role
		role := s.ActiveRole()
		if role == "" {
			continue
		}

		// Check priority rank — lower rank = more urgent
		rank := s.Priority.Rank()
		if bestResult == nil || rank < bestRank {
			agentSlug := s.Coder
			if role == slice.RoleReviewer {
				agentSlug = s.Reviewer
			}
			bestResult = &DispatchResult{
				Slice:     s,
				Role:      role,
				AgentSlug: agentSlug,
			}
			bestRank = rank
		}
		// If same rank as best, the first one wins (declaration order)
		// since we already found bestResult at this rank, we skip
	}

	return bestResult
}

// isBlocked returns true if a slice has incomplete dependencies.
func isBlocked(s *slice.Slice, doneIDs map[string]bool) bool {
	for _, dep := range s.BlockedBy {
		if !doneIDs[dep] {
			return true
		}
	}
	return false
}

// PendingSlices returns all slices that are not yet done, in ledger order.
func (l *Ledger) PendingSlices() []slice.Slice {
	var result []slice.Slice
	for _, s := range l.Slices {
		if !s.IsDone() {
			result = append(result, s)
		}
	}
	return result
}

// DoneSlices returns all slices that are fully done (coded && reviewed).
func (l *Ledger) DoneSlices() []slice.Slice {
	var result []slice.Slice
	for _, s := range l.Slices {
		if s.IsDone() {
			result = append(result, s)
		}
	}
	return result
}
