# Metis Auto Plan

> Feature: `metis handoff` and `metis auto` — metis renders the prompt each
> role needs and drives coder and reviewer sessions until a slice needs a
> person.
> Groundwork already shipped: `metis next -o json`, `metis wait`, scoped
> verify, several `metis block` calls per review cycle (v0.0.15).
> Evidence: warrantum phases 3 to 8, about 130 slices, two agents on two
> subscriptions, the human switching terminals and asking the coder to write
> each reviewer prompt.

---

## Problem

Three steps between "coded" and "reviewed" are done by hand today, and all
three are mechanical:

1. Reading `metis next` and switching to the right agent's terminal.
2. Asking the coder to write the reviewer's prompt. The requests are always
   one of two shapes: a first review (this slice only, the brief governs,
   collect every issue, record them in one cycle) and a re-review (the open
   findings only, re-raise in one go).
3. Pasting the prompt and waiting for a flip or a block.

The one part that is not mechanical is knowing when to stop. Unattended
agents fixate: a reviewer reopens decided mechanisms, a coder and reviewer
trade the same finding, a gate review audits tooling line by line. On
warrantum this cost five cycles on one slice and three on several others.
What ended it each time was a rule, not a better model.

## Design laws

1. **Metis never owns the agent's process.** It launches the agent's own
   CLI headless with one prompt and waits for it to exit. Auth, sandboxing
   and tools stay the agent's. (Same law as the VS Code extension plan.)
2. **The ledger is the only state.** Auto reads and writes what a person
   running the same commands would. Killing it at any point leaves a
   repository a person can continue by hand, and `metis auto` resumes from
   the ledger alone.
3. **A stop is a result, not an error.** Every stop names the slice, the
   rule that fired and what the person must decide, and parks the slice
   with `metis wait` so dispatch reads it as waiting.
4. **Rules bound the loop, not model judgment.** Cycle caps, block reasons
   and budgets are configuration that metis enforces.
5. **One slice at a time.** No parallel slices in this plan: verify, the
   working tree and most projects' test environments are single-tenant.

## Scope

The human runs `metis auto` once. It then takes every dispatchable slice
through coding and the review loop, spawning for each session the agent
and model the slice and the project configuration name, until a rule stops
it.

In: prompt rendering for every role and state, a launcher per agent
surface, agent and model resolution per slice, the loop, stop rules, a run
log, escalation output.

Out, and manual by decision: planning (plans, ADRs, seeding, briefs the
planning role pre-drafts) and gates. Also out: parallel slices, merges to
the release branch, any UI. The VS Code extension can render the
run log later through `-o json`.

---

## Phase 1 — `metis handoff` (prompt rendering)

Removes the step named in the problem, with no process control. Useful on
its own: the human pastes the output.

`metis handoff [<slice-id>]` prints the prompt for the active slice's next
role, chosen from ledger state:

| State | Template | Content |
|---|---|---|
| not coded, cycle 0 | `code` | kickoff, the brief or the instruction to write it, scope, verify, flip |
| not coded, cycle ≥ 1 | `rework` | the open findings of this slice, verbatim; fix only these; one commit per finding; flip |
| coded, cycle 0 | `review` | this slice only; the brief is the source of truth; read everything first; record every issue in this one cycle, one `metis block` per finding; block reasons |
| coded, cycle ≥ 1 | `re-review` | the findings of the last cycle and the commits since; confirm or re-raise each; new findings only at P1 |
| gate | `gate-review` | the review template plus the brief's evidence index; run nothing unless the brief says so |

Details:

- Templates live in `internal/templates/handoff/*.md.tmpl`, overridable by
  `.metis/templates/handoff/`. Data: slice, role, agent, brief path, open
  findings, cycle, commits since the last flip, project conventions.
- A brief may carry a `## Review` section; when present it is quoted into
  the review prompt in place of the default checklist.
