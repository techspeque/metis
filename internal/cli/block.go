package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/techspeque/metis/internal/findings"
	"github.com/techspeque/metis/internal/git"
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

		// Record finding if provided
		findingText, _ := cmd.Flags().GetString("finding")
		if findingText != "" {
			severity, _ := cmd.Flags().GetString("severity")
			category, _ := cmd.Flags().GetString("category")

			findingsPath := filepath.Join(ctx.repoRoot, ctx.cfg.Paths.Findings)
			store, err := findings.Load(findingsPath)
			if err != nil {
				return err
			}

			id := store.Add(args[0], severity, category, findingText)
			if err := store.Save(findingsPath); err != nil {
				return err
			}

			fmt.Printf("Finding %s recorded: [%s/%s] %s\n", id, severity, category, findingText)
		}

		// State transitions are atomic: commit the ledger and findings so
		// the block never leaves the tree dirty between sessions.
		s = l.FindByID(args[0])
		paths := []string{ctx.ledgerPath()}
		if findingText != "" {
			paths = append(paths, filepath.Join(ctx.repoRoot, ctx.cfg.Paths.Findings))
		}
		if err := git.Add(ctx.repoRoot, paths...); err != nil {
			return fmt.Errorf("staging block state: %w", err)
		}
		message := git.FormatCommitMessage(ctx.cfg, args[0], "chore",
			fmt.Sprintf("block review (cycle %d)", s.ReviewCycles))
		if err := git.CommitPaths(ctx.repoRoot, message, paths...); err != nil {
			return fmt.Errorf("committing block state: %w", err)
		}
		fmt.Printf("Committed: %s\n", message)

		return nil
	},
}
