package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/techspeque/metis/internal/seed"
	"github.com/techspeque/metis/internal/slice"
)

func init() {
	seedCmd.Flags().Bool("dry-run", false, "Show what would be created without writing")
	seedCmd.Flags().Bool("append", false, "Add to existing ledger (default: error if non-empty)")
	seedCmd.Flags().Int("phase", -1, "Seed only this phase number")
	seedCmd.Flags().String("type", "feat", "Work type for generated slices")
	rootCmd.AddCommand(seedCmd)
}

var seedCmd = &cobra.Command{
	Use:   "seed <plan-file>",
	Short: "Parse a plan file and generate ledger entries",
	Long:  `Parses a structured implementation plan and creates slice entries in the ledger.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadContext()
		if err != nil {
			return err
		}

		planFile := args[0]
		planPath := filepath.Join(ctx.repoRoot, planFile)

		data, err := os.ReadFile(planPath)
		if err != nil {
			return fmt.Errorf("reading plan file: %w", err)
		}

		result := seed.Parse(string(data))
		if len(result.Workstreams) == 0 {
			return fmt.Errorf("no workstreams found in %s", planFile)
		}

		// Filter by phase if specified
		phaseFilter, _ := cmd.Flags().GetInt("phase")
		if phaseFilter >= 0 {
			var filtered []seed.Workstream
			for _, ws := range result.Workstreams {
				if ws.Phase == phaseFilter {
					filtered = append(filtered, ws)
				}
			}
			result.Workstreams = filtered
			if len(result.Workstreams) == 0 {
				return fmt.Errorf("no workstreams found for phase %d", phaseFilter)
			}
		}

		// Get slice type
		typeStr, _ := cmd.Flags().GetString("type")
		wt := slice.WorkType(typeStr)
		if !wt.IsValid() {
			return fmt.Errorf("invalid type %q", typeStr)
		}

		slices := seed.ToSlices(result.Workstreams, planFile, wt)

		dryRun, _ := cmd.Flags().GetBool("dry-run")
		if dryRun {
			fmt.Printf("Would create %d slice(s):\n\n", len(slices))
			for _, s := range slices {
				fmt.Printf("  %-24s %-6s %-8s coder=%s reviewer=%s\n",
					s.ID, s.Risk, s.Stage, s.Coder, s.Reviewer)
				fmt.Printf("  %s\n\n", s.Title)
			}
			return nil
		}

		// Load ledger
		l, err := ctx.loadLedger()
		if err != nil {
			return err
		}

		appendMode, _ := cmd.Flags().GetBool("append")
		if !appendMode && len(l.Slices) > 0 {
			return fmt.Errorf("ledger is not empty (%d slices). Use --append to add to existing ledger", len(l.Slices))
		}

		// Add slices
		added := 0
		var skipped []string
		for _, s := range slices {
			if l.FindByID(s.ID) != nil {
				skipped = append(skipped, s.ID)
				continue
			}
			l.Slices = append(l.Slices, s)
			added++
		}

		if err := ctx.saveLedger(l); err != nil {
			return err
		}

		// Store overview hash to baseline drift detection
		if err := ctx.storeOverviewHash(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not store overview hash: %v\n", err)
		}

		fmt.Printf("Seeded %d slice(s) from %s\n", added, planFile)
		if len(skipped) > 0 {
			fmt.Printf("Skipped %d (already exist): %s\n", len(skipped), strings.Join(skipped, ", "))
		}
		return nil
	},
}
