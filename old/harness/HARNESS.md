# The Coding-Agent Harness

A reusable process and file set for driving a software project with autonomous
coding agents (Claude Code, Codex, opencode, or any mix) in disciplined
**coder / reviewer / human** lanes.

This document is **self-contained**: it carries the skeletons of every file the
harness needs, the end-to-end pipeline that turns a raw idea into shipped code,
a from-scratch recipe for an empty repository, and a checklist of what to
customize.

> **How to read this.** If you are adopting the harness into a new repo, read
> Parts A-D in order and create files as you go. If you are maintaining an
> existing harness, treat the Principles and File Manifest as the contract and
> the rest as reference.

---

## 1. Philosophy

The harness exists because a single long agent session drifts: it loses scope,
re-explores code it already understood, fixes things it was not asked to fix,
marks its own homework, and quietly works around contradictions instead of
surfacing them. The harness replaces "one big session" with **many small,
bounded, independently-reviewed units of work dispatched deterministically.**

Eight principles hold the whole thing together. Everything else is mechanism.

1. **Plan once, execute many.** Planning is a human-led, deliberate act that
   produces *frozen* artifacts (architecture, decisions, a phased plan). Agents
   do not re-plan mid-flight; they execute one pre-declared unit at a time.
2. **One active unit of work at a time.** Work is sliced into the smallest
   reviewable units ("slices"). Exactly one slice is *active*; a deterministic
   tool — never the agent eyeballing a list — says which one and who works it.
3. **Two lanes plus a human.** A **Coder** implements one slice. A **Reviewer**
   (different agent, ideally a different vendor) checks it against a fixed
   checklist. A **human** owns planning, escalations, and merges to the release
   branch.
4. **Scope is a written contract.** Before any code, the Coder commits a
   **brief** declaring exactly which files the slice may touch and what "done"
   means. The Reviewer measures the diff against the brief.
5. **Reality beats documents.** When the plan and the code disagree, the code
   wins; the agent fixes the wrong document in the same slice rather than
   silently coding around it. Priority is: **observed code > agent contract >
   plans.**
6. **Risk scales effort.** Each slice is tagged `low` / `medium` / `high`. Risk
   determines how much an agent must read, how senior a model is routed to it,
   and how hard the Reviewer looks.
7. **Determinism for the things agents fumble.** Walking a YAML list, comparing
   model slugs, evaluating booleans, telling an environment failure apart from a
   code failure — these go into small tools with one right answer, not into
   agent judgment.
8. **The system self-corrects.** Every blocking review finding is logged. Every
   N slices a human skims the log; recurring failures graduate into new rules or
   checklist items. Phase boundaries get a gate that proves the composed system,
   not just the individual slices.

---

## 2. Two regimes

The harness has two clearly separated halves. Keep them separate — most failures
come from blurring them.

### Planning regime (human-led, semi-manual, runs rarely)

Turns an idea into frozen, machine-dispatchable work. A human drives it, often
*with* an agent's help, but the **output is frozen**: agents in the execution
regime treat the plan as read-only intent, not a live design surface.

```
raw idea / notes / source docs
        |
        v
target architecture + roadmap        (docs/ARCHITECTURE.md, docs/ROADMAP.md)
        |
        v
architecture decision records        (docs/adr/NNNN-*.md)
        |
        v
implementation plan                  (plans/<name>-implementation-plan.md)
   phases -> workstreams -> tasks + acceptance criteria
        |
        v
backlog classification (optional)    (plans/backlog.md)
        |
        v
seeded slice ledger                  (plans/slices.yaml)
```

### Execution regime (autonomous, runs every session)

One agent, one session, one slice action. No pasted prompt: the agent reads the
contract, asks the tool what is active, confirms it is the right model, and
either codes or reviews.

```
session start
        |
        v
read contract (AGENTS.md) --> kickoff protocol (harness/AGENT-KICKOFF.md)
        |
        v
`next-slice` tool --> active slice id, role, required model, reading list
        |
        +-- role = Coder ----> brief -> implement (in scope) -> verify -> flip `coded`
        |
        +-- role = Reviewer -> independent verify -> 6-point checklist
                                   |
                          pass ----> flip `reviewed` -> archive slice
                          block ---> log finding, bump review_cycles, report
```

The two regimes meet at the **slice ledger**: planning fills it, execution
drains it.

---

## 3. File manifest

Everything the harness installs. Process machinery lives under `harness/`;
only files that must be at repo root for tooling auto-discovery remain there.
Living project content (`plans/`, `docs/adr/`) stays at standard locations.

