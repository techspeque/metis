package runner

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/techspeque/metis/internal/config"
	"github.com/techspeque/metis/internal/git"
	"github.com/techspeque/metis/internal/runs"
)

// Exit codes as specified in the spec (Appendix A).
const (
	ExitSuccess     = 0
	ExitCodeFailure = 1
	ExitEnvFailure  = 2
	ExitLedgerError = 3
)

// envFailureVerdict is the loud banner printed when env-check fails.
const envFailureVerdict = `
══════════════════════════════════════════════════════════════
VERDICT: ENVIRONMENT FAILURE — NOT A CODE FAILURE.

Do NOT modify code, tests, or config to make verify pass.
Do NOT flip ledger booleans.
Stop and report this verbatim to the human.
══════════════════════════════════════════════════════════════`

// EnvCheck runs the configured env_check command.
// Returns exit code 0 on success, 2 on environment failure.
func EnvCheck(cfg *config.Config, repoRoot string, sliceID string, store *runs.Store) (int, error) {
	if cfg.Commands.EnvCheck == "" {
		// No env-check configured — pass silently
		return ExitSuccess, nil
	}

	result := Run(cfg.Commands.EnvCheck, repoRoot, cfg.CommandTimeout())

	// Store the log
	if store != nil && sliceID != "" {
		logData := result.FormatLog(sliceID)
		if err := store.Write(sliceID, "env-check", logData, result.ExitCode); err != nil {
			return 0, fmt.Errorf("storing env-check log: %w", err)
		}
	}

	if result.ExitCode != 0 {
		fmt.Fprint(os.Stderr, envFailureVerdict)
		fmt.Fprintln(os.Stderr)
		if len(result.Output) > 0 {
			fmt.Fprintf(os.Stderr, "\nenv-check output:\n%s\n", strings.TrimSpace(string(result.Output)))
		}
		return ExitEnvFailure, nil
	}

	return ExitSuccess, nil
}

// VerifyOptions tunes one verify run.
type VerifyOptions struct {
	// Label names the log: "pre", "post", or "" for "verify-latest".
	Label string
	// Force runs the whole verify command even when this tree already passed.
	Force bool
	// Scopes runs only these configured scopes, whether or not they passed.
	Scopes []string
}

// VerifyOutcome says how a passing verify passed.
type VerifyOutcome struct {
	// Cached is the earlier green run of the same tree and commands that
	// this run reused, or nil when a command ran.
	Cached *Green
	// Ran and Reused are the scopes of a scoped run; Whole says the one
	// verify command ran instead, and Reason says why when scopes exist.
	Ran    []string
	Reused []ScopeState
	Whole  bool
	Reason string
}

