package brief

import (
	"fmt"
	"strings"
)

// ParseOwnedPaths extracts the declared owned_paths from a brief document.
// The convention (set by the brief template) is a "**owned_paths:**" bullet,
// with paths either inline after the colon (comma-separated) or as indented
// list items on the following lines:
//
//   - **owned_paths:** internal/foo/, cmd/metis/main.go
//   - **owned_paths:**
//   - internal/foo/
//   - cmd/metis/main.go
//
// A sub-bullet's annotation may wrap across indented continuation lines; the
// list ends at the next field bullet, or at the first non-empty line that is
// neither a sub-bullet nor such a continuation.
//
// Returns nil when the brief declares no scope — callers treat that as
// "scope not verifiable", not as "everything allowed".
func ParseOwnedPaths(content string) []string {
	paths, _ := ParseOwnedPathsWithWarnings(content)
	return paths
}

// cleanPathEntry strips markdown formatting agents naturally add, plus
// dash-separated prose annotations ("docs/copy.md — added in review cycle 1"):
// only the leading path survives.
func cleanPathEntry(p string) string {
	for _, dash := range []string{"—", "–"} {
		if idx := strings.Index(p, dash); idx >= 0 {
			p = p[:idx]
		}
	}
	return strings.Trim(strings.TrimSpace(p), "`*")
}

// isFieldBullet reports whether a bullet starts a new brief field
// ("- **read_only_paths:** ..."), which ends the owned_paths list.
func isFieldBullet(trimmed string) bool {
	rest := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
	return strings.HasPrefix(rest, "**")
}

// isIndented reports whether a line carries leading whitespace, which is what
// distinguishes a sub-bullet's wrapped annotation from the prose or heading
// that genuinely ends the list.
func isIndented(line string) bool {
	return len(line) > 0 && (line[0] == ' ' || line[0] == '\t')
}

// ParseOwnedPathsWithWarnings is ParseOwnedPaths plus diagnostics for entries
// that parse but look malformed — a silent mis-parse here costs a review
// cycle, so anything suspicious is named explicitly.
func ParseOwnedPathsWithWarnings(content string) (paths []string, warnings []string) {
	lines := strings.Split(content, "\n")
	collecting := false

	add := func(raw string) {
		p := cleanPathEntry(raw)
		if p == "" || isPlaceholder(p) {
			return
		}
		if strings.ContainsAny(p, " \t") {
			warnings = append(warnings, fmt.Sprintf("owned_paths entry %q contains whitespace — write bare paths; annotations belong after an em-dash (—)", p))
		}
		paths = append(paths, p)
	}

	// inItem tracks whether the previous line was a sub-bullet or its wrapped
	// continuation, so an indented line can be told apart from a fresh
	// indented paragraph that follows a blank line.
	inItem := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if idx := strings.Index(strings.ToLower(trimmed), "**owned_paths:**"); idx >= 0 {
			rest := strings.TrimSpace(trimmed[idx+len("**owned_paths:**"):])
			for _, p := range strings.Split(rest, ",") {
				add(p)
			}
			collecting = true
			inItem = false
			continue
		}

		if collecting {
			// Sub-bullets continue the list; a new field bullet or any
			// other non-empty line ends it.
			if strings.HasPrefix(trimmed, "- ") && !isFieldBullet(trimmed) {
				add(strings.TrimPrefix(trimmed, "- "))
				inItem = true
				continue
			}
			if trimmed == "" {
				// A blank line does not end the list, but it does end the
				// current item: what follows is a new block, not a wrap.
				inItem = false
				continue
			}
			// An indented line directly under a sub-bullet is that bullet's
			// annotation wrapped across lines. Treating it as a terminator
			// silently truncated the contract and dropped every entry after
			// it, so skip it and keep collecting.
			if inItem && isIndented(line) {
				continue
			}
			collecting = false
			inItem = false
		}
	}
	return paths, warnings
}

// isPlaceholder filters template placeholder text that was never filled in.
func isPlaceholder(p string) bool {
	return strings.HasPrefix(p, "<") || strings.Contains(p, "exact files")
}

// InScope reports whether a file falls under any declared owned path
// (exact match, or prefix match for directory-style declarations).
func InScope(file string, owned []string) bool {
	for _, o := range owned {
		o = strings.TrimSpace(o)
		if o == "" {
			continue
		}
		if file == o {
			return true
		}
		dir := strings.TrimSuffix(o, "/") + "/"
		if strings.HasPrefix(file, dir) {
			return true
		}
	}
	return false
}