"Reuse" = copy with minimal edits; "Customize" = a skeleton you fill in per
project; "Generate" = produced by tooling.

### Root files (required by agent tooling at repo root)

| Path | Role | Adoption |
|---|---|---|
| `AGENTS.md` | The governing contract: roles, branch/commit rules, hot-path zones, testing rules, non-goals, reviewer checklist, feedback loop. | **Customize** — §6 hot-path zones, §9 testing, §10 non-goals, §11 accuracy rules are project-specific. |
| `CLAUDE.md` | Points Claude Code at `AGENTS.md`; states the model-identity slugs. | **Reuse** (1-line edits). |
| `opencode.json` | Points opencode at `AGENTS.md` (Codex reads `AGENTS.md` natively — no adapter). | **Reuse**. |
| `.claude/settings.json` | Suppresses AI attribution in commits (and any repo-wide Claude Code settings). | **Reuse**. |
| `Makefile` | Task runner exposing harness targets: `next-slice`, `slices-check`, `slices-archive`, `verify`, `interfaces`, `env-check`. | **Customize** — the verify/interfaces/env commands are language-specific. |

### Harness directory (`harness/`)

| Path | Role | Adoption |
|---|---|---|
| `harness/AGENT-KICKOFF.md` | The step-by-step session protocol both lanes follow from step 1. | **Reuse** — language-neutral except the verify/interfaces commands. |
| `harness/HARNESS.md` | This file. The complete harness specification and reference. | **Reuse**. |
| `harness/slice-ledger.md` | Schema and lifecycle of the ledger. All ledger commentary lives here, not in the YAML. | **Reuse** (adjust field list if you extend the schema). |
| `harness/review-findings.md` | One line per blocking review finding. The self-correction substrate. | **Reuse** (starts empty). |
| `harness/tools/dispatch.py` | The deterministic `next` / `check` / `archive` dispatcher over the ledger. | **Customize** — reimplement in your stack if preferred; the *contract* is fixed (see §6). |

### Living project content

| Path | Role | Adoption |
|---|---|---|
| `plans/slices.yaml` | The dispatch ledger: pending + in-flight slices, plus the `agents:` identity map. Declaration order = execution order. | **Customize / Generate** — seed it once, then it is machine-managed. |
| `plans/slices-done.yaml` | Archive of fully-done slices. | **Generate** — written by the `archive` command. |
| `plans/briefs/<slice-id>.md` | Per-slice scope + DoD contract, committed by the Coder before code. | **Generate** — one per slice, from the template in `AGENT-KICKOFF.md`. |
| `plans/briefs/.gitkeep` | Keeps the empty briefs dir in git. | **Reuse**. |
| `plans/<name>-implementation-plan.md` | The frozen plan: phases -> workstreams -> tasks + acceptance criteria. | **Customize** — this is your project. |
| `plans/backlog.md` | Optional human-readable classification (stage, demo-critical, security-critical paths). | **Customize / optional**. |
| `plans/notes/`, raw source docs | Unstructured input that seeds planning. | **Project-specific**. |
| `docs/adr/_template.md`, `0000-*.md`, `README.md` | ADR template, the meta-ADR, and the ADR index. | **Reuse** template + meta-ADR; **Customize** the index as ADRs land. |
| `docs/generated/interfaces.txt` | Generated API summary so agents read real signatures, not hallucinated ones. | **Generate** — language-specific generator. |

---

# Part A — The planning regime

Run this once up front, then again at each major milestone. A human owns it.
Agents may help draft, but the human freezes the output.

### A.1 Capture the idea (`plans/notes/`, source docs)

Dump everything you know into `plans/` as plain markdown: the product vision, a
PDF extract, meeting notes, a napkin sketch. No structure required. This is the
only step with no template — it is deliberately unstructured.

### A.2 Target architecture + roadmap (`docs/ARCHITECTURE.md`, `docs/ROADMAP.md`)

Translate the idea into (a) the system you intend to build and (b) the order you
intend to build it. The roadmap is where vision becomes *phases*. Keep guiding
constraints explicit — they become the invariants your ADRs and `AGENTS.md`
accuracy rules will defend.

### A.3 Architecture Decision Records (`docs/adr/`)

Every binding, hard-to-reverse decision gets one ADR. ADRs are short and
immutable once accepted (supersede, never edit). They are the highest-authority
*intent* document — an agent reads the ADRs relevant to its slice before coding.

