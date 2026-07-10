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

### 1. Initialize a project

```bash
cd your-project
metis init
```

This creates:
- `metis.yaml` — project configuration
- `.metis/` — state directory (ledger, briefs, findings)
- `CLAUDE.md`, `AGENTS.md`, `opencode.json` — surface adapters

### 2. Configure agents and commands

Edit `metis.yaml`:

```yaml
version: 1

project:
  name: my-project
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

### 3. Add work slices

```bash
# From a plan file
metis seed plans/impl-plan.md --dry-run   # preview
metis seed plans/impl-plan.md             # create slices

# Or manually
metis add feat --title "Add auth middleware" \
  --coder opencode/opus --reviewer claude-code/opus \
  --risk high
```

### 4. Agents run the loop

Every agent session starts with:

```bash
metis kickoff
```

Which tells the agent to:

1. Check branch and clean tree
2. Run `metis next` to find the active slice
3. Self-identify and match the required model
4. Read `metis instructions --for <id>` for the risk-scaled contract
5. Run `metis verify --pre` (pre-flight)
6. Execute (code or review)
7. Run `metis verify --post`
8. Flip and report

## Workflow: Planning to Execution

Metis has two clearly separated regimes:

1. **Planning** (human-led, runs rarely) — produces frozen artifacts
2. **Execution** (agent-driven, runs every session) — drains the ledger one slice at a time

### Source of Truth

The authority chain at any point in time:

| Stage | Source of truth | Who owns it |
|---|---|---|
| Intent | Plan file (`plans/*.md`) | Human |
| Dispatch | Ledger (`.metis/slices.yaml`) | Human seeds, Metis manages |
| Scope | Brief (`.metis/briefs/<id>.md`) | Coder commits before coding |
| Reality | The code itself | Code > contract > plan |

When documents disagree: **observed code > brief > plan**. The agent fixes the
wrong document, never silently codes around a contradiction.

---

### Greenfield: Building a New App

When you have nothing but an idea, the workflow is:

#### Step 1: Plan (human, with agent assistance)

Write a structured plan decomposing the build into phases and workstreams:

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
- Create core interfaces

Acceptance criteria:
- Project compiles
- `metis verify` passes
- Interface summary generates

### Workstream 0.2: Domain model
- **Risk:** high
- **Coder:** opencode/opus
- **Reviewer:** claude-code/opus
- **Stage:** foundation

Tasks:
- Define domain entities
- Define repository interfaces

Acceptance criteria:
- Domain types compile
- Interfaces documented

## Phase 1 — Core Features
...
```

Save this as `plans/impl-plan.md`.

#### Step 2: Bootstrap Metis

```bash
metis init
# Edit metis.yaml with your agents, commands, hot paths, rules
metis surface generate
```

#### Step 3: Seed the ledger

```bash
metis seed plans/impl-plan.md --dry-run    # preview
metis seed plans/impl-plan.md              # create slices
metis check                                # validate
```

#### Step 4: Cold-start slices

The first slices in a greenfield project **create the things later slices
will read**. A typical Phase 0:

| Slice | Purpose | Why first |
|---|---|---|
| `phase-0-ws-0.1` | Project scaffold + verify gate | Makes `metis verify` meaningful |
| `phase-0-ws-0.2` | Core interfaces + ADRs | Makes `metis interfaces` meaningful |
| `phase-0-ws-0.3` | Domain model | Gives later slices real types to consume |

Only after Phase 0 do feature slices begin. Each completed brief becomes
archaeology for the next slice — the corpus grows itself.

#### Step 5: Execute

Launch agent sessions. Each agent runs `metis kickoff` and the system handles
dispatch, scope enforcement, and review.

---

### Extending: Adding Features to an Existing App

When you already have a running codebase:

#### Step 1: Plan the change

Write a plan for the new work (a feature, a refactoring campaign, a bug
batch). This can be small — even a single phase with a few workstreams:

```markdown
## Phase 1 — Payment Integration

### Workstream 1.1: Payment gateway adapter
- **Risk:** high
- **Coder:** claude-code/opus
- **Reviewer:** opencode/opus
- **Stage:** payments

Tasks:
- Define payment interface
- Implement Stripe adapter
- Add webhook handler

Acceptance criteria:
- Payments process in test mode
- Webhook signature verified
- Error cases handled
```

#### Step 2: Seed (append to existing ledger)

```bash
metis seed plans/payments.md --append
```

Or add individual slices for reactive work:

```bash
# Bug fix (jumps the queue at p0)
metis add fix --title "Race condition in checkout" \
  --coder claude-code/opus --reviewer opencode/opus \
  --priority p0 --risk high

# Refactoring
metis add refactor --title "Consolidate auth middleware" \
  --coder opencode/opus --reviewer claude-code/opus \
  --risk medium

# Tech debt
metis add debt --title "Replace hand-rolled errors" \
  --coder opencode/opus --reviewer claude-code/opus \
  --priority p3 --risk low
```

#### Step 3: Execute

Same as greenfield — agents run `metis kickoff` and the dispatch algorithm
handles priority, ordering, and blocked dependencies automatically.

---

### Ongoing Maintenance

Metis handles the full SDLC, not just feature building:

| Scenario | How |
|---|---|
| Bug report | `metis add fix --priority p0 --risk high ...` (interrupts queue) |
| Tech debt | `metis add debt --priority p3 ...` (runs when nothing urgent) |
| Refactoring | `metis add refactor ...` (broad scope, migration strategy brief) |
| Dependency updates | `metis add chore --risk low ...` |
| Security fix | `metis add security --risk high ...` (always high risk) |
| Code removal | `metis add remove ...` (deletion checklist brief) |
| Phase validation | `metis add gate ...` (composition proof, no code) |
| Plan/ADR drift check | `metis add recon ...` (docs-only, align with reality) |

---

### When Plans Change

- **Completed slices** are immutable (archived, never modified)
- **Pending slices** can be edited: `metis edit <id> --title "..." --risk high`
- **New work** is additive: `metis add` or `metis seed --append`
- **Abandoned work** is skipped: `metis skip <id> --reason "descoped"`
- **Always validate** after changes: `metis check`

The ledger is the single dispatch truth. The plan file is frozen intent — when
reality diverges, update the ledger (not the plan), and file a `recon` slice
if the drift is significant enough to warrant re-aligning documentation.

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
├── metis.yaml              # Configuration (you edit this)
├── .metis/
│   ├── slices.yaml         # Active ledger
│   ├── slices-done.yaml    # Archive
│   ├── briefs/             # Per-slice scope contracts
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
