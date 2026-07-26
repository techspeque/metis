package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/techspeque/metis/internal/slice"
)

func init() {
	addCmd.Flags().String("title", "", "Human-readable title (required)")
	addCmd.Flags().String("coder", "", "Agent slug for coding (required)")
	addCmd.Flags().String("reviewer", "", "Agent slug for review (required)")
	addCmd.Flags().String("risk", "medium", "Risk level: low|medium|high")
	addCmd.Flags().String("priority", "p2", "Priority: p0|p1|p2|p3")
	addCmd.Flags().String("stage", "", "Project taxonomy label")
	addCmd.Flags().String("plan", "", "Source plan file")
	addCmd.Flags().String("plan-section", "", "Plan section (required when --plan is set)")
	addCmd.Flags().String("blocked-by", "", "Comma-separated slice IDs this depends on")
	addCmd.Flags().String("id", "", "Explicit ID (auto-generated if omitted)")
	addCmd.Flags().String("notes", "", "Clarification text")
	addCmd.Flags().String("after", "", "Insert after this slice ID")
	addCmd.Flags().String("before", "", "Insert before this slice ID")

	_ = addCmd.MarkFlagRequired("title")
	_ = addCmd.MarkFlagRequired("coder")
	_ = addCmd.MarkFlagRequired("reviewer")

	rootCmd.AddCommand(addCmd)
}

var addCmd = &cobra.Command{
	Use:   "add <type>",
	Short: "Add a new slice to the ledger",
	Long:  `Add a new slice of the given type (feat, fix, refactor, debt, remove, chore, security, gate, recon).`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		wt := slice.WorkType(args[0])
		if !wt.IsValid() {
			return fmt.Errorf("invalid type %q (valid: feat, fix, refactor, debt, remove, chore, security, gate, recon)", args[0])
		}

		ctx, err := loadContext()
		if err != nil {
			return err
		}

		l, err := ctx.loadLedger()
		if err != nil {
			return err
		}

		title, _ := cmd.Flags().GetString("title")
		coder, _ := cmd.Flags().GetString("coder")
		reviewer, _ := cmd.Flags().GetString("reviewer")
		risk, _ := cmd.Flags().GetString("risk")
		priority, _ := cmd.Flags().GetString("priority")
		stage, _ := cmd.Flags().GetString("stage")
		plan, _ := cmd.Flags().GetString("plan")
		planSection, _ := cmd.Flags().GetString("plan-section")
		blockedByStr, _ := cmd.Flags().GetString("blocked-by")
		id, _ := cmd.Flags().GetString("id")
		notes, _ := cmd.Flags().GetString("notes")
		afterID, _ := cmd.Flags().GetString("after")
		beforeID, _ := cmd.Flags().GetString("before")

		// Validate risk
		r := slice.Risk(risk)
		if !r.IsValid() {
			return fmt.Errorf("invalid risk %q (valid: low, medium, high)", risk)
		}

		// Validate priority
		p := slice.Priority(priority)
		if !p.IsValid() {
			return fmt.Errorf("invalid priority %q (valid: p0, p1, p2, p3)", priority)
		}

		// Validate plan_section when plan is set
		if plan != "" && planSection == "" {
			return fmt.Errorf("--plan-section is required when --plan is set")
		}

		// Generate ID if not provided. Sequence numbering must consider the
		// archive too — otherwise archiving feat-0001 lets the next add mint
		// feat-0001 again, corrupting everything keyed by slice ID.
		if id == "" {
			allIDs := l.IDs()
			if archive, aerr := ctx.loadArchive(); aerr == nil {
				for i := range archive.Slices {
					allIDs = append(allIDs, archive.Slices[i].ID)
				}
			}
			id = slice.GenerateID(wt, slice.NextSequence(allIDs, wt))
		}

		// Parse blocked_by
		var blockedBy []string
		if blockedByStr != "" {
			for _, dep := range splitComma(blockedByStr) {
				if dep != "" {
					blockedBy = append(blockedBy, dep)
				}
			}
		}

		s := slice.Slice{
			ID:          id,
			Title:       title,
			Type:        wt,
			Priority:    p,
			Risk:        r,
			Stage:       stage,
			Coder:       coder,
			Reviewer:    reviewer,
			Plan:        plan,
			PlanSection: planSection,
			BlockedBy:   blockedBy,
			Notes:       notes,
			Created:     time.Now().Format("2006-01-02"),
		}

		// Insert at position
		switch {
		case afterID != "":
			if err := l.AddAfter(&s, afterID); err != nil {
				return err
			}
		case beforeID != "":
			if err := l.AddBefore(&s, beforeID); err != nil {
				return err
			}
		default:
			if err := l.Add(&s); err != nil {
				return err
			}
		}

		if err := ctx.saveLedger(l); err != nil {
			return err
		}

		fmt.Printf("Added slice: %s (%s)\n", s.ID, s.Title)
		ctx.commitStateSoft(s.ID, "add slice", ctx.ledgerPath())
		return nil
	},
}

func splitComma(s string) []string {
	var parts []string
	current := ""
	for _, c := range s {
		if c == ',' {
			parts = append(parts, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}
