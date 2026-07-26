// Package brief handles brief template generation and rendering for slices.
package brief

import (
	"fmt"
	"strings"
	"time"

	"github.com/techspeque/metis/internal/slice"
)

// Render generates a brief template for the given slice.
func Render(s *slice.Slice) string {
	switch s.Type {
	case slice.TypeRefactor, slice.TypeDebt:
		return renderRefactor(s)
	case slice.TypeRemove:
		return renderRemove(s)
	case slice.TypeGate:
		return renderGate(s)
	default:
		return renderStandard(s)
	}
}

func renderStandard(s *slice.Slice) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# %s — %s\n\n", s.ID, s.Title)
	fmt.Fprintf(&b, "- **Type:** %s | **Risk:** %s | **Priority:** %s\n", s.Type, s.Risk, s.Priority)
	fmt.Fprintf(&b, "- **Coder:** %s | **Date:** %s\n", s.Coder, today())
	if s.Plan != "" {
		fmt.Fprintf(&b, "- **Plan:** %s %s\n", s.Plan, s.PlanSection)
	}

	b.WriteString("\n## Goal\n\nOne sentence, drawn from the plan's stated goal.\n")
	b.WriteString("\n## Architectural context\n\nInterfaces, types, packages, and schema this slice consumes or implements.\nRead from `metis interfaces` output or prior briefs.\n")
	b.WriteString("\n## Declared file scope\n\n- **owned_paths:** exact files or directories this slice may edit (plain paths, comma-separated or as sub-bullets; directories match by prefix)\n- **read_only_paths:** packages/files this slice may inspect but not modify\n")
	b.WriteString("\n## Definition of Done\n\nSpecific, testable criteria.\n")
	b.WriteString("\n## Test plan\n\nWhich tests will exist and what they prove.\n")
	b.WriteString("\n## Out-of-scope touches\n\nEmpty unless a fix outside declared scope proved genuinely required.\nEach entry: what, where, and why.\n")

	return b.String()
}

func renderRefactor(s *slice.Slice) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# %s — %s\n\n", s.ID, s.Title)
	fmt.Fprintf(&b, "- **Type:** %s | **Risk:** %s | **Priority:** %s\n", s.Type, s.Risk, s.Priority)
	fmt.Fprintf(&b, "- **Coder:** %s | **Date:** %s\n", s.Coder, today())

	b.WriteString("\n## Goal\n\nOne sentence: what structural improvement, no behavior change.\n")
	b.WriteString("\n## Affected paths (broad scope)\n\n- <paths that will change structurally>\n")
	b.WriteString("\n## Migration strategy\n\n1. Introduce new structure\n2. Migrate callers\n3. Remove old structure\n4. Update docs/tests\n")
	b.WriteString("\n## Behavioral contract\n\nThese tests MUST pass unchanged (proving no behavior change):\n- <list>\n\nThese tests will be modified (testing new structure):\n- <list>\n")
	b.WriteString("\n## Definition of Done\n\n- New structure in place\n- All callers migrated\n- Old structure removed\n- Test coverage maintained or improved\n- `metis verify` green\n")
	b.WriteString("\n## Out-of-scope touches\n")

	return b.String()
}

func renderRemove(s *slice.Slice) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# %s — %s\n\n", s.ID, s.Title)
	fmt.Fprintf(&b, "- **Type:** remove | **Risk:** %s | **Priority:** %s\n", s.Risk, s.Priority)
	fmt.Fprintf(&b, "- **Coder:** %s | **Date:** %s\n", s.Coder, today())

	b.WriteString("\n## Goal\n\nWhat is being removed and why.\n")
	b.WriteString("\n## Removal scope\n\n- **Code to delete:** <files/modules>\n- **Tests to delete:** <test files that test deleted code>\n- **Docs to update:** <references to update/remove>\n- **Config to clean:** <configuration referencing deleted code>\n")
	b.WriteString("\n## Verification\n\n- No dangling references (compile clean)\n- No orphaned tests\n- Remaining test suite passes\n- Docs updated\n")
	b.WriteString("\n## Definition of Done\n\n- Target code is gone\n- Nothing references it\n- Tests pass\n- Docs reflect the removal\n")
	b.WriteString("\n## Out-of-scope touches\n")

	return b.String()
}

func renderGate(s *slice.Slice) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# %s — %s\n\n", s.ID, s.Title)
	fmt.Fprintf(&b, "- **Type:** gate | **Risk:** high | **Priority:** %s\n", s.Priority)
	fmt.Fprintf(&b, "- **Coder:** %s | **Date:** %s\n", s.Coder, today())

	b.WriteString("\n## Phase being validated\n\nPhase <N>: <title>\n")
	b.WriteString("\n## Composition scenarios\n\nWhat integration/contract scenarios prove the phase works as a composed system:\n1. <scenario>\n2. <scenario>\n")
	b.WriteString("\n## Evidence criteria\n\n- [ ] All phase slices coded and reviewed\n- [ ] Cross-module integration tests pass\n- [ ] No interface mismatches at seam points\n- [ ] Performance/resource usage acceptable\n")
	b.WriteString("\n## Report\n\n(Filled during execution with actual evidence)\n\n> Full gate-report structure: .metis/templates/gate.md\n")

	return b.String()
}

func today() string {
	return time.Now().Format("2006-01-02")
}