- `review.block_reasons` in `project.yaml` (default: a deviation from the
  brief, a missing or failing test the brief lists, a broken accuracy
  rule, a demonstrated defect). Anything else is recorded with
  `metis findings record`. The prompt states this.
- `--role`, `--template` to force one; `-o json` returns
  `{slice, role, agent_slug, template, prompt}`.
- Package `internal/handoff` (`internal/prompt` is the multi-select UI).

Tests: one golden file per template against `testdata/` ledgers; the
rework prompt lists exactly the open findings; a brief's Review section
replaces the checklist.

## Phase 2 — Launchers

How metis starts one agent session with one prompt.

```yaml
agents:
  claude-code/opus:
    surface: claude-code
    model: opus
    launch: ["claude", "-p", "{prompt}", "--model", "{model}", "--permission-mode", "acceptEdits"]
  codex/gpt:
    surface: codex
    model: gpt
    launch: ["codex", "exec", "--model", "{model}", "{prompt}"]
```

- `launch` is an argv template; `{prompt}`, `{prompt_file}`, `{model}`,
  `{slice}` are substituted. No shell. Built-in defaults per known surface
  so most projects configure nothing; `surface: human` has no launcher and
  always stops the loop.
- `metis launch [<slice-id>] [--dry-run]` renders the handoff prompt, runs
  the agent in the project root, streams its output to
  `.metis/runs/<slice>/auto/<n>-<role>.log`, and returns its exit code.
  `--dry-run` prints the argv.
