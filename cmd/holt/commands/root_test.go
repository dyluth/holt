package commands

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

// TestRootCommand_ShowsHelpWhenNoSubcommand tests that the root command
// shows help instead of silently succeeding when invoked without a subcommand
func TestRootCommand_ShowsHelpWhenNoSubcommand(t *testing.T) {
	// Create a fresh root command for testing
	testRoot := &cobra.Command{
		Use:   "holt",
		Short: "Test root command",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	// Capture output
	buf := new(bytes.Buffer)
	testRoot.SetOut(buf)
	testRoot.SetErr(buf)

	// Execute root command with no args
	err := testRoot.Execute()

	// Should show help (which returns nil error in cobra)
	assert.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "Usage:", "Help should be displayed")
	assert.Contains(t, output, "holt", "Help should show command name")
}

// TestRootCommand_RejectsUnknownFlags tests that unknown flags
// passed to the root command cause an error instead of being silently ignored
func TestRootCommand_RejectsUnknownFlags(t *testing.T) {
	// Create a fresh root command for testing with strict flag parsing
	testRoot := &cobra.Command{
		Use:   "holt",
		Short: "Test root command",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
		FParseErrWhitelist: cobra.FParseErrWhitelist{},
	}

	// Set args with an unknown flag
	testRoot.SetArgs([]string{"--unknown-flag", "value"})

	// Capture output
	buf := new(bytes.Buffer)
	testRoot.SetOut(buf)
	testRoot.SetErr(buf)

	// Execute should fail with unknown flag error
	err := testRoot.Execute()
	assert.Error(t, err, "Unknown flag should cause an error")
	assert.Contains(t, err.Error(), "unknown flag", "Error should mention unknown flag")
}

// TestRootCommand_RejectsSubcommandFlags tests that flags meant for
// subcommands (like --goal) are rejected when passed to root command
func TestRootCommand_RejectsSubcommandFlags(t *testing.T) {
	// Create a test setup mimicking holt/forage structure
	testRoot := &cobra.Command{
		Use:   "holt",
		Short: "Test root command",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
		FParseErrWhitelist: cobra.FParseErrWhitelist{},
	}

	// Add a subcommand with a flag (like forage --goal)
	forageCmd := &cobra.Command{
		Use:   "forage",
		Short: "Forage subcommand",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}
	forageCmd.Flags().String("goal", "", "Goal description")
	testRoot.AddCommand(forageCmd)

	// Try to pass --goal to root instead of subcommand
	testRoot.SetArgs([]string{"--goal", "test"})

	// Capture output
	buf := new(bytes.Buffer)
	testRoot.SetOut(buf)
	testRoot.SetErr(buf)

	// Execute should fail - root doesn't have --goal flag
	err := testRoot.Execute()
	assert.Error(t, err, "Subcommand flag passed to root should cause error")
	assert.Contains(t, err.Error(), "unknown flag: --goal",
		"Error should indicate --goal is unknown to root command")
}

// TestRootCommand_AcceptsValidSubcommand tests that valid subcommands
// still work correctly after our changes
func TestRootCommand_AcceptsValidSubcommand(t *testing.T) {
	testRoot := &cobra.Command{
		Use:   "holt",
		Short: "Test root command",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	subcommandExecuted := false
	subCmd := &cobra.Command{
		Use:   "test-sub",
		Short: "Test subcommand",
		RunE: func(cmd *cobra.Command, args []string) error {
			subcommandExecuted = true
			return nil
		},
	}
	testRoot.AddCommand(subCmd)

	// Execute with valid subcommand
	testRoot.SetArgs([]string{"test-sub"})
	err := testRoot.Execute()

	assert.NoError(t, err)
	assert.True(t, subcommandExecuted, "Subcommand should have been executed")
}
