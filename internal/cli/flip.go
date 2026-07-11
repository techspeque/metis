package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	flipCmd.AddCommand(flipCodedCmd)
	flipCmd.AddCommand(flipReviewedCmd)
	flipReviewedCmd.Flags().String("agent", "", "Agent slug of the reviewer")
	rootCmd.AddCommand(flipCmd)
}

var flipCmd = &cobra.Command{
	Use:   "flip",
	Short: "Flip slice lifecycle flags",
	Long:  `Flip lifecycle flags on slices. Use 'metis flip coded <id>' or 'metis flip reviewed <id>'.`,
}

var flipCodedCmd = &cobra.Command{
	Use:   "coded <id>",
	Short: "Mark a slice as coded",
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

		if err := l.FlipCoded(args[0]); err != nil {
			return err
		}

		if err := ctx.saveLedger(l); err != nil {
			return err
		}

		fmt.Printf("Flipped coded=true: %s\n", args[0])
		return nil
	},
}

var flipReviewedCmd = &cobra.Command{
	Use:   "reviewed <id>",
	Short: "Mark a slice as reviewed (sign-off)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		agent, _ := cmd.Flags().GetString("agent")

		ctx, err := loadContext()
		if err != nil {
			return err
		}

		l, err := ctx.loadLedger()
		if err != nil {
			return err
		}

		if err := l.FlipReviewed(args[0], agent); err != nil {
			return err
		}

		if err := ctx.saveLedger(l); err != nil {
			return err
		}

		fmt.Printf("Flipped reviewed=true: %s\n", args[0])
		return nil
	},
}
