package handoff

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/techspeque/metis/internal/config"
	"github.com/techspeque/metis/internal/findings"
	"github.com/techspeque/metis/internal/git"
	"github.com/techspeque/metis/internal/ledger"
	"github.com/techspeque/metis/internal/slice"
)

var update = flag.Bool("update", false, "rewrite the golden files")

const repo = "testdata/repo"

func fixture(t *testing.T, id string) Input {
	t.Helper()
	cfg, err := config.Load(filepath.Join(repo, ".metis", "project.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	l, err := ledger.Load(filepath.Join(repo, ".metis", "slices.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	store, err := findings.Load(filepath.Join(repo, ".metis", "findings.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	s := l.FindByID(id)
	if s == nil {
		t.Fatalf("no slice %s", id)
	}
	role := s.ActiveRole()
	slug := s.Coder
	if role == slice.RoleReviewer {
		slug = s.Reviewer
	}
	// A copy outside any git repository: the brief is read from disk, as
	// it would be before its first commit.
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS(repo)); err != nil {
		t.Fatal(err)
	}
	return Input{Cfg: cfg, RepoRoot: root, Slice: s, Role: role, AgentSlug: slug, Findings: store.ForSlice(id)}
}

func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", "golden", name+".md")
	if *update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run go test ./internal/handoff -update)", err)
	}
	if got != string(want) {
		t.Errorf("%s differs from %s:\n%s", name, path, got)
	}
}

func TestGoldenTemplates(t *testing.T) {
	cases := []struct {
		name, slice, tmpl string
		mutate            func(*Input)
	}{
		{"code", "phase-1-ws-1.1", Code, nil},
		{"rework", "phase-1-ws-1.2", Rework, nil},
		{"review", "phase-1-ws-1.1", Review, func(in *Input) {
			in.Slice.Coded = true
			in.Role, in.AgentSlug = slice.RoleReviewer, in.Slice.Reviewer
			in.Commits = []git.SliceCommit{{Hash: "abc1234", Subject: "feat(phase-1-ws-1.1): retry with backoff"}}
		}},
		{"re-review", "phase-1-ws-1.2", ReReview, func(in *Input) {
			in.Slice.Coded = true
			in.Role, in.AgentSlug = slice.RoleReviewer, in.Slice.Reviewer
			in.Branch = "auto/phase-1-ws-1.2"
			in.Commits = []git.SliceCommit{
				{Hash: "1111111", Subject: "chore(phase-1-ws-1.2): block review (cycle 2)"},
				{Hash: "2222222", Subject: "fix(phase-1-ws-1.2): fix f-003: drop the key log line"},
				{Hash: "3333333", Subject: "chore(phase-1-ws-1.2): flip coded"},
			}
		}},
		{"gate-review", "phase-1-gate", GateReview, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := fixture(t, c.slice)
			if c.mutate != nil {
				c.mutate(&in)
			}
			if got := Choose(in.Slice, in.Role); got != c.tmpl {
				t.Fatalf("Choose = %s, want %s", got, c.tmpl)
			}
			out, err := Render(&in, c.tmpl)
			if err != nil {
				t.Fatal(err)
			}
			golden(t, c.name, out)
		})
	}
}

// The rework prompt lists exactly the open findings: not the resolved
// one, not the advisory.
func TestReworkListsExactlyTheOpenFindings(t *testing.T) {
	in := fixture(t, "phase-1-ws-1.2")
	out, err := Render(&in, Rework)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"f-002", "f-003"} {
		if !strings.Contains(out, "**"+id+"**") {
			t.Errorf("rework prompt misses open finding %s", id)
		}
	}
	for _, id := range []string{"f-001", "f-004"} {
		if strings.Contains(out, id) {
			t.Errorf("rework prompt lists %s, which is not open", id)
		}
	}
}

// A brief's Review section replaces the project checklist.
func TestBriefReviewSectionReplacesChecklist(t *testing.T) {
	in := fixture(t, "phase-1-ws-1.2")
	in.Slice.Coded, in.Slice.ReviewCycles = true, 0
	in.Role, in.AgentSlug = slice.RoleReviewer, in.Slice.Reviewer
	out, err := Render(&in, Review)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "The signature covers the raw body") {
		t.Error("the brief's Review section is not quoted")
	}
	if strings.Contains(out, "Behavioral correctness") {
		t.Error("the project checklist is still listed")
	}

	in = fixture(t, "phase-1-ws-1.1")
	in.Slice.Coded = true
	in.Role, in.AgentSlug = slice.RoleReviewer, in.Slice.Reviewer
	if out, _ = Render(&in, Review); !strings.Contains(out, "1. Behavioral correctness") {
		t.Error("without a Review section the project checklist applies")
	}
}

// Narrowing follows review.narrow_after; 0 disables it.
func TestNarrowing(t *testing.T) {
	in := fixture(t, "phase-1-ws-1.2")
	in.Slice.Coded = true
	in.Role, in.AgentSlug = slice.RoleReviewer, in.Slice.Reviewer
	if out, _ := Render(&in, ReReview); !strings.Contains(out, "Narrowed review") {
		t.Error("review 3 should be narrowed by default")
	}
	zero := 0
	in.Cfg.Review.NarrowAfter = &zero
	if out, _ := Render(&in, ReReview); strings.Contains(out, "Narrowed review") {
		t.Error("narrow_after: 0 disables narrowing")
	}
}

// A project template in .metis/templates/handoff overrides the built-in one.
func TestOverride(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, OverrideDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, OverrideDir, "code.md.tmpl"), []byte("custom {{.Slice.ID}} as {{.Agent.Slug}}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	in := fixture(t, "phase-1-ws-1.1")
	in.RepoRoot = dir
	out, err := Render(&in, Code)
	if err != nil {
		t.Fatal(err)
	}
	if out != "custom phase-1-ws-1.1 as claude-code/opus\n" {
		t.Errorf("override not used: %q", out)
	}
}