// Verify runs the full verification pipeline:
//  1. env-check, always (fail -> exit 2)
//  2. the verify command (fail -> exit 1), unless the same working-tree
//     content already passed with the same commands and the cache is on;
//     with verify_scopes, each scope's own command runs only when its
//     paths' content has not passed, and the whole command runs when a
//     file outside every scope changed
//
// The label determines the log file name: "verify-pre", "verify-post", or "verify-latest".
func Verify(cfg *config.Config, repoRoot string, sliceID string, opts VerifyOptions, store *runs.Store) (int, *VerifyOutcome, error) {
	outcome := &VerifyOutcome{}
	exitCode, err := EnvCheck(cfg, repoRoot, sliceID, store)
	if err != nil {
		return 0, outcome, err
	}
	if exitCode != ExitSuccess {
		return exitCode, outcome, nil
	}

	if cfg.Commands.Verify == "" {
		return ExitSuccess, outcome, nil
	}

	logName := "verify-latest"
	switch opts.Label {
	case "pre":
		logName = "verify-pre"
	case "post":
		logName = "verify-post"
	}

	// A tree that cannot be keyed (not a git repository) is simply verified.
	key, tree := "", ""
	if cfg.Commands.VerifyCacheEnabled() {
		key, tree, _ = VerifyKey(cfg, repoRoot)
	}
	scoped := len(cfg.Commands.VerifyScopes) > 0 && tree != ""
	if len(opts.Scopes) > 0 {
		if !scoped {
			return 0, outcome, fmt.Errorf("--scope needs commands.verify_scopes and a git working tree")
		}
		return verifyScopes(cfg, repoRoot, sliceID, logName, tree, opts.Scopes, outcome, store)
	}
	if scoped && !opts.Force {
		states, err := ScopeStates(cfg, repoRoot, tree)
		if err != nil {
			return 0, outcome, err
		}
		if rest := states[len(states)-1]; rest.Green != nil {
			return verifyScopes(cfg, repoRoot, sliceID, logName, tree, nil, outcome, store)
		}
		outcome.Reason = "a file outside every scope changed since the last whole run"
	}
	if key != "" && !opts.Force {
		if g := LookupGreen(repoRoot, key); g != nil {
			outcome.Cached = g
			if store != nil && sliceID != "" {
				if err := store.Write(sliceID, logName, cachedLog(sliceID, cfg.Commands.Verify, g), ExitSuccess); err != nil {
					return 0, outcome, fmt.Errorf("storing verify log: %w", err)
				}
			}
			return ExitSuccess, outcome, nil
		}
	}
	outcome.Whole = true

	result := Run(cfg.Commands.Verify, repoRoot, cfg.CommandTimeout())

	logData := result.FormatLog(sliceID)
	if store != nil && sliceID != "" {
		if err := store.Write(sliceID, logName, logData, result.ExitCode); err != nil {
			return 0, outcome, fmt.Errorf("storing verify log: %w", err)
		}
	}

	if result.ExitCode != 0 {
		fmt.Fprintf(os.Stderr, "verify failed (exit %d):\n%s\n",
			result.ExitCode, strings.TrimSpace(string(result.Output)))
		return ExitCodeFailure, outcome, nil
	}

	// The key is taken again after the run: a verify that rewrote tracked
	// or unignored files did not verify the tree it started from.
	if key != "" {
		after, afterTree, err := VerifyKey(cfg, repoRoot)
		switch {
		case err != nil:
		case after == key:
			if err := RecordGreen(repoRoot, &Green{Key: key, Tree: tree, At: time.Now().UTC(), Slice: sliceID}, logData); err != nil {
				fmt.Fprintf(os.Stderr, "metis: verify cache not recorded: %v\n", err)
			}
			// The whole run covers every scope and the rest.
			if scoped {
				if states, err := ScopeStates(cfg, repoRoot, tree); err == nil {
					for _, st := range states {
						if st.Green == nil {
							_ = RecordGreen(repoRoot, &Green{Key: st.Key, Tree: tree, At: time.Now().UTC(), Slice: sliceID, Scope: st.Name}, logData)
						}
					}
				}
			}
		default:
			changed, _ := git.ChangedPaths(repoRoot, tree, afterTree)
			fmt.Fprintf(os.Stderr, "metis: green run not remembered — verify changed files git tracks or would add (a cache or build output belongs in .gitignore): %s\n", strings.Join(changed, ", "))
		}
	}
	return ExitSuccess, outcome, nil
}

