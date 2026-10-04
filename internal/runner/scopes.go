package runner

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os/exec"
	"path"
	"regexp"
	"strings"

	"github.com/techspeque/metis/internal/config"
)

// RestScope names the part of the tree no scope answers for; a change
// there runs the whole verify.
const RestScope = "(rest)"

// ScopeState is one scope's content in a tree and the green run that
// already covers it, if any.
type ScopeState struct {
	Name    string
	Command string
	// Content identifies the scope's files in the tree: their blob ids and paths.
	Content string
	// Key is what a green run of this scope is remembered under.
	Key   string
	Green *Green
	// Paths are the scope's files, for the rest's report of what changed.
	Paths []string
}

// treeEntry is one blob of a git tree listing.
type treeEntry struct {
	blob, path string
}

func listTree(repoRoot, tree string) ([]treeEntry, error) {
	cmd := exec.Command("git", "ls-tree", "-r", "--", tree)
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-tree: %w", err)
	}
	var entries []treeEntry
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		// <mode> <type> <object>\t<path>
		meta, p, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		fields := strings.Fields(meta)
		if len(fields) != 3 || fields[1] != "blob" {
			continue
		}
		entries = append(entries, treeEntry{blob: fields[2], path: p})
	}
	return entries, nil
}

// matcher reports whether a repository path belongs to a scope path.
type matcher func(p string) bool

var globChars = regexp.MustCompile(`[*?\[]`)

func compileMatcher(pattern string) matcher {
	pattern = strings.TrimPrefix(strings.TrimSpace(pattern), "./")
	if globChars.MatchString(pattern) {
		if !strings.Contains(pattern, "**") {
			return func(p string) bool {
				ok, _ := path.Match(pattern, p)
				return ok
			}
		}
		// ** spans segments; * and ? stay within one.
		var b strings.Builder
		b.WriteString("^")
		for i := 0; i < len(pattern); i++ {
			switch {
			case strings.HasPrefix(pattern[i:], "**/"):
				b.WriteString("(?:.*/)?")
				i += 2
			case strings.HasPrefix(pattern[i:], "**"):
				b.WriteString(".*")
				i++
			case pattern[i] == '*':
				b.WriteString("[^/]*")
			case pattern[i] == '?':
				b.WriteString("[^/]")
			default:
				b.WriteString(regexp.QuoteMeta(pattern[i : i+1]))
			}
		}
		b.WriteString("$")
		re := regexp.MustCompile(b.String())
		return re.MatchString
	}
	dir := strings.TrimSuffix(pattern, "/") + "/"
	return func(p string) bool {
		return p == pattern || strings.HasPrefix(p, dir)
	}
}

func contentHash(entries []treeEntry) string {
	h := sha256.New()
	for _, e := range entries {
		h.Write([]byte(e.blob))
		h.Write([]byte{' '})
		h.Write([]byte(e.path))
		h.Write([]byte{'\n'})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func scopeKey(name, content, command, envCheck string) string {
	h := sha256.New()
	for _, part := range []string{"scope", name, content, command, envCheck} {
		h.Write([]byte(part))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// ScopeStates partitions the tree's files among the configured scopes
// (the rest last) and looks each part up in the cache.
func ScopeStates(cfg *config.Config, repoRoot, tree string) ([]ScopeState, error) {
	entries, err := listTree(repoRoot, tree)
	if err != nil {
		return nil, err
	}
	scopes := cfg.Commands.VerifyScopes
	matchers := make([][]matcher, len(scopes))
	for i, scope := range scopes {
		for _, p := range scope.Paths {
			matchers[i] = append(matchers[i], compileMatcher(p))
		}
	}
	parts := make([][]treeEntry, len(scopes)+1)
	for _, e := range entries {
		covered := false
		for i, ms := range matchers {
			for _, m := range ms {
				if m(e.path) {
					parts[i] = append(parts[i], e)
					covered = true
					break
				}
			}
		}
		if !covered {
			parts[len(scopes)] = append(parts[len(scopes)], e)
		}
	}
	var states []ScopeState
	for i, scope := range scopes {
		states = append(states, ScopeState{Name: scope.Name, Command: scope.Command, Content: contentHash(parts[i]), Paths: pathsOf(parts[i])})
	}
	states = append(states, ScopeState{Name: RestScope, Command: cfg.Commands.Verify, Content: contentHash(parts[len(scopes)]), Paths: pathsOf(parts[len(scopes)])})
	for i := range states {
		states[i].Key = scopeKey(states[i].Name, states[i].Content, states[i].Command, cfg.Commands.EnvCheck)
		states[i].Green = LookupGreen(repoRoot, states[i].Key)
	}
	return states, nil
}

func pathsOf(entries []treeEntry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.path)
	}
	return out
}
