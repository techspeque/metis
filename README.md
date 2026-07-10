# Metis

> The meta-intelligence that orchestrates AI coding agents.

Metis is a Go CLI tool for managing autonomous coding-agent workflows. It
enforces disciplined, bounded, independently-reviewed units of work across
any technology stack, any agent surface (Claude Code, opencode, Codex, or
others), with the human retaining ownership of planning, escalations, and
releases.

## Why Metis

A single long agent session drifts: it loses scope, re-explores code it
already understood, fixes things it was not asked to fix, marks its own
homework, and quietly works around contradictions instead of surfacing them.

Metis replaces "one big session" with **many small, bounded,
independently-reviewed units of work dispatched deterministically.**

## Install

```bash
go install github.com/techspeque/metis/cmd/metis@latest
```

Or build from source:

```bash
git clone https://github.com/techspeque/metis.git
cd metis
go build -o metis ./cmd/metis
```

## Quick Start

### 1. Write your OVERVIEW

The OVERVIEW is the single source of intent — it describes what you're building
and why. This is a human-maintained specification that agents reference for
context during every session.

```bash
# Create your application spec
vim OVERVIEW.md
```

### 2. Initialize Metis

```bash
cd your-project
metis init
```

This creates:
- `metis.yaml` — project configuration
- `.metis/` — state directory (ledger, briefs, plans, ADRs, findings)
- `CLAUDE.md`, `AGENTS.md`, `opencode.json` — surface adapters

### 3. Configure

Edit `metis.yaml`:

```yaml
version: 1

project:
  name: my-project
  overview: OVERVIEW.md
  integration_branch: dev
  release_branch: main

agents:
  claude-code/opus:
    surface: claude-code
    model: opus
    label: "Claude Code (Opus)"
  opencode/opus:
    surface: opencode
    model: opus
    label: "opencode (Opus)"

routing:
  high: [claude-code/opus]
  medium: [opencode/opus]
  low: [opencode/opus]
  review: cross-vendor

commands:
  verify: "go build ./... && go test ./..."
  env_check: "go version"
```

### 4. Plan a phase (agent-assisted)

Ask a coding agent to create an implementation plan from your OVERVIEW.
The agent produces a plan file in `.metis/plans/` and ADRs in `.metis/adr/`.

### 5. Seed and execute

```bash
metis seed .metis/plans/phase-1-plan.md     # create slices from plan
metis check                                  # validate everything
# Launch agent sessions — each runs `metis kickoff`
```

## Workflow: OVERVIEW to Execution

Metis has two clearly separated regimes:

1. **Planning** (human-led, runs per phase) — produces frozen plan artifacts
2. **Execution** (agent-driven, runs every session) — drains the ledger one slice at a time

### Source of Truth

The authority chain, in order:

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

### Greenfield: Building a New App

```
OVERVIEW.md (you write the full application spec)
    ↓ agent-assisted planning, one phase at a time
.metis/plans/phase-N.md + .metis/adr/NNNN-*.md
    ↓ metis seed
.metis/slices.yaml → agents execute → code
```

#### Step 1: Write the OVERVIEW

The OVERVIEW describes the full application: what it does, its architecture,
constraints, non-goals, and a rough sketch of all phases. This is the
document you hand to an agent when you say "plan Phase 1."

#### Step 2: Initialize and configure

```bash
metis init
# Edit metis.yaml: set project.overview, agents, commands, hot_paths, routing
metis surface generate
```

#### Step 3: Plan one phase at a time

Ask a coding agent to read the OVERVIEW and produce:
- A structured plan file (`.metis/plans/phase-0-plan.md`)
- ADRs for binding decisions (`.metis/adr/0001-*.md`)

The plan file follows the workstream format that `metis seed` can parse.

#### Step 4: Seed the ledger

```bash
metis seed .metis/plans/phase-0-plan.md --dry-run   # preview
metis seed .metis/plans/phase-0-plan.md             # create slices
metis check                                          # validate
```

#### Step 5: Cold-start (Phase 0)

The first slices create the things later slices read:

| Slice | Purpose | Why first |
|---|---|---|
| `phase-0-ws-0.1` | Project scaffold + verify gate | Makes `metis verify` meaningful |
| `phase-0-ws-0.2` | Core interfaces + ADRs | Makes `metis interfaces` meaningful |
| `phase-0-ws-0.3` | Domain model | Gives later slices real types to consume |

Each completed brief becomes archaeology for the next slice.

#### Step 6: Execute, then plan the next phase

```bash
# After Phase 0 is done:
# Ask agent to plan Phase 1 (it now has the benefit of Phase 0 learnings)
metis seed .metis/plans/phase-1-plan.md --append
```

The hybrid approach: sketch all phases in the OVERVIEW upfront, but plan
each phase in detail only when you're about to execute it.

---

### Extending: Adding to an Existing App

When you already have a running codebase and want to add features, fix bugs,
or refactor:

#### Step 1: Update the OVERVIEW

Add the new requirements to OVERVIEW.md. This keeps it as the single source
of intent for the full application.

#### Step 2: Plan the change

Ask an agent to plan the new work from the updated OVERVIEW. The plan can
be small — even a single phase with a few workstreams:

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

#### Step 3: Execute

Same as greenfield — agents run `metis kickoff` and the dispatch algorithm
handles priority, ordering, and blocked dependencies.