`docs/adr/_template.md`:

```markdown
# ADR-NNNN: <decision title>

- **Status:** Proposed | Accepted | Superseded by ADR-MMMM | Deprecated
- **Date:** YYYY-MM-DD
- **Decision drivers:** <why this decision, why now>

## Context
<The problem, the forces, the constraints. Short. If it grows into an essay,
write a design doc and link it.>

## Decision
<What we are doing. Imperative and specific: 1-3 sentences, then the rules or
invariants it implies.>

## Consequences
<Positive and negative. What gets easier, what gets harder, what invariants now
hold, what can fail.>

## Alternatives considered
<Other options and why not. Show the decision was made on purpose.>
```

Maintain `docs/adr/README.md` as the index (a table of `# | Title | Status`) and
ship `0000-record-architecture-decisions.md` as the meta-ADR describing the
process. New ADRs are part of the slice that introduces the decision — never a
free-floating commit.

### A.4 Implementation plan (`plans/<name>-implementation-plan.md`)

The plan is the backbone. It decomposes the build into a strict hierarchy:

```
Plan
+-- Phase N            (a milestone-sized chunk; ends with a gate)
    +-- Workstream N.M  (one reviewable slice's worth of work)
        +-- Tasks                 (what to do)
        +-- Suggested packages    (where it lives)
        +-- Acceptance criteria   (how "done" is judged — becomes the brief's DoD)
```

The unit that matters is the **workstream**: each one maps to exactly one slice
in the ledger, and its *Acceptance criteria* become that slice's Definition of
Done. Open the plan with: the source material it draws from, the target outcome,
an explicit scope boundary (what this plan does *not* cover), and a model
assignment legend (which agent/model class codes vs reviews which risk tier).

Workstream skeleton:

```markdown
### Workstream N.M: <name>

Tasks:
- <imperative task>
- <imperative task>

Suggested packages:
- `path/to/package`

Acceptance criteria:
- <testable outcome>
- <testable outcome>
```

### A.5 Backlog classification (`plans/backlog.md`, optional)

A human-readable lens over the ledger: which slices are on the demo-critical
path, which gate the security review, what stage each is (`prototype` / `mvp` /
`beta`). Useful past ~20 slices. **The ledger is authoritative**; when the
backlog drifts, fix the backlog. Skip this for small projects.

### A.6 Seed the ledger (`plans/slices.yaml`)

Turn each workstream into a slice. This is the handoff from planning to
execution. See §5 for the schema; §6 for the tool that reads it. Order the
slices by dependency — declaration order *is* execution order. Assign
`coder`/`reviewer` per the routing intent in `AGENTS.md` §8, cross-vendor by
default. Run the `check` command to lint it before the first session.

---

# Part B — The execution regime

These are the files that govern every autonomous session. They are the heart of
the harness and the most reusable part.

## B.1 The contract — `AGENTS.md`

`AGENTS.md` is auto-loaded by every agent surface (Codex and opencode read it
natively; Claude Code is pointed at it from `CLAUDE.md`). It is the governing
law. Below is a generic skeleton — the section *structure* is the reusable part;
the bracketed content is what you customize.

