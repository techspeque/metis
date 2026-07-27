// Package templates provides document templates for Metis-managed artifacts.
// These templates enforce consistent structure across plans, ADRs, overviews,
// reconciliation reports, and phase gates.
package templates

import (
	"os"
	"path/filepath"
)

// WriteAll writes all templates to the given directory.
func WriteAll(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	templates := map[string]string{
		"plan.md":     PlanTemplate,
		"overview.md": OverviewTemplate,
		"adr.md":      ADRTemplate,
		"recon.md":    ReconTemplate,
		"gate.md":     GateTemplate,
	}

	for name, content := range templates {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// PlanTemplate is the implementation plan format that `metis seed` can parse.
const PlanTemplate = `---
type: plan
phase: <N>
title: <Phase title>
overview_ref: OVERVIEW.md §<section>
status: draft | approved | executing | completed
created: YYYY-MM-DD
---

# Phase <N> — <Title>

> Derived from: OVERVIEW.md §<section>
> Produces: slices phase-<N>-ws-<N.M> in .metis/slices.yaml
> ADRs: .metis/adr/NNNN-*.md (created alongside this plan)

## Context

<What this phase achieves. One paragraph. Reference the OVERVIEW section
that drives this phase. State what the system can do after this phase that
it cannot do before.>

## Dependencies

- Requires: <Phase N-1 complete, or "none" if first phase>
- External: <any external dependencies, services, credentials needed>

---

## Workstream <N.1>: <Title>

<!-- The heading number becomes the slice ID: "Workstream 0.1" under phase 0
     seeds slice phase-0-ws-0.1. -->

- **Risk:** low | medium | high
- **Coder:** <agent-slug — see ` + "`metis config get agents -o json`" + `>
- **Reviewer:** <agent-slug, must differ from coder>
- **Stage:** <free-form grouping label for progress reporting, e.g., foundation | mvp | beta — no fixed taxonomy>
- **Blocked by:** <workstream IDs if dependency exists, omit if none>

Tasks:
- <imperative verb> <specific deliverable>
- <imperative verb> <specific deliverable>
- <imperative verb> <specific deliverable>

Suggested packages:
- ` + "`path/to/package`" + `
- ` + "`path/to/other`" + `

Acceptance criteria:
- [ ] <testable, specific outcome — becomes the brief's Definition of Done>
- [ ] <testable, specific outcome>
- [ ] <testable, specific outcome>

---

## Workstream <N.2>: <Title>

- **Risk:** low | medium | high
- **Coder:** <agent-slug>
- **Reviewer:** <agent-slug>
- **Stage:** <taxonomy>

Tasks:
- <task>

Suggested packages:
- ` + "`path/to/package`" + `

Acceptance criteria:
- [ ] <outcome>
- [ ] <outcome>

---

## Phase Gate

> This section defines how Phase <N> is validated as a composed system.
> It becomes a ` + "`gate`" + ` slice automatically (phase-<N>-gate, risk high,
> blocked by every workstream in the phase).

Composition scenarios to validate:
- [ ] <integration scenario proving modules work together>
- [ ] <contract scenario proving interfaces align at seams>
- [ ] <end-to-end scenario proving the phase's stated goal>

Performance / resource checks:
- [ ] <metric within acceptable bounds>

---

## Sizing Guidance (for the planning agent — this section and Ordering
## Guidance are ignored by ` + "`metis seed`" + ` and may be kept or deleted)

Each workstream should be:
- ONE reviewable unit of work (a few hundred lines of diff, not thousands)
- Independently testable (has its own acceptance criteria)
- Completable in a single agent session
- Small enough that scope creep is immediately visible in review

If a workstream feels too large, split it. If it feels trivial, merge with adjacent.

## Ordering Guidance

- Declaration order = execution order within same priority
- Use "Blocked by" for hard dependencies
- Foundation/interface slices come before implementation slices
- Each workstream should be able to point its brief at real interfaces from prior slices
`

// OverviewTemplate is the structure for an application specification document.
const OverviewTemplate = `---
type: overview
project: <project-name>
status: living
last_updated: YYYY-MM-DD
---

# <Project Name> — Full Specification

> <One-line description of what this application does>

---

## 1. Purpose

<What this application does and why it exists. Who uses it and what problem
it solves. 2-3 paragraphs maximum. This section answers "why are we building
this?" — the motivation that drives all technical decisions below.>

---

## 2. Architecture

<High-level system design. Components, their responsibilities, and how they
interact. Use ASCII diagrams where helpful.>

### 2.1 System Boundaries

<What is inside the system vs. external. Integration points, APIs consumed,
services depended upon.>

### 2.2 Data Model

<Core entities, their relationships, and where they live. Schema decisions.
This is high-level — detailed schemas belong in ADRs or code.>

### 2.3 Component Structure

<Packages, modules, layers. The dependency graph between internal components.
Where the seam points are.>

---

## 3. Constraints

### 3.1 Technology

- **Language:** <primary language>
- **Build system:** <build tool>
- **Storage:** <database/storage approach>
- **Runtime:** <deployment target>

### 3.2 Non-Goals (do NOT build)

<Explicit things this project will never do. Agents must not build these
even if asked indirectly. These become ` + "`non_goals`" + ` in the project config
(the human applies them via ` + "`metis config set`" + `).>

- <non-goal>
- <non-goal>

### 3.3 Invariants (must ALWAYS hold)

<Project-wide rules that must never be violated. These become
` + "`accuracy_rules`" + ` (added via ` + "`metis rule add`" + ` or promoted from findings
via ` + "`metis rule promote`" + `) and are enforced in every review.>

- <invariant>
- <invariant>

---

## 4. Phases (High-Level Roadmap)

<Sketch all phases at a high level. Each phase gets detailed planning
(in .metis/plans/) only when you're about to execute it.>

### Phase 0 — Foundation
<2-3 sentences: what infrastructure/scaffolding this establishes>

### Phase 1 — Core
<2-3 sentences: what core functionality this delivers>

### Phase 2 — <Name>
<2-3 sentences>

### Phase N — Polish / Release
<2-3 sentences>

---

## 5. Security Model

<Authentication, authorization, trust boundaries, data sensitivity levels.
What needs to be protected and from whom.>

---

## 6. API / Interface Contracts

<Public interfaces this system exposes. REST endpoints, CLI commands,
library APIs, event schemas. Enough detail that consumers can code against
these contracts.>

---

## 7. Testing Strategy

<What gets tested at which level. Unit test boundaries, integration test
approach, contract tests, performance tests. Where mocking is appropriate
vs. hitting real dependencies.>

---

## 8. Operational Concerns

<Deployment, monitoring, logging, error handling strategy, configuration
management. How the system runs in production.>
`

// ADRTemplate is the Architecture Decision Record format.
const ADRTemplate = `---
type: adr
id: NNNN
title: <decision title>
status: proposed | accepted | superseded | deprecated
date: YYYY-MM-DD
phase: <N>
supersedes: <ADR-MMMM if this ADR replaces another, omit otherwise>
amends: <ADR-MMMM if this ADR modifies another without replacing it, omit otherwise>
---

# ADR-NNNN: <Decision Title>

> Phase: <N> | Status: <status> | Date: <YYYY-MM-DD>
> Decision drivers: <why this decision is being made now>

## Context

<The problem, the forces, the constraints. Keep it short — if this grows
into an essay, write a design doc and link it. Focus on WHY this decision
is needed, not what the decision is (that's the next section).>

## Decision

<What we are doing. Write in imperative mood. Be specific: 1-3 sentences
stating the decision, followed by the rules or invariants it implies.>

### Rules this decision implies:

1. <rule — concrete, enforceable, may become an accuracy_rule via ` + "`metis rule add`" + `>
2. <rule>
3. <rule>

## Consequences

### Positive
- <what gets easier>
- <what becomes possible>

### Negative
- <what gets harder>
- <what constraints are introduced>
- <what can fail>

### Neutral
- <what changes without being clearly better or worse>

## Alternatives Considered

| Option | Pros | Cons | Why rejected |
|---|---|---|---|
| <alternative 1> | <pros> | <cons> | <reason> |
| <alternative 2> | <pros> | <cons> | <reason> |

## References

- <link to relevant docs, prior ADRs, external resources>
`

// ReconTemplate is the format for reconciliation reports when the OVERVIEW changes.
const ReconTemplate = `---
type: recon
slice: <recon-slice-id>
overview_version: <sha256 short hash of current OVERVIEW>
date: YYYY-MM-DD
status: draft | complete
---

# Reconciliation Report — <slice-id>

> Triggered by: OVERVIEW.md change
> Date: <YYYY-MM-DD>
> Scope: pending slices, active plans, current ADRs

---

## 1. OVERVIEW Changes Summary

<Describe what changed in the OVERVIEW since last baseline. Be specific:
which sections were added, removed, or modified. Focus on changes that
affect pending or future work.>

### Added
- <new requirement/section>

### Modified
- <changed requirement — what was it before, what is it now>

### Removed
- <removed requirement>

---

## 2. Impact Assessment

### Affected Pending Slices

| Slice ID | Current Title | Impact | Action |
|---|---|---|---|
| <id> | <title> | <what changed for this slice> | edit / skip / no change |
| <id> | <title> | <what changed for this slice> | edit / skip / no change |

### Unaffected Slices (confirmed)

<List slices reviewed and confirmed as unaffected, so the record is complete.>

---

## 3. New Work Required

| Proposed Title | Type | Risk | Reason |
|---|---|---|---|
| <title> | feat/fix/refactor/... | low/medium/high | <why now needed> |

---

## 4. Documentation Updates

- [ ] Plan file <path> updated: <what changed>
- [ ] ADR <NNNN> superseded by ADR <MMMM>: <reason>
- [ ] ADR <NNNN> created: <new decision required by changes>
- [ ] Accuracy rules updated (` + "`metis rule add`" + ` / ` + "`metis rule promote`" + `): <if invariants changed>
- [ ] Non-goals updated (` + "`metis config set non_goals ...`" + `): <if scope boundaries changed>

---

## 5. Actions Taken

- [ ] ` + "`metis edit <id> --title \"...\" --risk ...`" + ` for affected slices
- [ ] ` + "`metis skip <id> --reason \"...\"`" + ` for obsolete slices
- [ ] ` + "`metis add <type> --title \"...\" ...`" + ` for new work
- [ ] ` + "`metis check`" + ` passes after all changes
- [ ] ` + "`metis surface generate`" + ` run (if rules/non-goals changed)

---

## 6. Verification

- [ ] No pending slice references removed OVERVIEW sections
- [ ] All new slices have valid coder/reviewer assignments
- [ ] Dependency ordering is still correct (no orphaned blocked_by)
- [ ] ` + "`metis check`" + ` passes clean
`

// GateTemplate is the format for phase gate evidence reports.
const GateTemplate = `---
type: gate
slice: <gate-slice-id>
phase: <N>
title: Phase <N> Gate — <Phase Title>
date: YYYY-MM-DD
verdict: pass | fail
---

# Phase <N> Gate — Evidence Report

> Slice: <gate-slice-id>
> Phase: <N> — <Phase Title>
> Date: <YYYY-MM-DD>

---

## 1. Prerequisite Check

- [ ] All phase <N> slices coded and reviewed
- [ ] No open P1 findings against phase <N> slices
- [ ] All phase <N> ADRs in accepted status
- [ ] ` + "`metis verify`" + ` passes on clean checkout

---

## 2. Composition Scenarios

> These prove the phase works as a composed system, not just individual slices.

### Scenario 1: <end-to-end scenario description>

- **Setup:** <preconditions>
- **Action:** <what was exercised>
- **Expected:** <expected behavior>
- **Actual:** <observed behavior>
- **Verdict:** pass | fail
- **Evidence:** <test name, log reference, or file:line>

### Scenario 2: <integration scenario description>

- **Setup:** <preconditions>
- **Action:** <what was exercised>
- **Expected:** <expected behavior>
- **Actual:** <observed behavior>
- **Verdict:** pass | fail
- **Evidence:** <reference>

### Scenario 3: <contract scenario at seam point>

- **Setup:** <preconditions>
- **Action:** <what was exercised>
- **Expected:** <expected behavior>
- **Actual:** <observed behavior>
- **Verdict:** pass | fail
- **Evidence:** <reference>

---

## 3. Interface Seam Verification

| Boundary | Provider | Consumer | Contract | Status |
|---|---|---|---|---|
| <seam name> | <module/package> | <module/package> | <interface/type> | verified / mismatch |
| <seam name> | <module/package> | <module/package> | <interface/type> | verified / mismatch |

---

## 4. Performance / Resource Check

| Metric | Value | Threshold | Status |
|---|---|---|---|
| <metric> | <measured> | <acceptable limit> | ok / concerning / fail |

---

## 5. Findings

<If composition failures were found, list them here. Each should be filed
via ` + "`metis block <offending-slice-id>`" + ` against the responsible slice.>

| Finding | Severity | Offending Slice | Filed As |
|---|---|---|---|
| <description> | P1/P2/P3 | <slice-id> | <finding-id> |

---

## 6. Verdict

**PASS** | **FAIL**

### If PASS:
Phase <N> is validated. The composed system meets the phase's stated goals.
Proceed to Phase <N+1> planning.

### If FAIL:
<Which scenarios failed. Which slices are blocked. What rework is needed
before the gate can be re-evaluated.>
`
