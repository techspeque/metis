# Command Guide

> **Audience:** both personas. Task-oriented guidance with examples. For
> the exhaustive command/flag/audience tables, see [cli.md](cli.md); for
> the agent protocol these commands serve, see [protocol.md](protocol.md).

## Output Format

Every read command supports structured output via the global `--output`
(`-o`) flag or the `METIS_OUTPUT` env var:

```bash
metis status --output json
metis next -o json
METIS_OUTPUT=json metis list
```

Resolution order: `--output` flag → `METIS_OUTPUT` → plain text. The format
is deliberately **not** configurable in `.metis/project.yaml`: output shape is a
property of the consumer (human, agent, or tool), not the project — several
consumers read the same repo at the same time.

JSON is emitted on stdout; exit codes are unchanged. Commands with JSON
support: `status`, `next`, `list`, `show`, `progress`, `findings`, `check`,
`version`, `config view`, `config get`, `rule list`, `brief`, `kickoff`,
`instructions`, `workspace list`, `workspace current`.

**For programmatic consumers (agents, editors, scripts): always pass
`--output json` explicitly and parse fields from it — never parse the
human-readable text, which may change between versions.**

---

## Dispatch

| Command | Purpose |
|---|---|
| `metis next` | Find the active slice, role, and required agent |
| `metis next -o json` | Same, as structured JSON (`--json` is a deprecated alias) |
| `metis next --quiet` | Print only the slice ID (for scripting) |
| `metis status` | One-line summary: `slice-id | Role | agent | progress` |

### The Dispatch Algorithm

`metis next` finds the active slice using a deterministic algorithm:

1. Filter to unblocked slices (no incomplete `blocked_by` dependencies)
2. Sort by priority (p0 > p1 > p2 > p3)
3. Within same priority, declaration order wins
4. First slice with `coded: false` → role is **Coder**
5. If `coded: true` but `reviewed: false` → role is **Reviewer**
6. If both true → skip (should be archived)

Returns the slice ID, title, type, risk, role, required agent slug, plan
reference, review cycles count, and reading rule.

---

## Work Management

### `metis add <type>`

Add a new slice to the ledger.

**Types:** `feat`, `fix`, `refactor`, `debt`, `remove`, `chore`, `security`, `gate`, `recon`

**Required flags:**
- `--title "..."` — human-readable title
- `--coder <slug>` — agent slug for coding
- `--reviewer <slug>` — agent slug for review

**Optional flags:**
- `--risk <low|medium|high>` — defaults to `medium`
- `--priority <p0|p1|p2|p3>` — defaults to `p2`
- `--stage <string>` — project taxonomy label
- `--plan <path>` — source plan file
- `--plan-section <string>` — required when `--plan` is set
- `--blocked-by <id>,<id>` — comma-separated dependencies
- `--id <slug>` — explicit ID (auto-generated as `type-NNNN` if omitted)
- `--notes "..."` — clarification text
- `--after <id>` — insert after this slice
- `--before <id>` — insert before this slice

**Examples:**

```bash
metis add feat --title "Add webhook handler" \
  --coder opencode/opus --reviewer claude-code/opus \
  --risk medium --plan .metis/plans/phase-1.md --plan-section "§1.3"

metis add fix --title "Auth bypass in token validation" \
  --coder claude-code/opus --reviewer opencode/opus \
  --priority p0 --risk high

metis add refactor --title "Consolidate auth middleware" \
  --coder opencode/opus --reviewer claude-code/opus --risk high

metis add gate --title "Phase 2 composition validation" \
  --coder claude-code/opus --reviewer opencode/opus --id phase-2-gate
```

### `metis edit <id>`

Edit fields of an existing slice. Only the flags you pass change; everything
else is untouched. Used during reconciliation to update slices affected by
OVERVIEW changes.

```bash
metis edit feat-0001 --title "Sharper title" --risk high
metis edit feat-0002 --priority p1 --blocked-by feat-0001
metis edit feat-0003 --coder opencode/opus --reviewer claude-code/opus
```

