# Contributing to Metis

Thank you for your interest in contributing to Metis. This guide explains
how to get set up, how we work, and how to submit changes.

## Prerequisites

- Go 1.22 or later
- git
- [golangci-lint](https://golangci-lint.run/welcome/install/) (for linting)

## Getting Started

1. **Fork** the repository on GitHub
2. **Clone** your fork:
   ```bash
   git clone https://github.com/<your-username>/metis.git
   cd metis
   ```
3. **Verify** the build works:
   ```bash
   go build ./...
   go test ./...
   ```

## Branch Naming

All branches must follow this naming convention:

| Prefix | Use for |
|---|---|
| `feature/` | New features or enhancements |
| `fix/` | Bug fixes |
| `docs/` | Documentation changes |
| `chore/` | Maintenance, dependencies, CI |
| `refactor/` | Code restructuring (no behavior change) |
| `test/` | Adding or improving tests |

Examples:
```bash
git checkout -b feature/add-parallel-dispatch
git checkout -b fix/ledger-validation-edge-case
git checkout -b docs/improve-workflow-guide
```

CI will reject PRs from branches that don't match these patterns.

## Development Workflow

```bash
# 1. Create a branch from main
git checkout main
git pull origin main
git checkout -b feature/my-change

# 2. Make your changes
# ...

# 3. Run checks locally
go build ./...
go vet ./...
go test -race ./...
golangci-lint run

# 4. Commit with conventional commit messages
git commit -m "feat: add parallel dispatch support"

# 5. Push and create PR
git push origin feature/my-change
# Open PR targeting main on GitHub
```

## Commit Messages

We use [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <description>

[optional body]
```

**Types:** `feat`, `fix`, `refactor`, `docs`, `test`, `chore`

**Examples:**
```
feat(dispatch): add --agent flag for filtered next
fix(ledger): handle circular dependency detection edge case
docs: improve quickstart guide
test(config): add validation tests for empty agents
refactor(cli): extract common context loading
chore: update golangci-lint to v1.60
```

## Pull Request Process

1. **Target branch:** `main`
2. **CI must pass:** build, vet, lint, and all tests green
3. **Title:** descriptive, follows commit message format
4. **Description:** explain what and why (link to issue if applicable)
5. **Scope:** keep PRs focused — one logical change per PR

### PR Checklist

- [ ] Branch name follows convention (`feature/`, `fix/`, etc.)
- [ ] All tests pass (`go test -race ./...`)
- [ ] Lint passes (`golangci-lint run`)
- [ ] New code has tests where appropriate
- [ ] Commit messages follow conventional commits

## Code Style

- Standard Go conventions (`gofmt` is enforced)
- Keep functions short and focused
- Package-level documentation for every package
- Exported types and functions need doc comments
- Error messages are lowercase, no trailing punctuation
- Use table-driven tests where appropriate

## Project Structure

```
cmd/metis/          — CLI entry point
internal/
├── cli/            — Command registration (cobra)
├── config/         — metis.yaml parsing + validation
├── ledger/         — Slice CRUD + lifecycle + dispatch
├── slice/          — Domain types (Slice, WorkType, Priority, Risk)
├── brief/          — Brief template generation
├── instructions/   — Dynamic agent contract engine
├── runner/         — External command execution
├── git/            — Git operations + enforcement
├── surface/        — Adapter file generation
├── findings/       — Review findings store
├── progress/       — Terminal dashboard
├── seed/           — Plan file parsing
├── runs/           — Run log storage
└── templates/      — Document template content
testdata/           — Test fixtures
docs/               — Documentation
scripts/            — Development scripts
```

## Reporting Issues

Use the [issue templates](https://github.com/techspeque/metis/issues/new/choose)
on GitHub. Please include:

- For bugs: steps to reproduce, expected vs actual behavior, Go version, OS
- For features: use case, proposed behavior, why it matters

## Questions?

Open a [discussion](https://github.com/techspeque/metis/discussions) on GitHub
or reach out via issues.
