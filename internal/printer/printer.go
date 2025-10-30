package printer

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
)

func init() {
	// Force color output even when not connected to TTY
	// Users can disable with NO_COLOR environment variable
	if os.Getenv("NO_COLOR") == "" {
		color.NoColor = false
	}
}

var (
	// Color definitions
	green  = color.New(color.FgGreen)
	yellow = color.New(color.FgYellow)
	red    = color.New(color.FgRed, color.Bold)
	cyan   = color.New(color.FgCyan)
)

// VerbosityLevel defines the output verbosity.
type VerbosityLevel int

const (
	// VerbosityDefault shows clean, summary-level output (default)
	VerbosityDefault VerbosityLevel = iota
	// VerbosityDebug shows verbose debug output
	VerbosityDebug
	// VerbosityQuiet shows only essential output (errors and final results)
	VerbosityQuiet
)

// globalVerbosity holds the current verbosity level (set by root command flags)
var globalVerbosity = VerbosityDefault

// SetVerbosity sets the global verbosity level.
// Should be called by the root command based on --debug/--quiet flags.
func SetVerbosity(level VerbosityLevel) {
	globalVerbosity = level
}

// GetVerbosity returns the current global verbosity level.
func GetVerbosity() VerbosityLevel {
	return globalVerbosity
}

// IsDebug returns true if debug verbosity is enabled.
func IsDebug() bool {
	return globalVerbosity == VerbosityDebug
}

// IsQuiet returns true if quiet verbosity is enabled.
func IsQuiet() bool {
	return globalVerbosity == VerbosityQuiet
}

// Debug prints a message only if debug verbosity is enabled.
// Used for verbose implementation details (port numbers, container IDs, Redis keys).
func Debug(format string, a ...any) {
	if !IsDebug() {
		return
	}
	msg := fmt.Sprintf(format, a...)
	fmt.Printf("[DEBUG] %s", msg)
}

// Success prints a success message in green with a checkmark prefix.
// Suppressed in quiet mode.
func Success(format string, a ...any) {
	if IsQuiet() {
		return
	}
	msg := fmt.Sprintf(format, a...)
	if !strings.HasPrefix(msg, "✓") {
		green.Printf("✓ %s", msg)
	} else {
		green.Print(msg)
	}
}

// Info prints an informational message in the default color.
// Suppressed in quiet mode.
func Info(format string, a ...any) {
	if IsQuiet() {
		return
	}
	fmt.Printf(format, a...)
}

// Warning prints a warning message in yellow with a warning emoji prefix.
// Always shown (not suppressed by quiet mode).
func Warning(format string, a ...any) {
	msg := fmt.Sprintf(format, a...)
	if !strings.HasPrefix(msg, "⚠️") {
		yellow.Printf("⚠️  %s", msg)
	} else {
		yellow.Print(msg)
	}
}

// Error creates a formatted error message with title, explanation, and suggestions
// Prints the formatted error to stderr with colors and returns a simple error for Cobra
func Error(title string, explanation string, suggestions []string) error {
	// Print title in red to stderr
	red.Fprintf(os.Stderr, "%s\n\n", title)

	// Print explanation
	fmt.Fprintf(os.Stderr, "%s\n", explanation)

	// Print suggestions
	if len(suggestions) > 0 {
		fmt.Fprintf(os.Stderr, "\n")
		if len(suggestions) == 1 {
			fmt.Fprintf(os.Stderr, "%s\n", suggestions[0])
		} else {
			fmt.Fprintf(os.Stderr, "Either:\n")
			for i, suggestion := range suggestions {
				fmt.Fprintf(os.Stderr, "  %d. %s\n", i+1, suggestion)
			}
		}
	}

	// Return simple error for Cobra (won't be printed due to SilenceErrors)
	return fmt.Errorf("%s", title)
}

// ErrorWithContext creates a formatted error with context details
// Prints the formatted error to stderr with colors and returns a simple error for Cobra
func ErrorWithContext(title string, explanation string, context map[string]string, suggestions []string) error {
	// Print title in red to stderr
	red.Fprintf(os.Stderr, "%s\n\n", title)

	// Print explanation
	if explanation != "" {
		fmt.Fprintf(os.Stderr, "%s\n", explanation)
	}

	// Print context details
	if len(context) > 0 {
		fmt.Fprintf(os.Stderr, "\n")
		for key, value := range context {
			fmt.Fprintf(os.Stderr, "  %s: %s\n", key, value)
		}
	}

	// Print suggestions
	if len(suggestions) > 0 {
		fmt.Fprintf(os.Stderr, "\n")
		if len(suggestions) == 1 {
			fmt.Fprintf(os.Stderr, "%s\n", suggestions[0])
		} else {
			fmt.Fprintf(os.Stderr, "Either:\n")
			for i, suggestion := range suggestions {
				fmt.Fprintf(os.Stderr, "  %d. %s\n", i+1, suggestion)
			}
		}
	}

	// Return simple error for Cobra (won't be printed due to SilenceErrors)
	return fmt.Errorf("%s", title)
}

// Step prints a step message with emphasis (used in multi-step operations).
// Suppressed in quiet mode.
func Step(format string, a ...any) {
	if IsQuiet() {
		return
	}
	cyan.Printf("→ %s", fmt.Sprintf(format, a...))
}

// Println prints a plain message (for output that doesn't need coloring)
func Println(a ...any) {
	fmt.Println(a...)
}

// Printf prints a plain formatted message (for output that doesn't need coloring)
func Printf(format string, a ...any) {
	fmt.Printf(format, a...)
}
