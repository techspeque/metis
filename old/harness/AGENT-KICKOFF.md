# Agent Kickoff — Autonomous Slice Protocol

You are a coding agent working on CAESAR (Context-Aware Evaluation System for
Agentic Reliability). Your job this session is to advance the `dev` integration
branch by **exactly one slice action** (one coding pass or one review). You take
no manual input; everything you need is in the repo.

This file is auto-loaded via `AGENTS.md` §1. A human typing "go" or "next
slice" — or nothing at all — means: execute this protocol from step 1.

Do the steps in order. Do not skip steps.

---

## 1. Establish actual state

```bash
git rev-parse --abbrev-ref HEAD     # must be dev
git status                          # working tree must be clean
git log --oneline -10
```

If the working tree is dirty, stop and report — do not adopt someone else's
half-finished work.

## 2. Find the active slice

```bash
make next-slice
```

This prints: active slice ID, title, risk, stage, active role
(Coder/Reviewer), required model slug, plan file + section, `review_cycles`,
and the scaled reading list. Trust its output over any manual reading of
`plans/slices.yaml`. (Fallback if the tool is broken: first slice in
declaration order with `coded: false` -> role Coder, else `reviewed: false` ->
role Reviewer; then fix the tool as a reported issue, not inside your slice.)

## 3. Self-identify and check the match

State your model identity in one line ("I am Claude Opus via Claude Code",
"I am Opus via opencode", "I am Codex"). Match against the `agents:` map
slugs: Claude Code sessions match `claude-code/opus`; opencode sessions
match `opencode/opus`; Codex sessions match `codex`.

- **Match** -> continue.
- **No match** -> stop. Report the active slice (id, title, risk), the active
  role, the model required, and the model you are. Do not work the slice. Do
  not invent work.

If the slice is `risk: high`, flag it in your first message so the human and
the eventual Reviewer apply heightened scrutiny.

## 4. Scaled reading

Read according to the slice's risk. This is a floor, not a ceiling — read
more if genuinely needed, never less.

| Risk | AGENTS.md | ADRs | Plan |
|---|---|---|---|
| low | §3, §4, §7, §9, §10 | only ADRs the plan section cites | the slice's `plan_section` only |
| medium | + §6, §11, §12 | + ADRs touching the packages in scope | `plan_section` + adjacent workstreams in the same phase |
| high | in full | every plausibly related ADR — do not skip | the full phase section |

Then, **regardless of risk**, read the prior briefs that map your terrain:

```bash
ls plans/briefs/        # read every brief whose slice touched the packages you will touch
```

Briefs are the accumulated archaeology of previous sessions. Read them before
re-exploring `internal/` from scratch.

**Greenfield note:** Early slices (especially Phase 0) will have no prior
briefs and a minimal or empty `docs/generated/interfaces.txt`. This is normal.
Each completed brief is the archaeology for the next slice — the corpus grows
itself. Early slices produce the interfaces and contracts later slices depend
on.

## 5. Pre-flight verification

```bash
make env-check
make verify
```

**If `env-check` fails: this is an environment problem, not a code problem.**
Report the ENV-FAIL line to the human and stop. Do not attempt to repair the
environment from inside the session — do not edit go.mod, do not vendor
dependencies, do not modify CI or lint config to make the error disappear.
The fix (warming the module cache, sandbox network access, PATH) belongs to
the human, outside the sandbox.

**If `env-check` passes but `make verify` fails before you have changed
anything: stop and report.** Do not fix pre-existing breakage inside your
slice — that is a scope violation even when well-intentioned. The human or a
dedicated fix slice handles it.

**Greenfield note:** Before `go.mod` exists, `make verify` will report
"skeleton only, build checks skipped" — this is expected and not a failure.
The first implementation slice that introduces `go.mod` establishes the build
gate.

---

## 6. Coder flow (only if your active role is Coder)

1. **Archaeology.** Beyond the prior briefs (step 4), consult
   `docs/generated/interfaces.txt` (regenerate with `make interfaces` if
   stale) for the authoritative Go interfaces, structs, and signatures on
   `dev`. Do not hallucinate data structures; if your slice interacts with an
   existing package, locate and read its real interface before designing
   against it.

   **Greenfield note:** For the very first slices, `interfaces.txt` is empty.
   You are *creating* the interfaces. Regenerate it at the end of your slice
   if you added public types or interfaces so the next slice has archaeology.

2. **Brief first.** Commit `plans/briefs/<slice-id>.md` *before any
   implementation code*, using the template at the bottom of this file. Commit
   subject: `docs(<slice-id>): slice brief`. If the brief is vague, refine it
   before implementing. The brief is your anchor if the session compacts, the
   Reviewer's scope baseline, and the next slice's archaeology.

