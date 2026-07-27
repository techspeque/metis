package cli

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/techspeque/metis/internal/git"
)

// writeCommitted writes a file and commits it alone with the given subject.
func writeCommitted(t *testing.T, dir, rel, content, subject string) {
	t.Helper()
	path := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	gitOut(t, dir, "add", rel)
	gitOut(t, dir, "commit", "-qm", subject)
}

// TestAuditReadsBriefAtHead pins that the scope contract comes from the
// committed brief, not the working tree — otherwise the audited party can
// widen owned_paths, uncommitted, and pass.
func TestAuditReadsBriefAtHead(t *testing.T) {
	dir := makeGitProjectWithLedger(t)
	writeCommitted(t, dir, ".metis/briefs/feat-0001.md",
		"## Declared file scope\n\n- **owned_paths:** src/\n", "chore(feat-0001): brief")
	writeCommitted(t, dir, "other/b.go", "package other\n", "feat(feat-0001): stray edit")

	ctx, err := loadContext()
	if err != nil {
		t.Fatal(err)
	}
	commits, err := git.SliceCommits(dir, "feat-0001")
	if err != nil {
		t.Fatal(err)
	}

	report := auditSlice(ctx, "feat-0001", commits)
	if report.OK || !slices.Contains(report.OutOfScope, "other/b.go") {
		t.Fatalf("audit = OK:%v out_of_scope:%v, want FAIL naming other/b.go", report.OK, report.OutOfScope)
	}
	if !report.BriefCommitted {
		t.Error("BriefCommitted = false for a committed brief")
	}

	// Widening the contract in the working tree only must change nothing.
	if err := os.WriteFile(filepath.Join(dir, ".metis", "briefs", "feat-0001.md"),
		[]byte("## Declared file scope\n\n- **owned_paths:** src/, other/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	report = auditSlice(ctx, "feat-0001", commits)
	if report.OK || !slices.Contains(report.OutOfScope, "other/b.go") {
		t.Fatalf("uncommitted brief edit changed the audit: OK:%v out_of_scope:%v", report.OK, report.OutOfScope)
	}
}

// TestAuditPreSeedCommitsExcluded reproduces the metiswww dogfood scenario:
// a planning commit tagged with a future slice's ID, made before the slice
// was seeded into the ledger, must not be judged against a scope contract
// that did not exist yet — it is labeled pre-seed instead of failing.
func TestAuditPreSeedCommitsExcluded(t *testing.T) {
	dir := makeGitProjectWithLedger(t)

	// Planning era: the commit carries the slice ID and touches a file the
	// future brief will not own.
	writeCommitted(t, dir, "OVERVIEW.md", "plan\n", "chore(feat-0002): plan the work")

	// The slice begins to exist: seeded into the ledger.
	ledgerPath := filepath.Join(dir, ".metis", "slices.yaml")
	data, err := os.ReadFile(ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	entry := `  - id: feat-0002
    title: "Second slice"
    type: feat
    priority: p2
    risk: low
    coder: claude-code/opus
    reviewer: opencode/opus
    coded: false
    reviewed: false
    review_cycles: 0
    created: 2026-07-27
`
	if err := os.WriteFile(ledgerPath, append(data, entry...), 0o644); err != nil {
		t.Fatal(err)
	}
	gitOut(t, dir, "add", ".metis/slices.yaml")
	gitOut(t, dir, "commit", "-qm", "chore(feat-0002): seed slice")

	writeCommitted(t, dir, ".metis/briefs/feat-0002.md",
		"- **owned_paths:** src/\n", "chore(feat-0002): brief")
	writeCommitted(t, dir, "src/f.go", "package src\n", "feat(feat-0002): implement")

	ctx, err := loadContext()
	if err != nil {
		t.Fatal(err)
	}
	commits, err := git.SliceCommits(dir, "feat-0002")
	if err != nil {
		t.Fatal(err)
	}

	report := auditSlice(ctx, "feat-0002", commits)
	if !report.OK || len(report.OutOfScope) != 0 {
		t.Fatalf("audit = OK:%v out_of_scope:%v, want PASS — planning commit predates the contract",
			report.OK, report.OutOfScope)
	}
	if report.SeedCommit == "" {
		t.Error("SeedCommit not identified")
	}
	if len(report.Commits) == 0 || !report.Commits[0].PreSeed {
		t.Errorf("planning commit not labeled pre-seed: %+v", report.Commits)
	}
}

// TestAuditGateSkipsScopeCollection reproduces the phase-1-gate dogfood
// verdict: the display said "gate slice — scope audit not applicable" while
// out-of-scope collection still ran and flipped the verdict to FAIL. Gates
// must not accumulate scope violations at all.
func TestAuditGateSkipsScopeCollection(t *testing.T) {
	dir := makeGitProjectWithLedger(t)
	replaceInFile(t, filepath.Join(dir, ".metis", "slices.yaml"), "type: feat", "type: gate")
	gitOut(t, dir, "commit", "-aqm", "chore(feat-0001): make it a gate")
	writeCommitted(t, dir, ".metis/briefs/feat-0001.md",
		"- **owned_paths:** .metis/briefs/feat-0001.md\n", "docs(feat-0001): brief")
	writeCommitted(t, dir, "docs/copy.md", "copy\n", "docs(feat-0001): gate evidence touches a doc")

	ctx, err := loadContext()
	if err != nil {
		t.Fatal(err)
	}
	commits, err := git.SliceCommits(dir, "feat-0001")
	if err != nil {
		t.Fatal(err)
	}

	report := auditSlice(ctx, "feat-0001", commits)
	if !report.Gate {
		t.Fatal("fixture slice not recognized as gate")
	}
	if !report.OK || len(report.OutOfScope) != 0 {
		t.Fatalf("gate audit = OK:%v out_of_scope:%v — scope must not apply to gates",
			report.OK, report.OutOfScope)
	}
}

// TestAuditUncommittedBrief: a brief that exists only in the working tree is
// not a contract yet — scope stays unverifiable, with the cause named.
func TestAuditUncommittedBrief(t *testing.T) {
	dir := makeGitProjectWithLedger(t)
	briefPath := filepath.Join(dir, ".metis", "briefs", "feat-0001.md")
	if err := os.MkdirAll(filepath.Dir(briefPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(briefPath, []byte("- **owned_paths:** src/\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, err := loadContext()
	if err != nil {
		t.Fatal(err)
	}
	report := auditSlice(ctx, "feat-0001", nil)
	if report.OK || report.ScopeVerifiable || !report.BriefUncommitted {
		t.Fatalf("audit = OK:%v verifiable:%v uncommitted:%v, want FAIL/unverifiable/uncommitted",
			report.OK, report.ScopeVerifiable, report.BriefUncommitted)
	}
}