```markdown
# <Project> Agent Contract

This file governs all autonomous work in this repository.

## §1. Session start protocol
Every autonomous session begins by following `harness/AGENT-KICKOFF.md` from
step 1. No pasted prompt is required. Read this file per the risk-scaled reading
rules in the kickoff; only `risk: high` slices require it in full.

## §2. Governing documents and conflict priority
Priority when documents disagree: **observed code > AGENTS.md > plans/*.md**.
Found a real contradiction? Propose a fix to the wrong document in the same
slice. Never silently work around it.

## §3. Branch, ledger, and commit protocol
- All slice work lands on the `<integration>` branch, never `<release>`. The
  human owns every `<integration>` -> `<release>` merge.
- `plans/slices.yaml` is the dispatch ledger; `plans/slices-done.yaml` the
  archive. Use the `next-slice` tool to find the active slice — do not walk the
  YAML by hand.
- Every commit subject contains the slice ID, e.g. `feat(<slice-id>): ...`.
  This makes commit<->slice mapping derivable via `git log --grep`.
- Conventional Commits prefixes (`feat`/`fix`/`refactor`/`docs`/`test`/`chore`).
- Attribution: commits are authored under the human's git identity only. No
  `Co-Authored-By:`, no "Generated with", no model names. Provenance lives in the
  ledger + the slice ID in the subject.
- Land slices in ledger declaration order.

## §4. Definition of Done
A slice is done only when ALL hold ("tests pass" alone is not done):
1. Implementation matches the brief in `plans/briefs/<slice-id>.md`.
2. Tests proportional to §<testing> exist and pass.
3. `make verify` is green, confirmed independently by the Reviewer.
4. The Reviewer walked the §<checklist> with no blocking findings and flipped
   `reviewed: true`.
5. Ledger and brief are committed; commit subjects carry the slice ID.

## §5. Roles
- **Coder** — implements one slice within its declared file scope; owns its tests.
- **Reviewer** — reviews one slice against §<checklist>; re-runs verification
  independently; owns the sign-off; logs blocking findings.
- **Human** — owns planning, scope conflicts, escalations, model routing, and
  release merges.
Reviews are cross-vendor by default.

## §6. Hot-path zones        <-- CUSTOMIZE
Any slice touching these is `risk: high` and gets the full-reading rule:
- <list the modules/paths where a mistake is expensive: auth, payments,
  migrations, the request hot path, anything security- or data-integrity-critical>

## §7. Scope discipline and briefs
- Before any code, the Coder commits `plans/briefs/<slice-id>.md` (template in
  the kickoff) declaring file scope.
- Implement only within declared files. Genuinely-required out-of-scope fixes go
  in the brief's "Out-of-scope touches" section — never silently.
- If a slice seems to need a §<non-goals> item, or its true scope differs
  materially from the plan, stop and report to the human.
- If `make verify` fails on a clean checkout BEFORE your changes, stop and report.

## §8. Model routing        <-- CUSTOMIZE
The ledger assigns `coder`/`reviewer` per slice; honor it exactly. A mismatched
agent stops and reports. Routing intent:
- <senior model> — high-risk, hot-path, security, failure semantics, phase gates.
- <mid model> — medium-risk implementation, wiring, packaging.
- <other vendor> — cross-vendor review.

## §9. Testing rules        <-- CUSTOMIZE
- Mock at trust boundaries only; never mock the thing under test.
- Integration tests hit real dependencies where cheap.
- Tests are part of the implementation. A slice without relevant tests is
  incomplete.

## §10. Out of scope (do not implement)   <-- CUSTOMIZE
Explicit non-goals. Do not invent them even if asked indirectly. If a slice
seems to require one, stop and ask the human.
- <non-goal>

## §11. Accuracy rules       <-- CUSTOMIZE / GROWS OVER TIME
Project invariants that must never be violated. Seed from your roadmap
constraints; grow from recurring review findings (§13).
- Do not hallucinate structures — read the real interfaces (see `interfaces`
  generator) before consuming or implementing them.

## §12. Reviewer checklist
Walk in order; one-line verdict per item citing `file:line` and evidence:
1. Behavioral correctness
2. Security and authorization correctness
3. Scope discipline (diff vs. the committed brief)
4. Test sufficiency (§9)
5. Architectural fit — no duplicated/hallucinated interfaces; matches ADRs
6. Maintainability
Any block: do not flip `reviewed`; increment `review_cycles`; append to
`harness/review-findings.md`; report. Disagreements escalate to the human.

## §13. Feedback loop and phase gates
- Findings log: every blocking finding -> one line in
  `harness/review-findings.md`. Every ~10 slices the human skims it; recurring
  categories become new §11 rules or §12 items.
- `review_cycles`: the ledger records review rounds per slice — evidence for
  revising §8 routing.
- Phase gates: each phase ends with a `phase-N-gate` slice that validates the
  composed system and commits an evidence report. Individually-green slices do
  not imply a green system.

## §14. Tooling map
| Command | Purpose |
|---|---|
| `make next-slice` | Print active slice, role, required model, reading list |
| `make slices-check` | Lint the ledger (CI) |
| `make slices-archive` | Move done slices to the archive |
| `make interfaces` | Regenerate the API summary for archaeology |
| `make verify` | The verification gate (build, lint, tests) |
| `make env-check` | Tell an environment failure apart from a code failure |
```

## B.2 The protocol — `harness/AGENT-KICKOFF.md`

This is the ordered procedure every session runs. It is almost entirely
language-neutral; only the verify/interfaces commands change. Reuse it close to
verbatim. The structure:

1. **Establish actual state.** Confirm branch, clean working tree, recent log.
   Dirty tree -> stop (don't adopt someone's half-finished work).
