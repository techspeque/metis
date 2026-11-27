package threeway

import (
	"errors"
	"strings"
	"testing"
)

type row struct{ id, v string }

func rows(s string) []row {
	var out []row
	for _, f := range strings.Fields(s) {
		id, v, _ := strings.Cut(f, "=")
		out = append(out, row{id, v})
	}
	return out
}

func text(list []row) string {
	parts := make([]string, 0, len(list))
	for _, r := range list {
		parts = append(parts, r.id+"="+r.v)
	}
	return strings.Join(parts, " ")
}

func merge(base, ours, theirs string, anchored bool) (string, error) {
	out, err := Merge(rows(base), rows(ours), rows(theirs),
		func(r *row) string { return r.id }, func(a, b *row) bool { return *a == *b }, anchored)
	return text(out), err
}

func TestMerge(t *testing.T) {
	for _, tc := range []struct {
		name, base, ours, theirs, want string
		anchored                       bool
	}{
		{"changed on theirs", "a=1 b=1", "a=1 b=1", "a=1 b=2", "a=1 b=2", false},
		{"changed on ours", "a=1 b=1", "a=2 b=1", "a=1 b=1", "a=2 b=1", false},
		{"each side changes its own", "a=1 b=1", "a=2 b=1", "a=1 b=2", "a=2 b=2", false},
		{"removed on ours", "a=1 b=1", "b=1", "a=1 b=2", "b=2", false},
		{"removed on theirs", "a=1 b=1", "a=2 b=1", "a=2", "a=2", false},
		{"removed on both", "a=1 b=1", "b=1", "b=1", "b=1", false},
		{"both add, theirs appended", "a=1", "a=1 b=1", "a=1 c=1", "a=1 b=1 c=1", false},
		{"same change on both", "a=1", "a=2", "a=2", "a=2", false},
		{"same addition on both", "", "a=1", "a=1", "a=1", false},
		{"anchored after its predecessor", "a=1 b=1", "a=1 b=1 x=1", "a=1 c=1 b=1", "a=1 c=1 b=1 x=1", true},
		{"anchored at the front", "a=1", "a=1", "c=1 a=1", "c=1 a=1", true},
		{"anchored run keeps its order", "a=1", "a=1 x=1", "a=1 c=1 d=1", "a=1 c=1 d=1 x=1", true},
		{"anchored, predecessor gone", "a=1 b=1", "b=1", "a=1 c=1 b=1", "c=1 b=1", true},
	} {
		got, err := merge(tc.base, tc.ours, tc.theirs, tc.anchored)
		if err != nil || got != tc.want {
			t.Errorf("%s: got %q, %v; want %q", tc.name, got, err, tc.want)
		}
	}
}

func TestMergeConflicts(t *testing.T) {
	for _, tc := range []struct{ name, base, ours, theirs, want string }{
		{"changed on both", "a=1", "a=2", "a=3", "a changed on both sides"},
		{"added on both", "", "a=1", "a=2", "a added on both sides with different content"},
		{"changed on ours, removed on theirs", "a=1", "a=2", "", "a changed on one side and removed on the other"},
		{"removed on ours, changed on theirs", "a=1", "", "a=2", "a changed on one side and removed on the other"},
	} {
		_, err := merge(tc.base, tc.ours, tc.theirs, false)
		var c *Conflict
		if !errors.As(err, &c) || err.Error() != tc.want {
			t.Errorf("%s: got %v, want %q", tc.name, err, tc.want)
		}
	}
}