- **Agent and model resolution.** The session's agent is the one `metis
  next -o json` names in `agent_slug` for the slice's current role: the
  slice's own `coder` or `reviewer` from the ledger (set at seeding or by
  `metis edit`), else the project's routing for the slice's risk, else
  `routing.review`. Its `model` fills `{model}`, so two slices in one run
  can use different models on the same surface. Auto never substitutes an
  agent: a slug with no entry under `agents`, or an agent with no launcher,
  stops the loop (`no_launcher`) before anything is spawned. The
  cross-vendor review rule is checked as it is today.
- The handoff prompt states the slug the session runs as, so the agent's
  self-identification step in `metis kickoff` matches without guessing.
- Sandbox and permission flags are the project's choice inside `launch`.
  The docs state plainly that a reviewer must be able to commit, since
  `metis block` and the flips commit.
- Timeout per session (`auto.session_timeout`, default 2h); on timeout the
  process group is terminated and the session counts as failed.

Tests: argv rendering; a fake agent script in `testdata/` that flips or
blocks, so launch and loop tests need no real model.

## Phase 3 — `metis auto` (the loop)

```
metis auto [--until gate|phase|slice:<id>|empty] [--max-slices N] [--dry-run]
```

One iteration:

1. `next`: no active slice → stop `backlog_empty`. Waiting slice → skip
   as dispatch does; nothing else dispatchable → stop `all_waiting`.
2. Check the stop rules below against the slice before launching.
3. `launch` the role's agent.
4. Read the ledger again. Progress is a flip, a block, a new wait or new
   commits naming the slice. No progress after a session → one retry, then
   stop `no_progress`.
5. Append one line to `.metis/runs/auto.jsonl`: time, slice, role, agent,
   template, duration, exit code, outcome, findings opened and resolved.
6. On `reviewed`: `metis archive`, continue.

### Modes: slice by slice or unattended

How far one `metis auto` run goes is a mode, set at four levels. The
narrowest setting wins: slice, then phase, then risk, then project.

| Mode | A run does |
|---|---|
| `manual` | nothing to this slice: the loop stops before it (`manual`). `metis handoff` and `metis launch` still work, so the human drives each session |
| `step` | takes one slice from its current state to reviewed and archived, then stops (`step_done`) for the human to look before the next |
| `auto` | continues to the next slice until a stop rule fires |

Every level can be overridden by the one below it, in either direction:
a slice can be `auto` inside a `manual` phase, and `manual` inside an
`auto` one.

| Precedence | Level | Persistent: set with | One run only |
|---|---|---|---|
| 1 (wins) | slice | `metis mode <slice-id> <mode>`; or an `Auto: <mode>` line in the slice's plan section, read at seeding | `metis auto --slice <id> --mode <mode>` |
| 2 | phase | `metis mode --phase <phase> <mode>`; or a `> Auto: <mode>` line in the plan file's header | `metis auto --phase <phase> --mode <mode>` |
| 3 | risk | `auto.by_risk` in `project.yaml`, e.g. `{ high: step }` | - |
| 4 | project | `auto.mode` in `project.yaml` (asked by `metis init`) | `metis auto --mode <mode>`, `--step` |
| 5 | default | `step`: an upgrade never makes an existing project unattended | - |

- `metis mode` is the one command for overrides. `metis mode <slice-id>
  <mode>` stores the mode on the slice in the ledger; `metis mode --phase
  <phase> <mode>` writes `auto.phases.<phase>` in `project.yaml`; `--clear`
  removes an override so the level above applies again. With no mode it
  prints the effective mode of every open slice, by phase, and the level
  each came from. Both writes are commits, like `metis edit`.
- A phase is what `metis progress` derives: the plan file name or the
  id's `phase-N` prefix. A phase override applies to slices seeded later
  as well, since it is resolved at dispatch, not copied.
- One-run overrides follow the same precedence and are not written
  anywhere. A scoped one (`--slice`, `--phase`) overrides in either
  direction, since the human named the target. The unscoped `--mode` and
  `--step` replace only the project level, so they never carry a run into
  a phase or slice someone set to `manual`.
- A one-run `--slice <id>` also limits the run to that slice, and
  `--phase <phase>` to that phase: "run this one slice unattended now" and
  "run phase 9 unattended, stop at its end" are each one command.

Examples:

```sh
metis mode --phase phase-9 auto        # phase 9 unattended from now on
metis mode phase-9-ws-9.4 manual       # except this slice: I drive it
metis mode phase-9-ws-9.4 --clear      # back to the phase's mode
metis auto --phase phase-10 --mode step   # this run only: phase 10, one slice
metis auto --slice phase-10-ws-10.2 --mode auto   # this run only: that slice
```

A mode boundary is a stop like any other: moving from an `auto` phase into
a `step` or `manual` one ends the run with the reason, and exit code 0 for
`step_done` and `manual` (nothing is wrong), 3 for rule stops.

`metis next -o json` reports the effective `auto_mode` and where it came
from (`slice`, `phase`, `risk`, `project`, `default`); `metis list` and
`metis progress` show it per slice and per phase.

### `metis init`

- Interactive init asks one question after the agents: "How should `metis
  auto` run? slice by slice (step) / unattended up to each gate (auto) /
  not at all (manual)", default `step`, and writes `auto.mode`.
- `metis init --auto-mode <mode>` for non-interactive use.
- Init writes a launcher for each agent whose surface metis knows, adds the
  `human` agent, and comments the `auto.phases` and `auto.by_risk` keys
  into the generated `project.yaml` as examples.
- Later changes: `metis config set auto.mode <mode>` for the project,
  `metis mode` for a phase or a slice. `metis check` rejects an unknown
  mode and a phase key that matches no plan.

### Stop rules

| Rule | Default | Fires when |
|---|---|---|
| `manual` | by mode | the next slice's effective mode is `manual` (exit 0) |
| `step_done` | by mode | a `step` slice reached reviewed (exit 0) |
| `gate` | on | the next slice is a gate (the human runs gates) |
| `max_review_cycles` | 3 | a slice would enter review cycle 4 |
| `repeat_finding` | on | a finding is re-raised after the coder claimed its fix (same slice, same `file`, text similarity over a threshold, or the reviewer cites the earlier id) |
| `growing_findings` | on | cycle N opens more blocking findings than cycle N−1 |
| `p1` | on | any P1 finding is recorded |
| `needs_human` | on | an agent runs `metis wait <id> --reason … --human` (new flag), or a brief has an unanswered `## Human decisions` item |
| `verify_env` | on | `metis verify` exits 2 (environment), before or after a session |
| `scope` | on | `metis log <id> --validate` fails after a coder session |
| `dirty_tree` | on | the tree is dirty and the dirty files are outside the slice's scope |
| `session_failed` | 1 retry | the agent exits non-zero or times out twice |
| `budget` | per slice 6h wall clock, 8 sessions | either is exceeded |
| `human_agent` | on | the role's agent has `surface: human` |
| `no_launcher` | on | the role's agent is not configured or has no `launch` |
| `planning` | on | the slice has no plan section, or its type is a planning type (recon, plan) |

