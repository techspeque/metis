package brief

import (
	"strings"
	"testing"

	"github.com/techspeque/metis/internal/slice"
)

func TestRender_Feat(t *testing.T) {
	s := &slice.Slice{
		ID: "feat-0001", Title: "Add auth", Type: slice.TypeFeat,
		Risk: slice.RiskMedium, Priority: slice.PriorityP2,
		Coder: "opencode/opus", Plan: "plans/impl.md", PlanSection: "§2.1",
	}
	out := Render(s)

	checks := []string{
		"# feat-0001 — Add auth",
		"**Type:** feat",
		"**Risk:** medium",
		"**Coder:** opencode/opus",
		"**Plan:** plans/impl.md §2.1",
		"## Goal",
		"## Declared file scope",
		"## Definition of Done",
		"## Test plan",
		"## Out-of-scope touches",
	}
	for _, c := range checks {
		if !strings.Contains(out, c) {
			t.Errorf("feat brief missing %q", c)
		}
	}
}

func TestRender_Fix(t *testing.T) {
	s := &slice.Slice{
		ID: "fix-0001", Title: "Fix auth bypass", Type: slice.TypeFix,
		Risk: slice.RiskHigh, Priority: slice.PriorityP0,
		Coder: "claude-code/opus",
	}
	out := Render(s)

	if !strings.Contains(out, "# fix-0001 — Fix auth bypass") {
		t.Error("fix brief missing header")
	}
	// Fix uses standard template
	if !strings.Contains(out, "## Goal") {
		t.Error("fix brief missing Goal section")
	}
}

func TestRender_Refactor(t *testing.T) {
	s := &slice.Slice{
		ID: "refactor-0001", Title: "Consolidate middleware", Type: slice.TypeRefactor,
		Risk: slice.RiskHigh, Priority: slice.PriorityP2,
		Coder: "opencode/opus",
	}
	out := Render(s)

	checks := []string{
		"## Affected paths",
		"## Migration strategy",
		"## Behavioral contract",
	}
	for _, c := range checks {
		if !strings.Contains(out, c) {
			t.Errorf("refactor brief missing %q", c)
		}
	}
}

func TestRender_Debt(t *testing.T) {
	s := &slice.Slice{
		ID: "debt-0001", Title: "Replace errors", Type: slice.TypeDebt,
		Risk: slice.RiskLow, Priority: slice.PriorityP3,
		Coder: "codex",
	}
	out := Render(s)

	// Debt uses refactor template
	if !strings.Contains(out, "## Migration strategy") {
		t.Error("debt brief should use refactor template")
	}
}

func TestRender_Remove(t *testing.T) {
	s := &slice.Slice{
		ID: "remove-0001", Title: "Drop v1 API", Type: slice.TypeRemove,
		Risk: slice.RiskMedium, Priority: slice.PriorityP2,
		Coder: "codex",
	}
	out := Render(s)

	checks := []string{
		"**Type:** remove",
		"## Removal scope",
		"## Verification",
		"Code to delete",
	}
	for _, c := range checks {
		if !strings.Contains(out, c) {
			t.Errorf("remove brief missing %q", c)
		}
	}
}

func TestRender_Gate(t *testing.T) {
	s := &slice.Slice{
		ID: "phase-1-gate", Title: "Phase 1 validation", Type: slice.TypeGate,
		Risk: slice.RiskHigh, Priority: slice.PriorityP2,
		Coder: "claude-code/opus",
	}
	out := Render(s)

	checks := []string{
		"**Type:** gate",
		"## Phase being validated",
		"## Composition scenarios",
		"## Evidence criteria",
	}
	for _, c := range checks {
		if !strings.Contains(out, c) {
			t.Errorf("gate brief missing %q", c)
		}
	}
}

func TestParseOwnedPaths(t *testing.T) {
	inline := "- **owned_paths:** src/auth/, cmd/main.go\n- **read_only_paths:** internal/\n"
	got := ParseOwnedPaths(inline)
	if len(got) != 2 || got[0] != "src/auth/" || got[1] != "cmd/main.go" {
		t.Errorf("inline parse = %v", got)
	}

	bullets := "- **owned_paths:**\n  - internal/foo/\n  - pkg/bar.go\n- **read_only_paths:** x\n"
	got = ParseOwnedPaths(bullets)
	if len(got) != 2 || got[0] != "internal/foo/" || got[1] != "pkg/bar.go" {
		t.Errorf("bullet parse = %v", got)
	}

	// Unfilled template placeholder must not count as scope.
	if got := ParseOwnedPaths("- **owned_paths:** exact files this slice may edit\n"); len(got) != 0 {
		t.Errorf("placeholder should parse to no paths, got %v", got)
	}
}

// Reproduction from the metiswww dogfood: an annotated sub-bullet used to
// terminate collection, silently dropping the path and every path after it —
// the audit then reported the file out of scope and cost a review cycle.
func TestParseOwnedPathsAnnotatedBullet(t *testing.T) {
	content := "- **owned_paths:**\n" +
		"  - src/index.html\n" +
		"  - docs/copy.md — **added in review cycle 1**, to close f-009 (§7)\n" +
		"  - src/style.css\n" +
		"- **read_only_paths:** internal/\n"
	got, warnings := ParseOwnedPathsWithWarnings(content)
	want := []string{"src/index.html", "docs/copy.md", "src/style.css"}
	if len(got) != len(want) {
		t.Fatalf("parse = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("parse[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	if len(warnings) != 0 {
		t.Errorf("clean annotation stripping should not warn, got %v", warnings)
	}

	// Inline form with a dash annotation on one entry.
	inline := "- **owned_paths:** src/x.go — hot path, docs/y.md\n"
	got, _ = ParseOwnedPathsWithWarnings(inline)
	if len(got) != 2 || got[0] != "src/x.go" || got[1] != "docs/y.md" {
		t.Errorf("inline annotated parse = %v", got)
	}

	// An entry that still contains whitespace after cleaning is suspicious
	// and must be named — silence here is what caused the dogfood cycle.
	suspect := "- **owned_paths:**\n  - docs/copy.md (added later)\n"
	got, warnings = ParseOwnedPathsWithWarnings(suspect)
	if len(got) != 1 {
		t.Fatalf("suspect parse = %v", got)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "whitespace") {
		t.Errorf("expected whitespace warning, got %v", warnings)
	}
}

func TestInScope(t *testing.T) {
	owned := []string{"src/auth/", "cmd/main.go"}
	for file, want := range map[string]bool{
		"src/auth/login.go": true,
		"cmd/main.go":       true,
		"src/other/x.go":    false,
		"cmd/main.gopher":   false,
	} {
		if InScope(file, owned) != want {
			t.Errorf("InScope(%q) != %v", file, want)
		}
	}
}
