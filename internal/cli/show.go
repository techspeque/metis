package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(showCmd)
}

var showCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show full details of a slice",
	Args:  cobra.ExactArgs(1),
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

		if jsonOutput() {
			return printJSON(cmd, listRow{Slice: *s, Status: string(s.Status())})
		}

		fmt.Printf("ID:            %s\n", s.ID)
		fmt.Printf("Title:         %s\n", s.Title)
		fmt.Printf("Type:          %s\n", s.Type)
		fmt.Printf("Priority:      %s\n", s.Priority)
		fmt.Printf("Risk:          %s\n", s.Risk)
		fmt.Printf("Stage:         %s\n", s.Stage)
		fmt.Printf("Status:        %s\n", s.Status())
		fmt.Printf("Coder:         %s\n", s.Coder)
		fmt.Printf("Reviewer:      %s\n", s.Reviewer)
		if s.Plan != "" {
			fmt.Printf("Plan:          %s %s\n", s.Plan, s.PlanSection)
		}
		fmt.Printf("Coded:         %v\n", s.Coded)
		fmt.Printf("Reviewed:      %v\n", s.Reviewed)
		fmt.Printf("Review cycles: %d\n", s.ReviewCycles)
		if len(s.BlockedBy) > 0 {
			fmt.Printf("Blocked by:    %v\n", s.BlockedBy)
		}
		if s.Notes != "" {
			fmt.Printf("Notes:         %s\n", s.Notes)
		}
		fmt.Printf("Created:       %s\n", s.Created)

		return nil
	},
}