2. **Find the active slice.** Run the `next-slice` tool. Trust it over manual
   YAML reading.
3. **Self-identify and check the match.** State your model identity in one line;
   match it against the slice's required `coder`/`reviewer` slug. No match ->
   stop and report which agent is needed. Do not invent work.
4. **Scaled reading.** Read by risk (floor, not ceiling):

   | Risk | Contract | ADRs | Plan |
   |---|---|---|---|
   | low | core sections | only ADRs the plan section cites | the slice's section only |
   | medium | + hot-path/accuracy/checklist | + ADRs touching packages in scope | section + adjacent workstreams |
   | high | in full | every plausibly related ADR | the full phase section |

   Then, regardless of risk, read prior briefs touching your packages —
   accumulated archaeology beats re-exploring from scratch.
5. **Pre-flight verification.** Run `env-check` then `verify`. **env-check
   fails -> environment problem, not code: report and stop, don't fix from inside
   the sandbox.** env-check passes but verify fails before you changed anything ->
   stop and report (pre-existing breakage is not your slice).
6. **Coder flow** (if your role is Coder): archaeology -> **brief first**
   (commit before any code) -> implement in scope -> verify (re-run env-check if
   it goes red mid-session) -> flip `coded: true` -> report.
7. **Reviewer flow** (if your role is Reviewer): locate the commits via
   `git log --grep <slice-id>` -> independent verify -> walk the 6-point
   checklist with `file:line` evidence -> verdict. Pass -> flip `reviewed`,
   archive. Block -> bump `review_cycles`, log finding, report. Disagreement ->
   escalate to human.
8. **Guardrails.** Non-goals are non-goals. Never merge to release. Conflict
   priority holds. Phase-gate slices validate composition.
9. **First message format.** Identity -> `next-slice` output -> match? -> one
   paragraph of current state and intent.

The kickoff ends with the **brief template** (see B.4).

## B.3 Surface adapters

Tiny files that point each agent vendor at the contract:

`CLAUDE.md` (Claude Code reads this automatically):

```markdown
# CLAUDE.md
This repository's agent contract lives in `AGENTS.md` — read it per its §1
session start protocol. For autonomous sessions, follow `harness/AGENT-KICKOFF.md`
from step 1 immediately; no pasted prompt is needed.

Identity note for ledger matching: state your model as one of
`<surface>/<model>` per the `agents:` map in `plans/slices.yaml`.
```

`opencode.json`:

```json
{ "$schema": "https://opencode.ai/config.json", "instructions": ["AGENTS.md"] }
```

`.claude/settings.json` (suppress AI attribution, per `AGENTS.md` §3):

```json
{ "attribution": { "commit": "", "pr": "" }, "includeCoAuthoredBy": false }
```

Codex reads `AGENTS.md` natively — no adapter needed.

## B.4 The brief template (`plans/briefs/<slice-id>.md`)

The Coder commits this *before* any implementation. It is the scope contract,
the compaction anchor if the session is interrupted, and the next slice's
archaeology. Lives at the bottom of `harness/AGENT-KICKOFF.md`:

```markdown
# <slice-id> — <title>

- **Plan:** <plan file> <plan_section>
- **Risk:** <low|medium|high>   **Coder:** <slug>   **Date:** <YYYY-MM-DD>

## Goal
One sentence, drawn from the workstream's stated goal.

## Architectural context
The specific interfaces, types, packages, and schema (from prior slices / the
generated interface summary) this slice consumes or implements.

## Declared file scope
- owned_paths: exact files this slice may edit
- read_only_paths: packages/files this slice may inspect but not modify
- primary: files this slice may edit
- secondary: files that may be touched if needed
- forbidden: hot-path files explicitly out of bounds

## Definition of Done
Specific, testable, drawn from the workstream's acceptance criteria.

## Test plan
Which unit / integration / contract / regression tests will exist.

## Out-of-scope touches
Empty unless a fix outside declared scope proved genuinely required; each entry
says what, where, and why.
```

A real brief does more than fill blanks: it records **design decisions** (which
later become ADRs), **deviations from the initial scope plan** (with rationale),
and **flagged scope boundaries** — pre-existing gaps the slice deliberately does
not close, surfaced for the human and Reviewer rather than silently crossed.

---

# Part C — Greenfield cold-start

An empty repo has no "current baseline", no prior briefs, and no generated
interface summary, yet the harness's archaeology steps assume history exists.
This part bootstraps the harness *and* the structures it depends on, in order.
You have: an idea, some notes, an empty git repo.

