package findings

import (
	"errors"
	"strings"
	"testing"

	"github.com/techspeque/metis/internal/threeway"
)

// store builds a store from "id:slice:text" fields.
func store(spec string) *Store {
	s := &Store{Findings: []Finding{}}
	for _, f := range strings.Fields(spec) {
		p := strings.Split(f, ":")
		s.Findings = append(s.Findings, Finding{ID: p[0], Date: "2026-11-27", Slice: p[1], Finding: p[2], Status: "open"})
	}
	return s
}

func (s *Store) with(id string, edit func(*Finding)) *Store {
	edit(s.byID(id))
	return s
}

// show lists a store as "id:text" with "=alias", "+status" and "^follow_on".
func show(s *Store) string {
	parts := make([]string, 0, len(s.Findings))
	for i := range s.Findings {
		f := &s.Findings[i]
		p := f.ID + ":" + f.Finding
		if f.Alias != "" {
			p += "=" + f.Alias
		}
		if f.Status != "open" {
			p += "+" + f.Status
		}
		if f.FollowOn != "" {
			p += "^" + f.FollowOn
		}
		parts = append(parts, p)
	}
	return strings.Join(parts, " ")
}

func resolved(f *Finding) { f.Status = "resolved" }

func mustMerge(t *testing.T, base, ours, theirs *Store) *Store {
	t.Helper()
	got, err := Merge(base, ours, theirs)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestMergeFindings(t *testing.T) {
	for _, tc := range []struct {
		name               string
		base, ours, theirs *Store
		want               string
	}{
		{"union",
			store("f-001:z:old"), store("f-001:z:old f-002:a:one"), store("f-001:z:old f-003:b:two"),
			"f-001:old f-002:one f-003:two"},
		{"each side closes its own",
			store("f-001:a:one f-002:b:two"), store("f-001:a:one f-002:b:two").with("f-001", resolved), store("f-001:a:one f-002:b:two").with("f-002", resolved),
			"f-001:one+resolved f-002:two+resolved"},
		{"an id created on both sides is renumbered on theirs",
			store("f-001:z:old"), store("f-001:z:old f-002:a:one"), store("f-001:z:old f-002:b:two"),
			"f-001:old f-002:one f-003:two=f-002"},
		{"no store on the base: both sides create f-001",
			&Store{}, store("f-001:a:one"), store("f-001:b:two"),
			"f-001:one f-002:two=f-001"},
		{"theirs' references follow the renumbering",
			store(""), store("f-001:a:one"), store("f-001:b:two f-002:b:again").with("f-002", func(f *Finding) { f.FollowOn = "f-001" }),
			"f-001:one f-003:two=f-001 f-002:again^f-003"},
	} {
		if got := show(mustMerge(t, tc.base, tc.ours, tc.theirs)); got != tc.want {
			t.Errorf("%s:\n got %q\nwant %q", tc.name, got, tc.want)
		}
	}
}

// A rebase replays the branch one commit at a time: after the first step
// renumbers a finding, the later commits still name it by its old id.
func TestMergeFindingsAcrossRebaseSteps(t *testing.T) {
	fork := store("")
	work := store("f-001:a:one")
	c1 := store("f-001:b:two")                         // the branch raises its own f-001
	c2 := store("f-001:b:two").with("f-001", resolved) // and resolves it
	c3 := store("f-001:b:two f-002:b:three").with("f-001", resolved)
	c4 := store("f-001:b:two f-002:b:three").with("f-001", resolved).with("f-002", resolved)

	head := mustMerge(t, fork, work, c1)
	if got, want := show(head), "f-001:one f-002:two=f-001"; got != want {
		t.Fatalf("step 1: got %q, want %q", got, want)
	}
	head = mustMerge(t, c1, head, c2)
	if got, want := show(head), "f-001:one f-002:two=f-001+resolved"; got != want {
		t.Fatalf("step 2: got %q, want %q", got, want)
	}
	// The branch's own f-002 meets the id its first finding now has.
	head = mustMerge(t, c2, head, c3)
	if got, want := show(head), "f-001:one f-002:two=f-001+resolved f-003:three=f-002"; got != want {
		t.Fatalf("step 3: got %q, want %q", got, want)
	}
	head = mustMerge(t, c3, head, c4)
	if got, want := show(head), "f-001:one f-002:two=f-001+resolved f-003:three=f-002+resolved"; got != want {
		t.Fatalf("step 4: got %q, want %q", got, want)
	}
}

func TestMergeFindingsConflict(t *testing.T) {
	base := store("f-001:a:one")
	ours := store("f-001:a:one").with("f-001", resolved)
	theirs := store("f-001:a:one").with("f-001", func(f *Finding) { f.Status = "waived" })
	_, err := Merge(base, ours, theirs)
	var c *threeway.Conflict
	if !errors.As(err, &c) || c.Key != "f-001" {
		t.Errorf("the same finding changed on both sides: got %v", err)
	}
}

func TestFindByIDResolvesAnAlias(t *testing.T) {
	s := store("f-001:a:one f-002:b:two f-003:c:three")
	s.Findings[1].Alias = "f-001"
	s.Findings[2].Alias = "f-009"
	if f := s.FindByID("f-001"); f == nil || f.Finding != "one" {
		t.Errorf("an id a finding has beats an alias: got %+v", f)
	}
	if f := s.FindByID("f-009"); f == nil || f.ID != "f-003" {
		t.Errorf("alias f-009: got %+v", f)
	}
	if s.FindByID("") != nil || s.FindByID("f-404") != nil {
		t.Error("an unknown id resolves to nothing")
	}
	s.Findings[0].Alias = "f-009"
	if s.FindByID("f-009") != nil {
		t.Error("an alias two findings carry resolves to neither")
	}

	// A reference given by alias is stored as the id.
	s = store("f-001:a:one f-002:b:two f-003:b:three")
	s.Findings[1].Alias = "f-007"
	if err := s.Follow("f-003", "f-007"); err != nil || s.Findings[2].FollowOn != "f-002" {
		t.Errorf("follow-on by alias: %v, %+v", err, s.Findings[2])
	}
	if err := s.Dispose("f-001", "duplicate", "f-007"); err != nil || s.Findings[0].Of != "f-002" {
		t.Errorf("duplicate-of by alias: %v, %+v", err, s.Findings[0])
	}
	if err := s.Follow("f-002", "f-007"); err == nil {
		t.Error("a finding cannot follow itself through its alias")
	}
}
