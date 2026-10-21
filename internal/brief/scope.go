package brief

import (
	"fmt"
	"strings"
)

// Scope is a brief's declared file scope as the audit reads it.
type Scope struct {
	// Owned is the list of paths the slice may edit. Empty with
	// OwnedDeclared set means the brief declared "none": read-only work.
	Owned []string
	// OwnedDeclared reports whether the brief carries an owned_paths field
	// at all; a missing field is "scope not verifiable", never "everything".
	OwnedDeclared bool
	// ReadOnly is the list of paths the slice may inspect but not modify.
	ReadOnly []string
	// Warnings name entries that parse but look malformed, and placeholders
	// left over from the template. Each costs a review cycle when it reaches
	// the audit unnoticed, so commit --brief refuses them (see Validate).
	Warnings []string
}

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
// The single word "none" declares an empty scope on purpose:
//
//   - **owned_paths:** none
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

// ParseOwnedPathsWithWarnings is ParseOwnedPaths plus diagnostics for entries
// that parse but look malformed — a silent mis-parse here costs a review
// cycle, so anything suspicious is named explicitly.
func ParseOwnedPathsWithWarnings(content string) (paths []string, warnings []string) {
	f := parseField(content, "owned_paths", true)
	return f.paths, f.warnings
}

// ParseScope reads both scope fields of a brief. read_only_paths is prose
// as often as it is paths ("everything else"), so only its path-shaped
// entries are kept and only a leftover template placeholder warns.
func ParseScope(content string) *Scope {
	owned := parseField(content, "owned_paths", true)
	readOnly := parseField(content, "read_only_paths", false)
	return &Scope{
		Owned:         owned.paths,
		OwnedDeclared: owned.declared,
		ReadOnly:      readOnly.paths,
		Warnings:      append(owned.warnings, readOnly.warnings...),
	}
}

// Validate reports why the scope cannot serve as the slice's contract: no
// owned_paths field, a placeholder left in place, an entry the matcher
// cannot read, or the same path declared both owned and read-only. Empty
// means the audit will be able to measure every commit against it.
//
// Owning a file inside a read-only package ("edit this one, read the
// rest") and reading one file inside an owned directory are both ordinary
// declarations and are not overlaps; only the identical path is.
func (sc *Scope) Validate() []string {
	var problems []string
	if !sc.OwnedDeclared {
		problems = append(problems, "the brief declares no owned_paths (write bare paths, or 'none' for read-only work)")
	}
	problems = append(problems, sc.Warnings...)
	for _, o := range sc.Owned {
		for _, r := range sc.ReadOnly {
			if strings.TrimSuffix(o, "/") == strings.TrimSuffix(r, "/") {
				problems = append(problems, fmt.Sprintf("%q is declared both owned and read-only", o))
			}
		}
	}
	return problems
}

// field is one parsed scope field.
type field struct {
	paths    []string
	warnings []string
	declared bool
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

// parseField collects one "**<name>:**" field's entries: inline after the
// colon, or as the sub-bullets that follow. With strict set an entry that
// still contains whitespace is kept and warned about; otherwise it is
// taken for prose and dropped.
func parseField(content, name string, strict bool) field {
	marker := "**" + name + ":**"
	lines := strings.Split(content, "\n")
	collecting := false
	none := false
	var f field

	add := func(raw string) {
		p := cleanPathEntry(raw)
		if p == "" {
			return
		}
		if strings.EqualFold(p, "none") {
			none = true
			return
		}
		if isPlaceholder(p) {
			f.warnings = append(f.warnings, fmt.Sprintf("%s still holds the template placeholder %q", name, p))
			return
		}
		if strings.ContainsAny(p, " \t") {
			if !strict {
				return
			}
			f.warnings = append(f.warnings, fmt.Sprintf("%s entry %q contains whitespace — write bare paths; annotations belong after an em-dash (—)", name, p))
		}
		f.paths = append(f.paths, p)
	}

	// inItem tracks whether the previous line was a sub-bullet or its wrapped
	// continuation, so an indented line can be told apart from a fresh
	// indented paragraph that follows a blank line.
	inItem := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if idx := strings.Index(strings.ToLower(trimmed), marker); idx >= 0 {
			f.declared = true
			rest := strings.TrimSpace(trimmed[idx+len(marker):])
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
	if none && len(f.paths) > 0 {
		f.warnings = append(f.warnings, fmt.Sprintf("%s says none but also lists %d path(s)", name, len(f.paths)))
	}
	return f
}

// isPlaceholder filters template placeholder text that was never filled in.
func isPlaceholder(p string) bool {
	return strings.HasPrefix(p, "<") || strings.Contains(p, "exact files") || strings.Contains(p, "packages/files")
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
