# Rework phase-1-ws-1.2

metis handoff · template rework · slice phase-1-ws-1.2 · role Coder · agent claude-code/opus

You run as `claude-code/opus` (Claude Code (Opus)). That is the slug to state when `metis kickoff` asks you to self-identify; it matches the `agent_slug` of `metis next -o json`.

Slice: `phase-1-ws-1.2` — Sign the payload (type feat, risk high, plan .metis/plans/phase-1.md §1.2).
Branch: work on `dev`.

## Start

Run `metis kickoff --role coder` and follow it from step 1. The brief at `.metis/briefs/phase-1-ws-1.2.md` still governs.

## The findings to fix

Review 2 blocked this slice. The open findings, verbatim:

- **f-002** [P2/behavior, review 1] internal/sign/sign.go:40 — the HMAC compares with == — use hmac.Equal
- **f-003** [P1/security, review 2] internal/sign/keys.go:8 — the key is logged at debug — drop the log line

Fix only these. Do not refactor, rename or extend beyond what a finding asks: anything else reopens the review.

- One commit per finding, its message naming the finding: `metis commit -m "fix <finding-id>: <what changed>"`.
- A finding you cannot fix inside the brief's scope, or one that asks for what the brief rules out, is a decision for a person (below), not a reason to widen the scope.

## Verify and flip

1. `metis verify --post --slice phase-1-ws-1.2` must be green.
2. `metis commit --flip coded --slice phase-1-ws-1.2`.

## If you need a person

A decision only a person can make (the brief is silent or contradicts the plan, a finding asks for what the brief rules out): run `metis wait phase-1-ws-1.2 --reason "<the question, one line>" --human` and stop. Do not guess. An environment failure (`metis verify` exit 2) is reported verbatim; do not change code to work around it.

## Conventions

- Comments say why, not what

Report: each finding id and the commit that fixes it, the verify result. Facts only, short.
