package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/techspeque/metis/internal/runner"
	"github.com/techspeque/metis/internal/runs"
)

func init() {
	verifyCmd.Flags().Bool("pre", false, "Label as pre-flight verification")
	verifyCmd.Flags().Bool("post", false, "Label as post-implementation verification")
	verifyCmd.Flags().Bool("env", false, "Run only the environment soundness check")
	rootCmd.AddCommand(verifyCmd)
	rootCmd.AddCommand(interfacesCmd)
}

var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Run the full verification pipeline",
	Long: `Runs the environment soundness check, then the configured verify command.
With --env, runs only the environment check.
Exit codes: 0=pass, 1=code failure, 2=environment failure (do NOT modify code).`,
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

		store := runs.NewStore(filepath.Join(ctx.repoRoot, ctx.cfg.Paths.Runs))

		envOnly, _ := cmd.Flags().GetBool("env")
		if envOnly {
			exitCode, err := runner.EnvCheck(ctx.cfg, ctx.repoRoot, sliceID, store)
			if err != nil {
				return err
			}
			if exitCode == 0 {
				fmt.Println("environment: OK")
				return nil
			}
			return exitWithCode(cmd, exitCode)
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

		exitCode, err := runner.Verify(ctx.cfg, ctx.repoRoot, sliceID, label, store)
		if err != nil {
			return err
		}

		if exitCode == 0 {
			fmt.Println("verify: ALL GREEN")
			return nil
		}
		return exitWithCode(cmd, exitCode)
	},
}

var interfacesCmd = &cobra.Command{
	Use:   "interfaces",
	Short: "Regenerate the interface summary",
	Long:  `Runs the configured interfaces command to regenerate .metis/interfaces.txt.`,
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
			return exitWithCode(cmd, exitCode)
		}
		return nil
	},
}
