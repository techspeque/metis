package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/techspeque/metis/internal/surface"
)

func init() {
	surfaceCmd.AddCommand(surfaceGenerateCmd)
	surfaceCmd.AddCommand(surfaceValidateCmd)
	rootCmd.AddCommand(surfaceCmd)
}

var surfaceCmd = &cobra.Command{
	Use:   "surface",
	Short: "Manage agent surface adapter files",
}

var surfaceGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Write/overwrite surface adapter files from current config",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadContext()
		if err != nil {
			return err
		}

		if err := surface.Generate(ctx.cfg, ctx.repoRoot); err != nil {
			return fmt.Errorf("generating surface adapters: %w", err)
		}

		fmt.Println("Generated surface adapters:")
		fmt.Println("  CLAUDE.md")
		fmt.Println("  AGENTS.md")
		fmt.Println("  opencode.json")
		fmt.Println("  .claude/settings.json")
		return nil
	},
}

var surfaceValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Check that adapter files exist and are not stale",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadContext()
		if err != nil {
			return err
		}

		warnings := surface.Validate(ctx.cfg, ctx.repoRoot)
		if len(warnings) == 0 {
			fmt.Println("Surface adapters: OK")
			return nil
		}

		for _, w := range warnings {
			fmt.Printf("  WARNING: %s\n", w)
		}
		return fmt.Errorf("%d surface adapter warning(s)", len(warnings))
	},
}