Each rule is a key under `auto.stop` in `project.yaml`; a slice may
override `max_review_cycles` in the plan (`Auto: max_review_cycles=4`);
`Auto: manual` keeps a slice out of the loop.

### Narrowing review

The fixation guard that needs no detection: the review templates narrow by
cycle. Cycle 0 is the whole slice. From cycle 1 the reviewer may confirm or
re-raise existing findings and open new ones only at P1; other new
observations are advisories. `review.narrow_after` (default 1) sets the
cycle; 0 disables.

### Escalation

On a stop metis:

- parks the slice: `metis wait <id> --reason "auto: <rule>" --human`;
- writes `.metis/runs/<slice>/auto/escalation.md`: the rule, the slice
  state, each open finding with its history across cycles, the last
  session's final message, and the ways out as ready-to-run commands:
  resume (`metis wait <id> --clear`, then `metis auto`), resolve findings
  by decision (`metis findings resolve <f-id> --note …`), and the human
  review override (`metis edit <id> --reviewer human`, `metis commit
  --flip reviewed --agent human --slice <id>`, `metis archive`). The
  override is offered on every stop inside a review loop; it needs a
  `surface: human` agent, which `metis init` now adds by default;
- prints it, exits 3, and runs `auto.notify` if set (an argv, e.g. a
  desktop notification or a webhook script).

`metis auto --status` shows the last run: slices done, sessions, cycles per
slice, the stop and its reason.

### Branching

`auto.branch` chooses where unattended work lands. The default suits a team:

| Value | Behaviour |
|---|---|
| `slice` (default) | work on `auto/<slice-id>` cut from the work branch; at the reviewed flip, rebased if the work branch moved, verified, then fast-forward merged and deleted; left in place on a stop |
| `pr` | as `slice`, but pushed, and a pull request opened at the reviewed flip (`auto.pr_command`, default `gh pr create --fill`); the merge is a person's or the forge's |
| `off` | commits on the work branch, as a person running the commands would |

Reasons for `slice`: other people's commits on the work branch do not
interleave with a slice in flight, an abandoned run never leaves half a
slice on the shared branch, and a stopped slice is one branch to inspect or
delete. The ledger flips travel with the slice's branch, so the work
branch sees a slice only once it is reviewed. A solo project sets `off`.

Tests: loop tests against the fake agents — clean pass; block then pass;
three cycles stops at the cap; a re-raised finding stops; a gate stops; an
interrupted run resumes; a human-surface reviewer stops; branch mode merges
only on reviewed; `step` stops after one slice; an `auto` phase followed by
a `manual` one stops at the boundary; slice beats phase beats project;
an unscoped `--mode auto` does not enter a `manual` phase while `--phase …
--mode auto` does; an `auto` slice runs inside a `manual` phase; `metis
mode --clear` restores the level above; init writes the chosen mode.

## Phase 4 — Hardening from use

Not designed up front; listed so they are not forgotten.

