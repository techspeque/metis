package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/techspeque/metis/internal/git"
)

func init() {
	rootCmd.AddCommand(archiveCmd)
}

var archiveCmd = &cobra.Command{
	Use:   "archive",
	Short: "Move all done slices to the archive",
	RunE: func(cmd *cobra.Command, args []string) error {
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

		archived := l.Archive(archive)
		if len(archived) == 0 {
			fmt.Println("Nothing to archive.")
			return nil
		}

		// Archive first, then shrink the ledger: if the second write fails,
		// slices exist in both files (recoverable) rather than neither.
		if err := ctx.saveArchive(archive); err != nil {
			return err
		}
		if err := ctx.saveLedger(l); err != nil {
			return err
		}

		fmt.Printf("Archived %d slice(s): %s\n", len(archived), strings.Join(archived, ", "))

		// State transitions are atomic: commit the ledger and archive so
		// the protocol ends with a clean tree.
		if err := git.Add(ctx.repoRoot, ctx.ledgerPath(), ctx.archivePath()); err != nil {
			return fmt.Errorf("staging archive state: %w", err)
		}
		message := git.FormatCommitMessage(ctx.cfg, archived[0], "chore",
			fmt.Sprintf("archive %d slice(s)", len(archived)))
		if err := git.Commit(ctx.repoRoot, message); err != nil {
			return fmt.Errorf("committing archive state: %w", err)
		}
		fmt.Printf("Committed: %s\n", message)
		return nil
	},
}
