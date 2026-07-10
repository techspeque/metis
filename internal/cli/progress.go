package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/techspeque/metis/internal/progress"
)

func init() {
	rootCmd.AddCommand(progressCmd)
}

var progressCmd = &cobra.Command{
	Use:   "progress",
	Short: "Show slice completion dashboard with progress bars",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadContext()
		if err != nil {
			return err
		}

		l, err := ctx.loadLedger()
		if err != nil {
			return err
		}

		// Include archived slices in the count
		archive, err := ctx.loadArchive()
		if err != nil {
			return err
		}

		allSlices := append(archive.Slices, l.Slices...)
		d := progress.Compute(allSlices)

		fmt.Print(d.Render())
		return nil
	},
}
