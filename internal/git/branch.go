package git

import (
	"fmt"
	"os/exec"
	"strings"
)

// AutoBranchPrefix starts the branch metis auto works a slice on.
const AutoBranchPrefix = "auto/"

// AutoBranch is the branch metis auto works the given slice on.
func AutoBranch(sliceID string) string {
	return AutoBranchPrefix + sliceID
}

// ValidateWorkBranch checks that the current branch is the integration
// branch, or the auto branch of the given slice.
func ValidateWorkBranch(repoDir, integration, sliceID string) error {
	branch, err := CurrentBranch(repoDir)
	if err != nil {
		return err
	}
	if sliceID != "" && branch == AutoBranch(sliceID) {
		return nil
	}
	if branch == "HEAD" {
		return fmt.Errorf("detached HEAD state — check out the %q branch first", integration)
	}
	if branch != integration {
		if strings.HasPrefix(branch, AutoBranchPrefix) {
			return fmt.Errorf("wrong branch: on %q, which is another slice's auto branch (expected %q or %q)", branch, integration, AutoBranch(sliceID))
		}
		return fmt.Errorf("wrong branch: on %q, expected %q", branch, integration)
	}
	return nil
}

func run(repoDir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(string(out)), err)
	}
	return string(out), nil
}

// DirtyFiles lists the repository-relative paths with uncommitted changes,
// untracked files included and ignored files excluded.
func DirtyFiles(repoDir string) ([]string, error) {
	out, err := run(repoDir, "status", "--porcelain", "-z", "--untracked-files=all")
	if err != nil {
		return nil, err
	}
	var files []string
	entries := strings.Split(out, "\x00")
	for i := 0; i < len(entries); i++ {
		e := entries[i]
		if len(e) < 4 {
			continue
		}
		files = append(files, e[3:])
		// A rename or copy carries its source as the next entry.
		if e[0] == 'R' || e[0] == 'C' {
			i++
		}
	}
	return files, nil
}

// HeadHash returns the full hash of HEAD.
func HeadHash(repoDir string) (string, error) {
	return RevParse(repoDir, "HEAD")
}

// RevParse resolves a revision to its full hash.
func RevParse(repoDir, rev string) (string, error) {
	out, err := run(repoDir, "rev-parse", "--verify", "--quiet", rev+"^{commit}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// BranchExists reports whether a local branch exists.
func BranchExists(repoDir, branch string) bool {
	_, err := RevParse(repoDir, "refs/heads/"+branch)
	return err == nil
}

// Checkout switches to an existing branch.
func Checkout(repoDir, branch string) error {
	_, err := run(repoDir, "checkout", "-q", branch)
	return err
}

// CreateBranch creates a branch at start and switches to it.
func CreateBranch(repoDir, branch, start string) error {
	_, err := run(repoDir, "checkout", "-q", "-b", branch, start)
	return err
}

// IsAncestor reports whether ancestor is reachable from rev.
func IsAncestor(repoDir, ancestor, rev string) bool {
	cmd := exec.Command("git", "merge-base", "--is-ancestor", ancestor, rev)
	cmd.Dir = repoDir
	return cmd.Run() == nil
}

// Rebase rebases the current branch onto upstream; on a conflict it aborts
// the rebase, leaving the branch as it was, and returns the error.
func Rebase(repoDir, upstream string) error {
	if _, err := run(repoDir, "rebase", "-q", upstream); err != nil {
		_, _ = run(repoDir, "rebase", "--abort")
		return err
	}
	return nil
}

// MergeFastForward fast-forwards the current branch to rev.
func MergeFastForward(repoDir, rev string) error {
	_, err := run(repoDir, "merge", "-q", "--ff-only", rev)
	return err
}

// DeleteBranch deletes a merged local branch.
func DeleteBranch(repoDir, branch string) error {
	_, err := run(repoDir, "branch", "-q", "-d", branch)
	return err
}

// Push pushes a branch to origin and sets its upstream.
func Push(repoDir, branch string) error {
	_, err := run(repoDir, "push", "-q", "-u", "origin", branch)
	return err
}

// FileAt returns the content of a repository-relative file at a revision.
func FileAt(repoDir, rev, relPath string) ([]byte, error) {
	cmd := exec.Command("git", "show", rev+":"+relPath)
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git show %s:%s: %w", rev, relPath, err)
	}
	return out, nil
}

// ListBranches lists the local branches whose names start with prefix.
func ListBranches(repoDir, prefix string) []string {
	out, err := run(repoDir, "for-each-ref", "--format=%(refname:short)", "refs/heads/"+prefix)
	if err != nil {
		return nil
	}
	return strings.Fields(out)
}
