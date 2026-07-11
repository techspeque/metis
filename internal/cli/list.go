package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(listCmd)
	listCmd.Flags().String("type", "", "Filter by work type")
	listCmd.Flags().String("priority", "", "Filter by priority")
	listCmd.Flags().String("status", "", "Filter by status: pending|coding|reviewing|done|rework")
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List slices with optional filters",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := loadContext()
		if err != nil {
			return err
		}

		l, err := ctx.loadLedger()
		if err != nil {
			return err
		}

		filterType, _ := cmd.Flags().GetString("type")
		filterPri, _ := cmd.Flags().GetString("priority")
		filterStatus, _ := cmd.Flags().GetString("status")

		if len(l.Slices) == 0 {
			fmt.Println("No slices in the ledger.")
			return nil
		}

		fmt.Printf("%-20s %-8s %-4s %-8s %-10s %s\n",
			"ID", "Type", "Pri", "Risk", "Status", "Title")
		fmt.Println("─────────────────────────────────────────────────────────────────────────────")

		for _, s := range l.Slices {
			status := s.Status()

			// Apply filters
			if filterType != "" && string(s.Type) != filterType {
				continue
			}
			if filterPri != "" && string(s.Priority) != filterPri {
				continue
			}
			if filterStatus != "" && string(status) != filterStatus {
				continue
			}

			title := s.Title
			if len(title) > 40 {
				title = title[:37] + "..."
			}

			fmt.Printf("%-20s %-8s %-4s %-8s %-10s %s\n",
				s.ID, s.Type, s.Priority, s.Risk, status, title)
		}

		return nil
	},
}
