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
	if branch == "HEAD" {
		return fmt.Errorf("detached HEAD state — check out the %q branch first", expectedBranch)
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

// CommitPaths commits ONLY the given paths, leaving the rest of the index
// untouched — used for metis state commits (flip, block, archive, brief) so
// they never sweep up unrelated changes an agent happened to have staged.
func CommitPaths(repoDir, message string, paths ...string) error {
	args := append([]string{"commit", "-m", message, "--"}, paths...)
	cmd := exec.Command("git", args...)
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

// SliceCommit is one commit associated with a slice.
type SliceCommit struct {
	Hash    string   `json:"hash"`
	Subject string   `json:"subject"`
	Body    string   `json:"body"`
	Files   []string `json:"files"`
}

// SliceCommits returns every commit whose message references the slice ID,
// oldest first, with the files each touched.
func SliceCommits(repoDir, sliceID string) ([]SliceCommit, error) {
	// Fixed-string match on the "(<id>)" token: a regex grep on the bare ID
	// matched prefixes (ws-2.1 pulled in ws-2.11) and treated '.' as any-char.
	cmd := exec.Command("git", "log", "--reverse", "--fixed-strings", "--grep", "("+sliceID+")", "--format=%H")
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}

	var commits []SliceCommit
	for _, hash := range strings.Fields(string(out)) {
		show := exec.Command("git", "show", "--name-only", "--format=%s%n%b%x00", hash)
		show.Dir = repoDir
		raw, err := show.Output()
		if err != nil {
			return nil, fmt.Errorf("git show %s: %w", hash, err)
		}
		parts := strings.SplitN(string(raw), "\x00", 2)
		header := strings.SplitN(parts[0], "\n", 2)
		c := SliceCommit{Hash: hash[:7], Subject: header[0]}
		if len(header) > 1 {
			c.Body = strings.TrimSpace(header[1])
		}
		if len(parts) > 1 {
			for _, f := range strings.Split(parts[1], "\n") {
				if f = strings.TrimSpace(f); f != "" {
					c.Files = append(c.Files, f)
				}
			}
		}
		commits = append(commits, c)
	}
	return commits, nil
}
