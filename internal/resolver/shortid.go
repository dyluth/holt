package resolver

import (
	"context"
	"fmt"
	"strings"

	"github.com/dyluth/holt/pkg/blackboard"
)

// MinShortIDLength is the minimum required length for short ID prefixes.
// Set to 6 characters to balance usability with collision avoidance.
const MinShortIDLength = 6

// ResolveArtefactID resolves a short ID prefix to a full UUID.
// Returns the full UUID if exactly one match found.
// Returns error if zero or multiple matches found.
//
// The function handles three cases:
// 1. Input is already a full UUID (36 chars, 4 hyphens) - validates existence
// 2. Input is too short (< 6 chars) - returns validation error
// 3. Input is a short prefix - scans for matches and returns unique result
func ResolveArtefactID(ctx context.Context, bbClient *blackboard.Client, shortID string) (string, error) {
	// If input is already a full UUID, verify it exists and return as-is
	if len(shortID) == 36 && strings.Count(shortID, "-") == 4 {
		// Verify it exists
		_, err := bbClient.GetArtefact(ctx, shortID)
		if err != nil {
			if blackboard.IsNotFound(err) {
				return "", fmt.Errorf("artefact not found: %s", shortID)
			}
			return "", fmt.Errorf("failed to verify artefact existence: %w", err)
		}
		return shortID, nil
	}

	// Validate minimum length
	if len(shortID) < MinShortIDLength {
		return "", fmt.Errorf("short ID must be at least %d characters (got %d)", MinShortIDLength, len(shortID))
	}

	// Scan for matching UUIDs
	matches, err := bbClient.ScanArtefacts(ctx, shortID)
	if err != nil {
		return "", fmt.Errorf("failed to search for artefact: %w", err)
	}

	switch len(matches) {
	case 0:
		return "", &NotFoundError{ShortID: shortID}
	case 1:
		return matches[0], nil
	default:
		return "", &AmbiguousError{ShortID: shortID, Matches: matches}
	}
}

// NotFoundError indicates no artefacts matched the short ID.
type NotFoundError struct {
	ShortID string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("no artefacts found matching '%s'", e.ShortID)
}

// AmbiguousError indicates multiple artefacts matched the short ID.
type AmbiguousError struct {
	ShortID string
	Matches []string
}

func (e *AmbiguousError) Error() string {
	return fmt.Sprintf("ambiguous short ID '%s' matches %d artefacts", e.ShortID, len(e.Matches))
}

// FormatAmbiguousError creates a user-friendly error message for ambiguous short IDs.
// Lists all matching UUIDs (up to 10, then "...and N more").
func FormatAmbiguousError(err *AmbiguousError) string {
	msg := fmt.Sprintf("Error: ambiguous short ID '%s' matches %d artefacts:\n", err.ShortID, len(err.Matches))

	// List up to 10 matches
	displayCount := len(err.Matches)
	if displayCount > 10 {
		displayCount = 10
	}

	for i := 0; i < displayCount; i++ {
		msg += fmt.Sprintf("  %s\n", err.Matches[i])
	}

	if len(err.Matches) > 10 {
		msg += fmt.Sprintf("  ...and %d more\n", len(err.Matches)-10)
	}

	msg += "\nUse a longer prefix to uniquely identify the artefact."
	return msg
}

// IsNotFoundError checks if an error is a NotFoundError.
func IsNotFoundError(err error) bool {
	_, ok := err.(*NotFoundError)
	return ok
}

// IsAmbiguousError checks if an error is an AmbiguousError.
func IsAmbiguousError(err error) bool {
	_, ok := err.(*AmbiguousError)
	return ok
}