### Step 0 — Install the harness skeleton

Create, from the templates in this document:

```
AGENTS.md                           # B.1 skeleton, brackets unfilled for now
CLAUDE.md                           # B.3
opencode.json                       # B.3
.claude/settings.json               # B.3
Makefile                            # §7 targets
harness/AGENT-KICKOFF.md            # B.2 (reuse near-verbatim)
harness/HARNESS.md                  # This file
harness/slice-ledger.md             # §5 (reuse)
harness/review-findings.md          # empty table header
harness/tools/dispatch.py           # §6 — next/check/archive
docs/adr/_template.md               # A.3
docs/adr/0000-record-architecture-decisions.md
docs/adr/README.md                  # empty index
docs/generated/interfaces.txt       # empty stub
plans/slices.yaml                   # version + empty agents map + empty slices list
plans/briefs/.gitkeep
```

Commit this as your first commit. The harness is now installed but the ledger is
empty and the plan does not exist yet.

### Step 1 — Run the planning regime (Part A)

Do A.1 -> A.6 with the human in the loop. The honest output of this step for a
greenfield project is usually small: a roadmap, 3-6 foundational ADRs, and a
Phase 0 + Phase 1 of the plan. **Do not over-plan a project you have not
started.** Plan the first one or two phases in detail; sketch the rest.

### Step 2 — Make the first slices *bootstrap* slices

The first few slices cannot do archaeology — there is nothing to read. So the
first slices' job is to *create the things later slices will read.* A typical
cold-start ledger opens with:

- `phase-0-ws-0.1` — **Foundational ADRs.** Risk `high`. Lock in the
  architecture-defining decisions (the module boundary, the system of record,
  the security model, the observability approach). Output: ADRs, not code.
- `phase-0-ws-0.2` — **Interface contracts / package seams.** Risk `high`.
  Create the empty packages and the core interfaces every later slice consumes.
  This is what makes `interfaces` generation meaningful from slice 3 onward.
- `phase-0-ws-0.3` — **Project scaffold + verify gate.** Wire up build, lint,
  test, and the `verify` command so `env-check` and `verify` actually run. Until
  this lands, the kickoff's pre-flight is vacuous.

Only after these do feature slices begin.

### Step 3 — Stub the archaeology inputs

Two harness inputs assume history. Handle their absence explicitly:

- **`docs/generated/interfaces.txt`** — does not exist until there is code. The
  `interfaces` generator produces an empty/near-empty file at first; that is
  fine. Regenerate it at the end of every slice that adds public interfaces. The
  kickoff already says "regenerate if stale" — for slice 1-2 that means
  "generate for the first time."
- **Prior briefs** — none exist for the first slice. The kickoff's "read prior
  briefs touching your packages" simply yields nothing. Each completed brief is
  the archaeology for the next slice, so the corpus grows itself. No action
  needed beyond writing every brief properly.

### Step 4 — First execution session

Run the kickoff for real. The agent's `next-slice` returns `phase-0-ws-0.1`,
role Coder. It writes ADRs, commits the brief, flips `coded`. The Reviewer
session checks them. The loop is now turning.

> **Cold-start smell test.** If your first feature slice (real product code)
> cannot point its brief's "Architectural context" at a real interface in
> `interfaces.txt` or a prior brief, your Phase 0 was too thin — add a seam
> slice before it.

---

# Part D — Customization checklist

When you drop the harness into a project, you must touch these. Everything else
is reusable as-is.

- [ ] **`AGENTS.md` §6 hot-path zones** — the modules where a mistake is
      expensive. This drives risk tagging and reading depth.
- [ ] **`AGENTS.md` §8 model routing** — which model classes you have and which
      risk tiers they take. Define your `agents:` slugs to match.
- [ ] **`AGENTS.md` §9 testing rules** — your project's test layers and the
      mocking boundary.
- [ ] **`AGENTS.md` §10 non-goals** — the things agents must not build.
- [ ] **`AGENTS.md` §11 accuracy rules** — seed from your roadmap's guiding
      constraints; let it grow from review findings.
- [ ] **The `agents:` map in `plans/slices.yaml`** — the surface/model/identity
      slugs your sessions will self-identify against.
- [ ] **Branch names** — replace the `<integration>`/`<release>` branch model
      with yours.
- [ ] **The `verify` / `env-check` / `interfaces` commands in `Makefile`** —
      language-specific. This is the largest mechanical change.
