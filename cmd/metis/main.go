// Package main is the entry point for the metis CLI.
package main

import (
	"os"

	"github.com/techspeque/metis/internal/cli"
)

// Build-time variables set via ldflags by GoReleaser.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	cli.SetVersionInfo(version, commit, date)
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
