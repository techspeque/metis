package findings

import (
	"fmt"
	"reflect"

	"github.com/techspeque/metis/internal/threeway"
)

// Merge is the three-way merge of the findings store, by finding id: the
// union of both sides, ours' findings first. A finding changed on one
// side takes that side; the same finding changed on both sides is a
// *threeway.Conflict.
//
// An id both sides created for different findings is not a conflict:
// theirs' finding (the branch being landed, in a rebase) is renumbered to
// the next free id and keeps the old one as its alias, and theirs'
// follow_on and of references move with it. Later steps of the same
// rebase still name the finding by its old id; they are matched to the
// renumbered finding by what it is (slice, date and text).
func Merge(base, ours, theirs *Store) (*Store, error) {
	rename := renumbering(base, ours, theirs)
	base, theirs = base.renumbered(rename), theirs.renumbered(rename)
	id := func(f *Finding) string { return f.ID }
	same := func(a, b *Finding) bool { return reflect.DeepEqual(*a, *b) }
	merged, err := threeway.Merge(base.Findings, ours.Findings, theirs.Findings, id, same, false)
	if err != nil {
		return nil, err
	}
	return &Store{Findings: merged}, nil
}

// renumbering maps the ids of base and theirs that name a different
// finding in ours to the id that finding has, or gets, in the merge.
func renumbering(base, ours, theirs *Store) map[string]string {
	next := 0
	for _, s := range []*Store{base, ours, theirs} {
		for i := range s.Findings {
			var n int
			if _, err := fmt.Sscanf(s.Findings[i].ID, "f-%d", &n); err == nil && n > next {
				next = n
			}
		}
	}
	rename := map[string]string{}
	consider := func(f *Finding, created bool) {
		if _, done := rename[f.ID]; done {
			return
		}
		mine := ours.byID(f.ID)
		if mine != nil && sameFinding(mine, f) {
			return
		}
		// Renumbered at an earlier step of this rebase.
		for i := range ours.Findings {
			if o := &ours.Findings[i]; o.Alias == f.ID && sameFinding(o, f) {
				rename[f.ID] = o.ID
				return
			}
		}
		if mine != nil && created {
			next++
			rename[f.ID] = fmt.Sprintf("f-%03d", next)
		}
	}
	for i := range theirs.Findings {
		consider(&theirs.Findings[i], base.byID(theirs.Findings[i].ID) == nil)
	}
	for i := range base.Findings {
		consider(&base.Findings[i], false)
	}
	return rename
}

// sameFinding reports whether two rows are one finding, whatever has been
// done to it since: nothing rewrites a finding's slice, date or text.
func sameFinding(a, b *Finding) bool {
	return a.Slice == b.Slice && a.Date == b.Date && a.Finding == b.Finding
}

// byID is the finding with exactly this id, aliases aside.
func (s *Store) byID(id string) *Finding {
	for i := range s.Findings {
		if s.Findings[i].ID == id {
			return &s.Findings[i]
		}
	}
	return nil
}

// renumbered returns a copy of the store with ids renamed, each renamed
// finding keeping its old id as its alias, and references following.
func (s *Store) renumbered(rename map[string]string) *Store {
	out := &Store{Findings: append([]Finding{}, s.Findings...)}
	for i := range out.Findings {
		f := &out.Findings[i]
		if id, ok := rename[f.ID]; ok {
			f.Alias, f.ID = f.ID, id
		}
		if id, ok := rename[f.FollowOn]; ok {
			f.FollowOn = id
		}
		if id, ok := rename[f.Of]; ok {
			if f.ResolvedBy == f.Disposition+" of "+f.Of {
				f.ResolvedBy = f.Disposition + " of " + id
			}
			f.Of = id
		}
	}
	return out
}
