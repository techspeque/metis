# CAESAR Plan Refactor Prompt

You are a senior software architect. `docs/OVERVIEW.md` has changed and the
plans must be updated to reflect the new specification. Your task is to
reconcile the plans with the updated OVERVIEW while preserving all progress
already made.

**Your output will be consumed by machines.** Updated workstreams become updated
slices; new workstreams become new slices. Be precise and testable. Do not
regenerate from scratch — this is a delta update.

---

## Source material

Read ALL of the following in full before producing anything:

### The changed specification

1. `docs/OVERVIEW.md` — **PRIMARY.** The updated product specification. This is
   the new source of truth for what to build.

### Existing architecture and contracts (what has already been decided)

2. `docs/generated/interfaces.txt` — Current Go interfaces, structs, and public
   signatures on `dev`. These represent code that EXISTS and cannot be
   contradicted without explicit rework slices.
3. `docs/adr/` — ALL architecture decision records (0001 through latest). These
   are committed decisions. If the OVERVIEW change contradicts an ADR, you must
   propose a superseding ADR — never silently ignore the conflict.
4. `docs/ARCHITECTURE.md` — System architecture (if it exists).
5. `docs/ROADMAP.md` — Phase-level roadmap (if it exists).

### Current progress (what has already been built or is in-flight)

6. `plans/slices-done.yaml` — Completed slices. These are IMMUTABLE historical
   records. Do not modify, renumber, or remove entries.
7. `plans/slices.yaml` — Active and pending slices. Slices with `coded: true`
   represent committed code. Slices with `coded: false` are pending and may be
   modified.
8. `plans/briefs/` — ALL committed briefs. These document the exact scope of
   completed work and the interfaces/contracts they established.
9. `plans/caesar-implementation-plan.md` — The current plan being executed.
10. Run `git log --oneline dev` — Actual commits are the ground truth of what
    code exists.

### Process constraints

11. `AGENTS.md` — The governing contract (especially §6 hot-path zones, §9
    testing rules, §10 non-goals, §11 accuracy rules, §8 model routing).
12. `harness/PLAN.md` — Planning prompt structure and quality criteria. Your
    updated plan must still satisfy the quality criteria defined there.
13. `harness/slice-ledger.md` — Ledger schema and field semantics.

---

## Constraints

1. **Completed slices are immutable.** Do not modify, renumber, or remove any
   slice where `coded: true` AND `reviewed: true`. Do not modify entries in
   `plans/slices-done.yaml`. Commit messages reference slice IDs — changing them
   breaks traceability.

2. **In-flight slices require escalation.** If the OVERVIEW change affects a
   slice that is `coded: true` but `reviewed: false`, or one that an agent is
   actively working on, flag it in your report for human decision. Do not
   silently re-scope it.

3. **Existing interfaces are real.** `docs/generated/interfaces.txt` and the
   actual source code on `dev` represent real contracts consumed by real code.
   If the OVERVIEW now wants a different interface shape, you must create
   explicit migration/rework slices — not pretend the old interface never existed.

4. **ADR consistency.** If the OVERVIEW change contradicts an existing ADR:
   propose a new superseding ADR in your outputs. Never silently work around it.

5. **Dependency order must hold.** The updated plan must not create dependency
   inversions where a pending slice now depends on something from a later slice.

6. **Additive over destructive.** Prefer adding new slices or modifying pending
   slices over removing completed work. If completed work needs rework, create
   explicit rework slices.

7. **Non-goals are still non-goals.** AGENTS.md §10 items remain out of scope
   even if the OVERVIEW seems to imply them. If you see a conflict, escalate —
   do not plan work for non-goals.

8. **Sizing.** Total slice count should remain 35–60. If you are adding many
   slices and the count exceeds 60, consolidate or escalate.

---

## Procedure

### Step 1: Diff analysis

Produce a structured summary of what changed in OVERVIEW:

```markdown
## OVERVIEW Changes

### Sections added
- §X.Y — <title> — <one-line summary>

### Sections removed
- §X.Y — <title> — <one-line summary>

### Sections materially modified
- §X.Y — <what changed semantically>

### Affected areas
- Design principles: <list any changes>
- Modules: <list affected modules from §7>
- Evaluators: <list affected evaluators from §8>
- Phases: <list affected phases from §21>
- Non-goals: <list any changes to §4>
- Success criteria: <list any changes to §22-24>
```

### Step 2: Impact classification

For each change, classify:

