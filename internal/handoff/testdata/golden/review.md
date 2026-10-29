# Review phase-1-ws-1.1

metis handoff · template review · slice phase-1-ws-1.1 · role Reviewer · agent codex/gpt

You run as `codex/gpt` (Codex (GPT)). That is the slug to state when `metis kickoff` asks you to self-identify; it matches the `agent_slug` of `metis next -o json`.

Slice: `phase-1-ws-1.1` — Retry the webhook (type feat, risk medium, plan .metis/plans/phase-1.md §1.1).
Branch: work on `dev`.

## Start

Run `metis kickoff --role reviewer` and follow it from step 1.

## Toolchain

Every command you run yourself — a test, a type check, a script, anything outside `metis verify` — starts in a shell that has selected the project's toolchain:

```sh
. "$HOME/.nvm/nvm.sh" && nvm use "$(cat web/.node-version)" >/dev/null
```

Run it in each new shell before the first such command. A command run without it uses whatever the shell finds first, and its result is not evidence.

## The contract of this review

- **This slice only.** Judge the slice's commits against its brief. Earlier slices, the wider codebase and decided mechanisms are out of this review.
- **The brief is the source of truth:** `.metis/briefs/phase-1-ws-1.1.md`. What the brief decides is not reopened here.
- **Read everything first:** the brief, every commit (`metis log phase-1-ws-1.1`), and the files they touch, before you record anything.
- **One review, every issue.** Record all findings in this one review: one `metis block phase-1-ws-1.1 --severity P1|P2|P3 --category <category> --finding "<file:line> — <the defect> — <the required change>"` per blocking finding; the second and later blocks join the same cycle. Do not hold findings back for a later review.

A finding blocks only when it is one of:
- a deviation from the brief
- a missing or failing test the brief lists
- a broken accuracy rule
- a demonstrated defect

Anything else is an advisory: `metis findings record phase-1-ws-1.1 --severity P3 --category <category> --finding "..."` (no state change).

## Checklist

1. Behavioral correctness
2. Test sufficiency

One line per item, citing file:line.

## Commits

- abc1234 feat(phase-1-ws-1.1): retry with backoff
- def5678 chore(phase-1-ws-1.1): flip coded

## Verdict

1. `metis log phase-1-ws-1.1 --validate` — a FAIL is a block.
2. Pass: `metis verify --post --slice phase-1-ws-1.1` green, then `metis commit --flip reviewed --agent codex/gpt --slice phase-1-ws-1.1`, then `metis archive`. Close the findings you verified fixed: `metis findings resolve <f-id> --note "<how>"`.
3. Block: the `metis block` calls above, all in this session.

## If you need a person

A decision only a person can make (the brief is silent or contradicts the plan, a finding asks for what the brief rules out): run `metis wait phase-1-ws-1.1 --reason "<the question, one line>" --human` and stop. Do not guess. An environment failure (`metis verify` exit 2) is reported verbatim; do not change code to work around it.

Report: the verdict and the finding ids. Facts only, short.
