package cli

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/techspeque/metis/internal/runner"
	"github.com/techspeque/metis/internal/runs"
)

func init() {
	verifyCmd.Flags().Bool("pre", false, "Label as pre-flight verification")
	verifyCmd.Flags().Bool("post", false, "Label as post-implementation verification")
	verifyCmd.Flags().Bool("env", false, "Run only the environment soundness check")
	verifyCmd.Flags().Bool("force", false, "Run the verify command even when this working tree already passed")
	verifyCmd.Flags().String("slice", "", "The slice ID you were dispatched (errors if dispatch has moved on)")
	rootCmd.AddCommand(verifyCmd)
	rootCmd.AddCommand(interfacesCmd)
}

var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Run the full verification pipeline",
	Long: `Runs the environment soundness check, then the configured verify command.
With --env, runs only the environment check.

A green run is remembered by the working tree's content (as git would stage
it: .gitignore'd files and metis's own ledger, briefs, findings and runs
excluded) and the configured commands. On the same content the env check
still runs and the earlier green run is reused, not repeated. --force runs
the command regardless; commands.verify_cache: false turns reuse off.
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

		// Bind to the dispatched slice: without this, a p0 slice arriving
		// mid-session would silently key this run's log to the wrong slice
		// and pre-satisfy its flip-coded precondition.
		if claimed, _ := cmd.Flags().GetString("slice"); claimed != "" && claimed != sliceID {
			return fmt.Errorf("slice mismatch: the active slice is %s but you passed --slice %s — if %s is what you were dispatched, dispatch has moved on; re-run 'metis next' and report to the human", sliceID, claimed, claimed)
		}

		store := runs.NewStore(filepath.Join(ctx.repoRoot, ctx.cfg.Paths.Runs))

		// The configured command may run for minutes; holding the exclusive
		// repository lock through it would starve every other metis process
		// (status polls, the reviewer's independent verify). All state reads
		// are done — release before spawning.
		releaseRepoLock()

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

		force, _ := cmd.Flags().GetBool("force")
		exitCode, outcome, err := runner.Verify(ctx.cfg, ctx.repoRoot, sliceID, runner.VerifyOptions{Label: label, Force: force}, store)
		if err != nil {
			return err
		}

		if exitCode == 0 {
			if strings.Contains(ctx.cfg.Commands.Verify, "no verify configured") {
				fmt.Println("verify: PLACEHOLDER — commands.verify is not configured; nothing was actually verified")
				return nil
			}
			if g := outcome.Cached; g != nil {
				fmt.Printf("verify: ALL GREEN (cached: tree %s passed at %s; --force re-runs)\n", shortTree(g.Tree), g.At.Format(time.RFC3339))
				if g.Log != "" {
					fmt.Printf("cached log: %s\n", g.Log)
				}
			} else {
				fmt.Println("verify: ALL GREEN")
			}
			if sliceID != "" {
				name := "verify-latest"
				if label != "" {
					name = "verify-" + label
				}
				fmt.Printf("log: %s\n", filepath.Join(ctx.cfg.Paths.Runs, sliceID, name+".log"))
			}
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
		releaseRepoLock()

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

func shortTree(tree string) string {
	if len(tree) > 12 {
		return tree[:12]
	}
	return tree
}
