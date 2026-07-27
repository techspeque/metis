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

// TestCheckCitationsQualifiedAndAmends covers the real-world frontmatter
// shapes from the metiswww dogfood: a supersedes value with a trailing
// qualifier, and an amends: key for decisions modified without replacement.
func TestCheckCitationsQualifiedAndAmends(t *testing.T) {
	root := t.TempDir()
	write(t, root, ".metis/adr/0002-policy.md", "---\nid: 0002\nstatus: accepted\n---\n# ADR-0002: Policy\n")
	write(t, root, ".metis/adr/0005-observer.md", "---\nid: 0005\nstatus: accepted\nsupersedes: ADR-0002 (script-purpose clause only)\n---\n# ADR-0005: Observer\n")
	write(t, root, ".metis/adr/0008-stats.md", "---\nid: 0008\nstatus: accepted\n---\n# ADR-0008: Stats\n")
	write(t, root, ".metis/adr/0009-human-owned.md", "---\nid: 0009\nstatus: accepted\namends: ADR-0008\n---\n# ADR-0009: Human owned\n")
	write(t, root, ".metis/briefs/ws-1.md", "Per ADR-0002 scripts must animate; ADR-0008 says stats are derived.\n")

	warnings := CheckCitations(root, ".metis/adr/", []string{".metis/briefs/"})
	if len(warnings) != 2 {
		t.Fatalf("warnings = %v, want qualified-supersede and amends flags", warnings)
	}
	joined := strings.Join(warnings, "\n")
	if !strings.Contains(joined, "ADR-0002, superseded by ADR-0005") {
		t.Errorf("missing qualified-supersede warning in %q", joined)
	}
	if !strings.Contains(joined, "ADR-0008, amended by ADR-0009") {
		t.Errorf("missing amends warning in %q", joined)
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
