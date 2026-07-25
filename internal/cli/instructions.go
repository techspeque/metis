package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/techspeque/metis/internal/instructions"
)

func init() {
	instructionsCmd.Flags().String("for", "", "Generate risk-scaled instructions for a specific slice ID")
	rootCmd.AddCommand(instructionsCmd)

	kickoffCmd.Flags().String("role", "", "Emit only coder or reviewer flow (coder|reviewer)")
	rootCmd.AddCommand(kickoffCmd)
}

var instructionsCmd = &cobra.Command{
	Use:   "instructions",
	Short: "Emit the full agent contract",
	Long:  `Generates the dynamic agent contract from .metis/project.yaml. Use --for <id> for risk-scaled subset.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadContext()
		if err != nil {
			return err
		}

		forSlice, _ := cmd.Flags().GetString("for")

		var content string
		if forSlice != "" {
			l, err := ctx.loadLedger()
			if err != nil {
				return err
			}
			s := l.FindByID(forSlice)
			if s == nil {
				return fmt.Errorf("slice %q not found", forSlice)
			}
			content = instructions.GenerateForSlice(ctx.cfg, s, ctx.repoRoot)
		} else {
			content = instructions.Generate(ctx.cfg, ctx.repoRoot)
		}

		if jsonOutput() {
			return printJSON(cmd, map[string]string{"for": forSlice, "content": content})
		}
		fmt.Println(content)
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
		content := instructions.GenerateKickoff(ctx.cfg, role)
		if jsonOutput() {
			return printJSON(cmd, map[string]string{"role": role, "content": content})
		}
		fmt.Println(content)
		return nil
	},
}
