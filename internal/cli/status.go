package cli

import (
	"fmt"

	"github.com/spf13/cobra"
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
			out := statusOutput{Done: done, Total: total, Percent: pct}
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
				fmt.Printf("(none active) | %d/%d done (%.0f%%)\n", done, total, pct)
			}
			return nil
		}

		fmt.Printf("%s | %s | %s | %d/%d done (%.0f%%)\n",
			result.Slice.ID, result.Role, result.AgentSlug, done, total, pct)
		return nil
	},
}

// statusOutput is the JSON shape of 'metis status'.
type statusOutput struct {
	State     string  `json:"state"` // active, none, or empty
	ID        string  `json:"id,omitempty"`
	Role      string  `json:"role,omitempty"`
	AgentSlug string  `json:"agent_slug,omitempty"`
	Done      int     `json:"done"`
	Total     int     `json:"total"`
	Percent   float64 `json:"percent"`
}
