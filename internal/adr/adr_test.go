package adr

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

const adr3 = `---
type: adr
id: 0003
title: original rule
status: accepted
date: 2026-07-01
---

# ADR-0003: Original rule
`

const adr9 = `---
type: adr
id: 0009
title: amended rule
status: accepted
date: 2026-07-20
supersedes: ADR-0003
---

# ADR-0009: Amended rule

## References

- Amends ADR-0003.
`

// TestCheckCitations pins the reverse walk: a document quoting a superseded
// decision is flagged; the superseding ADR and the superseded ADR itself are
// legitimate citation sites and are not.
func TestCheckCitations(t *testing.T) {
	root := t.TempDir()
	write(t, root, ".metis/adr/0003-original.md", adr3)
	write(t, root, ".metis/adr/0009-amended.md", adr9)
	write(t, root, ".metis/briefs/ws-1.md", "Per ADR-0003, the footer must be a landmark.\n")
	write(t, root, ".metis/briefs/ws-2.md", "Per ADR-0009, the footer rule changed.\n")

	warnings := CheckCitations(root, ".metis/adr/", []string{".metis/briefs/", ".metis/adr/"})
	if len(warnings) != 1 {
		t.Fatalf("warnings = %v, want exactly the stale brief", warnings)
	}
	w := warnings[0]
	if !strings.Contains(w, ".metis/briefs/ws-1.md") || !strings.Contains(w, "ADR-0003") || !strings.Contains(w, "ADR-0009") {
		t.Errorf("warning %q should name the citing file, the stale ADR, and its successor", w)
	}
}

// TestCheckCitationsStatusOnly: an ADR marked superseded/deprecated without a
// named successor still flags its citers.
func TestCheckCitationsStatusOnly(t *testing.T) {
	root := t.TempDir()
	write(t, root, ".metis/adr/0003-original.md", strings.Replace(adr3, "status: accepted", "status: deprecated", 1))
	write(t, root, ".metis/briefs/ws-1.md", "Per ADR-0003, do the thing.\n")

	warnings := CheckCitations(root, ".metis/adr/", []string{".metis/briefs/", ".metis/adr/"})
	if len(warnings) != 1 || !strings.Contains(warnings[0], "no longer current") {
		t.Fatalf("warnings = %v, want one status-only flag", warnings)
	}
}

// TestCheckCitationsTemplateIgnored: the shipped _template.md carries
// placeholder frontmatter and placeholder citations — neither may register.
func TestCheckCitationsTemplateIgnored(t *testing.T) {
	root := t.TempDir()
	write(t, root, ".metis/adr/_template.md", "---\nid: NNNN\nstatus: proposed | accepted | superseded | deprecated\nsupersedes: <ADR-MMMM if applicable, omit otherwise>\n---\n")
	write(t, root, ".metis/briefs/ws-1.md", "No stale citations here.\n")

	if warnings := CheckCitations(root, ".metis/adr/", []string{".metis/briefs/"}); len(warnings) != 0 {
		t.Fatalf("warnings = %v, want none", warnings)
	}
}

// TestCheckCitationsNoADRDir: projects without ADRs stay silent.
func TestCheckCitationsNoADRDir(t *testing.T) {
	if warnings := CheckCitations(t.TempDir(), ".metis/adr/", []string{".metis/briefs/"}); warnings != nil {
		t.Fatalf("warnings = %v, want nil", warnings)
	}
}
