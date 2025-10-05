package main

import (
	"fmt"
	"os"

	"github.com/dyluth/sett/cmd/sett/commands"
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
	if err := commands.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
