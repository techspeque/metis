package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func init() {
	checkCmd.Flags().Bool("config", false, "Validate only the configuration file")
	checkCmd.Flags().Bool("ledger", false, "Validate only the ledger")
	rootCmd.AddCommand(checkCmd)
}

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Validate configuration and ledger integrity",
	Long:  `Validates metis.yaml and the slice ledger. Exit code 0 = pass, 1 = failure.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		configOnly, _ := cmd.Flags().GetBool("config")
		ledgerOnly, _ := cmd.Flags().GetBool("ledger")

		ctx, err := loadContext()
		if err != nil {
			return err
		}

		var allErrors []error

		// Config validation
		if !ledgerOnly {
			errs := ctx.cfg.Validate()
			if len(errs) > 0 {
				fmt.Fprintln(os.Stderr, "Config validation errors:")
				for _, e := range errs {
					fmt.Fprintf(os.Stderr, "  - %s\n", e)
				}
				allErrors = append(allErrors, errs...)
			} else {
				fmt.Println("Config: OK")
			}
		}

		// Ledger validation
		if !configOnly {
			l, err := ctx.loadLedger()
			if err != nil {
				return err
			}

			errs := l.Validate(ctx.agentSlugs())
			if len(errs) > 0 {
				fmt.Fprintln(os.Stderr, "Ledger validation errors:")
				for _, e := range errs {
					fmt.Fprintf(os.Stderr, "  - %s\n", e)
				}
				allErrors = append(allErrors, errs...)
			} else {
				fmt.Printf("Ledger: OK (%d active slice(s))\n", len(l.Slices))
			}
		}

		if len(allErrors) > 0 {
			return fmt.Errorf("validation failed with %d error(s)", len(allErrors))
		}
		return nil
	},
}
