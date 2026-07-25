package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/techspeque/metis/internal/slice"
)

func init() {
	editCmd.Flags().String("title", "", "New title")
	editCmd.Flags().String("risk", "", "New risk level: low|medium|high")
	editCmd.Flags().String("priority", "", "New priority: p0|p1|p2|p3")
	editCmd.Flags().String("stage", "", "New project taxonomy label")
	editCmd.Flags().String("coder", "", "New agent slug for coding")
	editCmd.Flags().String("reviewer", "", "New agent slug for review")
	editCmd.Flags().String("plan", "", "New source plan file")
	editCmd.Flags().String("plan-section", "", "New plan section")
	editCmd.Flags().String("blocked-by", "", "Comma-separated slice IDs this depends on (replaces the list)")
	editCmd.Flags().String("notes", "", "New clarification text")
	rootCmd.AddCommand(editCmd)
}

var editCmd = &cobra.Command{
	Use:   "edit <id>",
	Short: "Edit fields of an existing slice",
	Long: `Edits an existing slice in the ledger. Only the flags you pass change;
everything else is untouched. Used during reconciliation to update slices
affected by OVERVIEW changes.

Done slices cannot be edited — 'metis reopen' them first.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadContext()
		if err != nil {
			return err
		}

		l, err := ctx.loadLedger()
		if err != nil {
			return err
		}

		s := l.FindByID(args[0])
		if s == nil {
			return fmt.Errorf("slice %q not found", args[0])
		}
		if s.IsDone() {
			return fmt.Errorf("slice %q is done — 'metis reopen %s' before editing", s.ID, s.ID)
		}

		var changed []string
		setString := func(flag string, dst *string) {
			if cmd.Flags().Changed(flag) {
				v, _ := cmd.Flags().GetString(flag)
				*dst = v
				changed = append(changed, flag)
			}
		}

		setString("title", &s.Title)
		setString("stage", &s.Stage)
		setString("coder", &s.Coder)
		setString("reviewer", &s.Reviewer)
		setString("plan", &s.Plan)
		setString("plan-section", &s.PlanSection)
		setString("notes", &s.Notes)

		if cmd.Flags().Changed("risk") {
			v, _ := cmd.Flags().GetString("risk")
			r := slice.Risk(v)
			if !r.IsValid() {
				return fmt.Errorf("invalid risk %q (valid: low, medium, high)", v)
			}
			s.Risk = r
			changed = append(changed, "risk")
		}

		if cmd.Flags().Changed("priority") {
			v, _ := cmd.Flags().GetString("priority")
			p := slice.Priority(v)
			if !p.IsValid() {
				return fmt.Errorf("invalid priority %q (valid: p0, p1, p2, p3)", v)
			}
			s.Priority = p
			changed = append(changed, "priority")
		}

		if cmd.Flags().Changed("blocked-by") {
			v, _ := cmd.Flags().GetString("blocked-by")
			var blockedBy []string
			for _, dep := range splitComma(v) {
				if dep = strings.TrimSpace(dep); dep != "" {
					if l.FindByID(dep) == nil {
						return fmt.Errorf("blocked-by references unknown slice %q", dep)
					}
					if dep == s.ID {
						return fmt.Errorf("slice cannot be blocked by itself")
					}
					blockedBy = append(blockedBy, dep)
				}
			}
			s.BlockedBy = blockedBy
			changed = append(changed, "blocked-by")
		}

		if len(changed) == 0 {
			return fmt.Errorf("nothing to change — pass at least one flag (see 'metis edit --help')")
		}

		if !ctx.allowSelfReview() && s.Coder != "" && s.Coder == s.Reviewer {
			return fmt.Errorf("coder and reviewer must differ (cross-vendor review)")
		}
		if s.Plan != "" && s.PlanSection == "" {
			return fmt.Errorf("--plan-section is required when a plan is set")
		}

		if err := ctx.saveLedger(l); err != nil {
			return err
		}

		fmt.Printf("Edited %s: %s\n", s.ID, strings.Join(changed, ", "))
		ctx.commitStateSoft(s.ID, "edit "+strings.Join(changed, ","), ctx.ledgerPath())
		return nil
	},
}
