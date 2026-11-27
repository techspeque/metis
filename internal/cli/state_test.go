package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/techspeque/metis/internal/findings"
)

// runMetis runs one metis command in-process.
func runMetis(t *testing.T, args ...string) error {
	t.Helper()
	setOutputFlag(t, "")
	for _, c := range rootCmd.Commands() {
		resetFlags(t, c.Flags())
		for _, sub := range c.Commands() {
			resetFlags(t, sub.Flags())
		}
	}
	rootCmd.SetArgs(args)
	t.Cleanup(func() { rootCmd.SetArgs([]string{}) })
	err := rootCmd.Execute()
	releaseRepoLock()
	return err
}

// stopOnBranch runs s-2 until its reviewer asks for a person, leaving
// auto/s-2 cut from the work branch as it is now and not yet reviewed.
func stopOnBranch(t *testing.T) {
	t.Helper()
	out, code := runAuto(t, "--slice", "s-2")
	expectStop(t, out, code, 3, ruleNeedsHuman)
}

// resumeOnBranch answers the reviewer's question and runs s-2 again from
// its branch.
func resumeOnBranch(t *testing.T, dir string) (string, int) {
	t.Helper()
	gitOut(t, dir, "checkout", "-q", "auto/s-2")
	if err := runMetis(t, "wait", "s-2", "--clear"); err != nil {
		t.Fatalf("wait --clear: %v", err)
	}
	return runAuto(t, "--slice", "s-2")
}

