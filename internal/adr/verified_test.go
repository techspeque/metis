package adr

import (
	"strings"
	"testing"
)

const adr12 = `---
type: adr
id: 0012
title: amended again
status: accepted
date: 2026-08-01
supersedes: ADR-0003
---

# ADR-0012: Amended again
`

// A hundred and ten warnings nobody reads are not a signal. A document
// that has re-read the amended decision says so by naming the amending
// ADR, and that one citation goes quiet — not the class.
func TestVerifiedAgainstSilencesOneCitation(t *testing.T) {
	root := t.TempDir()
	write(t, root, ".metis/adr/0003-original.md", adr3)
	write(t, root, ".metis/adr/0009-amended.md", adr9)
	write(t, root, ".metis/plans/checked.md", "Per ADR-0003, the footer must be a landmark.\n\n<!-- verified-against: ADR-0009 -->\n")
	write(t, root, ".metis/plans/unchecked.md", "Per ADR-0003, the footer must be a landmark.\n")

	r := Citations(root, ".metis/adr/", []string{".metis/plans/"})
	if len(r.Warnings) != 1 || !strings.HasPrefix(r.Warnings[0], ".metis/plans/unchecked.md") {
		t.Errorf("warnings = %v, want only the unchecked plan", r.Warnings)
	}
	if len(r.Silenced) != 1 || !strings.HasPrefix(r.Silenced[0], ".metis/plans/checked.md") {
		t.Errorf("silenced = %v, want the checked plan", r.Silenced)
	}
	// The warning tells the reader how to close it.
	if !strings.Contains(r.Warnings[0], "verified-against: ADR-0009") {
		t.Errorf("warning does not name the annotation: %s", r.Warnings[0])
	}
}

// The annotation names the ADR the document was verified against. When a
// later ADR amends the decision instead, that name is wrong and the
// warning comes back: a suppression never outlives its inputs.
func TestVerifiedAgainstLapsesWhenTheAmendingADRChanges(t *testing.T) {
	root := t.TempDir()
	write(t, root, ".metis/adr/0003-original.md", adr3)
	write(t, root, ".metis/adr/0009-amended.md", adr9)
	write(t, root, ".metis/plans/checked.md", "Per ADR-0003, the footer must be a landmark.\n<!-- verified-against: ADR-0009 -->\n")
	if r := Citations(root, ".metis/adr/", []string{".metis/plans/"}); len(r.Warnings) != 0 {
		t.Fatalf("warnings before the change = %v", r.Warnings)
	}

	write(t, root, ".metis/adr/0012-amended-again.md", adr12)
	r := Citations(root, ".metis/adr/", []string{".metis/plans/"})
	if len(r.Warnings) != 1 || !strings.Contains(r.Warnings[0], "ADR-0012") {
		t.Errorf("warnings after a new amending ADR = %v, want the citation flagged against ADR-0012", r.Warnings)
	}
	if len(r.Silenced) != 0 {
		t.Errorf("stale annotation still silences: %v", r.Silenced)
	}
}

// Naming an unrelated ADR, or naming the superseded one when an amending
// ADR exists, verifies nothing. A decision retired by status alone has no
// amending ADR and is verified by naming it directly.
func TestVerifiedAgainstMustNameTheRightADR(t *testing.T) {
	root := t.TempDir()
	write(t, root, ".metis/adr/0003-original.md", adr3)
	write(t, root, ".metis/adr/0009-amended.md", adr9)
	write(t, root, ".metis/adr/0005-retired.md", strings.Replace(strings.ReplaceAll(adr3, "0003", "0005"), "accepted", "deprecated", 1))
	write(t, root, ".metis/plans/wrong.md", "Per ADR-0003 and ADR-0005.\n<!-- verified-against: ADR-0003 -->\n<!-- verified-against: ADR-0099 -->\n")
	write(t, root, ".metis/plans/right.md", "Per ADR-0003 and ADR-0005.\n<!-- verified-against: 0009 -->\n<!-- verified-against: adr-0005 -->\n")

	r := Citations(root, ".metis/adr/", []string{".metis/plans/"})
	if len(r.Warnings) != 2 {
		t.Errorf("warnings = %v, want both of wrong.md's citations", r.Warnings)
	}
	for _, w := range r.Warnings {
		if !strings.HasPrefix(w, ".metis/plans/wrong.md") {
			t.Errorf("unexpected warning %q", w)
		}
	}
	if len(r.Silenced) != 2 {
		t.Errorf("silenced = %v, want both of right.md's citations", r.Silenced)
	}
	// CheckCitations keeps its contract for callers that only print.
	if got := CheckCitations(root, ".metis/adr/", []string{".metis/plans/"}); len(got) != 2 {
		t.Errorf("CheckCitations = %v", got)
	}
}
