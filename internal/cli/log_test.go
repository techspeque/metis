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
