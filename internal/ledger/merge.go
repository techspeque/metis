package ledger

import (
	"reflect"

	"github.com/techspeque/metis/internal/slice"
	"github.com/techspeque/metis/internal/threeway"
)

// Merge is the three-way merge of the ledger, by slice id: ours with
// theirs' changes since base. A slice changed on one side takes that side,
// one removed on one side (archived, retired) is removed, and the same
// slice changed on both sides is a *threeway.Conflict.
//
// blocked_by is not part of that comparison, since an archive on one side
// strips ids from slices the other side works on. It is merged as a set
// (a dependency either side added or dropped is added or dropped) and
// then cleared of every slice that has left the merged ledger.
func Merge(base, ours, theirs *Ledger) (*Ledger, error) {
	id := func(s *slice.Slice) string { return s.ID }
	same := func(a, b *slice.Slice) bool {
		x, y := *a, *b
		x.BlockedBy, y.BlockedBy = nil, nil
		return reflect.DeepEqual(x, y)
	}
	merged, err := threeway.Merge(base.Slices, ours.Slices, theirs.Slices, id, same, true)
	if err != nil {
		return nil, err
	}

	present := map[string]bool{}
	for i := range merged {
		present[merged[i].ID] = true
	}
	gone := map[string]bool{}
	for _, l := range []*Ledger{base, ours, theirs} {
		for i := range l.Slices {
			if !present[l.Slices[i].ID] {
				gone[l.Slices[i].ID] = true
			}
		}
	}
	for i := range merged {
		s := &merged[i]
		var deps []string
		for _, dep := range mergeDeps(base.FindByID(s.ID), ours.FindByID(s.ID), theirs.FindByID(s.ID)) {
			if !gone[dep] {
				deps = append(deps, dep)
			}
		}
		s.BlockedBy = deps
	}
	return &Ledger{Version: max(ours.Version, theirs.Version), Slices: merged}, nil
}

// mergeDeps merges one slice's blocked_by as a set, in ours' order with
// theirs' additions after it.
func mergeDeps(base, ours, theirs *slice.Slice) []string {
	if ours == nil {
		return theirs.BlockedBy
	}
	if theirs == nil {
		return ours.BlockedBy
	}
	inBase, inOurs, inTheirs := map[string]bool{}, map[string]bool{}, map[string]bool{}
	if base != nil {
		for _, dep := range base.BlockedBy {
			inBase[dep] = true
		}
	}
	for _, dep := range ours.BlockedBy {
		inOurs[dep] = true
	}
	for _, dep := range theirs.BlockedBy {
		inTheirs[dep] = true
	}
	var deps []string
	for _, dep := range ours.BlockedBy {
		if inTheirs[dep] || !inBase[dep] {
			deps = append(deps, dep)
		}
	}
	for _, dep := range theirs.BlockedBy {
		if !inOurs[dep] && !inBase[dep] {
			deps = append(deps, dep)
		}
	}
	return deps
}

// MergeArchive is the three-way merge of the archive, by slice id: the
// union of both sides, ours' entries first. The same id archived on both
// sides with different content is a *threeway.Conflict.
func MergeArchive(base, ours, theirs *Archive) (*Archive, error) {
	id := func(s *slice.Slice) string { return s.ID }
	same := func(a, b *slice.Slice) bool { return reflect.DeepEqual(*a, *b) }
	merged, err := threeway.Merge(base.Slices, ours.Slices, theirs.Slices, id, same, false)
	if err != nil {
		return nil, err
	}
	return &Archive{Version: max(ours.Version, theirs.Version), Slices: merged}, nil
}
