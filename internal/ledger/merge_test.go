package ledger

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/techspeque/metis/internal/slice"
	"github.com/techspeque/metis/internal/threeway"
)

// led builds a ledger from "id" or "id<dep,dep" fields; with() edits one.
func led(spec string) *Ledger {
	l := &Ledger{Version: 1, Slices: []slice.Slice{}}
	for _, f := range strings.Fields(spec) {
		id, deps, _ := strings.Cut(f, "<")
		s := slice.Slice{ID: id, Title: id}
		if deps != "" {
			s.BlockedBy = strings.Split(deps, ",")
		}
		l.Slices = append(l.Slices, s)
	}
	return l
}

func (l *Ledger) with(id string, edit func(*slice.Slice)) *Ledger {
	edit(l.FindByID(id))
	return l
}

func show(l *Ledger) string {
	parts := make([]string, 0, len(l.Slices))
	for i := range l.Slices {
		s := &l.Slices[i]
		p := s.ID
		if s.Coded {
			p += "+coded"
		}
		if len(s.BlockedBy) > 0 {
			p += "<" + strings.Join(s.BlockedBy, ",")
		}
		parts = append(parts, p)
	}
	return strings.Join(parts, " ")
}

func coded(s *slice.Slice) { s.Coded = true }

func TestMergeLedger(t *testing.T) {
	for _, tc := range []struct {
		name               string
		base, ours, theirs *Ledger
		want               string
	}{
		{"each side works its own slice",
			led("a b c"), led("a b c").with("a", coded), led("a b c").with("b", coded), "a+coded b+coded c"},
		{"ours archived a, theirs flips b",
			led("a b"), led("b"), led("a b").with("b", coded), "b+coded"},
		{"both archive",
			led("a b c"), led("b c"), led("a c"), "c"},
		{"ours' archive strips the blocked_by of the slice theirs works on",
			led("a b<a"), led("b"), led("a b<a").with("b", coded), "b+coded"},
		{"each archive strips one dependency of a third slice",
			led("a b c<a,b"), led("b c<b"), led("a c<a"), "c"},
		{"theirs adds a slice blocked by one ours archived",
			led("a b"), led("b"), led("a b d<a,b"), "b d<b"},
		{"a dependency added on each side",
			led("a b c d<a"), led("a b c d<a,b"), led("a b c d<a,c"), "a b c d<a,b,c"},
		{"a dependency dropped by hand stays dropped",
			led("a b c<a,b"), led("a b c<b"), led("a b c<a,b").with("c", coded), "a b c+coded<b"},
		{"theirs' new slice keeps its place",
			led("a b"), led("a b"), led("a x b"), "a x b"},
	} {
		got, err := Merge(tc.base, tc.ours, tc.theirs)
		if err != nil {
			t.Errorf("%s: %v", tc.name, err)
			continue
		}
		if show(got) != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, show(got), tc.want)
		}
	}
}

func TestMergeLedgerConflicts(t *testing.T) {
	note := func(text string) func(*slice.Slice) { return func(s *slice.Slice) { s.Notes = text } }
	for _, tc := range []struct {
		name               string
		base, ours, theirs *Ledger
	}{
		{"the same slice changed on both sides", led("a b"), led("a b").with("a", note("one")), led("a b").with("a", note("two"))},
		{"archived on one side, changed on the other", led("a b"), led("b"), led("a b").with("a", coded)},
	} {
		_, err := Merge(tc.base, tc.ours, tc.theirs)
		var c *threeway.Conflict
		if !errors.As(err, &c) || c.Key != "a" {
			t.Errorf("%s: got %v, want a conflict on a", tc.name, err)
		}
	}
}

func TestMergeArchive(t *testing.T) {
	arch := func(ids ...string) *Archive {
		a := &Archive{Version: 1, Slices: []slice.Slice{}}
		for _, id := range ids {
			a.Slices = append(a.Slices, slice.Slice{ID: id, Coded: true, Reviewed: true})
		}
		return a
	}
	got, err := MergeArchive(arch("z"), arch("z", "a"), arch("z", "b"))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, arch("z", "a", "b")) {
		t.Errorf("union with ours first: got %+v", got.Slices)
	}
	// No archive on the base or on ours: the file is new on the branch.
	if got, err = MergeArchive(&Archive{}, &Archive{}, arch("b")); err != nil || !reflect.DeepEqual(got, arch("b")) {
		t.Errorf("new archive: got %+v, %v", got, err)
	}

	theirs := arch("z", "a")
	theirs.Slices[1].ReviewCycles = 2
	_, err = MergeArchive(arch("z"), arch("z", "a"), theirs)
	var c *threeway.Conflict
	if !errors.As(err, &c) || c.Key != "a" {
		t.Errorf("the same id archived twice with different content: got %v", err)
	}
}
