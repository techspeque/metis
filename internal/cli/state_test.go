package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/techspeque/metis/internal/findings"
)

// useBuiltMetisAsMergeDriver makes a rebase run the built metis as the
// state files' merge driver: under test the running binary is the test
// binary, which has no 'state merge'.
func useBuiltMetisAsMergeDriver(t *testing.T) {
	t.Helper()
	exe := filepath.Join(metisBinary(t), "metis")
	old := stateMergeExe
	stateMergeExe = func() (string, error) { return exe, nil }
	t.Cleanup(func() { stateMergeExe = old })
}

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
// not. git would merge the two changes by line without a word (a title
// and a flag are lines apart): the stop comes from the merge by entry at
// the first step where both sides changed the ledger.
func TestAutoStopsWhenBothSidesChangedASlice(t *testing.T) {
	dir := makeAutoProject(t, "  mode: auto", []autoSlice{{id: "s-1"}, {id: "s-2"}}, "code", "human:which vendor?", "pass")
	stopOnBranch(t)

	gitOut(t, dir, "checkout", "-q", "dev")
	replaceInFile(t, filepath.Join(dir, ".metis", "slices.yaml"), "title: s-2", "title: s-2 as dev sees it")
	gitOut(t, dir, "commit", "-qam", "chore: retitle s-2")

	out, code := resumeOnBranch(t, dir)
	expectStop(t, out, code, 3, ruleMerge)
	if !strings.Contains(out, "conflict in .metis/slices.yaml: slice s-2 changed on both sides") {
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

// stderrOf returns what run wrote to stderr.
func stderrOf(t *testing.T, run func()) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stderr
	os.Stderr = f
	defer func() { os.Stderr = old }()
	run()
	os.Stderr = old
	_ = f.Close()
	data, err := os.ReadFile(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// An id that is also the id another finding had before a merge still
// names the finding that holds it now, and the command says so.
func TestFindingIDThatIsAlsoAnAliasIsNoted(t *testing.T) {
	dir := makeGitProjectWithLedger(t)
	path := filepath.Join(dir, ".metis", "findings.yaml")
	store := &findings.Store{}
	store.Add("feat-0001", "P2", "tests", "the empty case is untested")
	store.Add("feat-0002", "P2", "tests", "the retry bound is missing")
	store.Add("feat-0001", "P3", "maint", "a stale comment")
	store.Findings[1].Alias = "f-001"
	if err := store.Save(path); err != nil {
		t.Fatal(err)
	}
	gitOut(t, dir, "add", "-A")
	gitOut(t, dir, "commit", "-qm", "chore(feat-0001): findings")
	const note = "note: f-001 was also the id of f-002 (slice feat-0002) before a merge\n"

	for _, args := range [][]string{
		{"findings", "record", "feat-0001", "--finding", "still untested", "--follow-on", "f-001"},
		{"findings", "resolve", "f-003", "--duplicate-of", "f-001"},
		{"findings", "reclassify", "f-001", "--severity", "P1", "--by", "human", "--reason", "it is"},
		{"findings", "waive", "f-001", "--by", "human", "--reason", "accepted"},
	} {
		var err error
		if got := stderrOf(t, func() { err = runMetis(t, args...) }); err != nil || !strings.Contains(got, note) {
			t.Errorf("%v: err %v, stderr %q", args, err, got)
		}
	}
	got, err := findings.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Findings[0].Status != "waived" || got.Findings[1].Status != "open" {
		t.Errorf("the id's own finding is the one acted on: %+v", got.Findings[:2])
	}

	// No note for an id nothing else was called.
	var runErr error
	if out := stderrOf(t, func() { runErr = runMetis(t, "findings", "waive", "f-002", "--by", "human", "--reason", "accepted") }); runErr != nil || strings.Contains(out, "note:") {
		t.Errorf("f-002: err %v, stderr %q", runErr, out)
	}
}

// The landing rebase with the merge driver: a change to the branch's own
// f-002 follows the finding to the id the merge gave it, and nothing is
// left in the repository's configuration.
func TestRebaseWithTheMergeDriverLeavesNothingBehind(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the merge driver is run through a POSIX shell; not exercised on Windows")
	}
	dir := makeGitProjectWithLedger(t)
	path := filepath.Join(dir, ".metis", "findings.yaml")
	commit := func(subject string, change func(*findings.Store)) {
		t.Helper()
		store, err := findings.Load(path)
		if err != nil {
			t.Fatal(err)
		}
		change(store)
		if err := store.Save(path); err != nil {
			t.Fatal(err)
		}
		gitOut(t, dir, "add", "-A")
		gitOut(t, dir, "commit", "-qm", "chore(feat-0001): "+subject)
	}
	raise := func(text string) func(*findings.Store) {
		return func(s *findings.Store) {
			s.FindByID(s.Add("feat-0001", "P2", "tests", text)).Place(1, "src/x.go")
		}
	}
	work := strings.TrimSpace(gitOut(t, dir, "rev-parse", "--abbrev-ref", "HEAD"))
	commit("shared", raise("shared"))
	gitOut(t, dir, "checkout", "-q", "-b", "topic")
	commit("topic raises f-002", raise("from the topic"))
	commit("topic moves its f-002 to review 2", func(s *findings.Store) { s.FindByID("f-002").Cycle = 2 })
	gitOut(t, dir, "checkout", "-q", work)
	commit("work raises f-002", raise("from the work branch"))
	gitOut(t, dir, "checkout", "-q", "topic")

	useBuiltMetisAsMergeDriver(t)
	ctx, err := loadContext()
	if err != nil {
		t.Fatal(err)
	}
	err = rebaseMergingState(ctx, work)
	releaseRepoLock()
	if err != nil {
		t.Fatal(err)
	}

	store, err := findings.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(store.Findings) != 3 {
		t.Fatalf("findings = %+v", store.Findings)
	}
	if f := store.Findings[1]; f.ID != "f-002" || f.Finding != "from the work branch" || f.Cycle != 1 {
		t.Errorf("the work branch's finding must be untouched: %+v", f)
	}
	if f := store.Findings[2]; f.ID != "f-003" || f.Alias != "f-002" || f.Finding != "from the topic" || f.Cycle != 2 {
		t.Errorf("the topic's change follows its finding: %+v", f)
	}
	if status := gitOut(t, dir, "status", "--short"); strings.TrimSpace(status) != "" {
		t.Errorf("tree dirty after the rebase:\n%s", status)
	}
	if cfg := gitOut(t, dir, "config", "--local", "--list"); strings.Contains(cfg, "metis") {
		t.Errorf("the driver must not be written to the repository's config:\n%s", cfg)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git", "info", "attributes")); err == nil {
		t.Error("no attributes file may be left in the repository")
	}
}

// git merges these two histories by line without a conflict: the title
// and the coded flag of one slice are lines apart. By entry the slice
// changed on both sides, which is a person's to settle. Only the merge
// driver sees it, since git reports no conflict to resolve: without the
// driver the rebase succeeds.
func TestRebaseStopsWhereGitWouldMergeByLine(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the merge driver is run through a POSIX shell; not exercised on Windows")
	}
	dir := makeGitProjectWithLedger(t)
	ledger := filepath.Join(dir, ".metis", "slices.yaml")
	work := strings.TrimSpace(gitOut(t, dir, "rev-parse", "--abbrev-ref", "HEAD"))
	gitOut(t, dir, "checkout", "-q", "-b", "topic")
	replaceInFile(t, ledger, "coded: false", "coded: true")
	gitOut(t, dir, "commit", "-qam", "chore(feat-0001): coded")
	gitOut(t, dir, "checkout", "-q", work)
	replaceInFile(t, ledger, "Test slice", "Test slice, retitled")
	gitOut(t, dir, "commit", "-qam", "chore(feat-0001): retitle")
	gitOut(t, dir, "checkout", "-q", "topic")
	before := gitOut(t, dir, "rev-parse", "HEAD")

	ctx, err := loadContext()
	if err != nil {
		t.Fatal(err)
	}
	defer releaseRepoLock()

	useBuiltMetisAsMergeDriver(t)
	err = rebaseMergingState(ctx, work)
	if err == nil || !strings.Contains(err.Error(), "slice feat-0001 changed on both sides") {
		t.Fatalf("with the driver: got %v, want the slice named as changed on both sides", err)
	}
	if after := gitOut(t, dir, "rev-parse", "HEAD"); after != before {
		t.Error("the aborted rebase moved the branch")
	}
	if status := gitOut(t, dir, "status", "--short"); strings.TrimSpace(status) != "" {
		t.Errorf("the aborted rebase left the tree dirty:\n%s", status)
	}

	// The hole the driver closes: git alone lets both changes through.
	stateMergeExe = func() (string, error) { return "", nil }
	if err := rebaseMergingState(ctx, work); err != nil {
		t.Fatalf("without the driver git merges by line: %v", err)
	}
	if l := ledgerText(t, dir); !strings.Contains(l, "retitled") || !strings.Contains(l, "coded: true") {
		t.Errorf("both changes should be in the ledger:\n%s", l)
	}
}
