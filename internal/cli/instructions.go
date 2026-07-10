package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/techspeque/metis/internal/instructions"
)

func init() {
	instructionsCmd.Flags().String("for", "", "Generate risk-scaled instructions for a specific slice ID")
	instructionsCmd.Flags().Bool("json", false, "Output as JSON (not yet implemented)")
	rootCmd.AddCommand(instructionsCmd)

	kickoffCmd.Flags().String("role", "", "Emit only coder or reviewer flow (coder|reviewer)")
	rootCmd.AddCommand(kickoffCmd)
}

var instructionsCmd = &cobra.Command{
	Use:   "instructions",
	Short: "Emit the full agent contract",
	Long:  `Generates the dynamic agent contract from metis.yaml. Use --for <id> for risk-scaled subset.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadContext()
		if err != nil {
			return err
		}

		forSlice, _ := cmd.Flags().GetString("for")

		if forSlice != "" {
			l, err := ctx.loadLedger()
			if err != nil {
				return err
			}
			s := l.FindByID(forSlice)
			if s == nil {
				return fmt.Errorf("slice %q not found", forSlice)
			}
			fmt.Println(instructions.GenerateForSlice(ctx.cfg, s))
		} else {
			fmt.Println(instructions.Generate(ctx.cfg))
		}

		return nil
	},
}

var kickoffCmd = &cobra.Command{
	Use:   "kickoff",
	Short: "Emit the session protocol",
	Long:  `Generates the step-by-step procedure an agent follows from session start.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadContext()
		if err != nil {
			return err
		}

		role, _ := cmd.Flags().GetString("role")
		fmt.Println(instructions.GenerateKickoff(ctx.cfg, role))
		return nil
	},
}
