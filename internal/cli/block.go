package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	blockCmd.Flags().String("severity", "P2", "Severity: P1|P2|P3")
	blockCmd.Flags().String("category", "", "Category: auth|protocol|scope|tests|arch-dup|arch-fit|data|maint|security|behavior|performance")
	blockCmd.Flags().String("finding", "", "Description of the finding")
	rootCmd.AddCommand(blockCmd)
}

var blockCmd = &cobra.Command{
	Use:   "block <id>",
	Short: "Block a slice during review",
	Long:  `Block a slice: resets coded=false, increments review_cycles, and records the finding.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadContext()
		if err != nil {
			return err
		}

		l, err := ctx.loadLedger()
		if err != nil {
			return err
		}

		if err := l.Block(args[0]); err != nil {
			return err
		}

		if err := ctx.saveLedger(l); err != nil {
			return err
		}

		s := l.FindByID(args[0])
		fmt.Printf("Blocked slice: %s (review_cycles=%d)\n", args[0], s.ReviewCycles)

		// TODO: append finding to .metis/findings.yaml (Phase 6)
		finding, _ := cmd.Flags().GetString("finding")
		if finding != "" {
			fmt.Printf("Finding: %s\n", finding)
		}

		return nil
	},
}
