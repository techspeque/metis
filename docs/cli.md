# CLI Reference

> **Audience:** both personas. Every command, subcommand, and flag, with who
> uses it. For task-oriented guidance see [commands.md](commands.md); for
> the agent protocol see [protocol.md](protocol.md).

**Audience legend** — *Human*: the developer driving the project. *Agent*:
referenced by the generated protocol (`kickoff`, instructions, templates).
*Both*: appears in both worlds.

## Global Flags

Available on every command.

| Flag | Values | Audience | Description |
|---|---|---|---|
| `-o, --output` | `text` (default), `json` | Both | Output format. Resolution: flag → `METIS_OUTPUT` env → text. Agents always pass `-o json` when reading values. |
| `-w, --workspace` | registered name | Human | Operate on a registered workspace instead of the current directory. Also via `METIS_WORKSPACE` env. Never used by agents — inside a repo, cwd discovery always wins. |
| `-h, --help` | | Both | Help for any command. |
| `--version` | | Both | Version one-liner (see also `metis version`). |

## Dispatch & Orientation

| Command | Flags | JSON | Audience |
|---|---|---|---|
| `metis next` | `--quiet` (print only the slice ID) | ✓ | Agent, Both |
| `metis status` | — | ✓ | Both |
| `metis list` | `--type <worktype>` · `--priority <p0-p3>` · `--status <pending\|coding\|reviewing\|done\|rework>` | ✓ | Human |
| `metis show <id>` | — | ✓ | Human |
| `metis progress` | — | ✓ | Human |

`next -o json` fields: `active`, `id`, `title`, `type`, `priority`, `risk`,
`stage`, `role`, `agent_slug`, `agent_label`, `plan`, `plan_section`,
`review_cycles`, `reading_rule`. `{"active": false}` means the backlog is
empty.

## Work Management

| Command | Flags | JSON | Audience |
|---|---|---|---|
| `metis add <type>` | `--title`\* · `--coder`\* · `--reviewer`\* · `--risk <low\|medium\|high>` · `--priority <p0-p3>` · `--stage` · `--plan` · `--plan-section` · `--blocked-by <id,id>` · `--id` · `--notes` · `--after <id>` · `--before <id>` | — | Both (recon) |
| `metis edit <id>` | `--title` · `--risk` · `--priority` · `--stage` · `--coder` · `--reviewer` · `--plan` · `--plan-section` · `--blocked-by` (replaces list) · `--notes` | — | Both (recon) |
| `metis seed <plan-file>` | `--append` · `--dry-run` · `--phase <n>` · `--type <worktype>` | — | Human |
| `metis recon` | `--coder` · `--reviewer` · `--priority` | — | Human |

\* required. Slice types: `feat`, `fix`, `refactor`, `debt`, `remove`,
`chore`, `security`, `gate`, `recon`.

`edit` only changes the fields whose flags are passed. Done slices are
immutable — `reopen` first.

## Slice Lifecycle

All state transitions are atomic: the commands below commit the state files
they change, so the tree is never left dirty between protocol steps.

| Command | Flags | JSON | Audience |
|---|---|---|---|
| `metis commit` | `-m/--message` · `--prefix` · `--brief` · `--flip <coded\|reviewed>` · `--agent <slug>` (identity, required for reviewed flips) · `--slice <id>` (dispatch binding) · `--amend` | — | Agent |
| `metis log <id>` | `--validate` (scope + format audit; exit 1 on violations) | ✓ | Agent (reviewer), Both |
| `metis block <id>` | `--severity <P1\|P2\|P3>` · `--category <cat>` · `--finding "<text>"` | — | Agent (reviewer) |
| `metis skip <id>` | `--reason`\* | — | Both (recon) |
| `metis remove <id>` | `--reason`\* | — | Human |
| `metis reopen <id>` | `--reason`\* | — | Human |
| `metis archive` | — | — | Agent (reviewer) |

`commit --flip coded` requires the slice's brief and a green `verify --post`
run; `--flip reviewed` requires `--agent <slug>` (validated against the
coder — set `routing.review: self` for single-agent projects). `--slice <id>`
binds the command to the dispatched slice and errors if dispatch moved on.
`log --validate` audits commit format and touched files against the brief's
`owned_paths`; a passing audit is required before `--flip reviewed` (gate
slices are exempt from the scope portion; normal slices with no declared
scope fail the audit). `add`/`edit`/`skip`/`reopen`/`rule` auto-commit their
state changes (warning instead of failing outside a git flow).
Finding categories: `auth`, `protocol`, `scope`, `tests`, `arch-dup`,
`arch-fit`, `data`, `maint`, `security`, `behavior`, `performance`.

## Verification

| Command | Flags | JSON | Audience |
|---|---|---|---|
| `metis verify` | `--pre` · `--post` · `--env` (environment check only) | — | Agent, Both |
| `metis interfaces` | — | — | Agent |
| `metis check` | `--config` · `--ledger` | ✓ | Both |

Exit codes are the contract: `0` = pass, `1` = code failure, `2` =
environment failure (agents must NOT modify code on exit 2).

## Agent-Facing Generation

| Command | Flags | JSON | Audience |
|---|---|---|---|
| `metis kickoff` | `--role <coder\|reviewer>` | ✓ | Agent |
| `metis instructions` | `--for <id>` (risk-scaled) | ✓ | Agent |
| `metis brief <id>` | `--write` | ✓ (read mode) | Agent |
| `metis surface generate` | — | — | Human |
| `metis surface validate` | — | — | Human |

`surface validate` flags adapters as stale when the config **or the metis
version** changed since the last generate.

## Configuration & Rules

| Command | Flags / args | JSON | Audience |
|---|---|---|---|
| `metis config view` | — | ✓ | Human |
| `metis config get <key>` | dotted path, e.g. `project.name`, `agents.<slug>.model` | ✓ | Both |
| `metis config set <key> <value>` | comments preserved; lists comma-separated | — | Human |
| `metis rule add "<text>"` | — | — | Both |
| `metis findings record <slice-id>` | `--severity` · `--category` · `--finding`\* | — | Agent (reviewer) — advisory, no state change |
| `metis findings resolve <finding-id>` | `--note` | — | Agent (reviewer) / Both |
| `metis rule list` | — | ✓ | Human |
| `metis rule promote <finding-id>` | — | — | Both |

## Project & User Setup

| Command | Subcommands / flags | JSON | Audience |
|---|---|---|---|
| `metis init` | `--from <config-path>` | — | Human |
| `metis findings` | `--severity` · `--category` · `--slice` · `--stats` | ✓ | Human |
| `metis workspace` (`ws`) | `add <name> [path]` · `remove <name>` · `use <name>` · `current` · `list` | ✓ (`list`, `current`) | Human only |
| `metis version` | — | ✓ | Both |
| `metis completion <shell>` | bash · zsh · fish · powershell | — | Human |

`init` migrates a legacy root `metis.yaml` to `.metis/project.yaml` and
registers the project in the user-level workspace registry.

## Environment Variables

| Variable | Purpose |
|---|---|
| `METIS_OUTPUT` | Default output format (`text`/`json`); the `-o` flag wins |
| `METIS_WORKSPACE` | Select a registered workspace by name; cwd discovery still wins inside a repo |
| `METIS_USER_CONFIG` | Override the user config location (default `~/.metis/config.yaml`); mainly for tests |
