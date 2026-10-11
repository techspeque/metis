// Package launch starts one headless agent session with one prompt and
// waits for it to exit. Metis never owns the agent: it runs the agent's own
// CLI, as configured, in the project root — auth, sandboxing and tools stay
// the agent's.
package launch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/techspeque/metis/internal/config"
)

// Defaults are the built-in launchers per known surface, used when an
// agent sets no launch of its own. They are versioned with metis; a
// project that needs other flags (sandbox, permissions) sets launch.
//
// A headless session cannot answer a permission prompt, and every session
// must run metis and git (the flips and blocks commit), so each default
// grants exactly that much: Claude Code accepts edits and pre-approves
// metis and git; Codex runs in its workspace-write sandbox (--full-auto),
// which can commit. Without these the agent stops at its first metis
// command.
var Defaults = map[string][]string{
	"claude-code": {"claude", "-p", "{prompt}", "--model", "{model}", "--permission-mode", "acceptEdits", "--allowedTools", "Bash(metis:*)", "Bash(git:*)"},
	"codex":       {"codex", "exec", "--full-auto", "--model", "{model}", "{prompt}"},
	"opencode":    {"opencode", "run", "--model", "{model}", "{prompt}"},
}

// ErrNoLauncher means the agent has no launch command and its surface
// has no built-in one.
var ErrNoLauncher = errors.New("no launcher")

// Template returns the argv template an agent launches with: its own
// launch, else the surface default. A human agent has none.
func Template(a config.Agent) ([]string, error) {
	if a.Surface == config.SurfaceHuman {
		return nil, fmt.Errorf("%w: a human agent is not launched", ErrNoLauncher)
	}
	if len(a.Launch) > 0 {
		return a.Launch, nil
	}
	if d, ok := Defaults[a.Surface]; ok {
		return d, nil
	}
	return nil, fmt.Errorf("%w: surface %q has no built-in launcher — set agents.<slug>.launch", ErrNoLauncher, a.Surface)
}

// Vars are the values substituted into an argv template.
type Vars struct {
	Prompt     string
	PromptFile string
	Model      string
	Slice      string
}

// Argv substitutes vars into an argv template. Each element is one
// argument: there is no shell, so a prompt never needs quoting.
func Argv(tmpl []string, v Vars) []string {
	r := strings.NewReplacer(
		"{prompt}", v.Prompt,
		"{prompt_file}", v.PromptFile,
		"{model}", v.Model,
		"{slice}", v.Slice,
	)
	out := make([]string, len(tmpl))
	for i, a := range tmpl {
		out[i] = r.Replace(a)
	}
	return out
}

// Display renders an argv for a person to read, with the prompt elided.
func Display(argv []string, prompt string) string {
	parts := make([]string, len(argv))
	for i, a := range argv {
		if prompt != "" && strings.Contains(a, prompt) {
			a = strings.ReplaceAll(a, prompt, fmt.Sprintf("<prompt: %d bytes>", len(prompt)))
		}
		if strings.ContainsAny(a, " \t\n\"'") {
			a = strconv.Quote(a)
		}
		parts[i] = a
	}
	return strings.Join(parts, " ")
}

var sessionRe = regexp.MustCompile(`^(\d+)-`)

// NextSession returns the next session number in a slice's auto run
// directory: one past the highest numbered log there.
func NextSession(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 1
	}
	maxN := 0
	for _, e := range entries {
		if m := sessionRe.FindStringSubmatch(e.Name()); m != nil {
			if n, err := strconv.Atoi(m[1]); err == nil && n > maxN {
				maxN = n
			}
		}
	}
	return maxN + 1
}

// Sessions lists a slice's session logs in order, oldest first.
func Sessions(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var logs []string
	for _, e := range entries {
		if sessionRe.MatchString(e.Name()) && strings.HasSuffix(e.Name(), ".log") {
			logs = append(logs, filepath.Join(dir, e.Name()))
		}
	}
	sort.Slice(logs, func(i, j int) bool {
		return num(logs[i]) < num(logs[j])
	})
	return logs
}

func num(path string) int {
	m := sessionRe.FindStringSubmatch(filepath.Base(path))
	if m == nil {
		return 0
	}
	n, _ := strconv.Atoi(m[1])
	return n
}

// Result is how one session ended.
type Result struct {
	ExitCode int
	TimedOut bool
	Duration time.Duration
}

// Failed reports whether the session exited non-zero or timed out.
func (r *Result) Failed() bool {
	return r.ExitCode != 0 || r.TimedOut
}

// Run runs argv in dir until it exits, timeout passes or parent is
// cancelled, writing its combined output to logPath and to echo (when not
// nil). On timeout or cancellation the whole process group is terminated.
// A command that cannot start is an error; one that runs and fails is a
// Result.
func Run(parent context.Context, argv []string, dir, logPath string, timeout time.Duration, echo io.Writer) (*Result, error) {
	if len(argv) == 0 {
		return nil, fmt.Errorf("empty launch command")
	}
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return nil, fmt.Errorf("creating the session log directory: %w", err)
	}
	logFile, err := os.Create(logPath)
	if err != nil {
		return nil, fmt.Errorf("creating the session log: %w", err)
	}
	defer func() { _ = logFile.Close() }()

	var out io.Writer = logFile
	if echo != nil {
		out = io.MultiWriter(logFile, echo)
	}

	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.Stdout = out
	cmd.Stderr = out
	cmd.Stdin = nil
	setProcessGroup(cmd)
	cmd.Cancel = func() error { return killGroup(cmd) }
	cmd.WaitDelay = 10 * time.Second

	start := time.Now()
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting %s: %w", argv[0], err)
	}
	err = cmd.Wait()
	res := &Result{Duration: time.Since(start)}
	if ctx.Err() == context.DeadlineExceeded && parent.Err() == nil {
		res.TimedOut = true
		_, _ = fmt.Fprintf(out, "\nmetis: session timed out after %s (auto.session_timeout); its process group was terminated\n", timeout)
	}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() >= 0 {
			res.ExitCode = exitErr.ExitCode()
		} else {
			res.ExitCode = 1
		}
	}
	return res, nil
}
