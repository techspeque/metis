// Package git provides git operations with enforcement of branch, commit, and
// attribution rules as defined in .metis/project.yaml.
package git

import (
	"fmt"
	"os/exec"
	"strings"
)

// CurrentBranch returns the name of the currently checked-out branch.
func CurrentBranch(repoDir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("getting current branch: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// ValidateBranch checks that the current branch matches the expected integration branch.
func ValidateBranch(repoDir, expectedBranch string) error {
	branch, err := CurrentBranch(repoDir)
	if err != nil {
		return err
	}
	if branch != expectedBranch {
		return fmt.Errorf("wrong branch: on %q, expected %q", branch, expectedBranch)
	}
	return nil
}

// IsClean returns true if the working tree has no uncommitted changes.
func IsClean(repoDir string) (bool, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("checking git status: %w", err)
	}
	return len(strings.TrimSpace(string(out))) == 0, nil
}

// Add stages files for commit.
func Add(repoDir string, paths ...string) error {
	args := append([]string{"add"}, paths...)
	cmd := exec.Command("git", args...)
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git add: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// Commit creates a git commit with the given message.
func Commit(repoDir, message string) error {
	cmd := exec.Command("git", "commit", "-m", message)
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git commit: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// CommitAmend amends the previous commit with the given message.
func CommitAmend(repoDir, message string) error {
	cmd := exec.Command("git", "commit", "--amend", "-m", message)
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git commit --amend: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}