- [ ] **The dispatcher** — reimplement `harness/tools/dispatch.py` in your stack
      if preferred, or use the Python version as-is (requires `pyyaml`).
- [ ] **`CLAUDE.md` identity line** — your model slugs.
- [ ] **The plan + ledger** — your actual project (Part A).

---

## 5. The slice ledger — schema

`plans/slices.yaml` is the single source of dispatch truth. **All commentary
lives in `harness/slice-ledger.md`, never in the YAML** — the YAML is rewritten
by the tool (which strips comments) and read every session (every comment costs
tokens).

```yaml
# Machine-managed by harness/tools/dispatch.py. Schema & lifecycle: harness/slice-ledger.md
version: 2
agents:
  claude-code/opus:              # <-- the slug agents self-identify against
    surface: claude-code
    model: opus
    plan_tag: Claude Code (Opus)
  opencode/opus:
    surface: opencode
    model: opus
    plan_tag: opencode (Opus)
  codex:
    surface: codex
    model: codex
    plan_tag: Codex
  # ... one entry per agent/model you route to
slices:
  - id: phase-1-ws-1.1           # unique slug; appears in every related commit
    title: <human-readable; decoration only — fields are authoritative>
    plan: plans/<name>-implementation-plan.md   # or null for bespoke slices
    plan_section: §6             # required when plan is set; scopes the reading
    coder: claude-code/opus
    reviewer: codex              # cross-vendor by default; never == coder
    risk: high                   # low | medium | high
    stage: mvp                   # optional project taxonomy (prototype/mvp/beta)
    coded: false                 # lifecycle boolean
    reviewed: false              # lifecycle boolean
    review_cycles: 0             # bumped by the Reviewer on each block; absent = 0
    notes: <skip reasons, exceptions, clarifications>
```

**Lifecycle.** First slice with `coded:false` (-> role Coder) or, if coded,
`reviewed:false` (-> role Reviewer) is *the active slice*. Coder flips `coded`;
Reviewer approves (flip `reviewed`, then archive) or blocks (bump
`review_cycles`, log finding, leave booleans). **Redo** a slice: set both
booleans false with a `notes:` reason. **Skip**: set both true with a reason.

**Special slice types.** `phase-N-gate` validates a phase's composed system
(evidence report to `plans/briefs/phase-N-gate.md`). `docs-recon-N` sweeps
plan/ADR drift against observed code.

---

## 6. The dispatcher — a portable contract

The dispatcher is the determinism backbone. Agents are bad at walking a YAML
list in order, evaluating boolean pairs, and string-matching slugs — so a tiny
program does it, and the agent only compares its own identity to the output.

It lives at `harness/tools/dispatch.py` and has **three subcommands** over the
ledger schema:

- **`next`** — find the first slice that is not `coded && reviewed`; if `coded`
  is false the role is Coder (use `coder`), else Reviewer (use `reviewer`).
  Print: id, title, risk, stage, role, required model slug + its `plan_tag`,
  plan + section, `review_cycles`, and the **risk-scaled reading list**. If the
  required slug is absent from `agents:`, say so loudly. For a Reviewer, also
  print the `git log --grep <id>` locator and the brief path.
- **`check`** — lint both ledger files: unique ids, known agent slugs, valid
  risk values, `plan_section` present wherever `plan` is set, no
  `reviewed && !coded`, `coder != reviewer`, archive entries fully done. Exit
  non-zero on any failure. **Run this in CI** — an unmapped slug stalls dispatch.
- **`archive`** — move every fully-done (`coded && reviewed`) slice out of
  `slices.yaml` into `slices-done.yaml`, keeping the active ledger lean.

> **Portability.** The reference implementation is a single-file Python script
> (~150 lines, requires `pyyaml`). The *contract* above is language-agnostic —
> reimplement it in your project's primary language (so it shares your toolchain
> and CI) if you prefer. Keep it in the repo, wire it into your task runner, and
> gate CI on `check`.

---

## 7. Tooling and the environment gate

The harness needs a task runner (the `Makefile` at repo root) exposing these
targets:

| Target | Invokes | Language-specific? |
|---|---|---|
| `make next-slice` | `harness/tools/dispatch.py next` | No |
| `make slices-check` | `harness/tools/dispatch.py check` | No |
| `make slices-archive` | `harness/tools/dispatch.py archive` | No |
| `make verify` | Build + lint + test gate | **Yes** |
| `make interfaces` | Regenerate `docs/generated/interfaces.txt` | **Yes** |
| `make env-check` | Environment soundness check | **Yes** |

