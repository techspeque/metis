# Configuration Reference

> **Audience:** human developers — configuration is human-owned; agents
> only read it (`metis config get`).

> Tip: `metis config view` shows the effective configuration,
> `metis config get/set <dotted.key>` reads and writes individual values
> without hand-editing YAML (comments are preserved). See
> [docs/commands.md](commands.md#metis-config-view).

Complete reference for `.metis/project.yaml` — the single configuration file for a Metis project.

## Minimal Configuration

The smallest valid `.metis/project.yaml`:

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

#### `launch` — how `metis launch` and `metis auto` start a session

```yaml
agents:
  claude-code/opus:
    surface: claude-code
    model: opus
    label: "Claude Code (Opus)"
    launch: ["claude", "-p", "{prompt}", "--model", "{model}", "--permission-mode", "acceptEdits", "--allowedTools", "Bash(metis:*)", "Bash(git:*)"]
  codex/gpt:
    surface: codex
    model: gpt-5
    label: "Codex"
    launch: ["codex", "exec", "--full-auto", "--model", "{model}", "{prompt}"]
  human:
    surface: human              # a person: never launched; metis auto stops before its sessions
    model: human
    label: "Human"
```

`launch` is an argv, not a shell line: `{prompt}`, `{prompt_file}` (the
prompt written to `.metis/runs/<slice>/auto/<n>-<role>.prompt.md`),
`{model}` and `{slice}` are substituted inside each element, and nothing is
quoted or expanded. Unset, the built-in default for the surface applies:

| Surface | Default `launch` |
|---|---|
| `claude-code` | `claude -p {prompt} --model {model} --permission-mode acceptEdits --allowedTools Bash(metis:*) Bash(git:*)` |
| `codex` | `codex exec --full-auto --model {model} {prompt}` |
| `opencode` | `opencode run --model {model} {prompt}` |

Any other surface needs its own `launch`. Sandbox and permission flags are
the project's choice and belong in `launch`. A headless session cannot
answer a permission prompt, and **every session must run `metis` and
`git`** — the flips and blocks commit — so the defaults grant exactly that:
Claude Code accepts edits and pre-approves `metis` and `git`; Codex runs in
its workspace-write sandbox. An agent that may run other commands (a test
runner, a build tool) needs them allowed in its own `launch`. A session
that cannot run `metis` makes no progress and stops the run
(`no_progress`); `metis launch --dry-run` shows the argv.
The `model` fills `{model}`, so two slices on the same surface can run
different models. `metis init` adds the `human` agent and writes the
default `launch` for each agent whose surface it knows.

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

Cross-vendor review (reviewer ≠ coder) is enforced at validation, seed, and
flip time; `metis commit --flip reviewed --agent <slug>` checks the caller's
stated identity against the slice's coder. Set `review: self` to opt into
single-agent projects, where coder and reviewer may be the same slug.

---

### `hot_paths`

```yaml
hot_paths:
  - src/auth/
  - src/payments/
  - migrations/
  - src/crypto/
```

Paths where a mistake is expensive. They are included in agent instructions
as mandatory-caution zones, and `metis log --validate` makes any touch
visible to the reviewer. (Automatic risk escalation is planned; today the
risk level comes from the plan/slice declaration.)

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

### `conventions`

```yaml
conventions:
  - Code comments say why, never what; no narrative or history in source
  - Briefs, reports and findings are facts with file:line, no narrative
  - A finding is file:line, the defect, the required change
```

Writing conventions for code comments, briefs, reports, findings and
docs. Included in every agent's instructions at every risk level, and
referenced from the kickoff coder and reviewer steps. Use it to cap the
tokens agents spend on commentary.

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
| `timeout_seconds` | No | Bounds each command (default 600) — a hung command is killed, not waited on forever |
| `verify_cache` | No | Reuse a green verify of the same working-tree content (default true) |
| `verify_scopes` | No | Split verify by the paths each part covers (below) |

Commands are opaque strings — Metis runs them via `sh -c` and captures output.
Technology-agnostic: use whatever your stack needs.

#### Scoped verify

A large repository rarely changes everywhere at once. `verify_scopes`
declares which command answers for which paths:

```yaml
commands:
  verify: make verify                      # still required: the whole
  verify_scopes:
    - name: api
      paths: [api/, "**/openapi.json"]
      command: make verify-api
    - name: web
      paths: [web/, api/openapi.json]      # a path may belong to several scopes
      command: make verify-web
    - name: docs
      paths: ["**/*.md", docs/]
      command: make docs-check
```

Paths are repository-relative: a directory (with or without a trailing
slash), a file, or a glob where `*` matches within one path segment and
`**` across segments. On each run Metis keys every scope by the content of
its files in the working tree and by its command: a scope that already
passed is reused, a scope whose content changed runs its own command, and
a change to a file outside every scope (a root Makefile, `go.work`)
runs the whole `verify`, which then counts as green for every scope.
`metis verify --force` always runs the whole; `--scope <name>` runs named
scopes regardless. The slice's log says which scopes ran and which were
reused, and a reviewer reads it the same way: green means green for what
the tree touched.

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

### `review`

```yaml
review:
  narrow_after: 1           # from this many blocked reviews on, a review is narrowed; 0 disables
  block_reasons:            # the only grounds for a blocking finding
    - a deviation from the brief
    - a missing or failing test the brief lists
    - a broken accuracy rule
    - a demonstrated defect
```

Every review handoff prompt and the generated instructions state both. A
finding outside `block_reasons` is recorded as an advisory
(`metis findings record`). A narrowed review confirms or re-raises the open
findings and may open a new blocking one only at P1; other new
observations are advisories.

---

### `auto`

Configures [`metis auto`](commands.md#metis-auto). Every key is optional;
a project with launchable agents and no `auto:` section gets one slice per
run.

```yaml
auto:
  mode: step                # manual | step | auto (metis init asks)
  phases: {}                # e.g. { phase-9: auto, phase-10: manual } — metis mode --phase writes these
  by_risk: {}               # e.g. { high: step }
  until: gate               # gate | phase | empty | slice:<id>
  session_timeout: 2h       # per session; the process group is terminated after it
  branch: slice             # slice | pr | off
  pr_command: [gh, pr, create, --fill]   # branch: pr
  notify: []                # argv run on a stop: {slice} {rule} {detail} {escalation}
  stop:
    gate: true
    planning: true
    max_review_cycles: 3    # a slice may set its own (Auto: max_review_cycles=4, metis edit --max-review-cycles)
    repeat_finding: true
    growing_findings: true
    p1: true
    needs_human: true
    verify_env: true
    scope: true
    dirty_tree: true
    session_failed: 1       # retries of a failed (or idle) session before the stop
    budget: { wall_clock: 6h, sessions: 8 }   # per slice, between stops
```

Mode precedence: slice, phase (`auto.phases`, then a `> Auto: <mode>` line
in the plan file's header), risk (`auto.by_risk`), project (`auto.mode`),
then `step` — an upgrade never makes a project unattended. A phase name is
what `metis progress phase` shows: the plan file's name, or the id's
`phase-N` prefix. `metis config set auto.mode auto` changes the project
mode; `metis mode` the phase and slice ones.

A plan's workstream may carry an `Auto:` line, read at seeding:

```markdown
## Workstream 9.4: Rotate keys

- **Risk:** high
- **Auto:** manual, max_review_cycles=4
```

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
- `auto.mode`, `auto.phases.*` and `auto.by_risk.*` are modes; `auto.by_risk` keys are risks; `auto.branch` and `auto.until` are known values; durations parse; counts are not negative; a `human` agent has no `launch`

`metis check` (with the ledger) also rejects an `auto.phases` key that
matches no plan or slice phase, and a slice whose `auto_mode` is not a mode.

---

## User Configuration (`~/.metis/config.yaml`)

Separate from the per-project `.metis/project.yaml`, Metis keeps a small per-user file
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
| `workspaces` | map | Workspace name → absolute path of a directory containing `.metis/project.yaml`. |

Notes:

- The filename (`config.yaml`) is deliberately distinct from the project
  config (`project.yaml`): `~/.metis/` is on the upward-discovery path for
  repos under `~`, and project resolution must never mistake the user config
  for a project.
- A missing file means an empty registry — nothing is created until you (or
  `metis init`) write to it.
- Inside a repo, cwd discovery always beats the `active` selection. Agents
  run as the same OS user as you and share this file; switching workspaces
  must never redirect an agent session running in another repo.
- `METIS_USER_CONFIG` overrides the file location (mainly for tests).
