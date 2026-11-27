package git

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// divergedRepo has main and a branch that both changed src/a.go, the
// branch in two commits, with the branch checked out.
func divergedRepo(t *testing.T) (dir, main string) {
	t.Helper()
	dir = gitRepo(t)
	main = strings.TrimSpace(runGit(t, dir, "rev-parse", "--abbrev-ref", "HEAD"))
	runGit(t, dir, "checkout", "-q", "-b", "topic")
	write(t, dir, "src/a.go", "package a // topic\n")
	runGit(t, dir, "commit", "-qam", "topic one")
	write(t, dir, "src/b.go", "package a\n")
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-qm", "topic two")
	runGit(t, dir, "checkout", "-q", main)
	write(t, dir, "src/a.go", "package a // main\n")
	runGit(t, dir, "commit", "-qam", "main")
	runGit(t, dir, "checkout", "-q", "topic")
	return dir, main
}

func TestRebaseHandsConflictsToTheResolver(t *testing.T) {
	dir, main := divergedRepo(t)
	var seen []string
	err := Rebase(dir, main, func(paths []string) error {
		seen = append(seen, paths...)
		if got := string(StageBlob(dir, 1, "src/a.go")) + string(StageBlob(dir, 2, "src/a.go")) + string(StageBlob(dir, 3, "src/a.go")); got != "package a\npackage a // main\npackage a // topic\n" {
			t.Errorf("stages base, ours, theirs = %q", got)
		}
		write(t, dir, "src/a.go", "package a // both\n")
		return Add(dir, "src/a.go")
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(seen) != 1 || seen[0] != "src/a.go" {
		t.Errorf("conflicted paths = %v", seen)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "src", "a.go")); string(got) != "package a // both\n" {
		t.Errorf("a.go = %q", got)
	}
	if log := runGit(t, dir, "log", "--format=%s", main+"..HEAD"); log != "topic two\ntopic one\n" {
		t.Errorf("rebased commits:\n%s", log)
	}
}

// A resolution that leaves nothing to commit drops the step and goes on.
func TestRebaseDropsAStepTheResolutionEmpties(t *testing.T) {
	dir, main := divergedRepo(t)
	err := Rebase(dir, main, func([]string) error {
		write(t, dir, "src/a.go", "package a // main\n")
		return Add(dir, "src/a.go")
	})
	if err != nil {
		t.Fatal(err)
	}
	if log := runGit(t, dir, "log", "--format=%s", main+"..HEAD"); log != "topic two\n" {
		t.Errorf("rebased commits:\n%s", log)
	}
}

func TestRebaseAbortsWhenTheResolverRefuses(t *testing.T) {
	dir, main := divergedRepo(t)
	before, _ := HeadHash(dir)
	refused := errors.New("not mine to settle")
	if err := Rebase(dir, main, func([]string) error { return refused }); !errors.Is(err, refused) {
		t.Fatalf("err = %v", err)
	}
	if after, _ := HeadHash(dir); after != before {
		t.Error("the branch moved")
	}
	if b, _ := CurrentBranch(dir); b != "topic" {
		t.Errorf("on %q, want topic", b)
	}
	if clean, _ := IsClean(dir); !clean {
		t.Error("the aborted rebase left the tree dirty")
	}
}

// Two commits that are the same patch to git, as two slices' flips are,
// both survive the rebase.
func TestRebaseKeepsACommitWhosePatchIsUpstream(t *testing.T) {
	dir := gitRepo(t)
	main := strings.TrimSpace(runGit(t, dir, "rev-parse", "--abbrev-ref", "HEAD"))
	block := "  coder: x\n  plan: p\n  section: s\n  coded: false\n  reviewed: false\n  cycles: 0\n  created: d\n"
	write(t, dir, "ledger", "- id: one\n"+block+"- id: two\n"+block)
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-qm", "ledger")
	flipped := strings.Replace(block, "coded: false", "coded: true", 1)

	runGit(t, dir, "checkout", "-q", "-b", "topic")
	write(t, dir, "ledger", "- id: one\n"+block+"- id: two\n"+flipped)
	runGit(t, dir, "commit", "-qam", "flip two")
	runGit(t, dir, "checkout", "-q", main)
	write(t, dir, "ledger", "- id: one\n"+flipped+"- id: two\n"+block)
	runGit(t, dir, "commit", "-qam", "flip one")
	runGit(t, dir, "checkout", "-q", "topic")

	if err := Rebase(dir, main, func([]string) error { return errors.New("no conflict expected") }); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "ledger")); string(got) != "- id: one\n"+flipped+"- id: two\n"+flipped {
		t.Errorf("the second flip was lost:\n%s", got)
	}
}
