# Workflow Guide

How to use Metis from first idea to shipping code.

## Two Regimes

Metis separates work into two clearly distinct phases:

1. **Planning** (human-led, runs per phase) — produces frozen plan artifacts
2. **Execution** (agent-driven, runs every session) — drains the ledger one slice at a time

The two regimes meet at the **ledger**: planning fills it, execution drains it.

---

## Source of Truth

The authority chain, in order of precedence:

| Level | Artifact | Who owns it |
|---|---|---|
| Intent | `OVERVIEW.md` | Human (updated when requirements change) |
| Decisions | `.metis/adr/NNNN-*.md` | Human (created during planning, per phase) |
| How | `.metis/plans/phase-N.md` | Agent-generated from overview, human-approved |
| Dispatch | `.metis/slices.yaml` | Derived from plan via `metis seed` |
| Scope | `.metis/briefs/<id>.md` | Coder commits before coding |
| Reality | The code itself | Trumps everything above |

When documents disagree: **observed code > brief > plan > overview**.
The agent fixes the wrong document — never silently codes around a contradiction.

---

## Greenfield: Building a New App

```
OVERVIEW.md (you write the full application spec)
    ↓ agent-assisted planning, one phase at a time
.metis/plans/phase-N.md + .metis/adr/NNNN-*.md
    ↓ metis seed
.metis/slices.yaml → agents execute → code
```

### Step 1: Write the OVERVIEW

The OVERVIEW describes the full application: what it does, its architecture,
constraints, non-goals, and a rough sketch of all phases. This is the
document you hand to an agent when you say "plan Phase 1."

Use the template at `.metis/templates/overview.md` for structure guidance.

### Step 2: Initialize and configure

```bash
metis init
# Edit metis.yaml: set project.overview, agents, commands, hot_paths, routing
metis surface generate
```

### Step 3: Plan one phase at a time

Ask a coding agent to read the OVERVIEW and produce:
- A structured plan file (`.metis/plans/phase-0-plan.md`) — use `.metis/templates/plan.md` as the format
- ADRs for binding decisions (`.metis/adr/0001-*.md`) — use `.metis/templates/adr.md` as the format

The plan file follows the workstream format that `metis seed` can parse:

```markdown
## Phase 0 — Foundation

### Workstream 0.1: Project scaffold
- **Risk:** high
- **Coder:** claude-code/opus
- **Reviewer:** opencode/opus
- **Stage:** foundation

Tasks:
- Initialize project structure
- Set up build/test/lint pipeline

Acceptance criteria:
- [ ] Project compiles
- [ ] metis verify passes
```

### Step 4: Seed the ledger

```bash
metis seed .metis/plans/phase-0-plan.md --dry-run   # preview
metis seed .metis/plans/phase-0-plan.md             # create slices
metis check                                          # validate
```

### Step 5: Cold-start (Phase 0)

The first slices in a greenfield project create the things later slices read:

| Slice | Purpose | Why first |
|---|---|---|
| `phase-0-ws-0.1` | Project scaffold + verify gate | Makes `metis verify` meaningful |
| `phase-0-ws-0.2` | Core interfaces + ADRs | Makes `metis interfaces` meaningful |
| `phase-0-ws-0.3` | Domain model | Gives later slices real types to consume |

Each completed brief becomes archaeology for the next slice — the corpus
grows itself.

### Step 6: Execute, then plan the next phase

```bash
# After Phase 0 is done:
# Ask agent to plan Phase 1 (it now has Phase 0 learnings + real interfaces)
metis seed .metis/plans/phase-1-plan.md --append
```

The hybrid approach: sketch all phases in the OVERVIEW upfront, but plan
each phase in detail only when you're about to execute it.

---

## Extending: Adding to an Existing App

When you already have a running codebase and want to add features, fix bugs,
or refactor:

### Step 1: Update the OVERVIEW

Add the new requirements to OVERVIEW.md. This keeps it as the single source
of intent for the full application.

### Step 2: Plan the change

Ask an agent to plan the new work from the updated OVERVIEW:

```bash
# Agent produces .metis/plans/payments-plan.md
metis seed .metis/plans/payments-plan.md --append
```

Or add individual slices for reactive work:

```bash
# Bug fix (jumps the queue at p0)
metis add fix --title "Race condition in checkout" \
  --coder claude-code/opus --reviewer opencode/opus \
  --priority p0 --risk high

# Tech debt (runs when nothing urgent)
metis add debt --title "Replace hand-rolled errors" \
  --coder opencode/opus --reviewer claude-code/opus \
  --priority p3 --risk low
```

### Step 3: Execute

Same as greenfield — agents run `metis kickoff` and the dispatch algorithm
handles priority, ordering, and blocked dependencies.

---

## When the OVERVIEW Changes

Requirements change. The OVERVIEW is a living document. The rule is:
**update the OVERVIEW first, then reconcile forward.**

```bash
# 1. Edit OVERVIEW.md with new/changed requirements
vim OVERVIEW.md

# 2. Metis detects the drift
metis check
# WARNING: OVERVIEW has changed since last planning cycle. Consider: metis recon

# 3. Create a reconciliation slice
metis recon

# 4. The recon agent reads the changes and:
#    - Identifies affected pending slices
#    - Proposes edits, skips, or new slices
#    - Updates plan docs and ADRs
#    - Never touches completed/archived work
```

Completed work is **never rewritten** — always fix forward.

---

## Ongoing Maintenance

Metis handles the full SDLC, not just feature building:

| Scenario | How |
|---|---|
| Bug report | `metis add fix --priority p0 --risk high ...` |
| Tech debt | `metis add debt --priority p3 ...` |
| Refactoring | `metis add refactor ...` |
| Dependency updates | `metis add chore --risk low ...` |
| Security fix | `metis add security --risk high ...` |
| Code removal | `metis add remove ...` |
| Phase validation | `metis add gate ...` |
| OVERVIEW changed | `metis recon` |

---

## The Execution Loop (What Agents Do)

Every agent session follows the same protocol:

1. Run `metis kickoff`
2. Establish state (branch, tree, orientation)
3. Find active slice (`metis next`)
4. Self-identify and match required model
5. Read instructions (`metis instructions --for <id>`)
6. Pre-flight (`metis verify --pre`)
7. Execute (Coder flow or Reviewer flow)
8. Report

This is fully documented in [protocol.md](protocol.md).

---

## When Plans Change Mid-Execution

- **Completed slices:** immutable (archived, never modified)
- **Pending slices:** can be edited: `metis edit <id> --title "..." --risk high`
- **New work:** additive: `metis add` or `metis seed --append`
- **Abandoned work:** skipped: `metis skip <id> --reason "descoped"`
- **Always validate:** `metis check` after any batch of changes

The ledger is the single dispatch truth. The plan file is frozen intent.
When reality diverges, update the ledger and file a `recon` slice if the
drift is significant enough to warrant re-aligning documentation.
