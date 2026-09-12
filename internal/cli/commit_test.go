package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// makeDevProject is makeGitProjectWithLedger on the configured integration
// branch, so 'metis commit' gets past its branch check and the assertions
// below measure the behaviour under test rather than the branch guard.
func makeDevProject(t *testing.T) string {
	t.Helper()
	dir := makeGitProjectWithLedger(t)
	gitOut(t, dir, "checkout", "-qb", "dev")
	return dir
}

// setCommitFlags applies flags to the shared commitCmd and resets them after
// the test — cobra flag state is package-level and leaks between cases.
func setCommitFlags(t *testing.T, kv map[string]string) {
	t.Helper()
	for k, v := range kv {
		if err := commitCmd.Flags().Set(k, v); err != nil {
			t.Fatalf("set --%s: %v", k, err)
		}
	}
	t.Cleanup(func() {
		for _, k := range []string{"prefix", "message", "brief", "flip", "amend", "agent", "slice"} {
			def := ""
			if k == "brief" || k == "amend" {
				def = "false"
			}
			_ = commitCmd.Flags().Set(k, def)
		}
	})
}

// recordVerifyPost writes the run record that --flip coded requires.
func recordVerifyPost(t *testing.T, dir, sliceID string, exitCode string) {
	t.Helper()
	runDir := filepath.Join(dir, ".metis", "runs", sliceID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "verify-post.log"), []byte("ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "verify-post.exit"), []byte(exitCode+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func ledgerText(t *testing.T, dir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, ".metis", "slices.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// Flags that do not apply to a shortcut used to be dropped in silence, which
// reads exactly like a flag that was honoured.
func TestCommitRejectsInapplicableFlags(t *testing.T) {
	makeDevProject(t)
	setOutputFlag(t, "")

	for name, flags := range map[string]map[string]string{
		"brief with flip":  {"brief": "true", "flip": "coded"},
		"amend with flip":  {"amend": "true", "flip": "coded"},
		"amend with brief": {"amend": "true", "brief": "true"},
		"agent with coded": {"flip": "coded", "agent": "claude-code/opus"},
	} {
		t.Run(name, func(t *testing.T) {
			setCommitFlags(t, flags)
			if err := commitCmd.RunE(commitCmd, nil); err == nil {
				t.Fatalf("%s was accepted; expected an error naming the conflict", name)
			}
		})
	}
}

// The scope audit reads the contract at HEAD, so a brief that exists only on
// disk flips coded and then fails review for a reason the coder cannot see
// from the flip. Both steps must apply the same precondition.
func TestFlipCodedRequiresCommittedBrief(t *testing.T) {
	dir := makeDevProject(t)
	setOutputFlag(t, "")
	recordVerifyPost(t, dir, "feat-0001", "0")

	briefPath := filepath.Join(dir, ".metis", "briefs", "feat-0001.md")
	if err := os.MkdirAll(filepath.Dir(briefPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(briefPath, []byte("- **owned_paths:** src/\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	setCommitFlags(t, map[string]string{"flip": "coded"})
	err := commitCmd.RunE(commitCmd, nil)
	if err == nil {
		t.Fatal("flip coded accepted an uncommitted brief")
	}
	if !strings.Contains(err.Error(), "not committed") {
		t.Errorf("error = %q, want it to name the uncommitted brief", err)
	}
	if strings.Contains(ledgerText(t, dir), "coded: true") {
		t.Error("ledger was flipped despite the precondition failing")
	}
}

// The flip used to fail with "'metis log <id> --validate' fails", sending the
// reviewer to another command to discover a fact metis already had.
func TestFlipReviewedNamesTheBlocker(t *testing.T) {
	dir := makeDevProject(t)
	setOutputFlag(t, "")
	// The reviewer identifies itself by slug, so the slug must be a
	// configured agent for the flip to reach the audit at all.
	if err := os.WriteFile(filepath.Join(dir, ".metis", "project.yaml"),
		[]byte("version: 1\nagents:\n  opencode/opus:\n    surface: opencode\n    model: opus\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitOut(t, dir, "commit", "-aqm", "chore(feat-0001): configure agents")
	writeCommitted(t, dir, ".metis/briefs/feat-0001.md",
		"- **owned_paths:** src/, .gitignore\n", "docs(feat-0001): brief")
	writeCommitted(t, dir, "wildly/out/of/scope.go", "package x\n", "feat(feat-0001): stray")
	replaceInFile(t, filepath.Join(dir, ".metis", "slices.yaml"), "coded: false", "coded: true")
	gitOut(t, dir, "commit", "-aqm", "chore(feat-0001): flip coded")

	setCommitFlags(t, map[string]string{"flip": "reviewed", "agent": "opencode/opus"})
	err := commitCmd.RunE(commitCmd, nil)
	if err == nil {
		t.Fatal("flip reviewed passed a failing audit")
	}
	if !strings.Contains(err.Error(), "wildly/out/of/scope.go") {
		t.Errorf("error = %q, want it to name the out-of-scope file", err)
	}
}

// A flip is a state change plus its commit. When the commit cannot be made,
// leaving the flag set would tell 'metis next' the slice was reviewed with
// nothing in history to show for it.
func TestFlipRollsBackWhenTheCommitCannotBeMade(t *testing.T) {
	dir := makeDevProject(t)
	setOutputFlag(t, "")
	recordVerifyPost(t, dir, "feat-0001", "0")
	writeCommitted(t, dir, ".metis/briefs/feat-0001.md",
		"- **owned_paths:** src/\n", "docs(feat-0001): brief")

	// "chore" is what a flip commit uses; removing it from the allowed set
	// makes the commit step fail after the ledger has already been written.
	if err := os.WriteFile(filepath.Join(dir, ".metis", "project.yaml"),
		[]byte("version: 1\ncommits:\n  prefixes:\n    - feat\n    - docs\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	setCommitFlags(t, map[string]string{"flip": "coded"})
	if err := commitCmd.RunE(commitCmd, nil); err == nil {
		t.Fatal("flip succeeded with an unusable commit prefix")
	}
	if strings.Contains(ledgerText(t, dir), "coded: true") {
		t.Fatal("ledger kept the flip after the commit failed — state and history disagree")
	}
	if out := gitOut(t, dir, "status", "--porcelain", ".metis/slices.yaml"); strings.TrimSpace(out) != "" {
		t.Errorf("ledger left dirty after rollback: %q", out)
	}
}
