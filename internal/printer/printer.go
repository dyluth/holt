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

// Success prints a success message in green with a checkmark prefix
func Success(format string, a ...any) {
	msg := fmt.Sprintf(format, a...)
	if !strings.HasPrefix(msg, "✓") {
		green.Printf("✓ %s", msg)
	} else {
		green.Print(msg)
	}
}

// Info prints an informational message in the default color
func Info(format string, a ...any) {
	fmt.Printf(format, a...)
}

// Warning prints a warning message in yellow with a warning emoji prefix
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

// Step prints a step message with emphasis (used in multi-step operations)
func Step(format string, a ...any) {
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
