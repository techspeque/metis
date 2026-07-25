package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/techspeque/metis/internal/findings"
	"github.com/techspeque/metis/internal/slice"
)

func init() {
	findingsCmd.Flags().String("severity", "", "Filter by severity: P1|P2|P3")
	findingsCmd.Flags().String("category", "", "Filter by category")
	findingsCmd.Flags().String("slice", "", "Filter by slice ID")
	findingsCmd.Flags().Bool("stats", false, "Show summary statistics")
	rootCmd.AddCommand(findingsCmd)
}

// fillAgentStats joins findings with the ledger and archive to produce the
// per-coder routing evidence: slices owned, review blocks received, and
// first-pass completions (done with zero review cycles).
func fillAgentStats(ctx *context, store *findings.Store, stats *findings.Stats) {
	l, err := ctx.loadLedger()
	if err != nil {
		return
	}
	archive, err := ctx.loadArchive()
	if err != nil {
		return
	}

	sliceCoder := map[string]string{}
	agg := map[string]findings.AgentStats{}
	all := append(append([]slice.Slice{}, l.Slices...), archive.Slices...)
	for i := range all {
		s := &all[i]
		if s.Coder == "" {
			continue
		}
		sliceCoder[s.ID] = s.Coder
		as := agg[s.Coder]
		as.Slices++
		if s.IsDone() {
			as.Done++
			if s.ReviewCycles == 0 {
				as.FirstPass++
			}
		}
		agg[s.Coder] = as
	}
	for _, f := range store.Findings {
		if coder, ok := sliceCoder[f.Slice]; ok {
			as := agg[coder]
			as.Blocks++
			agg[coder] = as
		}
	}
	stats.ByAgent = agg
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
			fillAgentStats(ctx, store, &stats)
			if jsonOutput() {
				return printJSON(cmd, stats)
			}
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
			if len(stats.ByAgent) > 0 {
				fmt.Println("\nBy Agent (routing evidence):")
				fmt.Printf("  %-24s %7s %7s %10s\n", "coder", "slices", "blocks", "first-pass")
				for agent, as := range stats.ByAgent {
					rate := "-"
					if as.Done > 0 {
						rate = fmt.Sprintf("%d%%", as.FirstPass*100/as.Done)
					}
					fmt.Printf("  %-24s %7d %7d %10s\n", agent, as.Slices, as.Blocks, rate)
				}
			}
			return nil
		}

		severity, _ := cmd.Flags().GetString("severity")
		category, _ := cmd.Flags().GetString("category")
		sliceID, _ := cmd.Flags().GetString("slice")

		results := store.Filter(severity, category, sliceID)

		if jsonOutput() {
			if results == nil {
				results = []findings.Finding{}
			}
			return printJSON(cmd, results)
		}

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
