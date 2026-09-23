package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/techspeque/metis/internal/progress"
	"github.com/techspeque/metis/internal/slice"
)

func init() {
	progressCmd.AddCommand(progressStatsCmd)
	progressCmd.AddCommand(progressStageCmd)
	progressCmd.AddCommand(progressPhaseCmd)
	rootCmd.AddCommand(progressCmd)
}

var progressCmd = &cobra.Command{
	Use:   "progress",
	Short: "Show slice completion dashboard with progress bars (by stage)",
	Long: `Show the slice completion dashboard. With no view named it shows the
by-stage breakdown; name a view to see another:

  metis progress stats   summary counts (done, reviewing, rework, pending)
  metis progress stage   completion per stage, in plan order (default)
  metis progress phase   completion per phase, with each phase's stages

With -o json every view emits the same full dashboard.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runProgress(cmd, progress.ViewStage)
	},
}

var progressStatsCmd = &cobra.Command{
	Use:     "stats",
	Aliases: []string{"summary"},
	Short:   "Show summary counts: done, reviewing, rework, pending",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runProgress(cmd, progress.ViewStats)
	},
}

var progressStageCmd = &cobra.Command{
	Use:     "stage",
	Aliases: []string{"stages"},
	Short:   "Show completion per stage, in the order the plans reached them",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runProgress(cmd, progress.ViewStage)
	},
}

var progressPhaseCmd = &cobra.Command{
	Use:     "phase",
	Aliases: []string{"phases"},
	Short:   "Show completion per phase, with each phase's stages",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runProgress(cmd, progress.ViewPhase)
	},
}

func runProgress(cmd *cobra.Command, view progress.View) error {
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

	allSlices := make([]slice.Slice, 0, len(archive.Slices)+len(l.Slices))
	allSlices = append(allSlices, archive.Slices...)
	allSlices = append(allSlices, l.Slices...)
	d := progress.Compute(allSlices)

	if jsonOutput() {
		return printJSON(cmd, d)
	}

	_, err = fmt.Fprint(cmd.OutOrStdout(), d.RenderView(view))
	return err
}
