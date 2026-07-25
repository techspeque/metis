# Metis — Full Specification

> The meta-intelligence that orchestrates AI coding agents.

Metis is a Go CLI tool for managing autonomous coding-agent workflows. It
enforces disciplined, bounded, independently-reviewed units of work across
any technology stack, any agent surface (Claude Code, opencode, Codex, or
others), with the human retaining ownership of planning, escalations, and
releases.

---

## Table of Contents

1. [Philosophy](#1-philosophy)
2. [Architecture Overview](#2-architecture-overview)
3. [Configuration (`metis.yaml`)](#3-configuration-metisyaml)
4. [Data Model](#4-data-model)
5. [CLI Command Reference](#5-cli-command-reference)
6. [Instructions Engine](#6-instructions-engine)
7. [Git Enforcement](#7-git-enforcement)
8. [Verification Pipeline](#8-verification-pipeline)
9. [Surface Adapters](#9-surface-adapters)
10. [Planning & Seeding](#10-planning--seeding)
11. [Feedback Loop](#11-feedback-loop)
12. [Agent Session Protocol](#12-agent-session-protocol)
13. [Go Package Structure](#13-go-package-structure)
14. [Distribution & Installation](#14-distribution--installation)

---

## 1. Philosophy

Metis exists because a single long agent session drifts: it loses scope,
re-explores code it already understood, fixes things it was not asked to fix,
marks its own homework, and quietly works around contradictions instead of
surfacing them.

Metis replaces "one big session" with **many small, bounded,
independently-reviewed units of work dispatched deterministically.**

### Core Principles

1. **Plan once, execute many.** Planning is human-led and produces frozen
   artifacts. Agents execute one pre-declared unit at a time; they do not
   re-plan mid-flight.

2. **One active unit at a time.** Work is sliced into the smallest reviewable
   units. Exactly one slice is active; a deterministic tool — never the agent
   eyeballing a list — decides which one and who works it.

3. **Two lanes plus a human.** A Coder implements one slice. A Reviewer
   (different agent, ideally different vendor) checks it against a fixed
   checklist. A human owns planning, escalations, and merges.

4. **Scope is a written contract.** Before any code, the Coder commits a
   brief declaring exactly which files the slice may touch and what "done"
   means. The Reviewer measures the diff against the brief.

5. **Reality beats documents.** When the plan and the code disagree, the code
   wins; the agent fixes the wrong document, not silently codes around it.
   Priority: **observed code > contract > plans**.

6. **Risk scales effort.** Each slice is tagged low/medium/high. Risk
   determines reading depth, model routing, and review scrutiny.

7. **Determinism for the things agents fumble.** Walking a YAML list,
   comparing slugs, evaluating booleans, distinguishing environment failure
   from code failure — these go into the CLI with one right answer, not into
   agent judgment.

8. **The system self-corrects.** Every blocking review finding is logged.
   Recurring failures graduate into new rules. Phase gates prove composition.

9. **Technology agnosticism.** Metis knows nothing about the underlying
   language, framework, or build system. It runs configured commands as opaque
   strings. The spec is the same whether you're building Rust, Go, Python,
   TypeScript, or anything else.

10. **Full SDLC.** Not just greenfield feature building — also bug fixes,
    refactoring, tech debt removal, deletions, maintenance, and reactive work.

---

## 2. Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                        Human                                 │
│  Plans · Seeds ledger · Kicks off agents · Owns releases    │
└─────────────┬───────────────────────────────────┬───────────┘
              │                                   │
              v                                   v
┌─────────────────────┐             ┌─────────────────────────┐
│   metis CLI         │             │   Agent Surface          │
│                     │◄───────────►│   (Claude Code /         │
│  • Dispatch         │  agents     │    opencode / Codex)     │
│  • Instructions     │  execute    │                          │
│  • Verification     │  metis      │  Reads instructions via  │
│  • Git enforcement  │  commands   │  surface adapters        │
│  • Findings         │             │                          │
│  • Progress         │             │                          │
└─────────┬───────────┘             └──────────────────────────┘
          │
          v
┌─────────────────────────────────────────────────────────────┐
│                    .metis/ (state)                            │
│  slices.yaml · slices-done.yaml · briefs/ · findings.yaml   │
│  runs/ · interfaces.txt                                      │
└─────────────────────────────────────────────────────────────┘
```

---

## 3. Configuration (`metis.yaml`)

Lives at repo root. Single source of truth for all project-specific settings.
Replaces scattered AGENTS.md sections, Makefiles, and agent identity maps.

```yaml
# metis.yaml — project configuration for Metis
version: 1

project:
  name: my-project
  integration_branch: dev
  release_branch: main
  technology:
    # Informational only — used in generated instructions.
    # Metis does not interpret these; they help agents understand context.
    language: rust
    build_system: cargo
    test_runner: "cargo test"
    linter: clippy

# ─── Agent Identity Map ───────────────────────────────────────────────────────
# Each entry is a slug agents self-identify against.
# surface: which tool runs the agent (claude-code, opencode, codex, etc.)
# model: which model variant
# label: human-readable label for reports

agents:
  claude-code/opus:
    surface: claude-code
    model: opus
    label: "Claude Code (Opus)"
  opencode/opus:
    surface: opencode
    model: opus
    label: "opencode (Opus)"
  codex:
    surface: codex
    model: codex
    label: "Codex"

# ─── Model Routing ────────────────────────────────────────────────────────────
# Which agents handle which risk levels. Review is always cross-vendor.

routing:
  high: [claude-code/opus, opencode/opus]
  medium: [codex, opencode/opus]
  low: [codex]
  review: cross-vendor   # reviewer must be a different agent than the coder

# ─── Hot Paths ────────────────────────────────────────────────────────────────
# Paths where a mistake is expensive. Listed as mandatory-caution zones in
# agent instructions; touches are surfaced by 'metis log --validate'.
# (Automatic risk escalation from observed paths is deferred roadmap.)

hot_paths:
  - src/auth/
  - src/payments/
  - migrations/
  - src/crypto/

# ─── Accuracy Rules ──────────────────────────────────────────────────────────
# Project invariants that must never be violated. Seeded from architecture
# decisions; grown from recurring review findings.

accuracy_rules:
  - Do not hallucinate interfaces — read real source before consuming or implementing them
  - Hard gates override weighted scores — never average away a hard-gate failure
  - SQLite is the storage backend — do not introduce PostgreSQL dependencies

# ─── Non-Goals ────────────────────────────────────────────────────────────────
# Things agents must not build even if asked indirectly.

non_goals:
  - Frontend or web UI
  - gRPC (REST JSON API only)
  - Kubernetes deployment manifests
  - Multi-tenant SaaS features

# ─── Testing Contract ─────────────────────────────────────────────────────────

testing:
  - Mock at trust boundaries only; never mock the thing under test
  - Integration tests hit real dependencies where cheap (in-memory DBs, temp files)
  - A slice without relevant tests is incomplete
  - Test files live alongside the code they test

# ─── Review Checklist ─────────────────────────────────────────────────────────
# Walked in order by the Reviewer. One-line verdict per item with file:line evidence.

review_checklist:
  - Behavioral correctness
  - Security and authorization correctness
  - Scope discipline (diff vs. the committed brief)
  - Test sufficiency
  - Architectural fit — no duplicated/hallucinated interfaces; matches ADRs
  - Maintainability

# ─── Commands ─────────────────────────────────────────────────────────────────
# Technology-specific commands Metis wraps. Opaque to Metis — it just runs them
# and captures output. All are optional except verify.

commands:
  verify: "cargo build && cargo clippy -- -D warnings && cargo test"
  env_check: "rustc --version && cargo --version"
  interfaces: "cargo doc --document-private-items 2>/dev/null && scripts/extract-api.sh"
  # If interfaces is omitted, `metis interfaces` is a no-op and instructions
  # skip the "read interface summary" step.

# ─── Commit Convention ────────────────────────────────────────────────────────

commits:
  # Conventional Commits prefix required
  prefixes: [feat, fix, refactor, docs, test, chore]
  # Slice ID must appear in subject
  require_slice_id: true
  # No AI attribution in commits
  no_attribution: true
  # Commit message format template (slice_id and prefix are injected)
  format: "{prefix}({slice_id}): {message}"

# ─── Paths ────────────────────────────────────────────────────────────────────
# Where Metis stores its state. All relative to repo root.

paths:
  ledger: .metis/slices.yaml
  archive: .metis/slices-done.yaml
  briefs: .metis/briefs/
  findings: .metis/findings.yaml
  runs: .metis/runs/
  interfaces: docs/generated/interfaces.txt
```

### Configuration Validation

`metis check --config` validates the configuration:
- All required fields present
- Agent slugs in `routing` exist in `agents` map
- Commands are non-empty strings
- Paths are valid relative paths
- No agent appears as both coder and reviewer for the same risk level
  (the constraint is per-slice, but routing should make cross-vendor natural)

---

## 4. Data Model

### 4.1 Slice

The fundamental unit of work. Lives in the ledger (`.metis/slices.yaml`).

```yaml
slices:
  - id: phase-2-ws-2.3            # Unique slug. Appears in every commit.
    title: "Add payment webhook handler"
    type: feat                     # feat|fix|refactor|debt|remove|chore|security|gate|recon
    priority: p2                   # p0|p1|p2|p3 (default: p2)
    risk: medium                   # low|medium|high
    stage: mvp                     # optional project taxonomy
    coder: opencode/opus           # agent slug
    reviewer: codex                # agent slug (must differ from coder)
    plan: plans/impl-plan.md       # optional: source plan file
    plan_section: "§6.3"           # required when plan is set
    coded: false                   # lifecycle flag
    reviewed: false                # lifecycle flag
    review_cycles: 0               # bumped on each block
    blocked_by: []                 # optional: slice IDs that must complete first
    notes: ""                      # skip reasons, exceptions, clarifications
    created: 2026-07-09            # ISO date
```

### 4.2 Work Types

Each type has different semantics for scope, brief template, and DoD:

| Type | Scope | DoD focus | Brief shape |
|------|-------|-----------|-------------|
| `feat` | Narrow (declared owned_paths) | Behavior added, tests prove it | Standard brief |
| `fix` | Narrow | Bug gone, regression test added | Repro → Fix → Verify |
| `refactor` | Broad (affected_paths) | Same behavior, better structure | Migration strategy + behavioral contract |
| `debt` | Broad | Shortcuts removed, interfaces cleaner | Affected paths + migration |
| `remove` | Broad (deletions) | Code gone, no dangling refs, tests updated | What's removed + cleanup checklist |
| `chore` | Narrow | Maintenance done (deps, CI, config) | Minimal brief |
| `security` | Narrow, high-risk always | Vulnerability closed, test proves it | Threat → Fix → Verification |
| `gate` | None (validation only) | Evidence report committed | Scenarios + evidence |
| `recon` | Docs only | Plan/ADR aligned with observed code | Drift report |

### 4.3 Priority

| Level | Semantics | Queue behavior |
|-------|-----------|----------------|
| `p0` | Drop everything (security breach, prod down) | Interrupts: becomes active immediately |
| `p1` | Next up after current slice finishes | Jumps queue after current |
| `p2` | Normal (default) | Declaration order within priority |
| `p3` | Backlog / nice-to-have | Only reached when no p0-p2 remain |

The dispatcher (`metis next`) returns the highest-priority unfinished slice.
Within the same priority, declaration order holds.

### 4.4 Slice Lifecycle

```
                    ┌──────────┐
                    │ created  │
                    └────┬─────┘
                         │
                         v
              ┌──────────────────┐
         ┌────│    pending       │◄───────── reopen
         │    └────────┬─────────┘
         │             │  (metis next → role: Coder)
         │             v
         │    ┌──────────────────┐
  skip───┤    │   coding         │
         │    └────────┬─────────┘
         │             │  (metis flip coded <id>)
         │             v
         │    ┌──────────────────┐
         │    │   reviewing      │
         │    └───┬────────┬─────┘
         │        │        │
         │   pass │        │ block (metis block <id>)
         │        │        │
         │        v        v
         │    ┌──────┐  ┌──────────┐
         └───►│ done │  │ rework   │──► back to pending
              └──────┘  └──────────┘    (coded=false, review_cycles++)
```

### 4.5 Findings

Structured review findings stored in `.metis/findings.yaml`:

```yaml
findings:
  - id: f-001
    date: 2026-07-09
    slice: phase-2-ws-2.1
    severity: P1          # P1 (breaks guarantee) | P2 (wrong, contained) | P3 (debt)
    category: arch-dup    # auth|protocol|scope|tests|arch-dup|arch-fit|data|maint|security
    finding: "Duplicated Validator trait at src/eval/mod.rs:42 — use domain::Validator"
    status: open          # open | resolved | promoted
    promoted_to: null     # accuracy_rule index if promoted
```

### 4.6 Runs

Verification run results stored in `.metis/runs/<slice-id>/`:

```
.metis/runs/phase-2-ws-2.3/
  env-check.log          # stdout+stderr of env-check
  env-check.exit         # exit code
  verify-pre.log         # pre-flight verify (before changes)
  verify-pre.exit
  verify-post.log        # post-implementation verify
  verify-post.exit
  interfaces.log         # interfaces generation output
  interfaces.exit
```

Each run is timestamped and the latest is kept. The reviewer can inspect the
coder's verify output without re-running (though they must also verify
independently).

---

## 5. CLI Command Reference

### 5.1 Initialization & Configuration

```
metis init
```

Non-interactive setup; prints copy-pasteable `metis config set` next steps.
(A prompting wizard is deferred roadmap.) Formerly specced to prompt for
project name, language, branch names, agents,
hot paths, and commands. Writes `metis.yaml` and scaffolds the `.metis/`
directory structure. Generates surface adapters.

```
metis init --from metis.yaml
```

Non-interactive. Reads existing config and scaffolds state directories +
surface adapters without prompting.

```
metis check
```

Validates everything:
- Config (`.metis/project.yaml`) structural validity
- Ledger integrity (unique IDs, valid slugs, valid risk/priority/type values,
  no `reviewed && !coded`, coder != reviewer, `plan_section` present when
  `plan` is set)
- Archive integrity (all entries fully done)
- Surface adapter sync (warns if stale)

Exit code 0 = pass, 1 = failure. **CI-gatable.**

```
metis check --config
```

Validates only the configuration file.

```
metis check --ledger
```

Validates only the ledger.

---

### 5.2 Dispatch (agents call these)

```
metis next
```

Find the active slice. Returns: slice ID, title, type, risk, priority, stage,
role (Coder/Reviewer), required model slug + label, plan + section,
review_cycles, and the risk-scaled reading list.

Logic:
1. Filter to unblocked slices (no `blocked_by` with incomplete deps)
2. Sort by priority (p0 > p1 > p2 > p3)
3. Within same priority, declaration order
4. First slice with `coded: false` → role Coder
5. If coded but `reviewed: false` → role Reviewer
6. If both true → skip (should be archived)

Output (structured, parseable):

```
Active slice: phase-2-ws-2.3
  Title:          Add payment webhook handler
  Type:           feat
  Priority:       p2
  Risk:           medium
  Stage:          mvp
  Role:           Coder
  Required model: opencode/opus (opencode (Opus))
  Plan:           plans/impl-plan.md §6.3
  Review cycles:  0
  Reading rule:   metis.yaml §accuracy_rules + testing | ADRs touching packages in scope | plan_section + adjacent
  Blocked by:     (none)
```

For Reviewer role, also prints:
```
  Brief:          .metis/briefs/phase-2-ws-2.3.md
  Commits:        git log --oneline --grep "phase-2-ws-2.3"
  Coder verify:   .metis/runs/phase-2-ws-2.3/verify-post.log
```

```
metis next --json
```

Same as above but JSON output for programmatic consumption.

```
metis next --quiet
```

Prints only the slice ID (for scripting).

---

### 5.3 Work Management

```
metis add <type> [flags]
```

Add a new slice to the ledger.

Required flags:
- `--title "..."` — human-readable title
- `--coder <slug>` — agent slug for coding
- `--reviewer <slug>` — agent slug for review

Optional flags:
- `--risk <low|medium|high>` — defaults to `medium`
- `--priority <p0|p1|p2|p3>` — defaults to `p2`
- `--stage <string>` — project taxonomy label
- `--plan <path>` — source plan file
- `--plan-section <string>` — required when `--plan` is set
- `--blocked-by <id>,<id>` — slice dependencies
- `--id <slug>` — explicit ID (auto-generated if omitted)
- `--notes "..."` — clarification text
- `--after <id>` — insert after this slice (otherwise appends)
- `--before <id>` — insert before this slice

Auto-generated IDs follow the pattern: `<type>-<NNNN>` (e.g., `feat-0012`,
`fix-0003`) using a monotonic counter, unless `--id` overrides.

Examples:

```bash
# Feature from a plan
metis add feat --title "Add webhook handler" \
  --coder opencode/opus --reviewer codex \
  --risk medium --plan plans/impl.md --plan-section "§6.3"

# Urgent bug fix (jumps queue)
metis add fix --title "Auth bypass in token validation" \
  --coder claude-code/opus --reviewer opencode/opus \
  --priority p0 --risk high

# Refactoring
metis add refactor --title "Consolidate auth middleware variants" \
  --coder opencode/opus --reviewer claude-code/opus \
  --risk high

# Deletion
metis add remove --title "Drop deprecated v1 API handlers" \
  --coder codex --reviewer opencode/opus \
  --risk medium

# Tech debt
metis add debt --title "Replace hand-rolled errors with thiserror" \
  --coder codex --reviewer opencode/opus \
  --risk low

# Phase gate
metis add gate --title "Phase 2 composition validation" \
  --coder claude-code/opus --reviewer opencode/opus \
  --risk high --id phase-2-gate

# Maintenance
metis add chore --title "Update dependencies to latest" \
  --coder codex --reviewer opencode/opus \
  --risk low --priority p3
```

```
metis edit <id> [flags]
```

Modify an existing slice. Same flags as `add` (all optional here). Cannot
edit a slice that is already `coded: true` unless `--force` is passed (which
resets both flags — effectively a reopen).

```
metis skip <id> --reason "..."
```

Mark a slice as skipped (both `coded: true` and `reviewed: true` with the
reason in `notes`). Effectively removes it from the queue without deleting it.

```
metis reopen <id> --reason "..."
```

Reset a slice to `coded: false`, `reviewed: false`. Used when a review found
issues that require re-implementation, or when requirements changed.

```
metis list [--type <type>] [--priority <pri>] [--status <pending|coded|reviewed|done>]
```

List slices with optional filters. Shows ID, title, type, priority, risk,
status.

```
metis show <id>
```

Show full details of a slice (ledger fields). Inline brief, finding
history, and run-log display are deferred roadmap — use 'metis brief <id>',
'metis findings --slice <id>', and .metis/runs/<id>/ directly.

---

### 5.4 Slice Execution (agents call these)

```
metis brief <id>
```

Emit the type-appropriate brief template to stdout. If `.metis/briefs/<id>.md`
already exists, print it instead (allows re-reading).

The template adapts based on the slice's `type` field:

**feat/fix/security/chore:**
```markdown
# <id> — <title>

- **Type:** <type> | **Risk:** <risk> | **Priority:** <priority>
- **Coder:** <slug> | **Date:** <today>
- **Plan:** <plan> <plan_section>

## Goal
One sentence, drawn from the plan's stated goal.

## Architectural context
Interfaces, types, packages, and schema this slice consumes or implements.
Read from `metis interfaces` output or prior briefs.

## Declared file scope
- **owned_paths:** exact files this slice may edit
- **read_only_paths:** packages/files this slice may inspect but not modify

## Definition of Done
Specific, testable criteria.

## Test plan
Which tests will exist and what they prove.

## Out-of-scope touches
Empty unless a fix outside declared scope proved genuinely required.
Each entry: what, where, and why.
```

**refactor/debt:**
```markdown
# <id> — <title>

- **Type:** <type> | **Risk:** <risk> | **Priority:** <priority>
- **Coder:** <slug> | **Date:** <today>

## Goal
One sentence: what structural improvement, no behavior change.

## Affected paths (broad scope)
- <paths that will change structurally>

## Migration strategy
1. Introduce new structure
2. Migrate callers
3. Remove old structure
4. Update docs/tests

## Behavioral contract
These tests MUST pass unchanged (proving no behavior change):
- <list>

These tests will be modified (testing new structure):
- <list>

## Definition of Done
- New structure in place
- All callers migrated
- Old structure removed
- Test coverage maintained or improved
- `metis verify` green

## Out-of-scope touches
```

**remove:**
```markdown
# <id> — <title>

- **Type:** remove | **Risk:** <risk> | **Priority:** <priority>
- **Coder:** <slug> | **Date:** <today>

## Goal
What is being removed and why.

## Removal scope
- **Code to delete:** <files/modules>
- **Tests to delete:** <test files that test deleted code>
- **Docs to update:** <references to update/remove>
- **Config to clean:** <configuration referencing deleted code>

## Verification
- No dangling references (compile clean)
- No orphaned tests
- Remaining test suite passes
- Docs updated

## Definition of Done
- Target code is gone
- Nothing references it
- Tests pass
- Docs reflect the removal

## Out-of-scope touches
```

**gate:**
```markdown
# <id> — <title>

- **Type:** gate | **Risk:** high | **Priority:** <priority>
- **Coder:** <slug> | **Date:** <today>

## Phase being validated
Phase <N>: <title>

## Composition scenarios
What integration/contract scenarios prove the phase works as a composed system:
1. <scenario>
2. <scenario>

## Evidence criteria
- [ ] All phase slices coded and reviewed
- [ ] Cross-module integration tests pass
- [ ] No interface mismatches at seam points
- [ ] Performance/resource usage acceptable

## Report
(Filled during execution with actual evidence)
```

```
metis brief <id> --write
```

Write the template to `.metis/briefs/<id>.md` (creates the file). The agent
then edits and commits it.

```
metis flip coded <id>
```

Mark a slice as coded. Validates:
- Brief exists at `.metis/briefs/<id>.md`
- A verify run exists in `.metis/runs/<id>/verify-post.log` with exit code 0
- The current git branch is the configured integration branch

If validation fails, prints what's missing and exits non-zero.

```
metis flip reviewed <id>
```

Mark a slice as reviewed (sign-off). Validates:
- `coded` is already true
- A verify run exists (reviewer's independent verification)
- The caller is not the same agent as the coder (checked via `--agent <slug>` flag)

```
metis block <id> --severity <P1|P2|P3> --category <cat> --finding "..."
```

Block a slice during review:
- Increments `review_cycles`
- Sets `coded: false` (sends back to coder)
- Appends finding to `.metis/findings.yaml`
- Prints the finding for the human

Categories: `auth`, `protocol`, `scope`, `tests`, `arch-dup`, `arch-fit`,
`data`, `maint`, `security`, `behavior`, `performance`.

```
metis archive
```

Move all fully-done slices (`coded && reviewed`) from the active ledger to
`.metis/slices-done.yaml`. Keeps the active ledger lean.

---

### 5.5 Verification (agents call these)

```
metis env-check
```

Runs the configured `commands.env_check` command. Captures output to
`.metis/runs/<active-slice>/env-check.log`.

- Exit 0: environment is sound.
- Exit non-zero: prints loud verdict:

```
══════════════════════════════════════════════════════════════
VERDICT: ENVIRONMENT FAILURE — NOT A CODE FAILURE.

Do NOT modify code, tests, or config to make verify pass.
Do NOT flip ledger booleans.
Stop and report this verbatim to the human.
══════════════════════════════════════════════════════════════
```

Uses distinct exit code (exit 2) so agents/scripts can distinguish env failure
from code failure.

```
metis verify [--pre | --post]
```

Full verification pipeline:
1. Runs the environment check first ('metis verify --env'). Fail → exit 2.
2. Runs the configured `commands.verify`. Captures output.
   (A ledger pre-check with exit 3 is deferred roadmap.)

Flags:
- `--pre` — labels this as a pre-flight verification (before changes).
  Stored as `verify-pre.log`.
- `--post` — labels this as post-implementation verification.
  Stored as `verify-post.log`.
- Neither flag: stored as `verify-latest.log`.

Exit codes:
- 0: all pass
- 1: verify command failed (code error)
- 2: environment failure

```
metis interfaces
```

Runs the configured `commands.interfaces` command (if configured). Captures
output. If not configured, prints "interfaces command not configured — skipping"
and exits 0.

---

### 5.6 Instructions Engine

```
metis instructions
```

Emit the full agent contract. This is the dynamically-generated equivalent of
a static AGENTS.md. It assembles the contract from `metis.yaml` fields:

- Session start protocol (always run `metis next` → `metis kickoff`)
- Branch and commit rules (from `commits` config)
- Definition of Done
- Roles (Coder/Reviewer/Human)
- Hot-path zones (from `hot_paths`)
- Scope discipline rules
- Model routing (from `routing`)
- Testing rules (from `testing`)
- Non-goals (from `non_goals`)
- Accuracy rules (from `accuracy_rules`)
- Review checklist (from `review_checklist`)
- Feedback loop description
- Tooling map (metis commands)

Output is valid markdown, suitable for piping into a file or reading by an
agent.

```
metis instructions --for <slice-id>
```

Risk-scaled subset. Based on the slice's risk level:

| Risk | What's included |
|------|-----------------|
| low | Core rules (branch/commit, DoD, scope, testing, non-goals, tooling) |
| medium | + hot-path zones, accuracy rules, review checklist |
| high | Everything (full contract) |

Plus: always includes prior briefs touching the slice's declared packages (if
the brief exists), and the relevant plan section.

```
metis instructions --for <slice-id> --json
```

Structured JSON output for programmatic consumption by agent tooling.

```
metis kickoff
```

Emit the session protocol — the step-by-step procedure an agent follows from
session start. Dynamically generated from config (branch names, commands,
etc.):

1. Establish state (`git status`, confirm integration branch, clean tree)
2. Find active slice (`metis next`)
3. Self-identify and match (compare agent identity to required slug)
4. Read instructions (`metis instructions --for <id>`)
5. Pre-flight (`metis verify --pre`)
6. Execute (Coder flow or Reviewer flow based on role)
7. Report

```
metis kickoff --role coder
metis kickoff --role reviewer
```

Emit only the relevant flow section.

---

### 5.7 Git Enforcement

```
metis commit [--prefix <prefix>] --message "..."
```

Wrapper around `git commit` that enforces the project's commit conventions:

1. Validates the current branch is the configured integration branch
2. Validates a slice is active (from `metis next --quiet`)
3. Constructs the commit subject: `{prefix}({slice_id}): {message}`
4. Validates the prefix is in the allowed list (from `commits.prefixes`)
5. Strips any `Co-Authored-By`, `Generated with`, or model-name lines from
   the commit message (enforcing `no_attribution`)
6. Runs `git commit` with the formatted message

If `--prefix` is omitted, infers from the slice type:
- feat → `feat`
- fix → `fix`
- refactor → `refactor`
- debt → `refactor`
- remove → `refactor`
- chore → `chore`
- security → `fix`
- gate → `docs`
- recon → `docs`

```bash
# Agent just says what it did:
metis commit --message "add webhook handler and tests"
# Produces: feat(phase-2-ws-2.3): add webhook handler and tests

# Override prefix:
metis commit --prefix test --message "add integration tests for webhooks"
# Produces: test(phase-2-ws-2.3): add integration tests for webhooks
```

```
metis commit --brief
```

Shortcut for the brief commit. Equivalent to:
```bash
git add .metis/briefs/<active-id>.md
metis commit --prefix docs --message "slice brief"
```

```
metis commit --flip coded
```

Shortcut for the coded flip commit. Flips the ledger and commits:
```bash
metis flip coded <active-id>
git add .metis/slices.yaml
metis commit --prefix chore --message "flip coded"
```

```
metis commit --flip reviewed
```

Same for review sign-off.

```
metis commit --amend
```

Amend the previous commit (same validation rules apply to the existing
subject).

---

### 5.8 Observability

```
metis progress
```

Terminal dashboard showing:
- Per-phase progress bars (done / in-progress / pending)
- Overall completion percentage
- By-stage breakdown
- Quality stats — see 'metis findings --stats' (per-agent first-pass rate); inline in progress is deferred roadmap
- Currently active slice
- Next up

Supports `--no-color` for piping.

```
metis findings [--category <cat>] [--severity <sev>] [--slice <id>]
```

Show review findings, optionally filtered. Table format:

```
Sev  Category   Slice            Finding
P1   arch-dup   phase-2-ws-2.1   Duplicated Validator trait (src/eval/mod.rs:42)
P2   scope      phase-1-ws-1.4   Edited src/auth/ without declaring it
```

```
metis findings --stats
```

Summary statistics: findings per category, per severity, per agent. Evidence
for routing adjustments.

```
metis log <id>
```

Show the full history of a slice: creation, brief commit, code commits,
verify runs, review cycles, findings, sign-off. Derived from git log + metis
state.

```
metis status
```

Quick one-line status: active slice, role, model, progress fraction.

```bash
$ metis status
phase-2-ws-2.3 | Coder | opencode/opus | 14/42 done (33%)
```

---

### 5.9 Planning & Seeding

```
metis seed <plan-file> [flags]
```

Parse a structured implementation plan and generate ledger entries. The plan
file must follow a parseable structure (workstream headings with metadata):

Expected plan structure:
```markdown
## Phase 1 — Foundation

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
- Interfaces documented in interfaces.txt
```

`metis seed` extracts:
- Phase/workstream hierarchy → slice IDs (`phase-1-ws-1.1`)
- Risk/coder/reviewer/stage assignments
- Plan file + section references
- Titles from workstream names

Flags:
- `--dry-run` — show what would be created without writing
- `--append` — add to existing ledger (default: error if ledger non-empty)
- `--phase <N>` — seed only one phase
- `--type <type>` — override type for all generated slices (default: `feat`)

```
metis seed --interactive <plan-file>   # DEFERRED — not implemented; use --dry-run + edit the plan
```

Walks through each workstream and prompts for confirmation/overrides before
adding to the ledger.

---

### 5.10 Rules Management

```
metis rule add "..."
```

Append a new accuracy rule to `metis.yaml` → `accuracy_rules`. Used when a
recurring finding is promoted to a permanent rule.

```
metis rule list
```

Show all accuracy rules (numbered for reference in findings).

```
metis rule promote <finding-id>
```

Promote a finding to an accuracy rule. Marks the finding as `promoted` and
adds its text to `accuracy_rules`.

---

### 5.11 Surface Adapters

```
metis surface generate
```

Write/overwrite surface adapter files from current config:

**`CLAUDE.md`** (for Claude Code):
```markdown
# CLAUDE.md
Run `metis kickoff` from step 1 at the start of every session. No pasted
prompt is needed. For full contract details: `metis instructions`.

Identity: state your model as one of the slugs from `metis next` output.
```

**`AGENTS.md`** (for Codex and other AGENTS.md-native surfaces):
Generated from `metis instructions` output, written as a static file.
Regenerated on every `metis surface generate`.

**`opencode.json`** (for opencode):
```json
{
  "$schema": "https://opencode.ai/config.json",
  "instructions": ["AGENTS.md"]
}
```

**`.claude/settings.json`**:
```json
{
  "attribution": { "commit": "", "pr": "" },
  "includeCoAuthoredBy": false
}
```

```
metis surface validate
```

Check that adapter files exist and match current config. Warns if stale
(config changed since last `generate`). Part of `metis check`.

---

## 6. Instructions Engine (Detail)

The instructions engine is the core differentiation from a static AGENTS.md.
It computes the right instructions for the right context.

### Assembly Order

`metis instructions` assembles sections in this order:

1. **Header** — project name, date, branch model
2. **Session protocol** — "run `metis next`, then `metis kickoff`"
3. **Branch & commit rules** — from `commits` config
4. **Definition of Done** — the 5-point gate
5. **Roles** — Coder, Reviewer, Human
6. **Hot-path zones** — from `hot_paths`
7. **Scope discipline** — brief-before-code, owned_paths contract
8. **Model routing** — from `routing`
9. **Testing rules** — from `testing`
10. **Non-goals** — from `non_goals`
11. **Accuracy rules** — from `accuracy_rules`
12. **Review checklist** — from `review_checklist`
13. **Feedback loop** — findings → rules promotion, review_cycles, gates
14. **Tooling map** — all `metis` commands and their purpose

### Risk Scaling

When `--for <id>` is passed, sections are filtered:

| Section | Low | Medium | High |
|---------|-----|--------|------|
| Session protocol | ✓ | ✓ | ✓ |
| Branch & commit | ✓ | ✓ | ✓ |
| Definition of Done | ✓ | ✓ | ✓ |
| Roles | ✓ | ✓ | ✓ |
| Hot-path zones | | ✓ | ✓ |
| Scope discipline | ✓ | ✓ | ✓ |
| Model routing | | | ✓ |
| Testing rules | ✓ | ✓ | ✓ |
| Non-goals | ✓ | ✓ | ✓ |
| Accuracy rules | | ✓ | ✓ |
| Review checklist | | ✓ | ✓ |
| Feedback loop | | | ✓ |
| Tooling map | ✓ | ✓ | ✓ |

### Contextual Augmentation

When `--for <id>` is passed, the engine also appends:

- **Prior briefs** — any briefs whose `owned_paths` overlap with the current
  slice's plan section's suggested packages. These are the "archaeology" that
  prevents re-exploration.
- **Plan section** — the relevant section from the plan file (if configured).
- **Active findings** — any open findings against this slice (if it's been
  blocked and is being reworked).

---

## 7. Git Enforcement (Detail)

### Branch Protection

All `metis commit` and `metis flip` commands validate:
- Current branch matches `project.integration_branch`
- Working tree has no untracked `metis.yaml` changes (config must be committed)

### Commit Subject Format

Enforced pattern: `{prefix}({slice_id}): {message}`

- Prefix: from `commits.prefixes` allowed list
- Slice ID: from the currently active slice
- Message: free-form, lowercase first letter, no period

### Attribution Stripping

`metis commit` scans the full commit message and removes:
- Any line matching `Co-Authored-By:.*`
- Any line matching `Generated with.*`
- Any line containing model names (configurable pattern list)

### Commit Validation (for reviewers)

```
metis log <id> --validate
```

Checks that all commits for a slice:
- Have the slice ID in the subject
- Use valid prefixes
- Contain no attribution lines
- Are on the correct branch

---

## 8. Verification Pipeline (Detail)

### Pipeline Order

```
metis verify
  │
  ├─ 1. metis env-check
  │     └─ fail? → exit 2 (ENVIRONMENT FAILURE)
  │
  ├─ 2. metis check --ledger
  │     └─ fail? → exit 3 (LEDGER ERROR)
  │
  └─ 3. <configured verify command>
        └─ fail? → exit 1 (CODE FAILURE)
        └─ pass? → exit 0
```

### Output Capture

All output (stdout + stderr, interleaved) is captured to the appropriate log
file in `.metis/runs/<slice-id>/`. A timestamp header is prepended:

```
═══ metis verify ═══ 2026-07-09T14:32:01Z ═══ slice: phase-2-ws-2.3 ═══

[command output follows]

═══ exit code: 0 ═══
```

### Reviewer Access

The reviewer can inspect the coder's run without re-executing:

```bash
cat .metis/runs/phase-2-ws-2.3/verify-post.log   # ('metis show --runs' is deferred roadmap)
```

The stored verify logs are plain files. The reviewer MUST still run `metis verify`
independently — stored logs are evidence, not proof.

---

## 9. Surface Adapters (Detail)

### Purpose

Different agent surfaces discover instructions differently:
- **Claude Code** reads `CLAUDE.md` at repo root automatically
- **opencode** reads `opencode.json` → references `AGENTS.md`
- **Codex** reads `AGENTS.md` natively

Metis generates these thin adapter files so each surface finds its way to
`metis kickoff`.

### Regeneration

`metis surface generate` is idempotent. Run it:
- After `metis init`
- After changing `metis.yaml` (routing, agents, rules)
- After `metis rule add` (to update the static AGENTS.md)

`metis check` warns if adapters are stale (hash mismatch with current config).

### Custom Adapters

If a new agent surface emerges, add a template to the Metis codebase. The
adapter contract is: point the agent at running `metis kickoff` on session
start.

---

## 10. Planning & Seeding (Detail)

### The Planning Regime

Metis does NOT own planning. Planning is human-led (often with agent
assistance). Metis's role in planning:

1. **`metis seed`** — parse a structured plan into ledger entries
2. **`metis add`** — manually add individual slices
3. **`metis check`** — validate the result

The plan file format is a convention, not a hard schema. `metis seed` parses
markdown with expected heading patterns. If your plan doesn't match, use
`metis add` to manually create slices.

### Seed Validation

After seeding, `metis check` validates:
- All coder/reviewer slugs exist in the agents map
- No self-review (coder != reviewer for every slice)
- Risk values are valid
- plan_section present when plan is set
- IDs are unique
- No circular `blocked_by` dependencies

### Plan Refactoring

When the plan changes mid-execution:
- Completed slices are immutable (never modify archived entries)
- In-progress slices need human decision before re-scope
- New work is additive (`metis add`)
- Use `metis edit` to adjust pending slices
- Use `metis skip` to abandon slices that are no longer needed
- Run `metis check` after any batch of changes

---

## 11. Feedback Loop (Detail)

### The Correction Cycle

```
Review block
    │
    v
metis block <id> --severity P1 --category arch-dup --finding "..."
    │
    ├─ Appends to .metis/findings.yaml
    ├─ Increments review_cycles on the slice
    └─ Resets coded=false (back to coder)

    ... after ~10 slices ...

Human reviews findings:
    │
    v
metis findings --stats
    │
    ├─ "arch-dup: 4 findings across 3 slices"
    └─ Decision: promote to rule

    │
    v
metis rule promote f-001
    │
    ├─ Adds to accuracy_rules in metis.yaml
    ├─ Marks finding as promoted
    └─ Next `metis surface generate` updates AGENTS.md
```

### Routing Feedback

`metis findings --stats` includes per-agent breakdown:

```
Agent              Slices  Blocks  First-pass rate
claude-code/opus   8       1       87%
opencode/opus      12      2       83%
codex              6       3       50%  ← consider re-routing
```

Evidence for adjusting `routing` in `metis.yaml`.

### Phase Gates

A `gate` slice validates composition:
- Runs integration scenarios across the phase's surface
- Commits evidence to `.metis/briefs/<gate-id>.md`
- Composition failures are filed as blocks against the offending slices

---

## 12. Agent Session Protocol (Detail)

What an agent does on session start (generated by `metis kickoff`):

### Step 1: Establish State

```bash
git rev-parse --abbrev-ref HEAD   # must be integration branch
git status                        # must be clean
metis status                      # quick orientation
```

Dirty tree → stop. Wrong branch → stop. Report to human.

### Step 2: Find Active Slice

```bash
metis next
```

Trust this over any manual reading. If no slices remain, report "backlog
empty" and stop.

### Step 3: Self-Identify

State identity in one line. Match against the required model slug from
`metis next` output.

- Match → continue
- No match → stop, report which agent is needed

### Step 4: Read Instructions

```bash
metis instructions --for <slice-id>
```

Read the output. This is the risk-scaled contract plus contextual archaeology
(prior briefs, plan section). No more, no less.

### Step 5: Pre-flight Verification

```bash
metis verify --pre
```

- Exit 2 (env failure) → stop, report verbatim, do not modify code
- Exit 1 (code failure before your changes) → stop, report pre-existing
  breakage, do not fix it (scope violation)
- Exit 0 → continue

### Step 6a: Coder Flow

1. **Read interfaces** — `metis interfaces` output (if configured)
2. **Write brief** — `metis brief <id> --write`, edit it, `metis commit --brief`
3. **Implement** — within declared scope only
4. **Verify** — `metis verify --post`
   - If exit 2 mid-session: env degraded, stop and report
   - If exit 1: your code broke something, fix within scope
5. **Regenerate interfaces** — `metis interfaces` (if you changed public API)
6. **Flip** — `metis commit --flip coded`
7. **Report** — slice ID, files changed, verify result, what's next

### Step 6b: Reviewer Flow

1. **Locate commits** — `git log --oneline --grep "<slice-id>"`
2. **Read brief** — `metis brief <id>` (reads existing)
3. **Independent verify** — `metis verify --post`
4. **Walk checklist** — one-line verdict per item, citing `file:line`
5. **Verdict:**
   - Pass → `metis commit --flip reviewed` then `metis archive`
   - Block → `metis block <id> --severity ... --category ... --finding "..."`
6. **Report** — slice ID, verdict, findings (if any), what's next

---

## 13. Go Package Structure

```
github.com/adamjasinski/metis/
├── cmd/
│   └── metis/
│       └── main.go              # CLI entry point
├── internal/
│   ├── cli/                     # Command registration (cobra)
│   │   ├── root.go
│   │   ├── init.go
│   │   ├── next.go
│   │   ├── add.go
│   │   ├── edit.go
│   │   ├── flip.go
│   │   ├── block.go
│   │   ├── brief.go
│   │   ├── verify.go
│   │   ├── commit.go
│   │   ├── check.go
│   │   ├── archive.go
│   │   ├── progress.go
│   │   ├── findings.go
│   │   ├── instructions.go
│   │   ├── kickoff.go
│   │   ├── surface.go
│   │   ├── seed.go
│   │   ├── rule.go
│   │   ├── log.go
│   │   ├── show.go
│   │   ├── list.go
│   │   └── status.go
│   ├── config/                  # metis.yaml parsing + validation
│   │   ├── config.go           # Types + Load()
│   │   ├── validate.go         # Structural validation
│   │   └── config_test.go
│   ├── ledger/                  # Slice CRUD + lifecycle logic
│   │   ├── ledger.go           # Load, Save, query
│   │   ├── dispatch.go         # next-slice algorithm
│   │   ├── lifecycle.go        # flip, block, skip, reopen
│   │   ├── validate.go         # check (lint)
│   │   ├── archive.go          # archive done slices
│   │   └── ledger_test.go
│   ├── slice/                   # Slice domain types
│   │   ├── types.go            # Slice, WorkType, Priority, Risk
│   │   ├── id.go               # ID generation + parsing
│   │   └── types_test.go
│   ├── brief/                   # Brief template generation
│   │   ├── templates.go        # Per-type templates
│   │   ├── render.go           # Template rendering
│   │   └── brief_test.go
│   ├── instructions/            # Contract generation engine
│   │   ├── engine.go           # Assembly + risk scaling
│   │   ├── sections.go         # Section definitions
│   │   ├── context.go          # Contextual augmentation (briefs, plan)
│   │   └── engine_test.go
│   ├── runner/                  # External command execution
│   │   ├── runner.go           # Run + capture output
│   │   ├── verify.go           # Verification pipeline
│   │   ├── envcheck.go         # Environment check logic
│   │   └── runner_test.go
│   ├── git/                     # Git operations
│   │   ├── commit.go           # Commit with enforcement
│   │   ├── branch.go           # Branch validation
│   │   ├── attribution.go      # Attribution stripping
│   │   ├── log.go              # Commit history queries
│   │   └── git_test.go
│   ├── surface/                 # Adapter file generation
│   │   ├── generate.go         # Write adapter files
│   │   ├── validate.go         # Staleness check
│   │   ├── templates.go        # Adapter templates
│   │   └── surface_test.go
│   ├── findings/                # Findings store
│   │   ├── store.go            # CRUD for findings
│   │   ├── stats.go            # Aggregation + reporting
│   │   └── findings_test.go
│   ├── progress/                # Terminal dashboard
│   │   ├── dashboard.go        # Render progress
│   │   ├── bar.go              # Progress bar rendering
│   │   └── progress_test.go
│   ├── seed/                    # Plan file parsing
│   │   ├── parser.go           # Markdown plan → slice list
│   │   └── seed_test.go
│   └── runs/                    # Run log storage
│       ├── store.go            # Write/read run logs
│       └── runs_test.go
├── testdata/                    # Test fixtures
│   ├── metis.yaml              # Sample config
│   ├── slices.yaml             # Sample ledger
│   └── plan.md                 # Sample plan file
├── go.mod
├── go.sum
├── LICENSE
└── README.md
```

---

## 14. Distribution & Installation

### Primary: Go Install

```bash
go install github.com/adamjasinski/metis/cmd/metis@latest
```

### Binary Releases

GitHub Releases with pre-built binaries for:
- linux/amd64, linux/arm64
- darwin/amd64, darwin/arm64
- windows/amd64

Built with GoReleaser.

### Verification

```bash
metis --version
# metis v0.1.0 (abc1234) built 2026-07-09
```

### Dependencies (Go module)

Minimal:
- `github.com/spf13/cobra` — CLI framework
- `gopkg.in/yaml.v3` — YAML parsing
- `github.com/fatih/color` — terminal colors (progress dashboard)

No heavy frameworks. No network calls. No telemetry. Metis is a local tool
that reads files and runs commands.

---

## Appendix A: Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General failure (code error, validation failure) |
| 2 | Environment failure (env-check failed) |
| 3-11 | Reserved (deferred roadmap — these currently exit 1; only 0/1/2 are implemented) |

---

## Appendix B: File Layout in a Consuming Project

After `metis init`:

```
my-project/
├── metis.yaml                   # Project configuration (human-edits)
├── .metis/
│   ├── slices.yaml              # Active ledger (metis-managed + human-editable)
│   ├── slices-done.yaml         # Archive (metis-managed)
│   ├── briefs/                  # Per-slice scope contracts
│   │   └── .gitkeep
│   ├── findings.yaml            # Review findings (metis-managed)
│   └── runs/                    # Verification logs (gitignored)
│       └── .gitkeep
├── CLAUDE.md                    # Generated surface adapter
├── AGENTS.md                    # Generated surface adapter (full contract)
├── opencode.json                # Generated surface adapter
├── .claude/
│   └── settings.json            # Generated (no attribution)
├── docs/
│   ├── generated/
│   │   └── interfaces.txt       # Generated API summary
│   └── adr/                     # Architecture decision records
│       ├── _template.md
│       ├── 0000-record-architecture-decisions.md
│       └── README.md
└── plans/                       # Human-written plans (optional)
    └── impl-plan.md
```

### .gitignore additions

```
.metis/runs/
```

Runs are local verification evidence. The reviewer runs independently; stored
logs are for convenience, not for committing.

---

## Appendix C: Configuration Defaults

Fields with sensible defaults (can be omitted from `metis.yaml`):

```yaml
project:
  integration_branch: dev        # default: dev
  release_branch: main           # default: main

commits:
  prefixes: [feat, fix, refactor, docs, test, chore]  # default
  require_slice_id: true         # default: true
  no_attribution: true           # default: true
  format: "{prefix}({slice_id}): {message}"  # default

paths:
  ledger: .metis/slices.yaml     # default
  archive: .metis/slices-done.yaml
  briefs: .metis/briefs/
  findings: .metis/findings.yaml
  runs: .metis/runs/
  interfaces: docs/generated/interfaces.txt
```

---

## Appendix D: Minimal `metis.yaml` (smallest valid config)

```yaml
version: 1

project:
  name: my-project

agents:
  claude-code/opus:
    surface: claude-code
    model: opus
    label: "Claude Code (Opus)"

commands:
  verify: "echo 'no verify configured'"
```

Everything else uses defaults. Single-agent projects are valid (review is
skipped or self-review is allowed with a flag).

---

## Appendix E: Multi-Phase Example Ledger

```yaml
# .metis/slices.yaml
version: 1
slices:
  # ─── Phase 0: Foundation ────────────────────────────────
  - id: phase-0-ws-0.1
    title: "Foundational ADRs"
    type: feat
    priority: p2
    risk: high
    coder: claude-code/opus
    reviewer: opencode/opus
    plan: plans/impl-plan.md
    plan_section: "§3.1"
    coded: true
    reviewed: true
    review_cycles: 0
    created: 2026-07-01

  # ─── Phase 1: Core ─────────────────────────────────────
  - id: phase-1-ws-1.1
    title: "Domain model"
    type: feat
    priority: p2
    risk: high
    coder: opencode/opus
    reviewer: codex
    plan: plans/impl-plan.md
    plan_section: "§4.1"
    coded: true
    reviewed: false
    review_cycles: 0
    created: 2026-07-02

  - id: phase-1-ws-1.2
    title: "Storage layer"
    type: feat
    priority: p2
    risk: medium
    coder: codex
    reviewer: opencode/opus
    plan: plans/impl-plan.md
    plan_section: "§4.2"
    coded: false
    reviewed: false
    review_cycles: 0
    blocked_by: [phase-1-ws-1.1]
    created: 2026-07-02

  # ─── Interrupt: P0 security fix ─────────────────────────
  - id: fix-0001
    title: "Auth bypass in token validation"
    type: security
    priority: p0
    risk: high
    coder: claude-code/opus
    reviewer: opencode/opus
    coded: false
    reviewed: false
    review_cycles: 0
    created: 2026-07-09

  # ─── Tech debt (low priority) ──────────────────────────
  - id: debt-0001
    title: "Replace hand-rolled errors with thiserror"
    type: debt
    priority: p3
    risk: low
    coder: codex
    reviewer: opencode/opus
    coded: false
    reviewed: false
    review_cycles: 0
    created: 2026-07-08
```

In this ledger, `metis next` returns `fix-0001` (p0 priority) even though
`phase-1-ws-1.1` is awaiting review and `phase-1-ws-1.2` is next in
declaration order — priority wins.

---

## Appendix F: Comparison with Current Harness

| Aspect | Current (CAESAR harness) | Metis |
|--------|--------------------------|-------|
| Config | Spread across AGENTS.md, Makefile, slices.yaml agents map | Single `metis.yaml` |
| Dispatch | Python script + Makefile target | `metis next` (Go binary) |
| Instructions | Static AGENTS.md (agent decides reading depth) | Dynamic, risk-scaled (`metis instructions --for`) |
| Verification | `make env-check && make verify` (agent remembers order) | `metis verify` (pipeline built-in) |
| Git | Agent manages commit format manually | `metis commit` (enforced) |
| Findings | Markdown table, hand-appended | Structured YAML, `metis block` writes it |
| Progress | Python script | `metis progress` (built-in) |
| Surface adapters | Hand-written | `metis surface generate` |
| Work types | Implicit (everything is a feature slice) | Explicit types with adapted brief templates |
| Priority | Declaration order only | p0-p3 with interrupt semantics |
| Dependencies | `blocked_by` not supported | First-class `blocked_by` |
| Seeding | Manual YAML editing | `metis seed <plan>` |
| Rules | Edit AGENTS.md manually | `metis rule add/promote` |
| Technology | Go-specific Makefile + Python dispatcher | Technology-agnostic (configured commands) |
| Distribution | Copy files between repos | `go install` |
