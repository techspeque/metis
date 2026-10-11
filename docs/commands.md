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
`instructions`, `workspace list`, `workspace current`, `handoff`, `launch`,
`auto`, `mode`.

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
reference, review cycles count, and reading rule. `-o json` adds the
handoff `template` that applies, the slice's effective `auto_mode` and
`auto_mode_source`, and `auto`: `ok`, or the [`metis auto`](#metis-auto)
stop rule that would fire before the slice's next session.

A slice parked with `metis wait --human` is listed as **Needs you** by
`next` and `status`.

---

## Auto: Handoff, Launch and the Loop

Three layers, each usable alone. Planning and gates stay manual.

| Command | Does |
|---|---|
| `metis handoff [<id>]` | prints the prompt for the slice's next session |
| `metis launch [<id>]` | runs that session: the role's agent, headless, with that prompt |
| `metis auto` | loops: dispatch, stop rules, launch, check progress, until a slice needs a person |
| `metis mode` | shows or sets how far `metis auto` takes a slice or a phase |

### `metis handoff [<slice-id>]`

Prints the prompt for the active slice's next role (or the named slice's),
chosen from ledger state:

| State | Template | Content |
|---|---|---|
| not coded, review_cycles 0 | `code` | kickoff, the brief or the instruction to write it, scope, verify, flip |
| not coded, review_cycles ≥ 1 | `rework` | the slice's open findings verbatim; fix only these; one commit per finding; flip |
| coded, review_cycles 0 | `review` | this slice only; the brief governs; read everything first; every issue in one review, one `metis block` per finding; the block reasons |
| coded, review_cycles ≥ 1 | `re-review` | the last review's findings and the commits since; confirm or re-raise each; new blocking findings only at P1 (narrowed) |
| gate, reviewer | `gate-review` | the review contract plus the brief's evidence index; run nothing unless the brief says so |

```bash
metis handoff                       # paste into the agent the prompt names
metis handoff feat-0012 --template review
metis handoff -o json               # {slice, role, agent_slug, template, prompt}
```

