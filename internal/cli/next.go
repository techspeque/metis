package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/techspeque/metis/internal/ledger"
	"github.com/techspeque/metis/internal/slice"
)

var nextJSON bool
var nextQuiet bool

func init() {
	nextCmd.Flags().BoolVar(&nextJSON, "json", false, "Output as JSON")
	_ = nextCmd.Flags().MarkDeprecated("json", "use --output json")
	nextCmd.Flags().BoolVar(&nextQuiet, "quiet", false, "Print only the slice ID")
	rootCmd.AddCommand(nextCmd)
}

var nextCmd = &cobra.Command{
	Use:   "next",
	Short: "Find the active slice and display dispatch info",
	Long:  `Finds the highest-priority unblocked slice and reports its ID, role, required agent, and reading rules.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadContext()
		if err != nil {
			return err
		}

		l, err := ctx.loadLedger()
		if err != nil {
			return err
		}

		result := l.Next()
		waiting := waitingOutput(l)
		if result == nil {
			if nextQuiet {
				return nil
			}
			if nextJSON || jsonOutput() {
				return printJSON(cmd, map[string]any{"active": false, "waiting": waiting})
			}
			if len(waiting) == 0 {
				fmt.Println("No active slices. The backlog is empty.")
			} else {
				fmt.Println("No active slice: every pending slice is waiting.")
				printWaiting(waiting)
			}
			return nil
		}

		if nextQuiet {
			fmt.Println(result.Slice.ID)
			return nil
		}

		if nextJSON || jsonOutput() {
			return printNextJSON(cmd, result, ctx, waiting)
		}

		if err := printNextText(result, ctx); err != nil {
			return err
		}
		printWaiting(waiting)
		return nil
	},
}

// waitJSON is one parked slice as next and status report it.
type waitJSON struct {
	ID     string `json:"id"`
	Reason string `json:"reason"`
	Since  string `json:"since"`
	Until  string `json:"until,omitempty"`
}

func waitingOutput(l *ledger.Ledger) []waitJSON {
	var out []waitJSON
	waiting := l.WaitingSlices()
	for i := range waiting {
		s := &waiting[i]
		out = append(out, waitJSON{ID: s.ID, Reason: s.Waiting.Reason, Since: s.Waiting.Since, Until: s.Waiting.Until})
	}
	return out
}

func printWaiting(waiting []waitJSON) {
	for _, w := range waiting {
		line := fmt.Sprintf("Waiting: %s — %s (since %s", w.ID, w.Reason, w.Since)
		if w.Until != "" {
			line += ", until " + w.Until
		}
		fmt.Println(line + ")")
	}
}

func printNextText(result *ledger.DispatchResult, ctx *context) error {
	s := result.Slice
	agent, ok := ctx.cfg.Agents[result.AgentSlug]
	agentLabel := result.AgentSlug
	if ok {
		agentLabel = agent.Label
	}

	fmt.Printf("Active slice: %s\n", s.ID)
	fmt.Printf("  Title:          %s\n", s.Title)
	fmt.Printf("  Type:           %s\n", s.Type)
	fmt.Printf("  Priority:       %s\n", s.Priority)
	fmt.Printf("  Risk:           %s\n", s.Risk)
	if s.Stage != "" {
		fmt.Printf("  Stage:          %s\n", s.Stage)
	}
	fmt.Printf("  Role:           %s\n", result.Role)
	fmt.Printf("  Required model: %s (%s)\n", result.AgentSlug, agentLabel)
	if s.Plan != "" {
		fmt.Printf("  Plan:           %s %s\n", s.Plan, s.PlanSection)
	} else {
		fmt.Printf("  Plan:           (bespoke)\n")
	}
	fmt.Printf("  Review cycles:  %d\n", s.ReviewCycles)
	fmt.Printf("  Reading rule:   %s\n", readingRule(s.Risk))

	if result.Role == slice.RoleReviewer {
		fmt.Printf("  Brief:          .metis/briefs/%s.md\n", s.ID)
		fmt.Printf("  Commits:        git log --oneline --grep \"%s\"\n", s.ID)
	}

	return nil
}

type nextJSONOutput struct {
	Active       bool   `json:"active"`
	ID           string `json:"id"`
	Title        string `json:"title"`
	Type         string `json:"type"`
	Priority     string `json:"priority"`
	Risk         string `json:"risk"`
	Stage        string `json:"stage,omitempty"`
	Role         string `json:"role"`
	AgentSlug    string `json:"agent_slug"`
	AgentLabel   string `json:"agent_label"`
	Plan         string `json:"plan,omitempty"`
	PlanSection  string `json:"plan_section,omitempty"`
	ReviewCycles int    `json:"review_cycles"`
	ReadingRule  string `json:"reading_rule"`
	// Waiting lists the pending slices parked by metis wait.
	Waiting []waitJSON `json:"waiting,omitempty"`
}

func printNextJSON(cmd *cobra.Command, result *ledger.DispatchResult, ctx *context, waiting []waitJSON) error {
	s := result.Slice
	agent, ok := ctx.cfg.Agents[result.AgentSlug]
	agentLabel := result.AgentSlug
	if ok {
		agentLabel = agent.Label
	}

	out := nextJSONOutput{
		Active:       true,
		ID:           s.ID,
		Title:        s.Title,
		Type:         string(s.Type),
		Priority:     string(s.Priority),
		Risk:         string(s.Risk),
		Stage:        s.Stage,
		Role:         string(result.Role),
		AgentSlug:    result.AgentSlug,
		AgentLabel:   agentLabel,
		Plan:         s.Plan,
		PlanSection:  s.PlanSection,
		ReviewCycles: s.ReviewCycles,
		ReadingRule:  readingRule(s.Risk),
		Waiting:      waiting,
	}

	return printJSON(cmd, out)
}

func readingRule(risk slice.Risk) string {
	switch risk {
	case slice.RiskLow:
		return "core rules (branch/commit, DoD, scope, testing, non-goals, tooling)"
	case slice.RiskMedium:
		return "core + hot-path zones, accuracy rules, review checklist"
	case slice.RiskHigh:
		return "full contract (all sections)"
	default:
		return "unknown risk — read full contract"
	}
}
