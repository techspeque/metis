package cli

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version, commit, build date, and platform",
	RunE: func(cmd *cobra.Command, args []string) error {
		if jsonOutput() {
			return printJSON(cmd, map[string]string{
				"version":    version,
				"commit":     commit,
				"date":       date,
				"go_version": runtime.Version(),
				"os":         runtime.GOOS,
				"arch":       runtime.GOARCH,
			})
		}
		out := cmd.OutOrStdout()
		if _, err := fmt.Fprintf(out, "metis %s (%s) built %s\n", version, commit, date); err != nil {
			return err
		}
		_, err := fmt.Fprintf(out, "%s %s/%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
		return err
	},
}