Two of these deserve special attention because they prevent whole classes of
false signal:

### `env-check` — the false-red guard

Agents run in sandboxes that degrade: a read-only `$HOME`, an evicted build
cache, no network for dependency fetches. A red `verify` in a broken sandbox
proves *nothing* about the code — but an agent will "helpfully" start editing
code, tests, or lint config to make it pass. `env-check` runs *before* `verify`
and checks the environment is sound (toolchain on PATH, caches writable, a clean
build of a trivial target succeeds). If it fails, it prints a loud verdict:

```
VERDICT: ENVIRONMENT FAILURE — NOT A CODE FAILURE.
Do NOT modify code, tests, or config to make verify pass. Do NOT flip ledger
booleans. Stop and report this verbatim. The fix is sandbox/network config — a
human decision.
```

Make `verify` depend on `env-check` and `slices-check` so the protections are
automatic and ledger bugs fail fast in CI.

**Sandbox-proofing tips (generic):** relocate build/lint caches into the
workspace (sandboxes often make `$HOME` read-only); vendor or lock dependencies
so a network-less sandbox can still build; scope formatters/linters to
first-party code (never rewrite vendored/generated files).

### `interfaces` — anti-hallucination archaeology

Agents invent plausible-but-wrong type and function signatures. `interfaces`
generates an authoritative summary of your real public API (use your language's
doc extractor — `go doc`, `tsc --declaration`, `python -m pydoc`, `cargo doc`,
etc. — into `docs/generated/interfaces.txt`). The Coder reads it before
designing against existing code; the Reviewer checks the diff did not duplicate
or hallucinate something that already exists.

---

## 8. The feedback loop — how the system gets better

A harness that only executes will repeat its mistakes. Three mechanisms make it
*learn*:

1. **`harness/review-findings.md`.** One terse line per *blocking* finding (date,
   slice, severity, category, summary). Every ~10 slices a human skims it.
   Recurring categories graduate into new `AGENTS.md` §11 accuracy rules or §12
   checklist items — the defect stops being *caught* and starts being
   *prevented*.

   ```markdown
   | Date | Slice | Sev | Category | Finding (one line) |
   |---|---|---|---|---|
   <!-- append below this line -->
   ```

   Severity: P1 (breaks a guarantee) / P2 (wrong but contained) / P3 (debt).
   Category: a small fixed set, e.g. `auth`/`protocol`/`scope`/`tests`/
   `arch-dup`/`arch-fit`/`data`/`maint`.

2. **`review_cycles`.** The ledger counts review rounds per slice. A slice that
   took three rounds with a given model is evidence to re-route that work to a
   more senior model — routing is revised on data, not vibes.

3. **Phase gates.** Each phase ends with a `phase-N-gate` slice whose "code" is
   integration/contract scenarios across the phase's surface plus an evidence
   report committed to `plans/briefs/phase-N-gate.md`. Individually-green slices
   do **not** imply a green system; the gate is where composition is proven, and
   composition failures are filed as blocks against the offending slices.

   Plus periodic `docs-recon-N` slices that sweep plan/ADR drift against the
   observed code, enforcing the "reality beats documents" priority (§2).

---

## 9. Why each guardrail exists (rationale digest)

For maintainers tempted to "simplify" the harness, the non-obvious load-bearing
parts:

- **Brief-before-code** is not bureaucracy: it is the scope contract the Reviewer
  measures against *and* the recovery anchor when a long session is compacted or
  interrupted. Without it, scope creep is invisible and a dropped session loses
  all context.
- **Cross-vendor review** catches a whole class of blind spots — an agent that
  makes a mistake often fails to see it on re-read; a different model/vendor does
  not share the blind spot.
- **Commit subject carries the slice ID** (not commit hashes in the ledger):
  the mapping stays derivable via `git log --grep` and the ledger stays a clean,
  low-token dispatch list.
- **No AI attribution in commits** keeps the audit trail in the ledger
  (`coder`/`reviewer` fields + slice ID), where it is structured and queryable,
  rather than scattered in commit trailers.
- **Comments live in `harness/slice-ledger.md`, not the YAML**: the YAML is read
  every session by every agent — every comment line is a recurring token cost —
  and is machine-rewritten anyway.
- **env-check before verify** is the single highest-leverage guard against an
  agent "fixing" green code to satisfy a broken sandbox.
- **One active slice** removes the entire category of "two sessions editing the
  same hot-path file with divergent assumptions."
