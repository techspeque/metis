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
