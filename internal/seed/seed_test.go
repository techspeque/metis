package seed

import (
	"testing"

	"github.com/techspeque/metis/internal/slice"
)

const samplePlan = `## Phase 1 — Foundation

### Workstream 1.1: Domain model
- **Risk:** high
- **Coder:** claude-code/opus
- **Reviewer:** codex
- **Stage:** mvp

Tasks:
- Define core entities
- Create repository interfaces

Acceptance criteria:
- Domain types compile
- Interfaces documented

### Workstream 1.2: Storage layer
- **Risk:** medium
- **Coder:** codex
- **Reviewer:** opencode/opus
- **Stage:** mvp

Tasks:
- Implement SQLite storage
- Add migrations

Acceptance criteria:
- CRUD operations work
- Migrations run cleanly

## Phase 2 — API

### Workstream 2.1: REST endpoints
- **Risk:** medium
- **Coder:** opencode/opus
- **Reviewer:** claude-code/opus
- **Stage:** beta

Tasks:
- Define routes
- Implement handlers

Acceptance criteria:
- All routes respond
- Auth enforced
`

func TestParse_BasicPlan(t *testing.T) {
	result := Parse(samplePlan)

	if len(result.Workstreams) != 3 {
		t.Fatalf("expected 3 workstreams, got %d", len(result.Workstreams))
	}

	ws := result.Workstreams[0]
	if ws.Phase != 1 {
		t.Errorf("ws[0] Phase = %d, want 1", ws.Phase)
	}
	if ws.Workstream != "1.1" {
		t.Errorf("ws[0] Workstream = %q, want %q", ws.Workstream, "1.1")
	}
	if ws.Title != "Domain model" {
		t.Errorf("ws[0] Title = %q", ws.Title)
	}
	if ws.Risk != slice.RiskHigh {
		t.Errorf("ws[0] Risk = %q, want high", ws.Risk)
	}
	if ws.Coder != "claude-code/opus" {
		t.Errorf("ws[0] Coder = %q", ws.Coder)
	}
	if ws.Reviewer != "codex" {
		t.Errorf("ws[0] Reviewer = %q", ws.Reviewer)
	}
	if ws.Stage != "mvp" {
		t.Errorf("ws[0] Stage = %q", ws.Stage)
	}
	if len(ws.Tasks) != 2 {
		t.Errorf("ws[0] Tasks = %d, want 2", len(ws.Tasks))
	}
	if len(ws.Acceptance) != 2 {
		t.Errorf("ws[0] Acceptance = %d, want 2", len(ws.Acceptance))
	}
}

func TestParse_MultiplePhases(t *testing.T) {
	result := Parse(samplePlan)

	// Phase 1 has 2 workstreams, Phase 2 has 1
	phase1 := 0
	phase2 := 0
	for _, ws := range result.Workstreams {
		switch ws.Phase {
		case 1:
			phase1++
		case 2:
			phase2++
		}
	}
	if phase1 != 2 {
		t.Errorf("Phase 1: %d workstreams, want 2", phase1)
	}
	if phase2 != 1 {
		t.Errorf("Phase 2: %d workstreams, want 1", phase2)
	}
}

func TestParse_EmptyInput(t *testing.T) {
	result := Parse("")
	if len(result.Workstreams) != 0 {
		t.Errorf("empty input: got %d workstreams", len(result.Workstreams))
	}
}

func TestParse_NoWorkstreams(t *testing.T) {
	input := "## Phase 1 — Foundation\n\nSome text without workstreams.\n"
	result := Parse(input)
	if len(result.Workstreams) != 0 {
		t.Errorf("no workstreams input: got %d", len(result.Workstreams))
	}
}

func TestToSlices(t *testing.T) {
	result := Parse(samplePlan)
	slices := ToSlices(result.Workstreams, ".metis/plans/impl.md", slice.TypeFeat)

	if len(slices) != 3 {
		t.Fatalf("expected 3 slices, got %d", len(slices))
	}

	s := slices[0]
	if s.ID != "phase-1-ws-1.1" {
		t.Errorf("ID = %q, want %q", s.ID, "phase-1-ws-1.1")
	}
	if s.Title != "Domain model" {
		t.Errorf("Title = %q", s.Title)
	}
	if s.Type != slice.TypeFeat {
		t.Errorf("Type = %q, want feat", s.Type)
	}
	if s.Risk != slice.RiskHigh {
		t.Errorf("Risk = %q, want high", s.Risk)
	}
	if s.Plan != ".metis/plans/impl.md" {
		t.Errorf("Plan = %q", s.Plan)
	}
	if s.PlanSection != "§1.1" {
		t.Errorf("PlanSection = %q, want §1.1", s.PlanSection)
	}

	// Check phase 2 workstream
	s2 := slices[2]
	if s2.ID != "phase-2-ws-2.1" {
		t.Errorf("slice[2] ID = %q, want %q", s2.ID, "phase-2-ws-2.1")
	}
	if s2.Stage != "beta" {
		t.Errorf("slice[2] Stage = %q, want beta", s2.Stage)
	}
}