3. **Implement** only within the brief's declared files. Genuinely required
   out-of-scope fixes go in the brief's "Out-of-scope touches" section,
   explicitly — never silently. Tests per AGENTS.md §9, in the same slice.

4. **Verify.** `make verify` green on your tip. If it goes red **after your
   changes**, run `make env-check` before debugging: a sandbox can degrade
   mid-session (cache eviction, network state), and a red verify in a broken
   environment proves nothing about your code. Environment failure -> stop,
   report verbatim, leave the ledger untouched (`coded` stays false; the
   slice is resumable). Healthy environment -> the failure is yours; fix it
   within scope. Keep the relevant output tail for your report.

5. **Regenerate interfaces** if you added or changed public types/interfaces:
   ```bash
   make interfaces
   ```
   Commit the updated `docs/generated/interfaces.txt` in the same slice.

6. **Ledger.** Flip the slice's `coded: true` (same commit as the final code
   change, or an immediately following `chore(<slice-id>): flip coded`
   commit). No commit hashes go in the ledger — your commit subjects carry
   the slice ID, which is the mapping.

7. **Commit target.** Directly on `dev`, all risk levels. Commits are
   authored under the human's git identity — no `Co-Authored-By:`,
   "Generated with", or any other AI attribution (AGENTS.md §3). This applies
   to Reviewer sign-off commits too.

8. **Report** to the human: slice ID, goal, files changed vs. brief,
   `make verify` tail, ledger flip, and the output of `make next-slice` (what
   the system needs next).

## 7. Reviewer flow (only if your active role is Reviewer)

1. **Locate the work.** It is on `dev`. Identify the commits:

   ```bash
   git log --oneline --grep "<slice-id>"
   ```

   You are reviewing exactly those commits, against the committed brief
   `plans/briefs/<slice-id>.md`.

2. **Independent verification.** Run `make env-check` then `make verify`
   yourself on the slice tip. Do not trust the Coder's word. An ENV-FAIL is
   an environment problem — report it, do not Block the slice for it. A red
   `make verify` in a healthy environment is an automatic Block.

3. **Checklist.** Walk AGENTS.md §12 in order — one-line verdict per item
   with `file:line` evidence. Item 3 is the diff measured against the brief's
   declared scope. Item 5 includes verifying no duplicated or hallucinated
   interfaces vs. `docs/generated/interfaces.txt` and existing `internal/`
   code.

4. **Verdict.**
   - **All six pass** -> sign-off commit (`chore(<slice-id>): review sign-off`)
     that flips `reviewed: true`; then run `make slices-archive` so the
     ledger stays lean, and commit the result.
   - **Any block** -> do **not** flip `reviewed`. Increment the slice's
     `review_cycles`. Append one line per blocking finding to
     `harness/review-findings.md`. State what is wrong, where (`file:line`), and
     how to fix it. Commit subject: `chore(<slice-id>): review block #<n>`.
     Report; the assigned Coder picks it up next session.
   - **Disagreement with the Coder** -> escalate to the human. Do not litigate
     in commit messages.

5. **Report**: slice ID, verdict, ledger update, findings logged (if any),
   and the output of `make next-slice`.

---

## 8. Guardrails

- AGENTS.md §10 non-goals are non-goals. If the slice as written seems to
  require one, stop and ask the human.
- `dev` does not merge to `main` until the human declares the milestone
  operational. Never open or suggest that PR.
- Conflict priority: **observed code > AGENTS.md > plans/*.md**. Real
  contradictions get a proposed doc fix in the same slice, not a silent
  workaround.
- Phase-gate slices (`phase-N-gate`) are validation slices: the "code" is the
  scenario harness additions plus an evidence report committed to
  `plans/briefs/phase-N-gate.md`; composition failures are reported as blocks
  against the offending slices, escalated to the human for re-scoping.

## 9. First message format

1. Your model identity.
2. The `make next-slice` output (id, title, risk, role, required model).
3. Whether you match. If not — stop, name the agent needed.
4. If you match: one paragraph of current `dev` state and what you are about
   to do.

Then proceed to step 6 (Coder) or step 7 (Reviewer).

---

## Appendix: Brief template (`plans/briefs/<slice-id>.md`)

```markdown
# <slice-id> — <title>

- **Plan:** <plan file> <plan_section>
- **Risk:** <low|medium|high>   **Coder:** <slug>   **Date:** <YYYY-MM-DD>

## Goal
One sentence, drawn from the workstream's stated goal.

## Architectural context
The specific Go interfaces, structs, packages, and schema (from prior slices /
`docs/generated/interfaces.txt`) this slice consumes or implements. For early
slices with no prior context, state "greenfield — defining the initial contracts."

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
Empty unless a fix outside declared scope proved genuinely required; each
entry says what, where, and why.
```
