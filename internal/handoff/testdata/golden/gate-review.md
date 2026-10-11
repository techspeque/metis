# Gate review phase-1-gate

metis handoff · template gate-review · slice phase-1-gate · role Reviewer · agent codex/gpt

You run as `codex/gpt` (Codex (GPT)). That is the slug to state when `metis kickoff` asks you to self-identify; it matches the `agent_slug` of `metis next -o json`.

Slice: `phase-1-gate` — Phase 1 gate (type gate, risk high, plan .metis/plans/phase-1.md Phase Gate).
Branch: work on `dev`.

## Start

Run `metis kickoff --role reviewer` and follow it from step 1.

## The contract of this review

- **This gate only.** Judge the gate report against its brief: does the evidence show the phase works as a composed system?
- **The brief is the source of truth:** `.metis/briefs/phase-1-gate.md`.
- **Run nothing unless the brief says so.** The evidence is in the report; re-running suites or auditing tooling line by line is out of this review.
- **One review, every issue.** One `metis block phase-1-gate --severity P1|P2|P3 --category <category> --finding "<where> — <the gap> — <the evidence required>"` per blocking finding, all in this session.

A finding blocks only when it is one of:
- a deviation from the brief
- a missing or failing test the brief lists
- a broken accuracy rule
- a demonstrated defect

Anything else is an advisory: `metis findings record phase-1-gate --severity P3 --category <category> --finding "..."` (no state change).

## Evidence index (from the brief)

1. Composed run: .metis/runs/phase-1-gate/e2e.log
2. Contract tests: internal/webhook/contract_test.go

## Checklist

1. Behavioral correctness
2. Test sufficiency

## Verdict

1. Pass: `metis commit --flip reviewed --agent codex/gpt --slice phase-1-gate`, then `metis archive`.
2. Block: the `metis block` calls above.

## If you need a person

A decision only a person can make (the brief is silent or contradicts the plan, a finding asks for what the brief rules out): run `metis wait phase-1-gate --reason "<the question, one line>" --human` and stop. Do not guess. An environment failure (`metis verify` exit 2) is reported verbatim; do not change code to work around it.

Report: the verdict and the finding ids. Facts only, short.