- Rate-limit detection per surface (exit code or output pattern) → wait
  and retry rather than `session_failed`.
- `metis auto --watch`: a compact live view of the current session's tail.
- Cycle statistics per reviewer and per finding category in
  `metis findings --stats`, to tune `block_reasons` and routing.
- Optional gate support: `auto.stop.gate: off` with a gate handoff
  template, once the stop rules have proved themselves on ordinary slices.

---

## Configuration summary

```yaml
auto:
  mode: step            # manual | step | auto
  phases: {}            # e.g. { phase-9: auto, phase-10: manual }
  by_risk: {}           # e.g. { high: step }
  until: gate
  session_timeout: 2h
  branch: slice
  notify: []
  stop:
    max_review_cycles: 3
    repeat_finding: true
    growing_findings: true
    p1: true
    budget: { wall_clock: 6h, sessions: 8 }
review:
  narrow_after: 1
  block_reasons:
    - a deviation from the brief
    - a missing or failing test the brief lists
    - a broken accuracy rule
    - a demonstrated defect
```

Every key has the default shown; a project with launchable agents can run
`metis auto` with no `auto:` section and gets one slice per run.

## Changes to existing commands

- `metis wait`: `--human` marks a wait only a person clears; `next` and
  `status` show it as "needs you".
- `metis mode`: new, the override command above.
- `metis next -o json`: adds `template` (the handoff template that
  applies) and `auto` (`ok` or the stop rule that would fire).
- `metis instructions`: the review section states the block reasons and
  the narrowing rule, so manual sessions follow the same contract.
- `metis kickoff`: unchanged. A handoff prompt tells the agent to run it.

## Risks

| Risk | Mitigation |
|---|---|
| Agents loop on a finding | cycle cap, repeat and growth rules, narrowing review |
| The wrong model codes or reviews a slice | resolution only from the ledger and `agents`; no fallback agent; the slug is logged per session in `auto.jsonl` |
| A reviewer passes everything to end the loop | unchanged safeguards: cross-vendor review, verify before a pass, the gate stays manual |
| An unattended agent damages the tree | scope audit after each coder session, branch mode, the agents' own sandboxes |
| Headless flags differ between agent CLIs and change | `launch` is plain configuration; defaults are versioned with metis and covered by `--dry-run` |
| A long shared-environment step (a lab, a deploy) collides | one slice at a time; agents park with `metis wait`, which the loop honours |
| The human loses the thread | `auto.jsonl`, `--status`, one escalation file per stop |

## Order and releases

| Release | Content | Value alone |
|---|---|---|
| v0.0.16 | Phase 1: `metis handoff`, block reasons, narrowing in templates | the human stops asking for prompts |
| v0.0.17 | Phase 2: launchers, `metis launch` | one command per session |
| v0.0.18 | Phase 3: `metis auto`, modes (`step` first, then `auto`), `metis init` question, stop rules, escalation, branch mode | one slice per command, then unattended slices up to the gate |
| later | Phase 4 | as use shows |

Each phase is one PR to `main` per `CONTRIBUTING.md`: `go test -race
./...`, `golangci-lint`, docs in `docs/commands.md`, `docs/workflow.md`
and `docs/configuration.md` updated in the same PR.

## Decisions (human, 2026-10-04)

1. `max_review_cycles` defaults to 3.
2. An escalation offers the human review override, as commands.
3. Branching: the default must suit multi-person projects, so `slice`
   (with `pr` for forge-reviewed teams); `off` for a solo work branch.
4. Exit code 3 means "stopped, needs a person".
5. One `metis auto` run handles every slice's coding and review loop and
   spawns each session with the agent and model the slice and project
   configuration name.
6. Planning and gates are manual.
7. Slice by slice or unattended is a mode chosen at `metis init` and
   overridable per phase and per slice, in either direction, persistently
   (`metis mode`) or for one run (`metis auto --phase|--slice … --mode`).
