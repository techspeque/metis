package runner

import (
	"fmt"
	"os"
	"strings"

	"github.com/techspeque/metis/internal/config"
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

	result := Run(cfg.Commands.EnvCheck, repoRoot)

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

// Verify runs the full verification pipeline:
// 1. env-check (fail -> exit 2)
// 2. verify command (fail -> exit 1)
//
// The label determines the log file name: "verify-pre", "verify-post", or "verify-latest".
func Verify(cfg *config.Config, repoRoot string, sliceID string, label string, store *runs.Store) (int, error) {
	// Step 1: env-check
	exitCode, err := EnvCheck(cfg, repoRoot, sliceID, store)
	if err != nil {
		return 0, err
	}
	if exitCode != ExitSuccess {
		return exitCode, nil
	}

	// Step 2: verify command
	if cfg.Commands.Verify == "" {
		return ExitSuccess, nil
	}

	result := Run(cfg.Commands.Verify, repoRoot)

	// Determine log name
	logName := "verify-latest"
	switch label {
	case "pre":
		logName = "verify-pre"
	case "post":
		logName = "verify-post"
	}

	// Store the log
	if store != nil && sliceID != "" {
		logData := result.FormatLog(sliceID)
		if err := store.Write(sliceID, logName, logData, result.ExitCode); err != nil {
			return 0, fmt.Errorf("storing verify log: %w", err)
		}
	}

	if result.ExitCode != 0 {
		fmt.Fprintf(os.Stderr, "verify failed (exit %d):\n%s\n",
			result.ExitCode, strings.TrimSpace(string(result.Output)))
		return ExitCodeFailure, nil
	}

	return ExitSuccess, nil
}

// Interfaces runs the configured interfaces command.
func Interfaces(cfg *config.Config, repoRoot string, sliceID string, store *runs.Store) (int, error) {
	if cfg.Commands.Interfaces == "" {
		fmt.Println("interfaces command not configured — skipping")
		return ExitSuccess, nil
	}

	result := Run(cfg.Commands.Interfaces, repoRoot)

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
