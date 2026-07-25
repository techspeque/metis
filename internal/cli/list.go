package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/techspeque/metis/internal/slice"
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

		var matched []listRow
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

			matched = append(matched, listRow{Slice: s, Status: string(status)})
		}

		if jsonOutput() {
			if matched == nil {
				matched = []listRow{}
			}
			return printJSON(cmd, matched)
		}

		if len(l.Slices) == 0 {
			fmt.Println("No slices in the ledger.")
			return nil
		}

		fmt.Printf("%-20s %-8s %-4s %-8s %-10s %s\n",
			"ID", "Type", "Pri", "Risk", "Status", "Title")
		fmt.Println("─────────────────────────────────────────────────────────────────────────────")

		for _, row := range matched {
			title := row.Title
			if len(title) > 40 {
				title = title[:37] + "..."
			}

			fmt.Printf("%-20s %-8s %-4s %-8s %-10s %s\n",
				row.ID, row.Type, row.Priority, row.Risk, row.Status, title)
		}

		return nil
	},
}

// listRow is a ledger slice plus its computed lifecycle status.
type listRow struct {
	slice.Slice
	Status string `json:"status"`
}
