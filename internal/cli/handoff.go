package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/techspeque/metis/internal/findings"
	"github.com/techspeque/metis/internal/git"
	"github.com/techspeque/metis/internal/handoff"
	"github.com/techspeque/metis/internal/ledger"
	"github.com/techspeque/metis/internal/slice"
)

func init() {
	handoffCmd.Flags().String("role", "", "Force the role: coder or reviewer")
	handoffCmd.Flags().String("template", "", "Force the template: "+strings.Join(handoff.Templates, ", "))
	rootCmd.AddCommand(handoffCmd)
}

var handoffCmd = &cobra.Command{
	Use:   "handoff [<slice-id>]",
	Short: "Print the prompt for a slice's next session",
	Long: `Prints the prompt for the active slice's next role (or the named slice's),
chosen from ledger state:

  not coded, review_cycles 0    code         kickoff, the brief, scope, verify, flip
  not coded, review_cycles ≥ 1  rework       the open findings verbatim; fix only these
  coded, review_cycles 0        review       this slice only; every issue in one review
  coded, review_cycles ≥ 1      re-review    the last review's findings and the commits since
  gate, reviewer                gate-review  the review contract plus the brief's evidence index

Templates are overridable in .metis/templates/handoff/<name>.md.tmpl. A
brief's "## Review" section replaces the project checklist in a review
prompt. Paste the output into the agent named in the prompt, or let
'metis launch' / 'metis auto' run it.`,
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
		h, err := buildHandoff(ctx, l, id, role, tmpl, "")
		if err != nil {
			return err
		}
		if jsonOutput() {
			return printJSON(cmd, h)
		}
		_, err = fmt.Fprint(cmd.OutOrStdout(), h.Prompt)
		return err
	},
}

// handoffResult is a rendered prompt and what it was rendered for; it is
// also the JSON shape of 'metis handoff'.
type handoffResult struct {
	Slice     string `json:"slice"`
	Role      string `json:"role"`
	AgentSlug string `json:"agent_slug"`
	Template  string `json:"template"`
	Prompt    string `json:"prompt"`
}

// buildHandoff renders the prompt for a slice (the active one when id is
// empty). role and tmpl force a choice; branch names the branch the
// session works on (empty: the integration branch).
func buildHandoff(ctx *context, l *ledger.Ledger, id, role, tmpl, branch string) (*handoffResult, error) {
	var s *slice.Slice
	if id == "" {
		next := l.Next()
		if next == nil {
			return nil, fmt.Errorf("no active slice — name one: metis handoff <slice-id>")
		}
		s = next.Slice
	} else if s = l.FindByID(id); s == nil {
		return nil, fmt.Errorf("slice %q not found in the ledger", id)
	}
	if s.IsDone() {
		return nil, fmt.Errorf("slice %s is done — nothing to hand off", s.ID)
	}

	r := s.ActiveRole()
	switch strings.ToLower(role) {
	case "":
	case "coder":
		r = slice.RoleCoder
	case "reviewer":
		r = slice.RoleReviewer
	default:
		return nil, fmt.Errorf("--role must be coder or reviewer")
	}
	if tmpl == "" {
		tmpl = handoff.Choose(s, r)
	} else if !handoff.ValidTemplate(tmpl) {
		return nil, fmt.Errorf("--template must be one of %s", strings.Join(handoff.Templates, ", "))
	} else if role == "" {
		r = handoff.RoleOf(tmpl)
	}

	store, err := findings.Load(filepath.Join(ctx.repoRoot, ctx.cfg.Paths.Findings))
	if err != nil {
		return nil, err
	}
	commits, err := git.SliceCommits(ctx.repoRoot, s.ID)
	if err != nil {
		// Outside git there is no history to list; the prompt still renders.
		commits = nil
	}
	slug := agentFor(ctx, s, r)
	prompt, err := handoff.Render(&handoff.Input{
		Cfg:       ctx.cfg,
		RepoRoot:  ctx.repoRoot,
		Slice:     s,
		Role:      r,
		AgentSlug: slug,
		Findings:  store.ForSlice(s.ID),
		Commits:   commits,
		Branch:    branch,
	}, tmpl)
	if err != nil {
		return nil, err
	}
	return &handoffResult{Slice: s.ID, Role: string(r), AgentSlug: slug, Template: tmpl, Prompt: prompt}, nil
}

// agentFor resolves the agent of a slice's role: the slice's own coder or
// reviewer from the ledger, else the project's routing for its risk, else
// routing.review when that names an agent. Never a substitute: an empty
// result means no agent is named.
func agentFor(ctx *context, s *slice.Slice, role slice.Role) string {
	own := s.Coder
	if role == slice.RoleReviewer {
		own = s.Reviewer
	}
	if own != "" {
		return own
	}
	var routed []string
	switch s.Risk {
	case slice.RiskHigh:
		routed = ctx.cfg.Routing.High
	case slice.RiskMedium:
		routed = ctx.cfg.Routing.Medium
	case slice.RiskLow:
		routed = ctx.cfg.Routing.Low
	}
	for _, slug := range routed {
		if role == slice.RoleCoder || slug != s.Coder {
			return slug
		}
	}
	if _, ok := ctx.cfg.Agents[ctx.cfg.Routing.Review]; ok {
		return ctx.cfg.Routing.Review
	}
	return ""
}