Every prompt states the slug the session runs as, so `metis kickoff`'s
self-identification matches. A brief's `## Review` section replaces the
project checklist in a review prompt; `review.block_reasons` and
`review.narrow_after` ([configuration](configuration.md#review)) are stated
in it. Templates are overridable: copy one to
`.metis/templates/handoff/<name>.md.tmpl` (Go `text/template`; the shared
definitions live in `_header.md.tmpl`).

### `metis launch [<slice-id>] [--dry-run]`

Renders the handoff prompt, starts the role's agent headless with it in
the project root, streams its output to
`.metis/runs/<slice>/auto/<n>-<role>.log` (the prompt beside it as
`<n>-<role>.prompt.md`), and exits with the agent's exit code.

```bash
metis launch --dry-run              # the argv, the prompt elided
metis launch feat-0012
```

The agent is the slice's own `coder` or `reviewer`, else the routing for
its risk. Its argv is `agents.<slug>.launch`, else the built-in default for
its surface ([configuration](configuration.md#agents)). Metis never
substitutes an agent: an unconfigured slug, a `surface: human` agent, or a
surface with no launcher is refused before anything starts. A session ends
at `auto.session_timeout` (default 2h); its whole process group is then
terminated.

A reviewer must be able to commit — `metis block` and the flips commit —
so the agent's sandbox and permission flags in `launch` must allow running
`metis` and `git`. They are the project's choice; metis passes none.

### `metis auto`

```bash
metis auto                                   # until a stop rule or auto.until
metis auto --until phase                     # stop when the phase changes
metis auto --until slice:phase-9-ws-9.2
metis auto --max-slices 3
metis auto --dry-run                         # the next session, launched by nobody
metis auto --step                            # this run: one slice
metis auto --phase phase-10 --mode auto      # this run: phase 10 unattended, stop at its end
metis auto --slice phase-10-ws-10.2 --mode auto
metis auto --status                          # the last run
```

One iteration: dispatch the next slice (a waiting slice is skipped);
check the stop rules; launch the role's agent; read the ledger again.
Progress is a flip, a block, a new wait, a new finding or a new commit
naming the slice; a session without progress, or one that fails (exit
non-zero or timeout), is retried once and then stops the run. A reviewed
slice is archived (`metis archive`) and the run goes on.

**Modes.** How far a run goes is the slice's effective mode — `manual`
(the loop stops before it), `step` (one slice to reviewed and archived,
then stop), `auto` (continue). The narrowest setting wins:

| Precedence | Level | Persistent | One run only |
|---|---|---|---|
| 1 | slice | `metis mode <id> <mode>`, or `Auto: <mode>` in the plan section (read at seeding) | `--slice <id> --mode <mode>` |
| 2 | phase | `metis mode --phase <phase> <mode>` (`auto.phases`), or `> Auto: <mode>` in the plan file's header | `--phase <phase> --mode <mode>` |
| 3 | risk | `auto.by_risk` | — |
| 4 | project | `auto.mode` | `--mode <mode>`, `--step` |
| 5 | default | `step` | — |

A scoped one-run override (`--slice`, `--phase`) wins in either direction
and also limits the run to that slice or phase. The unscoped `--mode` and
`--step` replace only the project level, so they never carry a run into a
phase or slice set to `manual`. Moving from an `auto` slice into a `step`
or `manual` one ends the run.

**Stop rules.** Each is a key under `auto.stop`
([configuration](configuration.md#auto)):

| Rule | Fires when | Parks |
|---|---|---|
| `manual` | the next slice's mode is manual (exit 0) | no |
| `step_done` | a step slice reached reviewed, or the next slice is step after another finished (exit 0) | no |
| `gate` | the next slice is a gate: the person runs gates | no |
| `planning` | the slice has no plan section, or is a recon | no |
| `max_review_cycles` | the slice has had as many blocked reviews as the cap (default 3; `Auto: max_review_cycles=4` in the plan or `metis edit --max-review-cycles`) | yes |
| `repeat_finding` | a new blocking finding re-raises one from an earlier review: it cites its id, or names the same file and reads alike | yes |
| `growing_findings` | review N opens more blocking findings than review N−1 | yes |
| `p1` | a P1 finding is recorded | yes |
| `needs_human` | an agent ran `metis wait <id> --reason … --human`, or the brief's `## Human decisions` has an unchecked `- [ ]` item | yes |
| `verify_env` | the env check fails before a session, or `metis verify` exits 2 during one | yes |
| `scope` | `metis log <id> --validate` fails after a coder flipped coded | yes |
| `dirty_tree` | uncommitted files outside the slice's scope before a session | no |
| `session_failed` | a session exits non-zero or times out, after the retries (`auto.stop.session_failed`, default 1) | yes |
| `no_progress` | a session made no progress, after the retries | yes |
| `budget` | the slice used 8 sessions or 6h of sessions since its last stop | yes |
| `human_agent` | the role's agent has `surface: human` | no |
| `no_launcher` | the role's agent is not configured or has no launcher | no |

With `auto.until: gate` (the default) and the gate rule on, a run normally
ends at the phase gate with `gate`.

**Escalation.** A stop that needs a person exits 3. It parks the slice
(`metis wait <id> --reason "auto: <rule>" --human`) where the table says
so, writes `.metis/runs/<slice>/auto/escalation.md` — the rule, the slice
state, the findings by review with what repeats what, the last session's
final output, and the ways out as commands (resume with `metis wait <id>
--clear` then `metis auto`, resolve findings by decision, raise the cycle
cap, or the human review override: `metis edit <id> --reviewer human`,
`metis commit --flip reviewed --agent human --slice <id>`, `metis
archive`) — prints it, and runs `auto.notify`. `step_done`, `manual`, a
reached `--until`, `--max-slices` and an empty backlog exit 0.

Each session is one line of `.metis/runs/auto.jsonl` (time, slice, role,
agent and model, template, duration, exit code, outcome, findings opened
and resolved); `metis auto --status` summarises the last run.

**Branching** (`auto.branch`): with `slice` (default) a slice's sessions
work on `auto/<slice-id>`, cut from the work branch; once reviewed it is
rebased if the work branch moved, verified, fast-forward merged and
deleted. A stopped slice's branch is left checked out: `metis auto` there
resumes it. `pr` pushes the branch and opens a pull request
(`auto.pr_command`, default `gh pr create --fill`) instead of merging;
`off` commits on the work branch. `metis commit` accepts a slice's own
auto branch.

Interrupting `metis auto` (Ctrl-C) terminates the running session and
leaves the repository as the ledger says; run it again to resume.

### `metis mode [<slice-id>] [manual|step|auto]`

```bash
metis mode                              # every open slice's effective mode, by phase, and its source
metis mode --phase phase-9 auto         # phase 9 unattended from now on
metis mode phase-9-ws-9.4 manual        # except this slice
metis mode phase-9-ws-9.4 --clear       # back to the phase's mode
metis mode --phase phase-9 --clear
```

A slice's mode is stored in the ledger, a phase's in `auto.phases`; both
writes are committed. A phase is what `metis progress phase` shows: the
plan file's name, or the id's `phase-N` prefix. `metis list` shows each
slice's mode and `metis progress phase` each phase's.

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
`--plan`, `--plan-section`, `--blocked-by` (replaces the list), `--notes`,
`--max-review-cycles` (metis auto's cap for this slice; 0 = the project's).

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

A review with several blocking findings runs `metis block` once per
finding: on a slice already blocked in this cycle, each further call
records another open finding without starting a new cycle, so the ledger
carries the whole verdict. A bare second call is refused.

Each finding records the review that raised it (`cycle`) and the file it
is about (`file`: `--file`, else the first `file:line` in its text) —
`metis handoff` lists the last review's findings by them, and `metis auto`
detects a re-raised finding with them.

### `metis wait <id> --reason "..." [--until <RFC 3339> | --for <duration>]`

Park a slice on something outside the repository: a lab run that must
cross the hour, an approval, a deploy. Dispatch passes a waiting slice
over, `metis next` and `metis status` list it with its reason, and the
elapsed time reads as a wait rather than a stalled agent.

```bash
metis wait feat-0001 --reason "lab run; the detector sweeps after the hour" --for 90m
metis wait feat-0001 --clear      # back to dispatch; a flip clears it too
```

Without `--until` or `--for` the wait holds until `--clear`. While a slice
waits, `metis commit --slice <id>` and `metis verify --slice <id>` still
bind to it: it is its agent's slice, only parked.

`--human` marks a wait only a person clears: an agent that needs a
decision runs `metis wait <id> --reason "<the question>" --human` and
stops. `next` and `status` show it as **Needs you**, and `metis auto` stops
on it (`needs_human`).

### `metis findings record <slice-id>` / `metis findings resolve <finding-id>`

`record` files an advisory (non-blocking) observation — unlike `block`, it
never touches the slice lifecycle. `resolve` closes a finding after its fix
is verified, keeping the findings ledger a complete audit trail instead of
accumulating stale "open" entries.

```bash
metis findings record feat-0012 --severity P3 --category maint \
  --finding "footer landmark trade-off forced by plan — revisit in phase 1"
metis findings resolve f-002 --note "fixed in 4e16005"
```

### `metis skip <id> --reason "..."`

Mark a slice as done without implementation. The slice stays in the plan's
history as deliberately-not-implemented work; for a slice the plan no longer
needs at all, use `metis remove`.

### `metis remove <id> --reason "..."`

Retire a slice the plan no longer needs: the entry moves to the archive
marked `removed: true` with the reason recorded, and dependents' `blocked_by`
lists drop the retired ID. Nothing is erased — the audit trail keeps the
slice and why it left. Done slices are refused (completed work is archived,
not removed). Removed slices count toward nothing in `metis progress`.

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
metis verify --force   # run the whole command even if this tree passed
metis verify --scope web --scope api   # run named scopes (commands.verify_scopes)
```

A green run is remembered by the working tree's content and reused on the
same content. With `commands.verify_scopes` ([configuration](configuration.md#scoped-verify)),
only the scopes whose files changed run, and a change outside every scope
runs the whole command.

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

Also walks ADR citations in reverse: when an ADR replaces or modifies
another (via `supersedes:`/`amends:` frontmatter — trailing qualifiers OK —
or a `superseded`/`deprecated` status), any living document still citing the
old decision is flagged with a warning: plans, ADRs, the overview, and
active slices' briefs (archived briefs are frozen history and stay silent).
Prose quoting an amended rule passes every other check and is otherwise only
caught by a reviewer reading. Advisory, never a hard error.

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
- Current branch must be `project.integration_branch`, or `auto/<slice-id>` for the slice being committed (the branch `metis auto` cuts)
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

### `metis progress [stats|stage|phase]`

Terminal dashboard with progress bars. Every view opens with the overall
completion line; the view picks the breakdown under it:

```bash
metis progress          # by stage (default)
metis progress stage    # completion per stage, in the order the plans reached them
metis progress phase    # completion per phase, with each phase's stages
metis progress stats    # done / reviewing / rework / pending counts (alias: summary)
```

The phase view is a table: phase, bar with done/total counts, and the
phase's stages in plan order, wrapped to fit the terminal width.

With `-o json` every view emits the same full dashboard.

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

### `metis rule remove [rule-number...]`

Remove accuracy rules. With no arguments on an interactive terminal, lists
the current rules as a multi-select (↑/↓ move, space selects, `a` toggles
all, enter confirms, `q`/esc cancels). Otherwise name the rules by the
numbers `metis rule list` shows:

```bash
metis rule remove        # pick from the list
metis rule remove 2 5    # remove rules #2 and #5
```

Findings promoted to a removed rule stay `promoted` but lose their
`promoted_to` pointer; findings promoted to later rules are renumbered.

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
metis init --auto-mode step   # non-interactive answer to the metis auto question
metis init --from .metis/project.yaml  # non-interactive (reads existing config, scaffolds state)
```

At a terminal, init asks how `metis auto` should run — slice by slice
(`step`, the default), unattended up to each gate (`auto`), or not at all
(`manual`) — and writes `auto.mode`. A new `project.yaml` gets a `human`
agent (for the human review override) and commented `auto.phases` /
`auto.by_risk` examples. On an existing one, init adds only what is
missing: `auto.mode` when given, the `human` agent, and a `launch` for
each agent whose surface metis knows.

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
