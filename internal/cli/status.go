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

		result := l.Next()
		if result == nil {
			if total == 0 {
				fmt.Println("(empty) | No slices | 0/0 done")
			} else {
				fmt.Printf("(none active) | %d/%d done (%.0f%%)\n", done, total,
					float64(done)/float64(total)*100)
			}
			return nil
		}

		pct := float64(done) / float64(total) * 100
		fmt.Printf("%s | %s | %s | %d/%d done (%.0f%%)\n",
			result.Slice.ID, result.Role, result.AgentSlug, done, total, pct)
		return nil
	},
}
