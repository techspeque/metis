# Code phase-1-ws-1.1

metis handoff · template code · slice phase-1-ws-1.1 · role Coder · agent claude-code/opus

You run as `claude-code/opus` (Claude Code (Opus)). That is the slug to state when `metis kickoff` asks you to self-identify; it matches the `agent_slug` of `metis next -o json`.

Slice: `phase-1-ws-1.1` — Retry the webhook (type feat, risk medium, plan .metis/plans/phase-1.md §1.1).
Branch: work on `dev`.

## Start

Run `metis kickoff --role coder` and follow it from step 1.

## Brief

There is no brief yet. Write it before any code: `metis brief phase-1-ws-1.1 --write`, fill it from the plan section (goal, owned_paths, definition of done, test plan), then `metis commit --brief`.

## Scope

Edit only the files the brief's owned_paths declare. A fix genuinely needed outside them goes in the brief's "Out-of-scope touches" with what, where and why.

## Verify and flip

1. Commit your work with `metis commit -m "<message>"` (stage the files first).
2. `metis verify --post --slice phase-1-ws-1.1` must be green.
3. `metis commit --flip coded --slice phase-1-ws-1.1`.

## If you need a person

A decision only a person can make (the brief is silent or contradicts the plan, a finding asks for what the brief rules out): run `metis wait phase-1-ws-1.1 --reason "<the question, one line>" --human` and stop. Do not guess. An environment failure (`metis verify` exit 2) is reported verbatim; do not change code to work around it.

## Conventions

- Comments say why, not what

Report: files changed, the verify result. Facts only, short.
