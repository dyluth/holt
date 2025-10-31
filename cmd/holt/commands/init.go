package commands

import (
	"fmt"

	"github.com/dyluth/holt/internal/git"
	"github.com/dyluth/holt/internal/scaffold"
	"github.com/spf13/cobra"
)

var (
	forceInit bool
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new Holt project",
	Long: `Initialize a new Holt project with default configuration and example agent.

Creates:
  • holt.yml - Project configuration file
  • agents/example-agent/ - Example agent demonstrating the Holt agent contract

This command must be run from the root of a Git repository.

Use --force to reinitialize an existing project (WARNING: destroys existing configuration).`,
	RunE: runInit,
}

func init() {
	// Note: Cannot use -f shorthand because it conflicts with global --config flag
	initCmd.Flags().BoolVar(&forceInit, "force", false, "Force reinitialization (removes existing holt.yml and agents/)")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	// Validate Git context first
	checker := git.NewChecker()
	if err := checker.ValidateGitContext(); err != nil {
		return err
	}

	// Check for existing files (unless --force)
	if !forceInit {
		if err := scaffold.CheckExisting(); err != nil {
			return err
		}
	}

	// Initialize the project
	if err := scaffold.Initialize(forceInit); err != nil {
		return fmt.Errorf("initialization failed: %w", err)
	}

	// Print success message
	scaffold.PrintSuccess()

	return nil
}
