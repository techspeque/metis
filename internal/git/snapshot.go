package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// WorktreeTree returns the id of the tree git would commit if every file
// in the working tree were staged: tracked files as they are on disk,
// untracked files that .gitignore does not exclude, and nothing under
// exclude. Ignored files (caches, build output) never enter it. The real
// index is not touched; a copy seeds git's stat cache so unchanged files
// are not re-hashed.
func WorktreeTree(repoDir string, exclude []string) (string, error) {
	indexPath, err := GitPath(repoDir, "index")
	if err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp("", "metis-index-*")
	if err != nil {
		return "", fmt.Errorf("snapshot index: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if data, err := os.ReadFile(indexPath); err == nil {
		if _, err := tmp.Write(data); err != nil {
			_ = tmp.Close()
			return "", fmt.Errorf("snapshot index: %w", err)
		}
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("snapshot index: %w", err)
	}
	// An empty file is not a valid index; git starts afresh without one.
	if info, err := os.Stat(tmpPath); err == nil && info.Size() == 0 {
		if err := os.Remove(tmpPath); err != nil {
			return "", fmt.Errorf("snapshot index: %w", err)
		}
	}

	run := func(args ...string) (string, error) {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		cmd.Env = append(os.Environ(), "GIT_INDEX_FILE="+tmpPath)
		out, err := cmd.Output()
		if err != nil {
			msg := ""
			if exitErr, ok := err.(*exec.ExitError); ok {
				msg = strings.TrimSpace(string(exitErr.Stderr))
			}
			return "", fmt.Errorf("git %s: %s: %w", args[0], msg, err)
		}
		return strings.TrimSpace(string(out)), nil
	}

	if _, err := run("add", "--all", "--", "."); err != nil {
		return "", err
	}
	var paths []string
	for _, p := range exclude {
		if p = strings.TrimSpace(p); p != "" {
			paths = append(paths, filepath.ToSlash(p))
		}
	}
	if len(paths) > 0 {
		args := append([]string{"rm", "--cached", "-r", "-q", "--ignore-unmatch", "--"}, paths...)
		if _, err := run(args...); err != nil {
			return "", err
		}
	}
	return run("write-tree")
}

// GitPath resolves a path inside the repository's git directory: state
// kept there is per clone and never part of the working tree.
func GitPath(repoDir, name string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--git-path", name)
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("locating git %s: %w", name, err)
	}
	p := strings.TrimSpace(string(out))
	if !filepath.IsAbs(p) {
		p = filepath.Join(repoDir, p)
	}
	return p, nil
}

// ChangedPaths lists the paths whose content differs between two trees.
func ChangedPaths(repoDir, from, to string) ([]string, error) {
	cmd := exec.Command("git", "diff-tree", "-r", "--name-only", "--no-commit-id", from, to)
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff-tree: %w", err)
	}
	return strings.Fields(string(out)), nil
}
