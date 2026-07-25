# Metis

[![CI](https://github.com/techspeque/metis/actions/workflows/ci.yml/badge.svg)](https://github.com/techspeque/metis/actions/workflows/ci.yml)
[![Coverage](https://raw.githubusercontent.com/techspeque/metis/badges/.badges/main/coverage.svg)](https://raw.githubusercontent.com/techspeque/metis/badges/.badges/main/coverage.svg)
[![Release](https://img.shields.io/github/v/release/techspeque/metis)](https://github.com/techspeque/metis/releases/latest)
[![Go Report Card](https://goreportcard.com/badge/github.com/techspeque/metis)](https://goreportcard.com/report/github.com/techspeque/metis)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

> The meta-harness that orchestrates AI coding agents.

Metis is a Go CLI that replaces ad-hoc agent management with deterministic,
bounded, independently-reviewed units of work — across any technology stack,
any agent surface (Claude Code, opencode, Codex, or others), with the human
retaining ownership of planning, escalations, and releases.

## The Problem

AI coding agents are individually capable but systemically undisciplined.

A single long agent session **drifts**: it loses scope, re-explores code it
already understood, fixes things it was not asked to fix, marks its own
homework, and quietly works around contradictions instead of surfacing them.

At scale — a real project with multiple phases, multiple agents, multiple
review cycles — the failure modes compound:

- **Scope creep:** agents "helpfully" edit files they weren't asked to touch
- **Hallucinated interfaces:** agents invent plausible-but-wrong signatures
- **Self-review:** the agent that wrote the code cannot objectively evaluate it
- **Lost context:** each session re-discovers what prior sessions already established
- **Mechanical errors:** walking YAML lists, comparing slugs, evaluating booleans — agents fumble these reliably
- **No accountability:** who coded what, who reviewed it, was scope respected, did the pieces compose?

The manual workaround is a harness: Python scripts, Makefiles, copy-paste
protocols, markdown ledgers. This works but is fragile — agents forget steps,
skip the protocol, and make mistakes in the mechanical parts.

## The Insight

**Everything an agent fumbles should be deterministic tooling, not agent judgment.**

Agents are good at coding, reading code, pattern recognition, and test writing.
Agents are bad at process: sequencing, scope discipline, self-review, and
mechanical precision.

Metis is the **meta-harness** — a single binary that replaces scattered manual
process with deterministic enforcement:

| Concern | Without Metis | With Metis |
|---|---|---|
| "What should I work on?" | Agent reads a YAML list (fumbles) | `metis next` (deterministic) |
| "What are the rules?" | Agent reads long AGENTS.md (skips sections) | `metis instructions --for <id>` (risk-scaled) |
| "Is the environment broken?" | Agent guesses, "fixes" non-broken code | `metis verify` (distinct exit codes) |
| "Am I the right agent?" | Agent self-assesses (unreliable) | Protocol enforces slug match |
| "Did I stay in scope?" | Agent hopes so | Brief declares scope, reviewer measures |
| "Is this done?" | Agent says "looks good to me" | Cross-vendor review against fixed checklist |
| "What keeps going wrong?" | Nobody tracks | Findings → rules → prevention |

### Why "Meta"

- **Project-agnostic:** works for Go, Rust, Python, TypeScript — any stack with a build command
- **Agent-agnostic:** works with Claude Code, opencode, Codex, or any future surface
- **Portable:** one binary, `metis init` in any repo, agents self-govern via the protocol
- **Self-correcting:** recurring review failures graduate into permanent rules
- **Observable:** progress, findings, review cycles — data replaces vibes

The human retains what humans are good at: planning, architecture, priorities,
judgment calls. Agents do what agents are good at: writing code fast, within
declared bounds, subject to independent review.

## How It Works

```
OVERVIEW.md (you write the spec)
    ↓ agent plans one phase at a time
.metis/plans/ + .metis/adr/
    ↓ metis seed
slices in the ledger
    ↓ metis kickoff (agent self-governs)
brief → implement → verify → flip coded
    ↓ different agent reviews
checklist → pass/block → findings → rules
    ↓ repeat
working software, one bounded slice at a time
```

The human launches agent sessions. Each agent runs `metis kickoff`, gets told
exactly which slice to work on, reads the risk-scaled instructions, and
executes within declared scope. A different agent reviews. Blocking findings
feed back into the rules. The system gets better over time.

## Install

### Install script (Linux / macOS)

```bash
curl -fsSL https://raw.githubusercontent.com/techspeque/metis/main/scripts/install.sh | bash
```

The script detects your OS and architecture, downloads the latest release,
verifies its checksum, installs to `~/.local/bin`, and adds that directory to
your PATH in the right shell profile (bash, zsh, or fish) if it isn't there
already. Overrides:

| Variable | Effect |
|---|---|
| `METIS_VERSION` | Install a specific version (e.g. `v0.0.2`) instead of the latest |
| `METIS_INSTALL_DIR` | Install somewhere other than `~/.local/bin` |
| `METIS_NO_MODIFY_PATH=1` | Never touch shell profile files |

On Windows, download the zip from the
[latest release](https://github.com/techspeque/metis/releases/latest).

### With Go

```bash
go install github.com/techspeque/metis/cmd/metis@latest
```

### From source

```bash
git clone https://github.com/techspeque/metis.git
cd metis
go build -o metis ./cmd/metis
```

## Quick Start

1. **Write your OVERVIEW** — the application spec describing what you're building
2. **Initialize:** `metis init` — scaffolds config, state directory, surface adapters, templates
3. **Configure:** `metis config set` — overview path, agents, commands, routing
4. **Plan a phase:** ask an agent to create a plan from the OVERVIEW (using `.metis/templates/plan.md`)
5. **Seed and execute:** `metis seed .metis/plans/phase-0.md` then launch agent sessions

Each agent session runs `metis kickoff` and the protocol handles everything from there.

See [docs/workflow.md](docs/workflow.md) for the full workflow guide.

## Project Layout

After `metis init`:

```
your-project/
├── OVERVIEW.md             # Application spec (you maintain this)
├── .metis/
│   ├── project.yaml        # Configuration (managed via `metis config`)
│   ├── slices.yaml         # Active ledger
│   ├── slices-done.yaml    # Archive
│   ├── briefs/             # Per-slice scope contracts
│   ├── plans/              # Implementation plans (per phase)
│   ├── adr/                # Architecture Decision Records
│   ├── templates/          # Document templates (for agents)
│   ├── findings.yaml       # Review findings
│   └── runs/               # Verification logs (gitignored)
├── CLAUDE.md               # Surface adapter (generated)
├── AGENTS.md               # Governance + full agent contract (generated)
└── opencode.json           # Surface adapter (generated)
```

### The two configurations

| File | Scope | Committed | How you change it |
|---|---|---|---|
| `.metis/project.yaml` | This project | yes | `metis config set` (agents and humans alike) |
| `~/.metis/config.yaml` | You, across projects | no | `metis workspace …` commands |

The project config is versioned, commented YAML — reviewable in diffs like
any other file — but you never need to open it: `metis config view/get/set`
is the interface, for humans, agents, and tooling alike. The user config is
the per-user workspace registry, so humans working across many projects can
target any registered project from anywhere (`metis workspace use <name>`,
`metis -w <name> status`). Inside a repo, the repo always wins — agents are
unaffected. See [docs/commands.md](docs/commands.md#workspaces).

> Upgrading from ≤ v0.0.4? Projects with a root `metis.yaml` keep working
> with a deprecation warning; `metis init` migrates the file to
> `.metis/project.yaml` in place.

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

## Documentation

| Document | Content |
|---|---|
| [docs/workflow.md](docs/workflow.md) | Full workflow guide — greenfield, extending, OVERVIEW-first, reconciliation |
| [docs/commands.md](docs/commands.md) | Complete command reference with all flags and examples |
| [docs/configuration.md](docs/configuration.md) | `.metis/project.yaml` schema — every field, defaults, examples |
| [docs/protocol.md](docs/protocol.md) | Agent session protocol — kickoff, coder/reviewer flows, resume logic |
| [docs/templates.md](docs/templates.md) | Template system — how agents use the structured document templates |

## License

MIT
