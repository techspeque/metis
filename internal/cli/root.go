// Package cli implements the metis command-line interface using cobra.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"

	// workspaceFlag holds the value of the persistent --workspace flag.
	workspaceFlag string
)

// SetVersionInfo sets the version, commit, and build date displayed by --version.
func SetVersionInfo(v, c, d string) {
	version = v
	commit = c
	date = d
	rootCmd.Version = version
}

// SetVersion sets the version string (backwards compatibility).
func SetVersion(v string) {
	SetVersionInfo(v, commit, date)
}

// rootCmd is the base command for the metis CLI.
var rootCmd = &cobra.Command{
	Use:   "metis",
	Short: "The meta-harness that orchestrates AI coding agents",
	Long: `Metis is a CLI tool for managing autonomous coding-agent workflows.
It enforces disciplined, bounded, independently-reviewed units of work
across any technology stack and any agent surface.`,
	Version: version,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return validateOutputFormat()
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

func init() {
	rootCmd.SetVersionTemplate(fmt.Sprintf("metis %s (%s) built %s\n", version, commit, date))
	rootCmd.PersistentFlags().StringVarP(&workspaceFlag, "workspace", "w", "",
		"Operate on a registered workspace instead of the current directory")
	rootCmd.PersistentFlags().StringVarP(&outputFlag, "output", "o", "",
		"Output format: text or json (env: METIS_OUTPUT)")
}

// Execute runs the root command.
func Execute() error {
	// Update version template dynamically (since init() runs before SetVersionInfo)
	rootCmd.SetVersionTemplate(fmt.Sprintf("metis %s (%s) built %s\n", version, commit, date))
	return rootCmd.Execute()
}
