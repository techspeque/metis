# Harness — Coding-Agent Process Infrastructure

This directory contains the process machinery that governs how autonomous coding
agents build CAESAR. It is not application code — it is the control plane for
multi-agent development.

---

## Components

| File | Purpose |
|---|---|
| `HARNESS.md` | The source template. The complete harness specification extracted from the AIMux project. Describes philosophy, regimes, schemas, dispatcher contract, and customisation checklist. Reference material — not directly executed. |
| `AGENT-KICKOFF.md` | The session protocol. Every autonomous agent session begins here (step 1). Defines: state verification, slice dispatch, identity matching, scaled reading, pre-flight checks, coder flow, reviewer flow, guardrails, and the brief template. |
| `PLAN.md` | The planning prompt. A structured prompt for generating all planning artifacts (architecture, roadmap, ADRs, implementation plan, seeded ledger) from `docs/OVERVIEW.MD` as primary source of truth. |
| `REFACTOR.md` | The plan refactor prompt. A structured prompt for updating all planning artifacts when `docs/OVERVIEW.md` changes. The agent reads existing interfaces, ADRs, completed briefs, and the ledger, then produces a delta update — new/modified slices, updated plan, and an escalation report for anything that conflicts with completed work. |
| `slice-ledger.md` | Ledger schema documentation. Defines the fields, lifecycle, and semantics of `plans/slices.yaml` entries. All ledger commentary lives here — never in the YAML itself. |
| `review-findings.md` | Feedback loop log. One line per blocking review finding. The human skims this periodically; recurring patterns graduate into `AGENTS.md` accuracy rules or checklist items. |
| `BOOTSTRAP.MD` | Provenance record. The original prompt that initiated this harness bootstrap. Kept for traceability. |
| `tools/dispatch.py` | The dispatcher. A deterministic Python script implementing three subcommands over the ledger: `next` (find active slice + role), `check` (lint the ledger), `archive` (move done slices to archive). Agents are bad at walking YAML in order — this program removes that failure mode. |

### Files outside `harness/` that are part of the harness

| File | Purpose | Why here |
|---|---|---|
| `AGENTS.md` (root) | The governing contract. Roles, branch protocol, hot-path zones, testing rules, non-goals, accuracy rules, reviewer checklist, tooling map. | Auto-loaded by all three agent surfaces (Claude Code via `CLAUDE.md`, opencode via `opencode.json`, Codex natively). Must be at repo root. |
| `CLAUDE.md` (root) | Claude Code surface adapter. Points Claude Code at `AGENTS.md` and states identity slugs. | Claude Code reads this file from repo root automatically. |
| `opencode.json` (root) | opencode surface adapter. Points opencode at `AGENTS.md`. | opencode reads this file from repo root automatically. |
| `.claude/settings.json` | Suppresses AI attribution in commits per `AGENTS.md` §3. | Required by Claude Code at this path. |
| `Makefile` (root) | Task runner exposing harness targets: `next-slice`, `slices-check`, `slices-archive`, `verify`, `interfaces`, `env-check`. | Standard location; agents invoke targets via `make`. |
| `plans/slices.yaml` | The dispatch ledger (active/pending slices). | Living project content — declaration order is execution order. |
| `plans/slices-done.yaml` | Archive of completed slices (created by `make slices-archive`). | Living project content. |
| `plans/briefs/*.md` | Per-slice scope contracts committed by the Coder before implementation. | Living project content — each brief is the next slice's archaeology. |

---

## How the harness works

### Two regimes

1. **Planning regime** (human-led, runs rarely): Produces frozen artifacts —
   architecture, ADRs, implementation plan, seeded ledger. The human drives this
   with agent assistance. Output: `plans/`, `docs/adr/`, `docs/ARCHITECTURE.md`.

2. **Execution regime** (autonomous, runs every session): One agent, one session,
   one slice. The agent reads `AGENTS.md` -> follows `AGENT-KICKOFF.md` -> asks
   `make next-slice` what to do -> either codes or reviews -> reports.

### Refactor protocol