| Change | Impact | Reason |
|---|---|---|
| §X.Y ... | `none` / `forward-only` / `in-flight` / `retroactive` | ... |

- **none** — cosmetic or clarifying; no plan change needed
- **forward-only** — affects only pending/future slices; safe to update
- **in-flight** — affects a slice currently being coded/reviewed; escalate
- **retroactive** — contradicts completed work or committed ADRs; escalate

### Step 3: Produce outputs

For changes classified as `forward-only`, produce ALL of the following:

#### 3a. Updated `plans/caesar-implementation-plan.md`

- Add new workstreams for new OVERVIEW requirements
- Modify acceptance criteria of pending workstreams to match new OVERVIEW
- Remove or mark as cancelled any pending workstreams for removed OVERVIEW sections
- Preserve ALL workstream entries for completed/in-flight slices unchanged
- New workstreams get new IDs following the existing numbering scheme
  (e.g., if Phase 2 has ws-2.1 through ws-2.8, a new one is ws-2.9)
- Every new/modified workstream must cite its OVERVIEW section
- Dependency order must remain valid

#### 3b. Updated `plans/slices.yaml`

- Add new slice entries for new workstreams (using schema from
  `harness/slice-ledger.md`)
- Modify `title`, `plan_section`, or `risk` of pending slices (`coded: false`)
  if needed
- NEVER touch slices where `coded: true`
- Assign `coder` and `reviewer` per AGENTS.md §8 routing rules (cross-vendor)
- Run `make slices-check` to validate

#### 3c. New or superseding ADRs (if needed)

- If the OVERVIEW introduces a new architectural decision not covered by
  existing ADRs, draft it using `docs/adr/_template.md`
- If it contradicts an existing ADR, draft a superseding ADR that references
  the old one (status: Superseded by ADR-NNNN)

#### 3d. Updated `docs/ARCHITECTURE.md` and `docs/ROADMAP.md` (if they exist)

- Reflect new modules, phases, or structural changes
- Preserve descriptions of components that already exist in code

### Step 4: Escalation report

List everything that requires human decision:

```markdown
## Escalations

| Item | Type | Details |
|---|---|---|
| ... | `in-flight` / `retroactive` / `adr-conflict` / `non-goal-conflict` | ... |
```

### Step 5: Validation

Run:

```bash
make slices-check          # Ledger structural integrity
make verify                # Existing code still passes (OVERVIEW change must not break code)
```

Confirm:
- [ ] No completed slice was modified
- [ ] No in-flight slice was silently re-scoped
- [ ] All new workstreams cite their OVERVIEW section
- [ ] Dependency order is preserved
- [ ] ADR conflicts are resolved or escalated
- [ ] `make slices-check` passes
- [ ] Total slice count is within 35–60
- [ ] `make verify` still passes

### Step 6: Summary

Produce a final summary for the human:

```markdown
## Refactor Summary

**OVERVIEW changes:** <N sections added, N modified, N removed>
**Plan impact:** <N new slices, N modified slices, N cancelled slices>
**Escalations:** <N items requiring human decision>
**Validation:** slices-check <pass/fail>, verify <pass/fail>

### New slices added
| ID | Title | Phase | Risk | Coder |
|---|---|---|---|---|

### Modified slices
| ID | What changed |
|---|---|

### Cancelled slices
| ID | Reason |
|---|---|
```

---

## When to stop and escalate (do not proceed)

- A completed slice's implementation contradicts the new OVERVIEW
- An existing ADR is invalidated with no clear superseding decision
- The OVERVIEW removes a module or evaluator that already has code on `dev`
- The OVERVIEW changes a core interface shape already implemented
- An in-flight slice's scope is affected
- Total slice count would exceed 60
- A non-goal from AGENTS.md §10 appears to be required by the new OVERVIEW
- The OVERVIEW change requires reordering slices that have landed dependencies

In any of these cases: document the conflict, do not make the change, and
include it in the escalation report for the human.

---

## Anti-patterns

- **Do not** regenerate the entire plan from scratch. This loses progress
  tracking and brief archaeology.
- **Do not** renumber completed slice IDs.
- **Do not** silently drop workstreams that have completed work.
- **Do not** modify `plans/slices-done.yaml`.
- **Do not** assume interfaces can change freely — read the actual source.
- **Do not** plan work for AGENTS.md §10 non-goals.
- **Do not** skip reading the briefs — they document what was actually built,
  which may differ from what was originally planned.
