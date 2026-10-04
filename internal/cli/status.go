package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/techspeque/metis/internal/ledger"
)

func init() {
	rootCmd.AddCommand(statusCmd)
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Quick one-line status of the active slice",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadContext()
		if err != nil {
			return err
		}

		l, err := ctx.loadLedger()
		if err != nil {
			return err
		}

		total := len(l.Slices)
		done := 0
		for _, s := range l.Slices {
			if s.IsDone() {
				done++
			}
		}

		var pct float64
		if total > 0 {
			pct = float64(done) / float64(total) * 100
		}

		result := l.Next()

		if jsonOutput() {
			out := statusOutput{Done: done, Total: total, Percent: pct, Waiting: waitingOutput(l)}
			switch {
			case result != nil:
				out.State = "active"
				out.ID = result.Slice.ID
				out.Role = string(result.Role)
				out.AgentSlug = result.AgentSlug
			case total == 0:
				out.State = "empty"
			default:
				out.State = "none"
			}
			return printJSON(cmd, out)
		}

		if result == nil {
			if total == 0 {
				fmt.Println("(empty) | No slices | 0/0 done")
			} else {
				fmt.Printf("(none active) | %d/%d done (%.0f%%)%s\n", done, total, pct, waitingSuffix(l))
			}
			return nil
		}

		fmt.Printf("%s | %s | %s | %d/%d done (%.0f%%)%s\n",
			result.Slice.ID, result.Role, result.AgentSlug, done, total, pct, waitingSuffix(l))
		return nil
	},
}

// statusOutput is the JSON shape of 'metis status'.
type statusOutput struct {
	State     string     `json:"state"` // active, none, or empty
	ID        string     `json:"id,omitempty"`
	Role      string     `json:"role,omitempty"`
	AgentSlug string     `json:"agent_slug,omitempty"`
	Done      int        `json:"done"`
	Total     int        `json:"total"`
	Percent   float64    `json:"percent"`
	Waiting   []waitJSON `json:"waiting,omitempty"`
}

// waitingSuffix names the parked slices on the status line.
func waitingSuffix(l *ledger.Ledger) string {
	waiting := l.WaitingSlices()
	if len(waiting) == 0 {
		return ""
	}
	ids := make([]string, 0, len(waiting))
	for i := range waiting {
		ids = append(ids, waiting[i].ID)
	}
	return fmt.Sprintf(" | waiting: %s", strings.Join(ids, ", "))
}