When `docs/OVERVIEW.md` changes after planning is complete and execution has
begun, `REFACTOR.md` governs how to update the plans without invalidating
completed work. Key constraints:

- Completed slices are immutable (do not renumber IDs or modify done entries)
- In-flight slices require human decision before re-scoping
- New work is additive (new slices, not rewrites of old ones)
- ADR conflicts must be resolved explicitly
- All changes must be validated with `make slices-check` and `make verify`

### Session lifecycle

```bash
Agent starts
    |
    v
Read AGENTS.md §1 -> follow AGENT-KICKOFF.md
    |
    v
make next-slice -> active slice, role, required model
    |
    v
Self-identify -> match? (no -> stop)
    |
    v
Scaled reading (risk-based depth)
    |
    v
make env-check -> make verify (pre-flight)
    |
    +--> Coder: brief -> implement -> verify -> flip coded
    |
    +--> Reviewer: independent verify -> checklist -> verdict
    |
    v
Report to human
```

### Determinism backbone

Agents are unreliable at: walking YAML lists in order, evaluating boolean pairs,
matching string slugs, distinguishing environment failures from code failures.
The harness removes these from agent judgment:

- **Dispatcher** (`tools/dispatch.py`) decides the active slice and role
- **env-check** decides if the environment is sound before verify runs
- **slices-check** validates ledger integrity (CI-gated)
- **Identity matching** is a simple string comparison, not a judgment call

### Self-correction

The harness learns from its mistakes:

1. Blocking review findings -> `review-findings.md`
2. Recurring findings -> new `AGENTS.md` §11 accuracy rules
3. `review_cycles` count -> evidence for model routing changes
4. Phase gates -> composition validation (individual green != system green)

---

## How this harness was developed

### Source

The harness was extracted from the AIMux project (`HARNESS.md` is the complete
extraction). AIMux used it to build a Go modular monolith with autonomous agents
in disciplined lanes. The reusable structure is the process; the AIMux-specific
instances (package names, hot-path zones, Go dispatcher) are the worked example.

### Adaptation for CAESAR

The CAESAR harness was bootstrapped from `HARNESS.md` with these adaptations:

1. **Layout note improvement applied.** HARNESS.md §3 suggests that process files
   (kickoff, ledger schema, dispatcher, review log) can live under a single
   `harness/` directory rather than scattered across `docs/`, `tools/`, and
   `make/`. CAESAR uses this cleaner layout. Only files that must be at repo root
   for tooling reasons (`AGENTS.md`, `CLAUDE.md`, `opencode.json`, `Makefile`)
   remain there.

2. **Greenfield cold-start applied.** HARNESS.md Part C describes bootstrapping
   for an empty repo. `AGENT-KICKOFF.md` includes "Greenfield notes" at key steps
   explaining that empty archaeology is expected for early slices.

3. **Dispatcher reimplemented in Python.** AIMux used a Go dispatcher (~280
   lines). CAESAR uses a portable Python script (~150 lines) since the project
   toolchain does not yet exist at bootstrap time. Same contract: `next`, `check`,
   `archive`.

4. **Three agent surfaces.** The harness supports Claude Code (`claude-code/opus`),
   opencode (`opencode/opus`), and Codex (`codex`) with cross-vendor review as
   the default.

5. **CAESAR-specific customisations in AGENTS.md:**
   - §6 hot-path zones: `internal/evaluators/`, `internal/scoring/`,
     `internal/planner/`, `internal/storage/sqlite/migrations.go`, `internal/llm/`
   - §9 testing rules: mock LLM client, in-memory SQLite, exhaustive deterministic
     evaluator tests
   - §10 non-goals: derived from `docs/OVERVIEW.MD` §4
   - §11 accuracy rules: hard gates override scores, single Evaluator interface,
     LLM-as-judge last, SQLite only, domain model in `internal/domain/`

6. **Planning prompt created.** `PLAN.md` is a structured prompt that produces all
   planning artifacts with `docs/OVERVIEW.MD` as the authoritative source of truth.
   It encodes sizing guidance (35–50 slices), risk routing, and a quality checklist
   with OVERVIEW traceability requirements.

