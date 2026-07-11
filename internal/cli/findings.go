package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/techspeque/metis/internal/findings"
)

func init() {
	findingsCmd.Flags().String("severity", "", "Filter by severity: P1|P2|P3")
	findingsCmd.Flags().String("category", "", "Filter by category")
	findingsCmd.Flags().String("slice", "", "Filter by slice ID")
	findingsCmd.Flags().Bool("stats", false, "Show summary statistics")
	rootCmd.AddCommand(findingsCmd)
}

var findingsCmd = &cobra.Command{
	Use:   "findings",
	Short: "Show review findings",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadContext()
		if err != nil {
			return err
		}

		path := filepath.Join(ctx.repoRoot, ctx.cfg.Paths.Findings)
		store, err := findings.Load(path)
		if err != nil {
			return err
		}

		showStats, _ := cmd.Flags().GetBool("stats")
		if showStats {
			stats := store.GetStats()
			fmt.Printf("Total findings: %d\n\n", stats.Total)
			if len(stats.BySeverity) > 0 {
				fmt.Println("By Severity:")
				for sev, count := range stats.BySeverity {
					fmt.Printf("  %s: %d\n", sev, count)
				}
			}
			if len(stats.ByCategory) > 0 {
				fmt.Println("\nBy Category:")
				for cat, count := range stats.ByCategory {
					fmt.Printf("  %-12s %d\n", cat, count)
				}
			}
			return nil
		}

		severity, _ := cmd.Flags().GetString("severity")
		category, _ := cmd.Flags().GetString("category")
		sliceID, _ := cmd.Flags().GetString("slice")

		results := store.Filter(severity, category, sliceID)
		if len(results) == 0 {
			fmt.Println("No findings.")
			return nil
		}

		fmt.Printf("%-5s %-4s %-12s %-20s %s\n", "ID", "Sev", "Category", "Slice", "Finding")
		fmt.Println("─────────────────────────────────────────────────────────────────────────────")
		for _, f := range results {
			finding := f.Finding
			if len(finding) > 40 {
				finding = finding[:37] + "..."
			}
			fmt.Printf("%-5s %-4s %-12s %-20s %s\n", f.ID, f.Severity, f.Category, f.Slice, finding)
		}
		return nil
	},
}