---

### When the OVERVIEW Changes

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

### Ongoing Maintenance

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

## Commands

### Dispatch

| Command | Purpose |
|---|---|
| `metis next` | Find the active slice, role, and required agent |
| `metis next --json` | Same, as JSON |
| `metis next --quiet` | Print only the slice ID |
| `metis status` | One-line summary: `slice-id \| Role \| agent \| progress` |

### Work Management

| Command | Purpose |
|---|---|
| `metis add <type>` | Add a slice (`feat`, `fix`, `refactor`, `debt`, `remove`, `chore`, `security`, `gate`, `recon`) |
| `metis list` | List slices (filterable: `--type`, `--priority`, `--status`) |
| `metis show <id>` | Full details of a slice |
| `metis seed <plan>` | Parse a plan file into slices (`--dry-run`, `--append`, `--phase`) |

### Slice Lifecycle

| Command | Purpose |
|---|---|
| `metis flip coded <id>` | Mark as coded |
| `metis flip reviewed <id>` | Mark as reviewed (sign-off) |
| `metis block <id>` | Block during review (`--severity`, `--category`, `--finding`) |
| `metis skip <id> --reason "..."` | Skip without implementation |
| `metis reopen <id> --reason "..."` | Reset for re-implementation |
| `metis archive` | Move all done slices to the archive |

### Verification

| Command | Purpose |
|---|---|
| `metis verify --pre` | Pre-flight check (before changes) |
| `metis verify --post` | Post-implementation check |
| `metis env-check` | Environment soundness (exit 2 = env failure) |
| `metis interfaces` | Regenerate API summary |
| `metis check` | Validate config + ledger (`--config`, `--ledger`) |

### Git Enforcement

| Command | Purpose |
|---|---|
| `metis commit -m "..."` | Commit with enforced format: `prefix(slice-id): message` |
| `metis commit --brief` | Stage and commit the brief |
| `metis commit --flip coded` | Flip coded + commit ledger |
| `metis commit --flip reviewed` | Flip reviewed + commit ledger |

### Instructions

| Command | Purpose |
|---|---|
| `metis instructions` | Full agent contract |
| `metis instructions --for <id>` | Risk-scaled contract for a slice |
| `metis kickoff` | Session protocol steps |
| `metis brief <id>` | Show/generate brief template |
| `metis brief <id> --write` | Write brief to `.metis/briefs/` |

### Observability

| Command | Purpose |
|---|---|
| `metis progress` | Dashboard with progress bars |
| `metis findings` | Review findings (`--stats`, `--severity`, `--category`) |

### Rules & Adapters

| Command | Purpose |
|---|---|
| `metis rule add "..."` | Add an accuracy rule |
| `metis rule list` | Show all rules |
| `metis rule promote <finding-id>` | Promote a finding to a rule |
| `metis surface generate` | Write/overwrite adapter files |
| `metis surface validate` | Check for staleness |

## How It Works

### The Dispatch Algorithm

`metis next` finds the active slice:

1. Filter to unblocked slices (no incomplete `blocked_by` deps)
2. Sort by priority (p0 > p1 > p2 > p3)
3. Within same priority, declaration order wins
4. `coded: false` → role is Coder
5. `coded: true`, `reviewed: false` → role is Reviewer

### Risk Scaling

Instructions are filtered by the slice's risk level:

| Section | Low | Medium | High |
|---|---|---|---|
| Core rules (branch, DoD, scope, testing) | Yes | Yes | Yes |
| Hot-path zones, accuracy rules, checklist | | Yes | Yes |
| Model routing, feedback loop | | | Yes |

### The Feedback Loop

```
Review block → metis block (records finding)
     ↓
Recurring findings → metis rule promote (becomes permanent rule)
     ↓
Rules in instructions → prevents future occurrences
```

## Project Layout

After `metis init`:

```
your-project/
├── OVERVIEW.md             # Application spec (you maintain this)
├── metis.yaml              # Configuration (you edit this)
├── .metis/
│   ├── slices.yaml         # Active ledger
│   ├── slices-done.yaml    # Archive
│   ├── briefs/             # Per-slice scope contracts
│   ├── plans/              # Implementation plans (per phase)
│   ├── adr/                # Architecture Decision Records
│   │   └── _template.md
│   ├── findings.yaml       # Review findings
│   └── runs/               # Verification logs (gitignored)
├── CLAUDE.md               # Surface adapter (generated)
├── AGENTS.md               # Full contract (generated)
└── opencode.json           # Surface adapter (generated)
```

## Core Principles

1. **Plan once, execute many** — agents don't re-plan mid-flight
2. **One active slice at a time** — deterministic dispatch, not agent judgment
3. **Two lanes plus a human** — Coder implements, Reviewer checks, Human owns planning
4. **Scope is a written contract** — brief before code, reviewer measures against it
5. **Reality beats documents** — code > contract > plans
6. **Risk scales effort** — reading depth and model routing adapt to risk
7. **Determinism for things agents fumble** — no YAML walking, no slug matching by agents
8. **The system self-corrects** — findings become rules, data drives routing
9. **Technology agnosticism** — configured commands, not framework coupling
10. **Full SDLC** — features, bugs, refactoring, deletions, maintenance

## License

MIT
