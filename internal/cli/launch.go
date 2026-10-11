package cli

import (
	stdcontext "context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/techspeque/metis/internal/config"
	"github.com/techspeque/metis/internal/git"
	"github.com/techspeque/metis/internal/launch"
	"github.com/techspeque/metis/internal/ledger"
)

func init() {
	launchCmd.Flags().Bool("dry-run", false, "Print the argv instead of running it")
	launchCmd.Flags().String("role", "", "Force the role: coder or reviewer")
	launchCmd.Flags().String("template", "", "Force the handoff template")
	rootCmd.AddCommand(launchCmd)
}

var launchCmd = &cobra.Command{
	Use:   "launch [<slice-id>]",
	Short: "Run one agent session with the slice's handoff prompt",
	Long: `Renders the handoff prompt for the active slice's next role (or the named
slice's), starts that role's agent headless with it in the project root,
streams its output to .metis/runs/<slice>/auto/<n>-<role>.log, and exits
with the agent's exit code.

The agent is the one 'metis next -o json' names in agent_slug: the slice's
own coder or reviewer, else the routing for its risk. Its launch argv comes
from agents.<slug>.launch, else the built-in default for its surface
({prompt}, {prompt_file}, {model}, {slice} are substituted; no shell). A
human agent, or one without a launcher, is refused: metis never
substitutes an agent. The session ends at auto.session_timeout (default
2h), which terminates its process group.

--dry-run prints the argv (the prompt elided) without running it.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadContext()
		if err != nil {
			return err
		}
		l, err := ctx.loadLedger()
		if err != nil {
			return err
		}
		id := ""
		if len(args) == 1 {
			id = args[0]
		}
		role, _ := cmd.Flags().GetString("role")
		tmpl, _ := cmd.Flags().GetString("template")
		dryRun, _ := cmd.Flags().GetBool("dry-run")

		plan, err := planSession(ctx, l, id, role, tmpl, currentAutoBranch(ctx))
		if err != nil {
			return err
		}
		if dryRun {
			if jsonOutput() {
				return printJSON(cmd, plan.view())
			}
			fmt.Printf("slice:    %s\nrole:     %s\nagent:    %s\ntemplate: %s\nlog:      %s\nargv:     %s\n",
				plan.Handoff.Slice, plan.Handoff.Role, plan.Handoff.AgentSlug, plan.Handoff.Template,
				ctx.rel(plan.LogPath), launch.Display(plan.Argv, plan.Handoff.Prompt))
			return nil
		}

		// The agent runs metis itself: it needs the repository lock.
		releaseRepoLock()
		var echo io.Writer = os.Stdout
		if jsonOutput() {
			echo = nil
		}
		sigCtx, stop := signal.NotifyContext(stdcontext.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		res, err := plan.run(sigCtx, ctx, echo)
		if err != nil {
			return err
		}
		if jsonOutput() {
			v := plan.view()
			v.ExitCode, v.TimedOut, v.Duration = &res.ExitCode, res.TimedOut, res.Duration.Round(time.Second).String()
			if err := printJSON(cmd, v); err != nil {
				return err
			}
		} else {
			fmt.Fprintf(os.Stderr, "metis: session ended (exit %d, %s); log: %s\n", res.ExitCode, res.Duration.Round(time.Second), ctx.rel(plan.LogPath))
		}
		switch {
		case res.TimedOut && res.ExitCode == 0:
			return exitWithCode(cmd, 1)
		case res.ExitCode != 0:
			return exitWithCode(cmd, res.ExitCode)
		}
		return nil
	},
}

// sessionPlan is one session, resolved and rendered, before it runs.
type sessionPlan struct {
	Handoff    *handoffResult
	Agent      config.Agent
	Argv       []string
	Session    int
	LogPath    string
	PromptPath string
	Timeout    time.Duration
}

// sessionView is the JSON shape of 'metis launch'.
type sessionView struct {
	Slice     string   `json:"slice"`
	Role      string   `json:"role"`
	AgentSlug string   `json:"agent_slug"`
	Model     string   `json:"model"`
	Template  string   `json:"template"`
	Argv      []string `json:"argv"`
	Log       string   `json:"log"`
	ExitCode  *int     `json:"exit_code,omitempty"`
	TimedOut  bool     `json:"timed_out,omitempty"`
	Duration  string   `json:"duration,omitempty"`
}

func (p *sessionPlan) view() sessionView {
	argv := make([]string, len(p.Argv))
	for i, a := range p.Argv {
		argv[i] = strings.ReplaceAll(a, p.Handoff.Prompt, "{prompt}")
	}
	return sessionView{
		Slice: p.Handoff.Slice, Role: p.Handoff.Role, AgentSlug: p.Handoff.AgentSlug,
		Model: p.Agent.Model, Template: p.Handoff.Template, Argv: argv, Log: p.LogPath,
	}
}

// launchError is a session that cannot be started for a reason metis auto
// stops on: the rule names it.
type launchError struct {
	rule string
	msg  string
}

func (e *launchError) Error() string { return e.msg }

// planSession resolves the agent of a slice's next role and renders its
// prompt and argv. It refuses — before anything is spawned — an agent the
// project does not configure, a human, or one with no launcher.
func planSession(ctx *context, l *ledger.Ledger, id, role, tmpl, branch string) (*sessionPlan, error) {
	h, err := buildHandoff(ctx, l, id, role, tmpl, branch)
	if err != nil {
		return nil, err
	}
	if h.AgentSlug == "" {
		return nil, &launchError{ruleNoLauncher, fmt.Sprintf("slice %s names no %s and no routing applies — metis edit %s --%s <slug>", h.Slice, strings.ToLower(h.Role), h.Slice, strings.ToLower(h.Role))}
	}
	agent, ok := ctx.cfg.Agents[h.AgentSlug]
	if !ok {
		return nil, &launchError{ruleNoLauncher, fmt.Sprintf("agent %q (the %s of %s) is not configured under agents", h.AgentSlug, strings.ToLower(h.Role), h.Slice)}
	}
	if agent.Surface == config.SurfaceHuman {
		return nil, &launchError{ruleHumanAgent, fmt.Sprintf("the %s of %s is %s, a person: run this session yourself (metis handoff %s)", strings.ToLower(h.Role), h.Slice, h.AgentSlug, h.Slice)}
	}
	tmplArgv, err := launch.Template(agent)
	if err != nil {
		if errors.Is(err, launch.ErrNoLauncher) {
			return nil, &launchError{ruleNoLauncher, fmt.Sprintf("agent %s: %v", h.AgentSlug, err)}
		}
		return nil, err
	}

	dir := ctx.autoDir(h.Slice)
	n := launch.NextSession(dir)
	base := fmt.Sprintf("%d-%s", n, strings.ToLower(h.Role))
	p := &sessionPlan{
		Handoff:    h,
		Agent:      agent,
		Session:    n,
		LogPath:    filepath.Join(dir, base+".log"),
		PromptPath: filepath.Join(dir, base+".prompt.md"),
		Timeout:    ctx.cfg.Auto.Timeout(),
	}
	p.Argv = launch.Argv(tmplArgv, launch.Vars{Prompt: h.Prompt, PromptFile: p.PromptPath, Model: agent.Model, Slice: h.Slice})
	return p, nil
}

// run writes the prompt file and runs the session.
func (p *sessionPlan) run(parent stdcontext.Context, ctx *context, echo io.Writer) (*launch.Result, error) {
	if err := os.MkdirAll(filepath.Dir(p.PromptPath), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(p.PromptPath, []byte(p.Handoff.Prompt), 0o644); err != nil {
		return nil, fmt.Errorf("writing the prompt file: %w", err)
	}
	return launch.Run(parent, p.Argv, ctx.repoRoot, p.LogPath, p.Timeout, echo)
}

// autoDir is a slice's auto run directory: its session logs, prompts and
// escalation.
func (c *context) autoDir(sliceID string) string {
	return filepath.Join(c.repoRoot, c.cfg.Paths.Runs, sliceID, "auto")
}

// rel shows a path relative to the repository root when it is inside it.
func (c *context) rel(path string) string {
	if r, err := filepath.Rel(c.repoRoot, path); err == nil && !strings.HasPrefix(r, "..") {
		return r
	}
	return path
}

// currentAutoBranch is the checked-out branch when it is an auto slice
// branch, else "" (the session works on the integration branch).
func currentAutoBranch(ctx *context) string {
	b, err := git.CurrentBranch(ctx.repoRoot)
	if err != nil || !strings.HasPrefix(b, git.AutoBranchPrefix) {
		return ""
	}
	return b
}
