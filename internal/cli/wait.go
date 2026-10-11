package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func init() {
	waitCmd.Flags().String("reason", "", "What the slice waits for (required unless --clear)")
	waitCmd.Flags().String("until", "", "When the wait ends by itself (RFC 3339); without it the wait holds until --clear")
	waitCmd.Flags().Duration("for", 0, "How long the wait lasts from now, e.g. 90m")
	waitCmd.Flags().Bool("clear", false, "End the wait and return the slice to dispatch")
	waitCmd.Flags().Bool("human", false, "Only a person clears this wait: a decision is needed (metis auto stops on it)")
	rootCmd.AddCommand(waitCmd)
}

var waitCmd = &cobra.Command{
	Use:   "wait <id>",
	Short: "Park a slice on something outside the repository",
	Long: `Marks a slice as waiting — a lab run, an approval, a window that must
close — so dispatch passes it over and the time reads as a wait, not as a
stalled agent. The wait ends at --until / --for, or at --clear; a flip
clears it too. Commits and verify may still name the waiting slice with
--slice.

--human marks a wait only a person clears — an agent that needs a decision
runs it and stops; next and status show the slice as "needs you", and
metis auto stops there.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		reason, _ := cmd.Flags().GetString("reason")
		untilText, _ := cmd.Flags().GetString("until")
		forDuration, _ := cmd.Flags().GetDuration("for")
		clear, _ := cmd.Flags().GetBool("clear")
		human, _ := cmd.Flags().GetBool("human")

		ctx, err := loadContext()
		if err != nil {
			return err
		}
		l, err := ctx.loadLedger()
		if err != nil {
			return err
		}

		if clear {
			if err := l.ClearWait(id); err != nil {
				return err
			}
			if err := ctx.saveLedger(l); err != nil {
				return err
			}
			fmt.Printf("Cleared wait: %s\n", id)
			ctx.commitStateSoft(id, "wait cleared", ctx.ledgerPath())
			return nil
		}

		if reason == "" {
			return fmt.Errorf("--reason is required: say what %s waits for", id)
		}
		if untilText != "" && forDuration != 0 {
			return fmt.Errorf("--until and --for are one choice")
		}
		now := time.Now()
		var until time.Time
		switch {
		case untilText != "":
			until, err = time.Parse(time.RFC3339, untilText)
			if err != nil {
				return fmt.Errorf("--until %q is not RFC 3339 (e.g. 2026-10-03T22:30:00Z)", untilText)
			}
		case forDuration != 0:
			until = now.Add(forDuration)
		}
		if err := l.WaitFor(id, reason, until, now, human); err != nil {
			return err
		}
		if err := ctx.saveLedger(l); err != nil {
			return err
		}
		switch {
		case human:
			fmt.Printf("Needs you: %s — %s (until a person runs metis wait %s --clear)\n", id, reason, id)
		case until.IsZero():
			fmt.Printf("Waiting: %s — %s (until metis wait %s --clear)\n", id, reason, id)
		default:
			fmt.Printf("Waiting: %s — %s (until %s)\n", id, reason, until.UTC().Format(time.RFC3339))
		}
		ctx.commitStateSoft(id, "wait: "+reason, ctx.ledgerPath())
		return nil
	},
}