// Two slices run out of order: s-2's branch is cut before s-1 lands. Both
// archive, both were blocked with the finding id f-001, and each archive
// strips its slice from s-3's blocked_by. s-2 still lands: the state
// files are merged by entry, not by line.
func TestAutoLandsABranchAcrossStateFileConflicts(t *testing.T) {
	dir := makeAutoProject(t, "  mode: auto",
		[]autoSlice{{id: "s-1"}, {id: "s-2"}, {id: "s-3", blockedBy: "s-1, s-2"}},
		"code", "block:src/s-2.txt:1 — the retry bound is missing — add it", "code", "human:which vendor?",
		"code", "block:src/s-1.txt:9 — no test for the empty case — add one", "code", "pass",
		"pass")
	stopOnBranch(t)

	gitOut(t, dir, "checkout", "-q", "dev")
	out, code := runAuto(t, "--slice", "s-1")
	if code != 0 || !strings.Contains(out, "s-1 reviewed and archived") {
		t.Fatalf("s-1 should land first (exit %d):\n%s", code, out)
	}

	out, code = resumeOnBranch(t, dir)
	if code != 0 || !strings.Contains(out, "rebased, verified and fast-forward merged into dev") {
		t.Fatalf("s-2 should land across the state file conflicts (exit %d):\n%s", code, out)
	}
	if b := strings.TrimSpace(gitOut(t, dir, "rev-parse", "--abbrev-ref", "HEAD")); b != "dev" {
		t.Errorf("on %s, want dev", b)
	}
	if strings.Contains(gitOut(t, dir, "branch", "--list", "auto/s-2"), "auto/s-2") {
		t.Error("the landed branch should be deleted")
	}
	if status := gitOut(t, dir, "status", "--short"); strings.TrimSpace(status) != "" {
		t.Errorf("tree dirty after landing:\n%s", status)
	}
	if files := gitOut(t, dir, "ls-tree", "-r", "--name-only", "dev"); !strings.Contains(files, "src/s-1.txt") || !strings.Contains(files, "src/s-2.txt") {
		t.Errorf("both slices' work should be on dev:\n%s", files)
	}

	a := archivedIDs(t, dir)
	if one, two := strings.Index(a, "id: s-1"), strings.Index(a, "id: s-2"); one < 0 || two < one {
		t.Errorf("archive should hold s-1 then s-2:\n%s", a)
	}
	if strings.Count(a, "review_cycles: 1") != 2 || strings.Count(a, "reviewed: true") != 2 {
		t.Errorf("each archived slice keeps its own history:\n%s", a)
	}
	if l := ledgerText(t, dir); strings.Contains(l, "id: s-1") || strings.Contains(l, "id: s-2") || strings.Contains(l, "blocked_by") {
		t.Errorf("ledger should hold s-3 alone, unblocked:\n%s", l)
	}

	store, err := findings.Load(filepath.Join(dir, ".metis", "findings.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(store.Findings) != 2 {
		t.Fatalf("findings = %+v, want two", store.Findings)
	}
	first, second := store.Findings[0], store.Findings[1]
	if first.ID != "f-001" || first.Slice != "s-1" || first.Alias != "" {
		t.Errorf("the work branch's finding keeps its id: %+v", first)
	}
	if second.ID != "f-002" || second.Slice != "s-2" || second.Alias != "f-001" || !strings.Contains(second.Finding, "retry bound") {
		t.Errorf("the landed branch's finding is renumbered and keeps its alias: %+v", second)
	}

	if err := runMetis(t, "check"); err != nil {
		t.Errorf("metis check after landing: %v", err)
	}
}

// A conflict in a file that is not a state file is a person's: the rebase
// is aborted and the branch is as it was.
func TestAutoStopsOnASourceConflict(t *testing.T) {
	dir := makeAutoProject(t, "  mode: auto", []autoSlice{{id: "s-1"}, {id: "s-2"}}, "code", "human:which vendor?", "pass")
	stopOnBranch(t)

	gitOut(t, dir, "checkout", "-q", "dev")
	if err := os.MkdirAll(filepath.Join(dir, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "src", "s-2.txt"), []byte("someone else's line\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitOut(t, dir, "add", "src/s-2.txt")
	gitOut(t, dir, "commit", "-qm", "feat: the same file from elsewhere")

	out, code := resumeOnBranch(t, dir)
	expectStop(t, out, code, 3, ruleMerge)
	if !strings.Contains(out, "conflict in src/s-2.txt") {
		t.Errorf("the stop should name the file:\n%s", out)
	}
	expectBranchAsItWas(t, dir)
}

// The same slice changed on both sides is a real conflict, state file or
// not.
func TestAutoStopsWhenBothSidesChangedASlice(t *testing.T) {
	dir := makeAutoProject(t, "  mode: auto", []autoSlice{{id: "s-1"}, {id: "s-2"}}, "code", "human:which vendor?", "pass")
	stopOnBranch(t)

	gitOut(t, dir, "checkout", "-q", "dev")
	replaceInFile(t, filepath.Join(dir, ".metis", "slices.yaml"), "title: s-2", "title: s-2 as dev sees it")
	gitOut(t, dir, "commit", "-qam", "chore: retitle s-2")

	out, code := resumeOnBranch(t, dir)
	expectStop(t, out, code, 3, ruleMerge)
	if !strings.Contains(out, "conflict in .metis/slices.yaml: slice s-2 changed on") {
		t.Errorf("the stop should name the slice:\n%s", out)
	}
	expectBranchAsItWas(t, dir)
}

func expectBranchAsItWas(t *testing.T, dir string) {
	t.Helper()
	if b := strings.TrimSpace(gitOut(t, dir, "rev-parse", "--abbrev-ref", "HEAD")); b != "auto/s-2" {
		t.Errorf("on %s, want auto/s-2", b)
	}
	if status := gitOut(t, dir, "status", "--short"); strings.TrimSpace(status) != "" {
		t.Errorf("the aborted rebase left the tree dirty:\n%s", status)
	}
	for _, d := range []string{"rebase-merge", "rebase-apply"} {
		if _, err := os.Stat(filepath.Join(dir, ".git", d)); err == nil {
			t.Errorf("a rebase is still in progress (.git/%s)", d)
		}
	}
	if strings.TrimSpace(gitOut(t, dir, "rev-list", "--count", "auto/s-2..dev")) == "0" {
		t.Error("auto/s-2 should not have been rebased onto dev")
	}
}

// An old id resolves to the renumbered finding wherever an id is given.
func TestFindingAliasResolvesInCommands(t *testing.T) {
	dir := makeGitProjectWithLedger(t)
	path := filepath.Join(dir, ".metis", "findings.yaml")
	store := &findings.Store{}
	store.Add("feat-0001", "P2", "tests", "the empty case is untested")
	store.Add("feat-0001", "P2", "tests", "the retry bound is missing")
	store.Findings[1].ID, store.Findings[1].Alias = "f-007", "f-002"
	if err := store.Save(path); err != nil {
		t.Fatal(err)
	}
	gitOut(t, dir, "add", "-A")
	gitOut(t, dir, "commit", "-qm", "chore(feat-0001): findings")

	if err := runMetis(t, "findings", "record", "feat-0001", "--finding", "still unbounded", "--follow-on", "f-002"); err != nil {
		t.Fatalf("record --follow-on by alias: %v", err)
	}
	if err := runMetis(t, "findings", "resolve", "f-001", "--duplicate-of", "f-002"); err != nil {
		t.Fatalf("resolve --duplicate-of by alias: %v", err)
	}
	if err := runMetis(t, "findings", "waive", "f-002", "--by", "human", "--reason", "accepted"); err != nil {
		t.Fatalf("waive by alias: %v", err)
	}
	got, err := findings.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if f := got.Findings[2]; f.FollowOn != "f-007" {
		t.Errorf("follow_on = %q, want the id f-007", f.FollowOn)
	}
	if f := got.Findings[0]; f.Of != "f-007" || f.ResolvedBy != "duplicate of f-007" {
		t.Errorf("duplicate: %+v", f)
	}
	if f := got.Findings[1]; f.ID != "f-007" || f.Status != "waived" {
		t.Errorf("waived by alias: %+v", f)
	}
	if log := gitOut(t, dir, "log", "--format=%s", "-1"); !strings.Contains(log, "waive finding f-007") {
		t.Errorf("the commit names the finding by its id: %s", log)
	}
}

func TestStateMergeAsAMergeDriver(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) string {
		t.Helper()
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	finding := func(id, sliceID, text string) string {
		return "    - id: " + id + "\n      date: \"2026-11-27\"\n      slice: " + sliceID +
			"\n      severity: P2\n      category: tests\n      finding: " + text + "\n      status: open\n"
	}
	base := write("base", "findings: []\n")
	ours := write("ours", "findings:\n"+finding("f-001", "s-1", "one"))
	theirs := write("theirs", "findings:\n"+finding("f-001", "s-2", "two"))

	// git names the three versions with temporary files: %P says which
	// file they are.
	if err := runMetis(t, "state", "merge", base, ours, theirs, "--path", ".metis/findings.yaml"); err != nil {
		t.Fatalf("state merge: %v", err)
	}
	merged, _ := os.ReadFile(ours)
	if want := "findings:\n" + finding("f-001", "s-1", "one") + strings.Replace(finding("f-002", "s-2", "two"), "id: f-002\n", "id: f-002\n      alias: f-001\n", 1); string(merged) != want {
		t.Errorf("the result is written to <ours>:\n%s\nwant:\n%s", merged, want)
	}
	if left, _ := os.ReadFile(theirs); !strings.Contains(string(left), "id: f-001") {
		t.Errorf("<theirs> must be left alone:\n%s", left)
	}

	// A real conflict: exit 1, <ours> as it was.
	entry := func(title string) string {
		return "version: 1\nslices:\n    - id: s-1\n      title: " + title + "\n"
	}
	base, ours, theirs = write("b.yaml", entry("one")), write("slices.yaml", entry("two")), write("t.yaml", entry("three"))
	err := runMetis(t, "state", "merge", base, ours, theirs)
	if resolveExitCode(err) != 1 || !strings.Contains(err.Error(), "slice s-1 changed on both sides") {
		t.Errorf("conflict: got %v", err)
	}
	if left, _ := os.ReadFile(ours); string(left) != entry("two") {
		t.Errorf("<ours> changed on a conflict:\n%s", left)
	}

	if err := runMetis(t, "state", "merge", base, write("notes.txt", ""), theirs); err == nil {
		t.Error("a file that is not a state file is refused")
	}
}

// git runs the merge as a driver during a rebase done by hand.
func TestStateMergeDriverUnderGitRebase(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the driver line is a POSIX shell command")
	}
	dir := makeGitProjectWithLedger(t)
	driver := filepath.Join(metisBinary(t), "metis") + " state merge %O %A %B --path %P"
	gitOut(t, dir, "config", "merge.metis-state.driver", driver)
	if err := os.MkdirAll(filepath.Join(dir, ".git", "info"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".git", "info", "attributes"), []byte(".metis/findings.yaml merge=metis-state\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, ".metis", "findings.yaml")
	commit := func(text string) {
		t.Helper()
		store, err := findings.Load(path)
		if err != nil {
			t.Fatal(err)
		}
		store.Add("feat-0001", "P2", "tests", text)
		if err := store.Save(path); err != nil {
			t.Fatal(err)
		}
		gitOut(t, dir, "add", "-A")
		gitOut(t, dir, "commit", "-qm", "chore(feat-0001): "+text)
	}
	work := strings.TrimSpace(gitOut(t, dir, "rev-parse", "--abbrev-ref", "HEAD"))
	commit("shared")
	gitOut(t, dir, "checkout", "-q", "-b", "topic")
	commit("from the topic")
	gitOut(t, dir, "checkout", "-q", work)
	commit("from the work branch")
	gitOut(t, dir, "checkout", "-q", "topic")
	gitOut(t, dir, "rebase", "-q", work)

	store, err := findings.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(store.Findings) != 3 {
		t.Fatalf("findings = %+v", store.Findings)
	}
	if f := store.Findings[1]; f.ID != "f-002" || f.Finding != "from the work branch" || f.Alias != "" {
		t.Errorf("the work branch's finding: %+v", f)
	}
	if f := store.Findings[2]; f.ID != "f-003" || f.Finding != "from the topic" || f.Alias != "f-002" {
		t.Errorf("the rebased finding: %+v", f)
	}
}
