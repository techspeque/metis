// Package cli implements the metis command-line interface using cobra.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var version = "dev"

// SetVersion sets the version string displayed by --version.
func SetVersion(v string) {
	version = v
}

// rootCmd is the base command for the metis CLI.
var rootCmd = &cobra.Command{
	Use:   "metis",
	Short: "The meta-intelligence that orchestrates AI coding agents",
	Long: `Metis is a CLI tool for managing autonomous coding-agent workflows.
It enforces disciplined, bounded, independently-reviewed units of work
across any technology stack and any agent surface.`,
	Version: version,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

func init() {
	rootCmd.SetVersionTemplate(fmt.Sprintf("metis %s\n", version))
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
