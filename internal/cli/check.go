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
	Long:  `Validates .metis/project.yaml and the slice ledger. Exit code 0 = pass, 1 = failure.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		configOnly, _ := cmd.Flags().GetBool("config")
		ledgerOnly, _ := cmd.Flags().GetBool("ledger")

		ctx, err := loadContext()
		if err != nil {
			return err
		}

		result := checkOutput{}
		var allErrors []error

		// Config validation
		if !ledgerOnly {
			errs := ctx.cfg.Validate()
			section := &checkSection{OK: len(errs) == 0, Errors: []string{}}
			for _, e := range errs {
				section.Errors = append(section.Errors, e.Error())
			}
			result.Config = section
			if len(errs) > 0 {
				if !jsonOutput() {
					fmt.Fprintln(os.Stderr, "Config validation errors:")
					for _, e := range errs {
						fmt.Fprintf(os.Stderr, "  - %s\n", e)
					}
				}
				allErrors = append(allErrors, errs...)
			} else if !jsonOutput() {
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
			section := &checkSection{OK: len(errs) == 0, Errors: []string{}, Slices: len(l.Slices)}
			for _, e := range errs {
				section.Errors = append(section.Errors, e.Error())
			}
			result.Ledger = section
			if len(errs) > 0 {
				if !jsonOutput() {
					fmt.Fprintln(os.Stderr, "Ledger validation errors:")
					for _, e := range errs {
						fmt.Fprintf(os.Stderr, "  - %s\n", e)
					}
				}
				allErrors = append(allErrors, errs...)
			} else if !jsonOutput() {
				fmt.Printf("Ledger: OK (%d active slice(s))\n", len(l.Slices))
			}
		}

		result.OK = len(allErrors) == 0
		if result.OK {
			result.Overview = ctx.checkOverviewDrift()
		}

		if jsonOutput() {
			if err := printJSON(cmd, result); err != nil {
				return err
			}
		}

		if len(allErrors) > 0 {
			if jsonOutput() {
				// The JSON body carries the details; the error sets the exit code.
				cmd.SilenceErrors = true
				return fmt.Errorf("validation failed")
			}
			return fmt.Errorf("validation failed with %d error(s)", len(allErrors))
		}

		if !jsonOutput() {
			// Overview drift detection (warning only, not a hard error)
			switch result.Overview {
			case "drifted":
				fmt.Println("WARNING: OVERVIEW has changed since last planning cycle. Consider: metis recon")
			case "no-baseline":
				fmt.Println("NOTE: No overview hash stored. Run 'metis seed' or 'metis recon' to baseline.")
			}
		}

		return nil
	},
}

// checkOutput is the JSON shape of 'metis check'.
type checkOutput struct {
	OK       bool          `json:"ok"`
	Config   *checkSection `json:"config,omitempty"`
	Ledger   *checkSection `json:"ledger,omitempty"`
	Overview string        `json:"overview,omitempty"` // ok, drifted, no-baseline, not-configured
}

// checkSection reports one validation area.
type checkSection struct {
	OK     bool     `json:"ok"`
	Errors []string `json:"errors"`
	Slices int      `json:"slices,omitempty"`
}