Flags: `--title`, `--risk`, `--priority`, `--stage`, `--coder`, `--reviewer`,
`--plan`, `--plan-section`, `--blocked-by` (replaces the list), `--notes`.

Guard rails: done slices are immutable (`metis reopen` first), `blocked-by`
entries must exist and cannot self-reference, coder and reviewer must differ,
a plan requires a plan section.

### `metis list`

List slices with optional filters.

```bash
metis list                          # all slices
metis list --type feat              # only features
metis list --priority p0            # only urgent
metis list --status pending         # only pending
```

### `metis show <id>`

Full details of a single slice including status, coder/reviewer, plan reference,
review cycles, notes, and creation date.

### `metis seed <plan-file>`

Parse a structured plan file and generate ledger entries.

```bash
metis seed .metis/plans/phase-1.md --dry-run    # preview without writing
metis seed .metis/plans/phase-1.md              # create slices
metis seed .metis/plans/phase-2.md --append     # add to existing ledger
metis seed .metis/plans/phase-1.md --phase 1    # only phase 1 workstreams
metis seed .metis/plans/phase-1.md --type feat  # override type (default: feat)
```

---

## Slice Lifecycle

### Flipping lifecycle flags

Lifecycle flags are flipped through `metis commit --flip coded` /
`metis commit --flip reviewed` (see [Git Enforcement](#metis-commit)) — the
flip and the ledger commit are one atomic step, so the ledger can never be
left dirty between them. For `reviewed`, the dispatching agent is validated
against the slice's coder (cross-vendor review).

### `metis block <id>`

Block a slice during review. Resets `coded=false`, increments `review_cycles`,
and records the finding.

```bash
metis block feat-0001 \
  --severity P1 \
  --category arch-dup \
  --finding "Duplicated Validator trait at src/eval/mod.rs:42"
```

**Severities:** P1 (breaks guarantee), P2 (wrong, contained), P3 (debt)

**Categories:** `auth`, `protocol`, `scope`, `tests`, `arch-dup`, `arch-fit`,
`data`, `maint`, `security`, `behavior`, `performance`

Blocking is atomic: the ledger and findings changes are committed in the
same step (`chore(<id>): block review (cycle N)`), so a block never leaves
the tree dirty between sessions.

### `metis skip <id> --reason "..."`

Mark a slice as done without implementation (effectively removes from queue).

### `metis reopen <id> --reason "..."`

Reset a slice to uncoded/unreviewed for re-implementation.

### `metis archive`

Move all fully-done slices (`coded && reviewed`) to `.metis/slices-done.yaml`.
Atomic: the ledger and archive changes are committed in the same step, so
the protocol ends with a clean tree.

---

## Verification

### `metis verify`

Full verification pipeline:

1. Run the environment soundness check (`commands.env_check`) → fail = exit 2
2. Run configured `commands.verify` → fail = exit 1 (code failure)

```bash
metis verify --pre     # pre-flight (before changes), stored as verify-pre.log
metis verify --post    # post-implementation, stored as verify-post.log
metis verify --env     # environment soundness check only
metis verify           # stored as verify-latest.log
```

**Exit codes:**
- 0: all pass
- 1: verify command failed (code error)
- 2: environment failure (do NOT modify code)

On environment failure (exit 2) it prints:
```
VERDICT: ENVIRONMENT FAILURE — NOT A CODE FAILURE.
Do NOT modify code, tests, or config to make verify pass.
Do NOT flip ledger booleans. Stop and report this verbatim.
```

### `metis interfaces`

Run the configured `commands.interfaces` to regenerate the API summary.
If not configured, prints a skip message and exits 0.

### `metis check`

Validate configuration and ledger integrity.

```bash
metis check             # validate everything
metis check --config    # config only
metis check --ledger    # ledger only
```

Also detects OVERVIEW drift (warns if overview has changed since last seed/recon).

---

## Git Enforcement

### `metis commit`

Git commit wrapper that enforces branch, format, and attribution rules.

```bash
metis commit -m "add webhook handler"           # auto-infers prefix from slice type
metis commit --prefix test -m "add tests"       # explicit prefix
metis commit --brief                            # stage and commit the brief
metis commit --flip coded                       # flip + commit ledger
metis commit --flip reviewed                    # flip + commit ledger
metis commit --amend                            # amend previous commit
```

**Enforces:**
- Current branch must be `project.integration_branch`
- Commit format: `{prefix}({slice_id}): {message}`
- Prefix must be in `commits.prefixes` list
- `--flip coded`: the slice's brief must exist and the last `verify --post` must have passed
- `--flip reviewed`: requires `--agent <your-slug>`, validated against the slice's coder (`routing.review: self` relaxes this for single-agent projects), and the slice must pass `metis log <id> --validate` — a failing audit blocks the sign-off
- `--slice <id>` (any mode): binds the command to the slice you were dispatched — errors if a higher-priority slice arrived since your `metis next`

### `metis log <id>`

Show a slice's commit history (oldest first). With `--validate`, runs the
reviewer's deterministic audit: every subject checked for the slice ID and
an allowed prefix, attribution lines detected, and every touched file
compared against the brief's declared `owned_paths` (metis state files and
the brief itself are always in scope; an unfilled scope declaration reports
NOT VERIFIABLE). Exit 1 on any violation — CI-gatable.

```bash
metis log feat-0012                 # commit history
metis log feat-0012 --validate      # format + scope audit
metis log feat-0012 --validate -o json
```
- Attribution stripped (Co-Authored-By, Generated with, model names)

---

## Instructions

### `metis instructions`

Emit the full dynamic agent contract assembled from `.metis/project.yaml`.

```bash
metis instructions                  # full contract
metis instructions --for feat-0001  # risk-scaled for specific slice
```

### Risk Scaling

When `--for <id>` is used, sections are filtered by the slice's risk:

| Section | Low | Medium | High |
|---|---|---|---|
| Overview, session protocol, branch/commit, DoD, scope, testing, non-goals, tooling | Yes | Yes | Yes |
| Hot-path zones, accuracy rules, review checklist | | Yes | Yes |
| Model routing, feedback loop | | | Yes |

### `metis kickoff`

Emit the session protocol — the step-by-step procedure agents follow.

```bash
metis kickoff               # full protocol (coder + reviewer)
metis kickoff --role coder  # coder flow only
metis kickoff --role reviewer  # reviewer flow only
```

### `metis brief <id>`

Show or generate the brief template for a slice.

```bash
metis brief feat-0001          # print brief (or template if none exists)
metis brief feat-0001 --write  # write template to .metis/briefs/feat-0001.md
```

Brief templates adapt based on slice type (feat, refactor, remove, gate, etc.).

---

## Observability

### `metis progress`

Terminal dashboard showing completion stats with progress bars, by-stage
breakdown, and done/reviewing/rework/pending counts.

### `metis findings`

Show review findings with optional filters.

```bash
metis findings                      # all findings
metis findings --severity P1        # only P1
metis findings --category auth      # only auth category
metis findings --slice feat-0001    # only for specific slice
metis findings --stats              # aggregated statistics
```

### `metis status`

One-line status for quick orientation:

```
feat-0001 | Coder | opencode/opus | 14/42 done (33%)
```

---

## Configuration, Rules & Surface Adapters

### `metis rule add "..."`

Append a new accuracy rule to `.metis/project.yaml`.

### `metis rule list`

Show all accuracy rules (numbered).

### `metis rule promote <finding-id>`

Promote a review finding to a permanent accuracy rule.

### `metis config view`

Show the full effective configuration (defaults applied) as YAML, with a
header naming the source file. With `-o json`, emits the same as JSON.

### `metis config get <key>`

Show one effective value by dotted key path:

```bash
metis config get project.name
metis config get agents.claude-code/opus.model
metis config get commits.prefixes
```

Scalars print raw (script-friendly); lists and sections print as YAML.

### `metis config set <key> <value>`

Set one value in `.metis/project.yaml` by dotted key path. The edit goes through the
YAML node tree, so **comments and unrelated formatting are preserved**.
Unknown keys are rejected with the list of valid keys at that level; values
are type-checked against the schema (bools, ints, strings, string lists).
Lists take comma-separated values; missing intermediate sections are created.

```bash
metis config set commands.verify "go test ./..."
metis config set commits.require_slice_id true
metis config set routing.high claude-code/opus,opencode/opus
metis config set agents.codex.model gpt-5   # creates the agents.codex entry
```

`set` guarantees the result still parses; run `metis check --config` to
validate the full configuration semantically.

### `metis surface generate`

Write/overwrite all surface adapter files (CLAUDE.md, AGENTS.md, opencode.json,
.claude/settings.json) from current config.

### `metis surface validate`

Check adapter files exist and are not stale (config changed since last generate).

---

## Workspaces

Workspaces are a user-level registry (`~/.metis/config.yaml`) for the human
persona working across many projects. Agents are unaffected: inside a repo,
upward discovery of `.metis/project.yaml` always wins and the registry is never
consulted.

Every command resolves its target project in this order:

1. `--workspace <name>` (`-w`) — global flag, explicit always wins
2. `METIS_WORKSPACE` env var — scripting/CI override
3. Upward discovery of `.metis/project.yaml` from the current directory
4. The active workspace from `~/.metis/config.yaml`

When a command resolves via anything other than cwd discovery, it prints a
provenance line to stderr: `workspace: <name> (<path>) [via --workspace]`.

### `metis workspace list`

List registered workspaces. The active one is marked `[active]`; entries
whose path no longer contains a `.metis/project.yaml` are marked `[missing]` (they
are displayed, never auto-pruned — an unmounted volume is not a deleted
project).

### `metis workspace add <name> [path]`

Register a workspace. The path defaults to the repo root discovered from the
current directory and must contain a `.metis/project.yaml`.

### `metis workspace remove <name>`

Remove a registry entry. The project itself is never touched. Removing the
active workspace clears the active selection.

### `metis workspace use <name>`

Set the active workspace — the fallback project for commands run outside any
repo.

### `metis workspace current`

Show the active workspace, or "none".

```bash
metis ws list                 # 'ws' is an alias for 'workspace'
metis -w acme-api status      # operate on a registered project from anywhere
METIS_WORKSPACE=acme-api metis next
```

---

## Initialization

### `metis init`

Scaffold a new Metis project.

```bash
metis init                    # interactive (creates minimal .metis/project.yaml)
metis init --from .metis/project.yaml  # non-interactive (reads existing config, scaffolds state)
```

Creates:
- `.metis/project.yaml` (if not present)
- `.metis/` directory structure (slices, briefs, plans, adr, templates, runs)
- Surface adapters (CLAUDE.md, AGENTS.md, opencode.json, .claude/settings.json)
- Document templates in `.metis/templates/`

Also registers the project in the user-level workspace registry
(`~/.metis/config.yaml`) under the project name, so `metis workspace list`
populates itself through normal use. A name collision with a different
project is skipped silently.

### `metis version`

Show the version, commit, build date, and platform. Equivalent to
`metis --version`, plus the Go runtime and OS/arch — useful in bug reports.

```
$ metis version
metis 0.0.2 (62f1ed9) built 2026-07-24T18:15:01Z
go1.26.0 darwin/arm64
```

### `metis recon`

Create a reconciliation slice when the OVERVIEW has changed.

```bash
metis recon                                    # uses high-risk routing agents
metis recon --coder claude-code/opus --reviewer opencode/opus
metis recon --priority p1                      # default priority
```
