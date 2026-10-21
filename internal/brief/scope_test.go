package brief

import (
	"strings"
	"testing"
)

// The same mistake reached review seventeen times across one project's
// history: an owned_paths entry with prose in it, parsed as a path, with
// every real file then reported out of scope. Validate names it before
// the brief is committed.
func TestScopeValidateRefusesWhatTheAuditCannotRead(t *testing.T) {
	for name, tc := range map[string]struct {
		content string
		want    string
	}{
		"no field": {
			"## Goal\n\nShip it.\n",
			"declares no owned_paths",
		},
		"template placeholder": {
			"- **owned_paths:** exact files or directories this slice may edit (plain paths)\n- **read_only_paths:** packages/files this slice may inspect but not modify\n",
			"template placeholder",
		},
		"annotated entry": {
			"- **owned_paths:**\n  - internal/api/ (handlers only)\n",
			"contains whitespace",
		},
		"owned and read-only": {
			"- **owned_paths:** internal/api/, cmd/main.go\n- **read_only_paths:** internal/api\n",
			"both owned and read-only",
		},
		"none with paths": {
			"- **owned_paths:** none\n  - internal/api/\n",
			"says none",
		},
	} {
		t.Run(name, func(t *testing.T) {
			problems := ParseScope(tc.content).Validate()
			if len(problems) == 0 {
				t.Fatalf("validated a brief the audit cannot measure:\n%s", tc.content)
			}
			if !strings.Contains(strings.Join(problems, "; "), tc.want) {
				t.Errorf("problems = %v, want one naming %q", problems, tc.want)
			}
		})
	}
}

// Read-only work declares an empty scope on purpose, and the audit then
// treats every touched file as out of scope — the opposite of "no scope".
func TestScopeNoneIsADeclaredEmptyScope(t *testing.T) {
	sc := ParseScope("- **owned_paths:** none\n- **read_only_paths:** everything\n")
	if !sc.OwnedDeclared || len(sc.Owned) != 0 {
		t.Errorf("scope = %+v, want declared and empty", sc)
	}
	if problems := sc.Validate(); len(problems) != 0 {
		t.Errorf("none should validate, got %v", problems)
	}
	if InScope("README.md", sc.Owned) {
		t.Error("an empty scope admitted a file")
	}

	missing := ParseScope("- **read_only_paths:** everything\n")
	if missing.OwnedDeclared {
		t.Error("a brief without owned_paths reads as declared")
	}
}

// read_only_paths is prose as often as paths; only its path-shaped entries
// take part in the overlap check, and prose there never refuses a brief.
func TestScopeReadOnlyProseIsNotAnError(t *testing.T) {
	sc := ParseScope("- **owned_paths:** internal/api/\n- **read_only_paths:** everything else, platform/wire/ for the decoder\n")
	if problems := sc.Validate(); len(problems) != 0 {
		t.Errorf("prose in read_only_paths refused the brief: %v", problems)
	}
	if len(sc.ReadOnly) != 0 {
		t.Errorf("ReadOnly = %v, want prose dropped", sc.ReadOnly)
	}
	sc = ParseScope("- **owned_paths:** internal/api/\n- **read_only_paths:** platform/wire/, docs/\n")
	if len(sc.ReadOnly) != 2 {
		t.Errorf("ReadOnly = %v, want two paths", sc.ReadOnly)
	}
	if problems := sc.Validate(); len(problems) != 0 {
		t.Errorf("disjoint paths refused: %v", problems)
	}
}

// "Edit this file, read the package around it" and "edit the package,
// leave this file alone" are the two ordinary ways a brief narrows its
// scope; neither is a contradiction, and refusing them would refuse a
// sixth of a real project's briefs.
func TestScopeNestedReadOnlyIsNotAnOverlap(t *testing.T) {
	for name, content := range map[string]string{
		"owned file in a read-only package":  "- **owned_paths:** internal/store/ledger.go\n- **read_only_paths:** internal/store/\n",
		"read-only file in an owned package": "- **owned_paths:** internal/store/\n- **read_only_paths:** internal/store/events.go\n",
	} {
		if problems := ParseScope(content).Validate(); len(problems) != 0 {
			t.Errorf("%s: refused as %v", name, problems)
		}
	}
}

// A well-formed brief, with annotations after em-dashes and a wrapped
// annotation, validates: the refusal is for what the parser cannot read,
// not for style.
func TestScopeValidateAcceptsTheConvention(t *testing.T) {
	content := "- **owned_paths:**\n" +
		"  - internal/api/\n" +
		"  - docs/copy.md — added in review cycle 1, to close f-009; the\n" +
		"    wording follows the plan's §7\n" +
		"- **read_only_paths:** internal/model/\n"
	sc := ParseScope(content)
	if problems := sc.Validate(); len(problems) != 0 {
		t.Errorf("problems = %v, want none", problems)
	}
	if len(sc.Owned) != 2 || sc.Owned[1] != "docs/copy.md" {
		t.Errorf("Owned = %v", sc.Owned)
	}
}
