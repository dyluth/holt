package main

import (
	"os"

	"github.com/dyluth/holt/cmd/holt/commands"
)

// Version information - set during build
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	// Set version information on root command
	commands.SetVersionInfo(version, commit, date)

	// Execute root command
	// Errors are printed directly by the printer package with color formatting
	if err := commands.Execute(); err != nil {
		os.Exit(1)
	}
}
