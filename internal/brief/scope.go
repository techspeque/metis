package brief

import (
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
// Returns nil when the brief declares no scope — callers treat that as
// "scope not verifiable", not as "everything allowed".
func ParseOwnedPaths(content string) []string {
	lines := strings.Split(content, "\n")
	var paths []string
	collecting := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if idx := strings.Index(strings.ToLower(trimmed), "**owned_paths:**"); idx >= 0 {
			rest := strings.TrimSpace(trimmed[idx+len("**owned_paths:**"):])
			for _, p := range strings.Split(rest, ",") {
				if p = strings.TrimSpace(p); p != "" && !isPlaceholder(p) {
					paths = append(paths, p)
				}
			}
			collecting = true
			continue
		}

		if collecting {
			// Sub-bullets continue the list; anything else ends it.
			if strings.HasPrefix(trimmed, "- ") && !strings.Contains(trimmed, "**") {
				if p := strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")); p != "" && !isPlaceholder(p) {
					paths = append(paths, p)
				}
				continue
			}
			if trimmed != "" {
				collecting = false
			}
		}
	}
	return paths
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
