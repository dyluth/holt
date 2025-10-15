package hoard

import (
	"context"
	"fmt"
	"io"

	"github.com/dyluth/sett/pkg/blackboard"
	"github.com/google/uuid"
)

// GetArtefact retrieves a single artefact by ID and writes it as pretty-printed JSON to the writer.
// Returns an error if the artefact ID is invalid or the artefact does not exist.
// Uses IsNotFound() to distinguish "not found" errors from other errors.
func GetArtefact(ctx context.Context, bbClient *blackboard.Client, artefactID string, w io.Writer) error {
	// Validate artefact ID format
	if _, err := uuid.Parse(artefactID); err != nil {
		return fmt.Errorf("invalid artefact ID format: must be a valid UUID")
	}

	// Fetch artefact from blackboard
	artefact, err := bbClient.GetArtefact(ctx, artefactID)
	if err != nil {
		if blackboard.IsNotFound(err) {
			return &ArtefactNotFoundError{ArtefactID: artefactID}
		}
		return fmt.Errorf("failed to fetch artefact: %w", err)
	}

	// Format and write as JSON
	if err := FormatSingleJSON(w, artefact); err != nil {
		return fmt.Errorf("failed to format artefact: %w", err)
	}

	return nil
}

// ArtefactNotFoundError represents a specific "artefact not found" error.
// This allows callers to distinguish not-found errors from other failures.
type ArtefactNotFoundError struct {
	ArtefactID string
}

func (e *ArtefactNotFoundError) Error() string {
	return fmt.Sprintf("artefact with ID '%s' not found", e.ArtefactID)
}

// IsNotFound returns true if the error is an ArtefactNotFoundError.
func IsNotFound(err error) bool {
	_, ok := err.(*ArtefactNotFoundError)
	return ok
}