### Bootstrap sequence

The bootstrap was performed in this order (per HARNESS.md Part C, Step 0):

1. Created `harness/slice-ledger.md` and `harness/review-findings.md`
2. Created `docs/adr/_template.md`, `0000-record-architecture-decisions.md`, index
3. Created `AGENTS.md` (customised for CAESAR)
4. Created surface adapters (`CLAUDE.md`, `opencode.json`, `.claude/settings.json`)
5. Created `harness/tools/dispatch.py` (the dispatcher)
6. Created `Makefile` (all harness + verification targets)
7. Created `plans/slices.yaml` (empty ledger with agent identity map)
8. Created `plans/briefs/.gitkeep` and `docs/generated/interfaces.txt` (stubs)
9. Rewrote `harness/AGENT-KICKOFF.md` (CAESAR-specific, greenfield-aware)
10. Created `harness/PLAN.md` (planning prompt)
11. Verified: `make verify` green, dispatcher tested with sample slices

### What comes next

The planning regime (Part A of HARNESS.md):

1. A human (with agent assistance) runs the `PLAN.md` prompt to produce:
   - `docs/ARCHITECTURE.md`
   - `docs/ROADMAP.md`
   - `docs/adr/0001–000N` (foundational ADRs)
   - `plans/caesar-implementation-plan.md`
   - Seeded `plans/slices.yaml`
2. `make slices-check` validates the seeded ledger
3. First execution session: `make next-slice` returns Phase 0, workstream 0.1
4. The loop turns — agents code and review one slice at a time until done

---

## Backing up the harness

The harness includes a backup mechanism to snapshot all process files into a
timestamped tarball stored in `harness/backups/` (git-ignored).

### Commands

```bash
# Full backup: harness machinery + slice state (ledger, briefs)
make harness-backup

# Core only: harness machinery without slice state
make harness-backup-core
```

### What gets backed up

**Always included (harness core):**
- `harness/` — kickoff, HARNESS.md, PLAN.md, REFACTOR.md, slice-ledger docs,
  review findings, dispatcher
- `AGENTS.md`, `CLAUDE.md`, `opencode.json`, `.claude/settings.json` — surface
  adapters and contract
- `Makefile` — task runner (part of the harness machinery)
- `docs/adr/` — architecture decision records

**Included in full backup only (slice state):**
- `plans/slices.yaml` — the active dispatch ledger
- `plans/slices-done.yaml` — the archive of completed slices
- `plans/briefs/` — all committed per-slice scope contracts

### Drift warning

> **WARNING:** When slice state is included in a backup, restoring it into a
> repo whose code has advanced beyond the backed-up ledger state will cause
> **drift**. The ledger may show slices as `coded: false` that have already been
> implemented, or `reviewed: false` for work already signed off.
>
> **After any restore that includes slice state:**
> 1. Run `make slices-check` to verify structural integrity
> 2. Compare `plans/slices.yaml` against `git log --oneline` to identify which
>    slices have commits that postdate the backup
> 3. Manually reconcile: flip `coded`/`reviewed` booleans for slices whose work
>    is already landed, or reset them if the code was reverted
> 4. Run `make slices-archive` to clean up any fully-done slices
>
> If you only need to recover the harness *process* (rules, dispatcher, templates)
> without risking ledger drift, use `make harness-backup-core` which excludes all
> slice state.

### Restoring a backup

```bash
# List available backups
ls harness/backups/

# Inspect contents before restoring
tar -tzf harness/backups/harness-backup-YYYYMMDD-HHMMSS.tar.gz

# Restore (overwrites current files)
tar -xzf harness/backups/harness-backup-YYYYMMDD-HHMMSS.tar.gz

# Verify integrity after restore
make slices-check
```

### When to back up

- Before major harness changes (editing AGENTS.md rules, restructuring the
  dispatcher, changing the Makefile targets)
- Before experimental plan changes that might need rollback
- Before a phase gate, as a checkpoint of known-good state
- Periodically as insurance (the backup is <100KB and costs nothing)
