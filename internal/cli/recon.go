package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/techspeque/metis/internal/config"
	"github.com/techspeque/metis/internal/slice"
)

func init() {
	reconCmd.Flags().String("coder", "", "Agent slug for the recon coder (uses first high-risk agent if omitted)")
	reconCmd.Flags().String("reviewer", "", "Agent slug for the recon reviewer (uses second high-risk agent if omitted)")
	reconCmd.Flags().String("priority", "p1", "Priority for the recon slice")
	rootCmd.AddCommand(reconCmd)
}

var reconCmd = &cobra.Command{
	Use:   "recon",
	Short: "Create a reconciliation slice for overview changes",
	Long: `Creates a recon slice when the OVERVIEW has changed. The recon agent
reconciles pending work against the updated overview: edits slices,
skips obsolete ones, adds new ones, and updates plan documentation.

Completed/archived work is never touched — always fix forward.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadContext()
		if err != nil {
			return err
		}

		if ctx.cfg.Project.Overview == "" {
			return fmt.Errorf("project.overview is not configured in metis.yaml")
		}

		// Determine coder/reviewer
		coder, _ := cmd.Flags().GetString("coder")
		reviewer, _ := cmd.Flags().GetString("reviewer")

		if coder == "" {
			coder = pickAgent(ctx.cfg.Routing.High, 0)
			if coder == "" {
				coder = pickFirstAgent(ctx.cfg.Agents)
			}
		}
		if reviewer == "" {
			reviewer = pickAgent(ctx.cfg.Routing.High, 1)
			if reviewer == "" || reviewer == coder {
				// Try medium routing for cross-vendor
				reviewer = pickAgent(ctx.cfg.Routing.Medium, 0)
			}
			if reviewer == "" || reviewer == coder {
				reviewer = pickSecondAgent(ctx.cfg.Agents, coder)
			}
		}

		if coder == "" || reviewer == "" {
			return fmt.Errorf("cannot determine coder/reviewer — specify --coder and --reviewer")
		}

		priority, _ := cmd.Flags().GetString("priority")
		p := slice.Priority(priority)
		if !p.IsValid() {
			return fmt.Errorf("invalid priority %q", priority)
		}

		// Load ledger
		l, err := ctx.loadLedger()
		if err != nil {
			return err
		}

		// Generate ID
		id := slice.GenerateID(slice.TypeRecon, slice.NextSequence(l.IDs(), slice.TypeRecon))

		s := slice.Slice{
			ID:       id,
			Title:    "Reconcile pending work against updated OVERVIEW",
			Type:     slice.TypeRecon,
			Priority: p,
			Risk:     slice.RiskMedium,
			Coder:    coder,
			Reviewer: reviewer,
			Notes:    fmt.Sprintf("Auto-created by metis recon. Overview: %s", ctx.cfg.Project.Overview),
			Created:  time.Now().Format("2006-01-02"),
		}

		if err := l.Add(&s); err != nil {
			return err
		}
		if err := ctx.saveLedger(l); err != nil {
			return err
		}

		// Update overview hash (stops the drift warning)
		if err := ctx.storeOverviewHash(); err != nil {
			return fmt.Errorf("storing overview hash: %w", err)
		}

		fmt.Printf("Created recon slice: %s\n", id)
		fmt.Printf("  Coder:    %s\n", coder)
		fmt.Printf("  Reviewer: %s\n", reviewer)
		fmt.Printf("  Priority: %s\n", p)
		fmt.Println("\nThe recon agent should:")
		fmt.Println("  1. Read the OVERVIEW changes")
		fmt.Println("  2. Identify affected pending slices")
		fmt.Println("  3. Propose edits/skips/new slices")
		fmt.Println("  4. Update plan docs and ADRs if needed")
		fmt.Println("  5. Never modify completed/archived work")
		return nil
	},
}

func pickAgent(slugs []string, index int) string {
	if index < len(slugs) {
		return slugs[index]
	}
	return ""
}

func pickFirstAgent(agents map[string]config.Agent) string {
	for slug := range agents {
		return slug
	}
	return ""
}

func pickSecondAgent(agents map[string]config.Agent, exclude string) string {
	for slug := range agents {
		if slug != exclude {
			return slug
		}
	}
	return ""
}
