# Document Templates

> **Audience:** agents produce these documents; humans review and approve
> them.

Metis provides structured templates in `.metis/templates/` that agents use
to produce consistently-formatted artifacts. These are created during
`metis init` and are designed for **agent consumption** — comprehensive enough
that an agent can pick up any template and produce correct output without
additional human guidance.

## Available Templates

| Template | Purpose | Produces |
|---|---|---|
| `plan.md` | Implementation plan for a phase | `.metis/plans/phase-N.md` |
| `overview.md` | Application specification | `OVERVIEW.md` |
| `adr.md` | Architecture Decision Record | `.metis/adr/NNNN-title.md` |
| `recon.md` | Reconciliation report | Committed as the recon slice's brief |
| `gate.md` | Phase gate evidence report | Committed as the gate slice's brief |

---

## Structural Conventions

All templates share consistent conventions that enforce structure across
documents:

### Metadata Header

Every document begins with a YAML-style metadata block:

```markdown
---
type: plan | overview | adr | recon | gate
status: draft | approved | executing | completed
date: YYYY-MM-DD
---
```

This tells any agent reading the document what kind of artifact it is and
what state it's in.

### Traceability References

Every document explicitly states what it derives from and what derives from it:

```markdown
> Derived from: OVERVIEW.md §<section>
> Produces: slices phase-N-ws-N.M in .metis/slices.yaml
```

### Checkable Criteria

All acceptance criteria, evidence items, and verification steps use checkbox
format:

```markdown
- [ ] Specific, testable outcome
- [ ] Another measurable criterion
```

### Cross-References

Standard notation for referencing other Metis artifacts:

- Plan sections: `§N.M` (e.g., `§2.3`)
- ADRs: `ADR-NNNN` (e.g., `ADR-0003`)
- Slices: `phase-N-ws-N.M` or `type-NNNN` (e.g., `feat-0012`)
- OVERVIEW sections: `OVERVIEW.md §N`

---

## Template: `plan.md`

**Purpose:** Create implementation plans that `metis seed` can parse.

**Key constraints:**
- Workstream headings MUST follow the format: `### Workstream N.M: Title`
- Metadata fields (`Risk`, `Coder`, `Reviewer`, `Stage`) MUST use exact casing
- Each workstream = one slice = one reviewable unit of work

**How to use:** Tell an agent: "Create an implementation plan for Phase N
using the template at `.metis/templates/plan.md` and the OVERVIEW as input."

**What `metis seed` extracts:**
- Phase/workstream hierarchy → slice IDs (`phase-N-ws-N.M`)
- Risk/coder/reviewer/stage → slice metadata
- Plan file + section references → traceability
- Workstream titles → slice titles

---

## Template: `overview.md`

**Purpose:** Structure an application specification document.

**Key constraints:**
- Sections are numbered and ordered (1-8)
- Non-goals and invariants sections feed directly into `.metis/project.yaml` configuration
- Phase sketch provides the high-level roadmap (detailed planning is per-phase)

**How to use:** When starting a new project, use this template as the
structure for your OVERVIEW.md. You write it (with agent assistance if desired),
then agents reference it during planning and execution.

---

## Template: `adr.md`

**Purpose:** Record binding architecture decisions.

**Key constraints:**
- One ADR per decision — keep them short and specific
- Status field tracks lifecycle (proposed → accepted → superseded)
- "Rules this decision implies" section feeds into accuracy_rules
- Immutable once accepted — supersede, never edit

**When to create an ADR:**
- During phase planning (binding decisions for the phase)
- When a slice reveals a decision that affects multiple future slices
- When a review finding reveals an undocumented architectural constraint

---

## Template: `recon.md`

**Purpose:** Document what changed in the OVERVIEW and reconcile pending work.

**Used by:** The agent assigned to a `recon` slice (created via `metis recon`).

**Key constraints:**
- Must assess ALL pending slices (not just obviously affected ones)
- Must explicitly state "no change" for unaffected slices (complete record)
- Must never recommend changes to completed/archived work
- Actions Taken section must include the actual `metis` commands run

**How to use:** When `metis recon` creates a recon slice, the assigned agent
reads this template, reads the OVERVIEW diff, and produces the report as
the slice's brief.

---

## Template: `gate.md`

**Purpose:** Validate that a phase works as a composed system.

**Used by:** The agent assigned to a `gate` slice.

**Key constraints:**
- Must verify composition, not just individual slice correctness
- Scenarios test cross-module integration, not unit behavior
- Interface seam table proves contracts are aligned at boundaries
- Verdict is binary: PASS or FAIL (no "partial pass")

**When to use:** At the end of each phase. The gate slice is typically the
last slice in a phase, with `blocked_by` set to all other phase slices.

---

## Customizing Templates

Templates are generated during `metis init` and live in your repository.
You can edit them to match your project's specific needs:

- Add project-specific sections
- Adjust the metadata fields
- Add examples relevant to your domain
- Tighten or relax structural requirements

After editing, run `metis surface generate` to regenerate AGENTS.md (which
includes the full instructions that reference these conventions).
