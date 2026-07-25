# Configuration Reference

> Tip: `metis config view` shows the effective configuration,
> `metis config get/set <dotted.key>` reads and writes individual values
> without hand-editing YAML (comments are preserved). See
> [docs/commands.md](commands.md#metis-config-view).

Complete reference for `metis.yaml` — the single configuration file for a Metis project.

## Minimal Configuration

The smallest valid `metis.yaml`:

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

Everything else uses defaults.

---

## Full Schema

### `version`

```yaml
version: 1
```

Required. Must be `1`. Used for future schema migrations.

---

### `project`

```yaml
project:
  name: my-project              # Required. Project name used in generated contracts.
  overview: OVERVIEW.md         # Optional. Path to the application spec (relative to repo root).
  integration_branch: dev       # Default: "dev". Where all slice work lands.
  release_branch: main          # Default: "main". Human-owned; agents never touch this.
  technology:                   # Optional. Informational only — helps agents understand context.
    language: go
    build_system: go
    test_runner: "go test ./..."
    linter: "go vet"
```

| Field | Required | Default | Purpose |
|---|---|---|---|
| `name` | Yes | — | Project name in generated contracts |
| `overview` | No | — | Path to OVERVIEW.md (enables drift detection) |
| `integration_branch` | No | `dev` | Branch where slice work lands |
| `release_branch` | No | `main` | Branch human merges to for releases |
| `technology.*` | No | — | Informational context for agents |

---

### `agents`

```yaml
agents:
  claude-code/opus:
    surface: claude-code        # Which tool runs this agent
    model: opus                 # Which model variant
    label: "Claude Code (Opus)" # Human-readable label for reports
  opencode/opus:
    surface: opencode
    model: opus
    label: "opencode (Opus)"
  codex:
    surface: codex
    model: codex
    label: "Codex"
```

Each key is a **slug** — the identifier agents self-report against during the
session protocol. The slug appears in `metis next` output and in ledger
`coder`/`reviewer` fields.

At least one agent must be defined.

---

### `routing`

```yaml
routing:
  high: [claude-code/opus, opencode/opus]   # Agents for high-risk slices
  medium: [codex, opencode/opus]            # Agents for medium-risk slices
  low: [codex]                              # Agents for low-risk slices
  review: cross-vendor                      # Review policy
```

All slugs in routing lists must exist in the `agents` map.

`review: cross-vendor` means the reviewer must be a different agent than the
coder. This is enforced during ledger validation.

---

### `hot_paths`

```yaml
hot_paths:
  - src/auth/
  - src/payments/
  - migrations/
  - src/crypto/
```

Paths where a mistake is expensive. Any slice touching these paths is
auto-escalated to `risk: high` and gets full-depth reading in the instructions.

---

### `accuracy_rules`

```yaml
accuracy_rules:
  - Do not hallucinate interfaces — read real source before consuming or implementing them
  - Hard gates override weighted scores — never average away a hard-gate failure
  - SQLite is the storage backend — do not introduce PostgreSQL dependencies
```

Project invariants that must never be violated. Seeded from architecture
decisions; grown from recurring review findings via `metis rule promote`.

Included in agent instructions at medium and high risk levels.

---

### `non_goals`

```yaml
non_goals:
  - Frontend or web UI
  - gRPC (REST JSON API only)
  - Kubernetes deployment manifests
  - Multi-tenant SaaS features
```

Explicit things agents must not build, even if asked indirectly.

---

### `testing`

```yaml
testing:
  - Mock at trust boundaries only; never mock the thing under test
  - Integration tests hit real dependencies where cheap (in-memory DBs, temp files)
  - A slice without relevant tests is incomplete
  - Test files live alongside the code they test
```

Testing rules included in every agent's instructions.

---

### `review_checklist`

```yaml
review_checklist:
  - Behavioral correctness
  - Security and authorization correctness
  - Scope discipline (diff vs. the committed brief)
  - Test sufficiency
  - Architectural fit — no duplicated/hallucinated interfaces; matches ADRs
  - Maintainability
```

Walked in order by the Reviewer. One-line verdict per item with `file:line` evidence.

---

### `commands`

```yaml
commands:
  verify: "go build ./... && go vet ./... && go test ./..."  # Required
  env_check: "go version"                                     # Optional
  interfaces: "go doc ./..."                                  # Optional
```

| Field | Required | Purpose |
|---|---|---|
| `verify` | Yes | The verification gate — build, lint, test |
| `env_check` | No | Environment soundness check (exit ≠ 0 means env is broken) |
| `interfaces` | No | Regenerate API summary for anti-hallucination archaeology |

Commands are opaque strings — Metis runs them via `sh -c` and captures output.
Technology-agnostic: use whatever your stack needs.

---

### `commits`

```yaml
commits:
  prefixes: [feat, fix, refactor, docs, test, chore]  # Allowed prefixes
  require_slice_id: true                               # Slice ID in every commit
  no_attribution: true                                 # Strip AI attribution
  format: "{prefix}({slice_id}): {message}"           # Commit message format
```

| Field | Default | Purpose |
|---|---|---|
| `prefixes` | `[feat, fix, refactor, docs, test, chore]` | Conventional Commits prefixes |
| `require_slice_id` | `true` | Slice ID must appear in subject |
| `no_attribution` | `true` | Strip Co-Authored-By, Generated with, model names |
| `format` | `{prefix}({slice_id}): {message}` | Template for commit subjects |

---

### `paths`

```yaml
paths:
  ledger: .metis/slices.yaml              # Active ledger
  archive: .metis/slices-done.yaml        # Completed slices
  briefs: .metis/briefs/                  # Per-slice scope contracts
  plans: .metis/plans/                    # Implementation plan files
  adr: .metis/adr/                        # Architecture Decision Records
  findings: .metis/findings.yaml          # Review findings
  runs: .metis/runs/                      # Verification logs (gitignored)
  interfaces: .metis/interfaces.txt       # Generated API summary
```

All paths are relative to the repo root. These are the defaults — override
only if you have a reason to.

---

## Defaults Applied

When a field is omitted, these defaults are used:

```yaml
project:
  integration_branch: dev
  release_branch: main

commits:
  prefixes: [feat, fix, refactor, docs, test, chore]
  require_slice_id: true
  no_attribution: true
  format: "{prefix}({slice_id}): {message}"

paths:
  ledger: .metis/slices.yaml
  archive: .metis/slices-done.yaml
  briefs: .metis/briefs/
  plans: .metis/plans/
  adr: .metis/adr/
  findings: .metis/findings.yaml
  runs: .metis/runs/
  interfaces: .metis/interfaces.txt
```

---

## Validation

Run `metis check --config` to validate:

- `version` is 1
- `project.name` is set
- At least one agent defined
- All agent entries have `surface`, `model`, `label`
- All slugs in `routing.*` lists exist in `agents` map
- `commands.verify` is non-empty
- `commits.prefixes` has at least one entry
- `commits.format` is non-empty
- `paths.ledger` is set

---

## User Configuration (`~/.metis/config.yaml`)

Separate from the per-project `metis.yaml`, Metis keeps a small per-user file
at `~/.metis/config.yaml` holding the workspace registry for the human
persona. It is managed by the `metis workspace` commands and auto-populated
by `metis init` — you rarely edit it by hand.

```yaml
# ~/.metis/config.yaml — user-level configuration for Metis
active: metis
workspaces:
  acme-api: /Users/you/proj/acme-api
  metis: /Users/you/proj/metis
```

| Field | Type | Meaning |
|---|---|---|
| `active` | string | Name of the active workspace — the fallback project for commands run outside any repo. Must be a key of `workspaces`, or empty. |
| `workspaces` | map | Workspace name → absolute path of a directory containing `metis.yaml`. |

Notes:

- The file is deliberately **not** named `metis.yaml`: home is on the
  upward-discovery path for repos under `~`, and project resolution must
  never mistake the user config for a project.
- A missing file means an empty registry — nothing is created until you (or
  `metis init`) write to it.
- Inside a repo, cwd discovery always beats the `active` selection. Agents
  run as the same OS user as you and share this file; switching workspaces
  must never redirect an agent session running in another repo.
- `METIS_USER_CONFIG` overrides the file location (mainly for tests).
