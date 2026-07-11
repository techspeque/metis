package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	skipCmd.Flags().String("reason", "", "Reason for skipping (required)")
	_ = skipCmd.MarkFlagRequired("reason")
	rootCmd.AddCommand(skipCmd)

	reopenCmd.Flags().String("reason", "", "Reason for reopening (required)")
	_ = reopenCmd.MarkFlagRequired("reason")
	rootCmd.AddCommand(reopenCmd)
}

var skipCmd = &cobra.Command{
	Use:   "skip <id>",
	Short: "Skip a slice (mark as done without implementation)",
	Args:  cobra.ExactArgs(1),
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

		if err := l.Skip(args[0], reason); err != nil {
			return err
		}

		if err := ctx.saveLedger(l); err != nil {
			return err
		}

		fmt.Printf("Skipped slice: %s\n", args[0])
		return nil
	},
}

var reopenCmd = &cobra.Command{
	Use:   "reopen <id>",
	Short: "Reopen a slice for re-implementation",
	Args:  cobra.ExactArgs(1),
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

		if err := l.Reopen(args[0], reason); err != nil {
			return err
		}

		if err := ctx.saveLedger(l); err != nil {
			return err
		}

		fmt.Printf("Reopened slice: %s\n", args[0])
		return nil
	},
}
