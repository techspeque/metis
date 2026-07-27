package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	removeCmd.Flags().String("reason", "", "Reason for removal (required)")
	_ = removeCmd.MarkFlagRequired("reason")
	rootCmd.AddCommand(removeCmd)
}

var removeCmd = &cobra.Command{
	Use:   "remove <id>",
	Short: "Retire a slice the plan no longer needs",
	Long: `Retires a slice from the plan: the entry moves to the archive marked
removed, with the reason recorded, and dependents' blocked_by lists drop the
retired ID. Nothing is erased — the audit trail keeps the slice and why it
left.

For work that is deliberately marked done without implementation, use
'metis skip'; for completed work, 'metis archive'.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		reason, _ := cmd.Flags().GetString("reason")

		ctx, err := loadContext()
		if err != nil {
			return err
		}

		l, err := ctx.loadLedger()
		if err != nil {
			return err
		}
		archive, err := ctx.loadArchive()
		if err != nil {
			return err
		}

		if err := l.Retire(archive, args[0], reason); err != nil {
			return err
		}

		// Archive first, then shrink the ledger: if the second write fails,
		// the slice exists in both files (recoverable) rather than neither.
		if err := ctx.saveArchive(archive); err != nil {
			return err
		}
		if err := ctx.saveLedger(l); err != nil {
			return err
		}

		fmt.Printf("Removed slice: %s (retired to archive)\n", args[0])
		ctx.commitStateSoft(args[0], "remove slice (retired)", ctx.ledgerPath(), ctx.archivePath())
		return nil
	},
}
