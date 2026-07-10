// Package runner executes external commands and captures their output.
// It handles the verification pipeline (env-check, verify) and interfaces generation.
package runner

import (
	"bytes"
	"fmt"
	"os/exec"
	"time"
)

// Result holds the outcome of running a command.
type Result struct {
	Command  string
	Output   []byte
	ExitCode int
	Duration time.Duration
	Err      error
}

// Run executes a shell command and captures combined stdout+stderr.
func Run(command string, dir string) *Result {
	start := time.Now()

	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = dir

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	err := cmd.Run()
	duration := time.Since(start)

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
