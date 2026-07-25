# Agent Session Protocol

> **Audience:** agents — and humans who want to understand exactly what
> agents do. This is what `metis kickoff` generates dynamically from your
> configuration; agents receive it live, so the binary is always the
> authority. The human-side guide is [workflow.md](workflow.md).

## Overview

Every autonomous agent session follows the same deterministic protocol:

```
Session start
    → Establish state (branch, tree)
    → Find active slice (metis next)
    → Self-identify and match
    → Read instructions (risk-scaled)
    → Pre-flight verification
    → Execute (Coder flow or Reviewer flow)
    → Report
```

No pasted prompt is needed. The agent reads `AGENTS.md` (auto-loaded by all
surfaces), which directs it to run `metis kickoff` immediately.

**Structured output contract:** every read command accepts `-o json`
(see [commands.md](commands.md#output-format)). Whenever the protocol needs
an exact value — slice ID, agent slug, role, status — agents read it from
the JSON field rather than parsing the human-readable text. The text output
may change between versions; the JSON fields are the stable contract.

---

## Step 1: Establish State

```bash
git rev-parse --abbrev-ref HEAD   # must be integration branch
git status                        # check for uncommitted changes
metis status                      # quick orientation
```

### Branch Check

If the current branch is not the configured `integration_branch`: **STOP**.
Report to human. Agents never switch branches.

### Dirty Tree Handling

**Clean tree:** continue to Step 2.

**Dirty tree (uncommitted changes):** attempt to resume:

1. Run `metis next` to identify the active slice.
2. If a brief exists (`metis brief <id>`), check whether the dirty files
   fall within the brief's declared `owned_paths`:
   - **Files are in scope:** This is a resumed/interrupted session.
     Read the brief, review `git log --oneline` for progress so far,
     and continue implementation from where it was left off.
   - **Files are outside scope:** STOP. Report to human:
     "Dirty tree with out-of-scope files. Cannot safely resume."
3. If no brief exists yet, check whether dirty files are only in `.metis/`:
   - **Yes:** Safe to continue (partial brief or template in progress).
   - **No:** STOP. Report to human.

---

## Step 2: Find Active Slice

```bash
metis next -o json
```

Trust this output over any manual YAML reading. The dispatch algorithm is
deterministic — priority ordering, dependency resolution, and role assignment
are handled by the tool. The `id`, `role`, and `agent_slug` fields drive the
following steps.

If the output is `{"active": false}`: report "backlog empty" and stop.

---

## Step 3: Self-Identify

State your model identity in one line. Compare against the `agent_slug`
field from Step 2.

- **Match:** continue
- **No match:** STOP. Report which agent is needed. Do not invent work.

---

## Step 4: Read Instructions

```bash
metis instructions --for <slice-id>
```

Read the full output. This is the risk-scaled contract plus the complete
project OVERVIEW. It contains everything needed for this slice — no more,
no less.

---

## Step 5: Pre-flight Verification

```bash
metis verify --pre
```

- **Exit 0:** continue
- **Exit 2 (environment failure):** STOP. Report verbatim. Do NOT modify code.
  The fix is sandbox/network configuration — a human decision.
- **Exit 1 (code failure before your changes):** STOP. Report pre-existing
  breakage. Do not fix it — scope violation.

---

## Step 6a: Coder Flow

If `metis next` assigned role = **Coder**:

1. **Read interfaces** — `metis interfaces` output (if configured).
   Real signatures prevent hallucinated ones.

2. **Write brief** — `metis brief <id> --write` generates the template.
   Edit it to declare:
   - Goal (one sentence)
   - Architectural context (interfaces consumed/implemented)
   - Declared file scope (`owned_paths`, `read_only_paths`)
   - Definition of Done (testable criteria)
   - Test plan
   
   Then commit: `metis commit --brief`

3. **Implement** — within declared scope only.
   - Only touch files in `owned_paths`
   - If you genuinely need to touch something outside scope, declare it
     in the brief's "Out-of-scope touches" section with rationale
   - If the slice needs a non-goal item, STOP and report

4. **Verify** — `metis verify --post`
   - Exit 0: continue
   - Exit 2 mid-session: environment degraded, STOP and report
   - Exit 1: your code broke something, fix it (within scope)

5. **Regenerate interfaces** — `metis interfaces` (if you changed public API)

6. **Flip** — `metis commit --flip coded --slice <id>` — binding the flip to the slice from Step 2 makes it error loudly if dispatch moved on (e.g. a p0 arrived) instead of flipping the wrong slice

7. **Report** — slice ID, files changed, verify result, what's next

---

## Step 6b: Reviewer Flow

If `metis next` assigned role = **Reviewer**:

1. **Locate commits** — `metis log <id>`

2. **Read brief** — `metis brief <id>` (reads the committed brief)

3. **Independent verify** — `metis verify --post`
   (You MUST verify independently — stored logs are evidence, not proof)

3b. **Audit scope** — `metis log <id> --validate`: deterministic check that
   every commit matches the format and every touched file falls inside the
   brief's declared `owned_paths` (gate slices are exempt from the scope
   portion). FAIL → block with category `scope`. This is enforced:
   `commit --flip reviewed` refuses while the audit fails.

4. **Walk checklist** — one-line verdict per item, citing `file:line`:
   1. Behavioral correctness
   2. Security and authorization correctness
   3. Scope discipline (diff vs. the committed brief)
   4. Test sufficiency
   5. Architectural fit — no duplicated/hallucinated interfaces; matches ADRs
   6. Maintainability

5. **Verdict:**
   - **Pass:** `metis commit --flip reviewed --agent <your-slug> --slice <id>` then `metis archive`
   - **Block:** `metis block <id> --severity P1 --category <cat> --finding "..."`

   Both paths are atomic — `block` and `archive` commit the ledger and
   findings changes themselves, so the session always ends with a clean tree.

6. **Report** — slice ID, verdict, findings (if any), what's next

---

## Risk Scaling

Instructions are filtered by the slice's risk level:

| Section | Low | Medium | High |
|---|---|---|---|
| Project overview | Yes | Yes | Yes |
| Core rules (branch, DoD, scope, testing, non-goals, tooling) | Yes | Yes | Yes |
| Hot-path zones | | Yes | Yes |
| Accuracy rules | | Yes | Yes |
| Review checklist | | Yes | Yes |
| Model routing | | | Yes |
| Feedback loop | | | Yes |

Higher risk = more context = more thorough reading required.

---

## Hard Rules (Governance)

These rules are non-negotiable and embedded in `AGENTS.md`:

1. ONE slice at a time — `metis next` decides which, not you
2. Brief BEFORE code — commit scope contract before implementation
3. Scope is a contract — only touch files declared in your brief
4. Cross-vendor review — you cannot review your own work
5. `metis commit` for all commits — enforces format, strips attribution
6. STOP on environment failure — do not modify code to fix a broken sandbox
7. Dirty tree with in-scope files — resume the interrupted session
8. Dirty tree with out-of-scope files — STOP and report to human
9. Reality beats documents — if code contradicts plan, fix the document
10. No planning in execution — do not re-scope or invent additional work
11. Report mismatches — if you're the wrong agent for this slice, STOP
12. Trust the tools — do not walk YAML, compare slugs, or evaluate booleans manually
13. Exact values come from `-o json` — every read command supports it; never parse human-readable output

---

## The Feedback Loop

```
Review block → metis block (records finding, increments review_cycles)
    ↓
Recurring findings → metis rule promote (becomes permanent accuracy rule)
    ↓
Rules in instructions → prevents future occurrences
    ↓
Phase gates → validate composition (individually-green ≠ system-green)
```

Every blocking finding is structured data. Every ~10 slices, the human reviews
findings. Categories that recur become permanent rules. The system learns.
