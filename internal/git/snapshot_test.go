package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func gitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "t@example.com"},
		{"config", "user.name", "t"},
	} {
		runGit(t, dir, args...)
	}
	write(t, dir, ".gitignore", "cache/\n*.log\n")
	write(t, dir, "src/a.go", "package a\n")
	write(t, dir, ".metis/slices.yaml", "slices: []\n")
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "-m", "init")
	return dir
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %s", args, out)
	}
	return string(out)
}

func write(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func tree(t *testing.T, dir string) string {
	t.Helper()
	id, err := WorktreeTree(dir, []string{".metis/slices.yaml", ".metis/runs"})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestWorktreeTreeFollowsContentNotIgnoredFilesOrMetisRecords(t *testing.T) {
	dir := gitRepo(t)
	base := tree(t, dir)
	if tree(t, dir) != base {
		t.Fatal("same content, different tree")
	}

	write(t, dir, "cache/state.bin", "x")
	write(t, dir, "run.log", "x")
	write(t, dir, ".metis/runs/s/verify-pre.log", "x")
	write(t, dir, ".metis/slices.yaml", "slices: [changed]\n")
	if got := tree(t, dir); got != base {
		t.Fatal("an ignored file or a metis record changed the tree")
	}
	runGit(t, dir, "commit", "-q", "-am", "ledger")
	if got := tree(t, dir); got != base {
		t.Fatal("a committed metis record changed the tree")
	}

	write(t, dir, "src/new.go", "package a\n")
	untracked := tree(t, dir)
	if untracked == base {
		t.Fatal("an untracked, unignored file did not change the tree")
	}
	if err := os.Remove(filepath.Join(dir, "src/new.go")); err != nil {
		t.Fatal(err)
	}
	write(t, dir, "src/a.go", "package a // edited\n")
	if got := tree(t, dir); got == base || got == untracked {
		t.Fatal("an edited tracked file did not change the tree")
	}
}

func TestWorktreeTreeLeavesTheIndexAlone(t *testing.T) {
	dir := gitRepo(t)
	write(t, dir, "src/new.go", "package a\n")
	before := runGit(t, dir, "status", "--porcelain")
	tree(t, dir)
	if after := runGit(t, dir, "status", "--porcelain"); after != before {
		t.Fatalf("status changed:\n%s\n->\n%s", before, after)
	}
}
