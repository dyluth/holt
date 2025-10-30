package commands

import (
	"fmt"

	"github.com/dyluth/holt/internal/printer"
	"github.com/spf13/cobra"
)

var (
	version string
	commit  string
	date    string
)

// Global flags
var (
	globalConfigPath string
	globalDebug      bool
	globalQuiet      bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "holt",
	Short: "Holt - Container-native AI agent orchestrator",
	Long: `Holt is a container-native AI agent orchestrator designed to manage
a clan of specialized, tool-equipped AI agents for automating complex
software engineering tasks.

Holt provides an event-driven architecture with Redis-based state management,
enabling transparent, auditable AI workflows.`,
	Version: version,
	// Prevent silent success when unknown flags are passed to root command
	// e.g., "holt --goal test" instead of "holt forage --goal test"
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Apply verbosity settings before any command runs
		if globalDebug {
			printer.SetVerbosity(printer.VerbosityDebug)
		} else if globalQuiet {
			printer.SetVerbosity(printer.VerbosityQuiet)
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		// If no subcommand is specified, show help
		return cmd.Help()
	},
	// Enable strict flag parsing - unknown flags will cause an error
	FParseErrWhitelist: cobra.FParseErrWhitelist{},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() error {
	// Silence Cobra's default error and usage printing
	// We print formatted colored errors directly in the printer package
	rootCmd.SilenceErrors = true
	rootCmd.SilenceUsage = true
	return rootCmd.Execute()
}

// SetVersionInfo sets the version information for the CLI
func SetVersionInfo(v, c, d string) {
	version = v
	commit = c
	date = d
	rootCmd.Version = fmt.Sprintf("%s (commit: %s, built: %s)", v, c, d)
}

func init() {
	// Global configuration flag
	rootCmd.PersistentFlags().StringVarP(&globalConfigPath, "config", "f", "",
		"Path to holt.yml configuration file")

	// Global verbosity flags (mutually exclusive)
	rootCmd.PersistentFlags().BoolVarP(&globalDebug, "debug", "d", false,
		"Enable verbose debug output")
	rootCmd.PersistentFlags().BoolVarP(&globalQuiet, "quiet", "q", false,
		"Suppress all non-essential output")

	// Mark as mutually exclusive
	rootCmd.MarkFlagsMutuallyExclusive("debug", "quiet")
}

// GetGlobalConfigPath returns the global config path if specified.
// Commands that need configuration should use this.
func GetGlobalConfigPath() string {
	return globalConfigPath
}
