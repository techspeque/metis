# Metis Auto: hardening from use

Status: draft, 2026-11-27. Replaces the first bullet of Phase 4 in
[`auto.md`](auto.md) and adds two items that Phase 4 did not foresee. The
other Phase 4 bullets (`--watch`, cycle statistics, unattended gates) stay
where they are, unscheduled.

Three changes, in the order they should ship. Each is one pull request and
one patch release.

| # | Change | Why now | Size |
|---|---|---|---|
| 1 | Rate limits: wait and retry | The reviewer's usage limit stopped a run twice in one day while building Ship's Phase 0 | about a day |
| 2 | State files survive a rebase | A slice run out of order cannot land when both branches touched the ledger or findings | about a day |
| 3 | Remote sessions: push before, fetch after | `gaia run` as a launcher needs it (Gaia ADR-0010); nothing in Metis fetches or pushes around a session today | half a day |

The design laws of `auto.md` hold throughout: the ledger is the only state,
a stop is a result, rules bound the loop, and Metis never owns the agent's
process.

---

## 1. Rate limits: wait and retry

### Today

`runSession` (`internal/cli/auto.go`) counts any failed session that made no
progress towards `auto.stop.session_failed` (default one retry) and retries
at once. A usage limit therefore burns the retry in seconds and parks the
slice for a person. Nothing reads the session log.

### Design

A failed session is classified before it is counted:

1. **Limited:** the agent's exit code is in `rate_limit.exit_codes`, or the
   tail of the session log (last 16 KB) matches one of
   `rate_limit.patterns`.
2. **Failed:** anything else, handled as today.

A limited session does not count towards `session_failed` or `no_progress`.
Auto waits and runs the same session again:

- If `rate_limit.reset_pattern` matches the log and captures a time, wait
  until then plus a minute.
- Otherwise back off: 15 minutes, doubling, capped at one hour.
- The waiting slice is marked with `metis wait <id> --reason "auto:
  rate_limited" --until <time>`, without `--human`, so the ledger says why
  nothing is happening and a killed run resumes by itself once the time
  passes.
- Total waiting per slice is bounded by `auto.rate_limit.max_wait` (default
  6h). Past it, auto stops with the new rule `rate_limited`, exit 3, parked
  with `--until` and not `--human`: the next `metis auto` carries on.
- Time spent waiting is not charged to the slice's wall-clock budget.
- An interrupt during the wait exits 130 and leaves the `--until` in place.

A limited session that did make progress (commits, a flip) is treated as
progress; the next session meets the limit and waits.

### Configuration

```yaml
agents:
  codex/gpt-6-astra:
    rate_limit:
      exit_codes: []
      patterns: ["usage limit", "rate limit", "429"]
      reset_pattern: "resets? (?:at|in) (.+)"
auto:
  rate_limit:
    max_wait: 6h
    backoff: 15m
```

Each surface Metis knows gets default patterns. They are taken from real
logs, not guessed: the two stopped reviewer sessions in Ship's
`.metis/runs/` are the first samples, and each becomes a test fixture.
`metis check` rejects a pattern that does not compile.

### Optional, same release if cheap: a fallback agent

`agents.<slug>.fallback: <slug>` names an agent to use for the rest of the
slice when the first is limited beyond `backoff`. `metis check` refuses a
reviewer fallback that is the slice's coder. The ledger records which agent
ran each session, as now. Leave it out if it complicates the loop.

### Tests

Fake agent prints a limit message and exits non-zero, then succeeds: the
slice completes, `session_failed` is not charged, and the wait is recorded.
Limit lasting past `max_wait`: stop `rate_limited`, resumable. Reset time
parsed from the log. Interrupt during the wait. A non-matching failure
behaves exactly as before.

---

## 2. State files survive a rebase

### Today

Each `auto/<slice>` branch carries its own commits to `.metis/slices.yaml`,
`.metis/slices-done.yaml` and `.metis/findings.yaml`. `landBranch` rebases
the branch when the work branch has moved, and aborts on the first conflict
with stop `merge`. Two branches conflict almost always, for four reasons:

- `ledger.Save` and `findings.Save` rewrite the whole file.
- Archiving deletes the slice from the ledger and strips its id from other
  slices' `blocked_by`, touching neighbours.
- Both branches append to the end of `slices-done.yaml`.
- `findings.Store.Add` numbers a finding one past the highest id it can
  see, so two branches both create `f-005`.

None of these is a real disagreement. The files are lists keyed by id, and
each branch changes only its own slice's entries.

### Design

`landBranch` resolves conflicts itself when, and only when, every conflicted
path in a rebase step is one of the three state files. A conflict in any
other file aborts as today.

The resolution is a three-way merge on the parsed data, not on text:

