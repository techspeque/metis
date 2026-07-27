package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// makeGitProjectWithLedger is makeProjectWithLedger plus an initialized git
// repo with everything committed, so state-transition commands can commit.
func makeGitProjectWithLedger(t *testing.T) string {
	t.Helper()
	dir := makeProjectWithLedger(t)
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(".metis/.lock\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "t@t.co"},
		{"config", "user.name", "T"},
		{"add", "-A"},
		{"commit", "-qm", "chore(feat-0001): fixture"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dir
}

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

// TestBlockCommitsState pins that blocking commits the ledger and findings
// atomically — the tree must be clean afterwards.
func TestBlockCommitsState(t *testing.T) {
	dir := makeGitProjectWithLedger(t)
	replaceInFile(t, filepath.Join(dir, ".metis", "slices.yaml"), "coded: false", "coded: true")
	gitOut(t, dir, "commit", "-aqm", "chore(feat-0001): coded")
	setOutputFlag(t, "")

	rootCmd.SetArgs([]string{"block", "feat-0001", "--severity", "P2", "--category", "tests", "--finding", "missing edge case"})
	t.Cleanup(func() { rootCmd.SetArgs([]string{}) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("block: %v", err)
	}

	if status := gitOut(t, dir, "status", "--short"); strings.TrimSpace(status) != "" {
		t.Errorf("tree dirty after block:\n%s", status)
	}
	if log := gitOut(t, dir, "log", "-1", "--format=%s"); !strings.Contains(log, "block review (cycle 1)") {
		t.Errorf("last commit = %q, want block state commit", strings.TrimSpace(log))
	}
}

// TestArchiveCommitsState pins that archiving commits the ledger and the
// archive file — the protocol must end with a clean tree.
func TestArchiveCommitsState(t *testing.T) {
	dir := makeGitProjectWithLedger(t)
	ledger := filepath.Join(dir, ".metis", "slices.yaml")
	replaceInFile(t, ledger, "coded: false", "coded: true")
	replaceInFile(t, ledger, "reviewed: false", "reviewed: true")
	gitOut(t, dir, "commit", "-aqm", "chore(feat-0001): done")
	setOutputFlag(t, "")

	rootCmd.SetArgs([]string{"archive"})
	t.Cleanup(func() { rootCmd.SetArgs([]string{}) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("archive: %v", err)
	}

	if status := gitOut(t, dir, "status", "--short"); strings.TrimSpace(status) != "" {
		t.Errorf("tree dirty after archive:\n%s", status)
	}
	if log := gitOut(t, dir, "log", "-1", "--format=%s"); !strings.Contains(log, "archive 1 slice(s)") {
		t.Errorf("last commit = %q, want archive state commit", strings.TrimSpace(log))
	}
}

// TestRemoveCommitsState pins that retiring a slice moves it to the archive
// marked removed and commits both state files — clean tree, honest trail.
func TestRemoveCommitsState(t *testing.T) {
	dir := makeGitProjectWithLedger(t)
	setOutputFlag(t, "")

	rootCmd.SetArgs([]string{"remove", "feat-0001", "--reason", "descoped by product owner"})
	t.Cleanup(func() { rootCmd.SetArgs([]string{}) })
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("remove: %v", err)
	}

	if status := gitOut(t, dir, "status", "--short"); strings.TrimSpace(status) != "" {
		t.Errorf("tree dirty after remove:\n%s", status)
	}
	ledgerData, err := os.ReadFile(filepath.Join(dir, ".metis", "slices.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(ledgerData), "feat-0001") {
		t.Error("retired slice still in active ledger")
	}
	archiveData, err := os.ReadFile(filepath.Join(dir, ".metis", "slices-done.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(archiveData), "feat-0001") || !strings.Contains(string(archiveData), "removed: true") {
		t.Errorf("archive missing removed entry:\n%s", archiveData)
	}
	if log := gitOut(t, dir, "log", "-1", "--format=%s"); !strings.Contains(log, "remove slice (retired)") {
		t.Errorf("last commit = %q, want remove state commit", strings.TrimSpace(log))
	}
}
