package cli

import (
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
		cmd.Printf("metis %s (%s) built %s\n", version, commit, date)
		cmd.Printf("%s %s/%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
		return nil
	},
}