// verifyScopes runs the scopes that are not green (or exactly `only`),
// in configured order, stopping at the first failure. One log holds every
// scope's section.
func verifyScopes(cfg *config.Config, repoRoot, sliceID, logName, tree string, only []string, outcome *VerifyOutcome, store *runs.Store) (int, *VerifyOutcome, error) {
	states, err := ScopeStates(cfg, repoRoot, tree)
	if err != nil {
		return 0, outcome, err
	}
	states = states[:len(states)-1]
	if len(only) > 0 {
		wanted := map[string]bool{}
		for _, name := range only {
			wanted[name] = true
		}
		var chosen []ScopeState
		for _, st := range states {
			if wanted[st.Name] {
				chosen = append(chosen, st)
				delete(wanted, st.Name)
			}
		}
		for name := range wanted {
			return 0, outcome, fmt.Errorf("--scope %s: no such scope in commands.verify_scopes", name)
		}
		states = chosen
	}

	var log strings.Builder
	fmt.Fprintf(&log, "═══ metis ═══ %s ═══ slice: %s ═══ scoped verify ═══\n", time.Now().UTC().Format(time.RFC3339), sliceID)
	write := func(exitCode int) error {
		if store == nil || sliceID == "" {
			return nil
		}
		if err := store.Write(sliceID, logName, []byte(log.String()), exitCode); err != nil {
			return fmt.Errorf("storing verify log: %w", err)
		}
		return nil
	}
	for _, st := range states {
		if st.Green != nil && len(only) == 0 {
			outcome.Reused = append(outcome.Reused, st)
			fmt.Fprintf(&log, "\n── scope %s: reused, passed at %s", st.Name, st.Green.At.Format(time.RFC3339))
			if st.Green.Slice != "" {
				fmt.Fprintf(&log, " (slice %s)", st.Green.Slice)
			}
			if st.Green.Log != "" {
				fmt.Fprintf(&log, "; log %s", st.Green.Log)
			}
			log.WriteString("\n")
			continue
		}
		outcome.Ran = append(outcome.Ran, st.Name)
		result := Run(st.Command, repoRoot, cfg.CommandTimeout())
		fmt.Fprintf(&log, "\n── scope %s: ran\n%s", st.Name, result.FormatLog(sliceID))
		if result.ExitCode != 0 {
			if err := write(result.ExitCode); err != nil {
				return 0, outcome, err
			}
			fmt.Fprintf(os.Stderr, "verify failed in scope %s (exit %d):\n%s\n", st.Name, result.ExitCode, strings.TrimSpace(string(result.Output)))
			return ExitCodeFailure, outcome, nil
		}
		// The scope's content is read again: a command that rewrote its
		// own files did not verify what it started from.
		_, afterTree, err := VerifyKey(cfg, repoRoot)
		if err != nil || afterTree != tree {
			fmt.Fprintf(os.Stderr, "metis: scope %s not remembered — its command changed files git tracks or would add\n", st.Name)
			continue
		}
		if err := RecordGreen(repoRoot, &Green{Key: st.Key, Tree: tree, At: time.Now().UTC(), Slice: sliceID, Scope: st.Name}, result.FormatLog(sliceID)); err != nil {
			fmt.Fprintf(os.Stderr, "metis: verify cache not recorded: %v\n", err)
		}
	}
	if err := write(ExitSuccess); err != nil {
		return 0, outcome, err
	}
	return ExitSuccess, outcome, nil
}

// cachedLog is the log of a run that reused an earlier green run.
func cachedLog(sliceID, command string, g *Green) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "═══ metis ═══ %s ═══ slice: %s ═══\n", time.Now().UTC().Format(time.RFC3339), sliceID)
	fmt.Fprintf(&b, "command: %s\n", command)
	fmt.Fprintf(&b, "cached: tree %s passed at %s", g.Tree, g.At.Format(time.RFC3339))
	if g.Slice != "" {
		fmt.Fprintf(&b, " (slice %s)", g.Slice)
	}
	b.WriteString("; not re-run — 'metis verify --force' runs it\n")
	if g.Log != "" {
		fmt.Fprintf(&b, "log: %s\n", g.Log)
	}
	return []byte(b.String())
}

// Interfaces runs the configured interfaces command.
func Interfaces(cfg *config.Config, repoRoot string, sliceID string, store *runs.Store) (int, error) {
	if cfg.Commands.Interfaces == "" {
		fmt.Println("interfaces command not configured — skipping")
		return ExitSuccess, nil
	}

	result := Run(cfg.Commands.Interfaces, repoRoot, cfg.CommandTimeout())

	// Store the log
	if store != nil && sliceID != "" {
		logData := result.FormatLog(sliceID)
		if err := store.Write(sliceID, "interfaces", logData, result.ExitCode); err != nil {
			return 0, fmt.Errorf("storing interfaces log: %w", err)
		}
	}

	if result.ExitCode != 0 {
		fmt.Fprintf(os.Stderr, "interfaces command failed (exit %d):\n%s\n",
			result.ExitCode, strings.TrimSpace(string(result.Output)))
		return ExitCodeFailure, nil
	}

	return ExitSuccess, nil
}
