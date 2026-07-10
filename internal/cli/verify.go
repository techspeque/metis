package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/techspeque/metis/internal/runner"
	"github.com/techspeque/metis/internal/runs"
)

func init() {
	verifyCmd.Flags().Bool("pre", false, "Label as pre-flight verification")
	verifyCmd.Flags().Bool("post", false, "Label as post-implementation verification")
	rootCmd.AddCommand(verifyCmd)
	rootCmd.AddCommand(envCheckCmd)
	rootCmd.AddCommand(interfacesCmd)
}

var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Run the full verification pipeline",
	Long: `Runs env-check, then the configured verify command.
Exit codes: 0=pass, 1=code failure, 2=environment failure.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadContext()
		if err != nil {
			return err
		}

		l, err := ctx.loadLedger()
		if err != nil {
			return err
		}

		// Determine active slice for log storage
		sliceID := ""
		result := l.Next()
		if result != nil {
			sliceID = result.Slice.ID
		}

		// Determine label
		pre, _ := cmd.Flags().GetBool("pre")
		post, _ := cmd.Flags().GetBool("post")
		label := ""
		if pre {
			label = "pre"
		} else if post {
			label = "post"
		}

		store := runs.NewStore(filepath.Join(ctx.repoRoot, ctx.cfg.Paths.Runs))

		exitCode, err := runner.Verify(ctx.cfg, ctx.repoRoot, sliceID, label, store)
		if err != nil {
			return err
		}

		if exitCode == 0 {
			fmt.Println("verify: ALL GREEN")
		}

		if exitCode != 0 {
			os.Exit(exitCode)
		}
		return nil
	},
}

var envCheckCmd = &cobra.Command{
	Use:   "env-check",
	Short: "Verify the development environment is sound",
	Long: `Runs the configured env_check command to verify the environment.
Exit code 2 means environment failure — do NOT modify code.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadContext()
		if err != nil {
			return err
		}

		l, err := ctx.loadLedger()
		if err != nil {
			return err
		}

		sliceID := ""
		result := l.Next()
		if result != nil {
			sliceID = result.Slice.ID
		}

		store := runs.NewStore(filepath.Join(ctx.repoRoot, ctx.cfg.Paths.Runs))

		exitCode, err := runner.EnvCheck(ctx.cfg, ctx.repoRoot, sliceID, store)
		if err != nil {
			return err
		}

		if exitCode == 0 {
			fmt.Println("env-check: OK")
		}

		if exitCode != 0 {
			os.Exit(exitCode)
		}
		return nil
	},
}

var interfacesCmd = &cobra.Command{
	Use:   "interfaces",
	Short: "Regenerate the interface summary",
	Long:  `Runs the configured interfaces command to regenerate docs/generated/interfaces.txt.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadContext()
		if err != nil {
			return err
		}

		l, err := ctx.loadLedger()
		if err != nil {
			return err
		}

		sliceID := ""
		result := l.Next()
		if result != nil {
			sliceID = result.Slice.ID
		}

		store := runs.NewStore(filepath.Join(ctx.repoRoot, ctx.cfg.Paths.Runs))

		exitCode, err := runner.Interfaces(ctx.cfg, ctx.repoRoot, sliceID, store)
		if err != nil {
			return err
		}

		if exitCode != 0 {
			os.Exit(exitCode)
		}
		return nil
	},
}