| File | Key | Rule |
|---|---|---|
| `slices.yaml` | slice id | An entry changed on one side takes that side. An entry removed on one side (archived) is removed. `blocked_by` is recomputed against the merged archive. The same entry changed on both sides is a real conflict: abort. |
| `slices-done.yaml` | slice id | Union, ordered by archive time. The same id on both sides with different content: abort. |
| `findings.yaml` | finding id | Union. An id created on both sides for different findings is renumbered on the branch being landed, to the next free id, and keeps `alias: <old id>` so earlier citations in briefs, commit messages and reviews still resolve. |

`metis findings` and the handoff templates resolve an alias wherever an id
is accepted. The merged files are written with the normal `Save`, staged,
and the rebase continues. Verify runs after the rebase, as now.

The same merge is exposed as `metis state merge <base> <ours> <theirs>` so
it can be a git merge driver for people who rebase by hand; `metis init`
does not install it.

### As built

- The merge runs at every rebase step where both sides changed a state
  file, not only on a conflict: `landBranch` runs the binary's own `metis
  state merge` as a git merge driver for that rebase alone (`git -c
  merge.metis-<kind>.driver=… -c core.attributesFile=<temp file>`).
  Nothing is written to the repository or the user's configuration. A
  conflict git reports anyway (the driver could not run, a state path the
  attributes file cannot name) is settled in process by the same merge.
- The rebase uses `--reapply-cherry-picks`. Two slices' flips are the same
  patch to git, and a plain rebase drops the second as already upstream.
- `slices-done.yaml` is ordered ours first, then theirs: slices carry no
  archive time. In a landing that is landing order.
- `blocked_by` is merged as a set and then stripped of every slice that
  left the merged ledger, not recomputed against the archive: the same
  result, and the driver merges one file at a time.
- An alias is by construction an id another finding holds. The id's own
  finding wins; an alias resolves only when no finding has the id. A
  command given an id that is also an alias says so on stderr, and
  `metis findings` and the handoff prompts show `(was f-NNN)`.
- A later step of the same rebase still names a renumbered finding by its
  old id; it is matched by alias plus slice, date and text.
- `metis state merge` takes `--path` (git's `%P`) or `--kind`, since git
  hands a driver temporary files. It writes to `<ours>` and exits 1 on a
  conflict.
- "A's archive strips B's `blocked_by`" is tested in `metis auto` as a
  third slice blocked by both, since a blocked slice is not dispatched;
  the literal case is a unit test of the ledger merge.

### Not doing

- Per-slice state files. It would end the conflicts at the source but
  changes every consumer of the ledger, including Gaia's reader.
- Union merge through `.gitattributes`. It cannot handle the deletion from
  `slices.yaml` or the id collision.

### Tests

Two auto branches, B cut before A landed: both archive; both block with a
colliding finding id; A's archive strips B's `blocked_by`. Each lands
cleanly and `metis check` passes afterwards. A conflict in a source file
still stops with `merge`. A slice entry edited on both sides still stops.
A cited old finding id resolves through its alias.

---

## 3. Remote sessions: push before, fetch after

### Today

Metis runs the launcher in the repository and reads the outcome from the
local ledger and `git log`. It never fetches, and pushes only for
`auto.branch: pr`. A launcher that runs the agent on another machine (`gaia
run`, or plain `ssh`) leaves the agent's commits on the remote, where Metis
does not look. The session then reads as no progress.

### Design

One per-agent switch, with no knowledge of Gaia (Gaia ADR-0001):

```yaml
agents:
  claude-code/opus-5-5:
    remote: true
    transient_exit_codes: [91, 95]
    launch: [gaia, run, --node, studio, --wait, --, claude, -p, "{prompt}", …]
```

For a session whose agent has `remote: true`, `metis launch` and `metis
auto`:

1. **Before:** push the current branch to `origin`, setting upstream. A
   rejected push stops with the new rule `sync`.
2. **Run** the launcher as now.
3. **After:** `git fetch origin <branch>` and fast-forward. A branch that
   cannot fast-forward stops with `sync`. Metis never rebases or resets
   here.
4. Read the outcome from the ledger, as now.

When a slice with any remote agent lands, the work branch is pushed and the
remote `auto/<slice>` branch is deleted.

`transient_exit_codes` lists launcher exit codes that mean "try again
later" and not "the agent failed". They are handled by the waiting logic of
item 1, with the same backoff and bound. For `gaia run` they are 91 (node
unavailable) and 95 (session lost); the full table is in Gaia's
`docs/contracts/launcher-v1.md`. Any other non-zero code is a failed
session, as today, and the launcher's own stderr is in the session log.

### Tests

A fake remote launcher that clones a bare remote elsewhere, commits and
pushes: the flip is seen and the slice completes. Caller cannot push: stop
`sync`. Remote diverged: stop `sync`, nothing rewritten. A transient code
waits and retries. With `remote` unset, no `git push` or `git fetch` is
ever run, checked in a test.

---

## Order and release

1. Item 1, release, and use it for the rest of Ship's Phase 1.
2. Item 2, release.
3. Item 3, release, before Gaia's Phase 1 workstream 1.8, which otherwise
   uses a wrapper script.

`docs/commands.md` and the `auto.md` stop-rule table gain `rate_limited`
and `sync` with each release.
