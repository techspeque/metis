package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
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

		if err := ctx.saveLedger(l); err != nil {
			return err
		}
		if err := ctx.saveArchive(archive); err != nil {
			return err
		}

		fmt.Printf("Archived %d slice(s): %s\n", len(archived), strings.Join(archived, ", "))
		return nil
	},
}
