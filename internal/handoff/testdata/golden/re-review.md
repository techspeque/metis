# Re-review phase-1-ws-1.2

metis handoff · template re-review · slice phase-1-ws-1.2 · role Reviewer · agent codex/gpt

You run as `codex/gpt` (Codex (GPT)). That is the slug to state when `metis kickoff` asks you to self-identify; it matches the `agent_slug` of `metis next -o json`.

Slice: `phase-1-ws-1.2` — Sign the payload (type feat, risk high, plan .metis/plans/phase-1.md §1.2).
Branch: work on `auto/phase-1-ws-1.2`. metis auto cut it from `dev` for this slice; `metis commit` accepts it.

## Start

Run `metis kickoff --role reviewer` and follow it from step 1. The brief at `.metis/briefs/phase-1-ws-1.2.md` is the source of truth.

## What this review covers

This is review 3. Review 2 blocked the slice with:

- **f-003** [P1/security, review 2] internal/sign/keys.go:8 — the key is logged at debug — drop the log line

Still open from earlier reviews:

- **f-002** [P2/behavior, review 1] internal/sign/sign.go:40 — the HMAC compares with == — use hmac.Equal

The commits since that review:

- 2222222 fix(phase-1-ws-1.2): fix f-003: drop the key log line
- 3333333 chore(phase-1-ws-1.2): flip coded

For each finding above, in this one review:

- Fixed: `metis findings resolve <f-id> --note "<commit>"`.
- Not fixed: re-raise it, citing its id: `metis block phase-1-ws-1.2 --severity <as before> --category <as before> --finding "re-raise <f-id>: <what is still wrong>"`.

**Narrowed review.** Do not re-review the whole slice. A new blocking finding is allowed only at P1 (a demonstrated defect that matters); every other new observation is an advisory. Do not reopen what the brief decides or what an earlier review accepted.

A finding blocks only when it is one of:
- a deviation from the brief
- a missing or failing test the brief lists
- a broken accuracy rule
- a demonstrated defect

Anything else is an advisory: `metis findings record phase-1-ws-1.2 --severity P3 --category <category> --finding "..."` (no state change).

## Verdict

1. `metis log phase-1-ws-1.2 --validate` — a FAIL is a block.
2. Pass: every finding resolved, `metis verify --post --slice phase-1-ws-1.2` green, then `metis commit --flip reviewed --agent codex/gpt --slice phase-1-ws-1.2`, then `metis archive`.
3. Block: the `metis block` calls above, all in this session.

## If you need a person

A decision only a person can make (the brief is silent or contradicts the plan, a finding asks for what the brief rules out): run `metis wait phase-1-ws-1.2 --reason "<the question, one line>" --human` and stop. Do not guess. An environment failure (`metis verify` exit 2) is reported verbatim; do not change code to work around it.

Report: each finding id with resolved or re-raised, any new finding. Facts only, short.
