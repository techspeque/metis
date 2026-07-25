// Package runner executes external commands and captures their output.
// It handles the verification pipeline (env-check, verify) and interfaces generation.
package runner

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

// DefaultTimeout bounds configured commands so a hung verify/env-check can
// never deadlock an unattended agent session. Overridable per project via
// commands.timeout_seconds.
const DefaultTimeout = 10 * time.Minute

// Result holds the outcome of running a command.
type Result struct {
	Command  string
	Output   []byte
	ExitCode int
	Duration time.Duration
	Err      error
}

// Run executes a shell command with a timeout, capturing combined
// stdout+stderr. On timeout the process is killed and the result carries a
// non-zero exit code with an explanatory trailer in the output.
func Run(command string, dir string, timeout time.Duration) *Result {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	start := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = dir
	cmd.WaitDelay = 5 * time.Second

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	err := cmd.Run()
	duration := time.Since(start)

	if ctx.Err() == context.DeadlineExceeded {
		fmt.Fprintf(&buf, "\nmetis: command timed out after %s (commands.timeout_seconds to adjust)\n", timeout)
		if err == nil {
			err = ctx.Err()
		}
	}

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	return &Result{
		Command:  command,
		Output:   buf.Bytes(),
		ExitCode: exitCode,
		Duration: duration,
		Err:      err,
	}
}

// FormatLog formats a run result as a timestamped log entry.
func (r *Result) FormatLog(sliceID string) []byte {
	var buf bytes.Buffer
	ts := time.Now().UTC().Format(time.RFC3339)

	fmt.Fprintf(&buf, "═══ metis ═══ %s ═══ slice: %s ═══\n", ts, sliceID)
	fmt.Fprintf(&buf, "command: %s\n", r.Command)
	fmt.Fprintf(&buf, "duration: %s\n\n", r.Duration.Round(time.Millisecond))
	buf.Write(r.Output)
	if len(r.Output) > 0 && r.Output[len(r.Output)-1] != '\n' {
		buf.WriteByte('\n')
	}
	fmt.Fprintf(&buf, "\n═══ exit code: %d ═══\n", r.ExitCode)
	return buf.Bytes()
}
